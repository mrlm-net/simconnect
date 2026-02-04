//go:build windows
// +build windows

package engine

import (
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Tiered byte pools reduce GC pressure by reusing byte slices for message copying.
// Messages are pooled in size-appropriate tiers to minimize waste.
var (
	pool4KB  = sync.Pool{New: func() any { return make([]byte, 4*1024) }}
	pool16KB = sync.Pool{New: func() any { return make([]byte, 16*1024) }}
	pool64KB = sync.Pool{New: func() any { return make([]byte, 64*1024) }}
)

// getPooledSlice returns a byte slice from the appropriate pool tier and a release function.
// For sizes > 64KB, allocates a fresh slice without pooling to prevent memory bloat.
func getPooledSlice(size uint32) ([]byte, func()) {
	switch {
	case size <= 4*1024:
		s := pool4KB.Get().([]byte)
		return s[:size], func() { pool4KB.Put(s) }
	case size <= 16*1024:
		s := pool16KB.Get().([]byte)
		return s[:size], func() { pool16KB.Put(s) }
	case size <= 64*1024:
		s := pool64KB.Get().([]byte)
		return s[:size], func() { pool64KB.Put(s) }
	default:
		// No pooling for very large messages to prevent memory bloat
		return make([]byte, size), func() {}
	}
}

// Adaptive polling constants for exponential backoff
const (
	minSleep = 1 * time.Millisecond
	maxSleep = 50 * time.Millisecond
)

const (
	HEARTBEAT_EVENT_ID types.DWORD = 999999999 // SimConnect_SystemState_6Hz ID
)

func (e *Engine) dispatch() error {
	e.logger.Debug("[dispatcher] Starting dispatcher goroutine")
	e.queue = make(chan Message, e.config.BufferSize)
	// Subscribe to a system event to receive regular updates about the simulator connection state
	e.api.SubscribeToSystemEvent(uint32(HEARTBEAT_EVENT_ID), string(e.config.Heartbeat)) // SimConnect_SystemState_6Hz
	e.sync.Go(func() {
		defer e.logger.Debug("[dispatcher] Exiting dispatcher goroutine")

		// Adaptive sleep for backoff when no messages available
		sleepDuration := minSleep

		for {
			select {
			case <-e.ctx.Done():
				e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
				return
			default:
				recv, size, err := e.api.GetNextDispatch()

				if err != nil {
					e.logger.Error("[dispatcher] Error", "error", err)
					select {
					case <-e.ctx.Done():
						e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
						return
					case e.queue <- Message{Err: err}:
						continue
					}

				}

				if recv == nil {
					// No message available, apply adaptive backoff to reduce CPU usage
					time.Sleep(sleepDuration)
					// Exponential backoff: 1ms -> 2ms -> 4ms -> 8ms -> 16ms -> 32ms -> 50ms (cap)
					if sleepDuration < maxSleep {
						sleepDuration *= 2
						if sleepDuration > maxSleep {
							sleepDuration = maxSleep
						}
					}
					continue
				}

				// Reset sleep duration on activity
				sleepDuration = minSleep

				// Copy the received message using tiered pooling
				dataCopy, release := getPooledSlice(size)
				copy(dataCopy, unsafe.Slice((*byte)(unsafe.Pointer(recv)), size))
				recvCopy := (*types.SIMCONNECT_RECV)(unsafe.Pointer(&dataCopy[0]))

				recvID := types.SIMCONNECT_RECV_ID(recvCopy.DwID)

				if recvID == types.SIMCONNECT_RECV_ID_EVENT {
					event := (*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(recvCopy))
					// Ignore those events to reduce noise (maybe consider making this configurable later)
					if event.UEventID == HEARTBEAT_EVENT_ID { // Heartbeat event ID
						e.logger.Debug("[dispatcher] Heartbeat event received")
						release() // Return buffer to pool
						continue
					}

				}

				if recvID == types.SIMCONNECT_RECV_ID_OPEN {
					e.logger.Debug("[dispatcher] Connection to simulator established")
				}

				if recvID == types.SIMCONNECT_RECV_ID_QUIT {
					e.logger.Debug("[dispatcher] Received SIMCONNECT_RECV_ID_QUIT, simulator is closing the connection")
					// Sent message that simulator is quitting
					e.queue <- newMessage(recvCopy, size, err, dataCopy, release)
					e.cancel()
					close(e.queue)
					return
				}

				if recvID == types.SIMCONNECT_RECV_ID_EXCEPTION {
					exception := (*types.SIMCONNECT_RECV_EXCEPTION)(unsafe.Pointer(recvCopy))
					e.logger.Error("[dispatcher] Exception received", "exceptionID", exception.DwException, "sendID", exception.DwSendID)
				}

				if size > 0 {
					e.logger.Debug("[dispatcher] Message received", "recvID", types.SIMCONNECT_RECV_ID(recvCopy.DwID))
					// Send the copied message to the queue, respecting context cancellation
					select {
					case <-e.ctx.Done():
						e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
						return
					case e.queue <- newMessage(recvCopy, size, err, dataCopy, release):
					}
				}
			}
		}
	})

	return nil
}
