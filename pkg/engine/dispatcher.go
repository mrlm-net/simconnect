//go:build windows

package engine

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/internal/simconnect"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Tiered byte pools reduce GC pressure by reusing byte slices for message copying.
// Messages are pooled in size-appropriate tiers to minimize waste. The pools
// hold *[]byte: putting a slice header into an interface would allocate on
// every Put.
var (
	pool4KB  = sync.Pool{New: func() any { b := make([]byte, 4*1024); return &b }}
	pool16KB = sync.Pool{New: func() any { b := make([]byte, 16*1024); return &b }}
	pool64KB = sync.Pool{New: func() any { b := make([]byte, 64*1024); return &b }}
)

// getPooledSlice returns a byte slice from the appropriate pool tier and a release function.
// For sizes > 64KB, allocates a fresh slice without pooling to prevent memory bloat.
func getPooledSlice(size uint32) ([]byte, func()) {
	var pool *sync.Pool
	switch {
	case size <= 4*1024:
		pool = &pool4KB
	case size <= 16*1024:
		pool = &pool16KB
	case size <= 64*1024:
		pool = &pool64KB
	default:
		// No pooling for very large messages to prevent memory bloat
		return make([]byte, size), func() {}
	}
	p := pool.Get().(*[]byte)
	return (*p)[:size], func() { pool.Put(p) }
}

// Adaptive polling constants for exponential backoff
const (
	minSleep = 1 * time.Millisecond
	maxSleep = 50 * time.Millisecond
)

// ErrConnectionLost ends a stream: the simulator quit or restarted (its
// pipe closed), or GetNextDispatch failed maxErrorsInRow times in a row.
// The last Message carries it, then the stream closes.
var ErrConnectionLost = simconnect.ErrConnectionLost

// maxErrorsInRow dispatch errors in a row (with the backoff, some seconds)
// are a lost connection.
const maxErrorsInRow = 100

const (
	HEARTBEAT_EVENT_ID types.DWORD = 999999999 // SimConnect_SystemState_6Hz ID
)

// closeQueue safely closes the queue channel exactly once
func (e *Engine) closeQueue() {
	e.closeOnce.Do(func() {
		close(e.queue)
	})
}

func (e *Engine) dispatch() error {
	e.logger.Debug("[dispatcher] Starting dispatcher goroutine")
	// Subscribe to a system event to receive regular updates about the simulator connection state
	// SimConnect_SystemState_6Hz. Messages still flow without it, so a
	// failure is logged, not fatal.
	if err := e.api.SubscribeToSystemEvent(uint32(HEARTBEAT_EVENT_ID), string(e.config.Heartbeat)); err != nil {
		e.logger.Warn("[dispatcher] Heartbeat subscription failed", "heartbeat", e.config.Heartbeat, "error", err)
	}
	e.sync.Go(func() {
		defer func() {
			e.logger.Debug("[dispatcher] Exiting dispatcher goroutine")
			// Ensure queue is closed on all exit paths
			e.closeQueue()
		}()

		// Adaptive sleep for backoff when no messages available
		sleepDuration := minSleep
		errorsInRow := 0

		for {
			select {
			case <-e.ctx.Done():
				e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
				return
			default:
				recv, size, err := e.api.GetNextDispatch()

				if err != nil {
					// The simulator gone (its pipe closed), or errors without
					// end: the connection is lost. Said once, then the stream
					// ends, so a manager reconnects (it spun here before, an
					// error logged 280,000 times a second, on a sim restart).
					errorsInRow++
					lost := errors.Is(err, ErrConnectionLost) || errorsInRow >= maxErrorsInRow
					if errorsInRow == 1 || lost {
						e.logger.Error("[dispatcher] Error", "error", err, "lost", lost)
					}
					if lost {
						err = fmt.Errorf("%w: %w", ErrConnectionLost, err)
					}
					select {
					case <-e.ctx.Done():
						e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
						return
					case e.queue <- Message{Err: err}:
					}
					if lost {
						return
					}
					time.Sleep(sleepDuration)
					sleepDuration = min(sleepDuration*2, maxSleep)
					continue
				}
				errorsInRow = 0

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

				// Shorter than a SIMCONNECT_RECV header: nothing to read
				// (and &dataCopy[0] would panic on an empty buffer).
				if size < uint32(unsafe.Sizeof(types.SIMCONNECT_RECV{})) {
					e.logger.Warn("[dispatcher] Message too short, dropped", "size", size)
					continue
				}

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
					// Send message that simulator is quitting; a full queue
					// nobody reads must not hang Disconnect, so the context
					// ends the wait.
					select {
					case e.queue <- newMessage(recvCopy, size, err, dataCopy, release):
					case <-e.ctx.Done():
						release()
					}
					e.cancel()
					return // closeQueue called by defer
				}

				if recvID == types.SIMCONNECT_RECV_ID_EXCEPTION {
					exception := (*types.SIMCONNECT_RECV_EXCEPTION)(unsafe.Pointer(recvCopy))
					if call, ok := e.CallFor(uint32(exception.DwSendID)); ok {
						e.logger.Error("[dispatcher] Exception received", "exceptionID", exception.DwException, "sendID", exception.DwSendID, "index", exception.DwIndex, "call", call)
					} else {
						e.logger.Error("[dispatcher] Exception received", "exceptionID", exception.DwException, "sendID", exception.DwSendID)
					}
				}

				if size > 0 {
					e.logger.Debug("[dispatcher] Message received", "recvID", types.SIMCONNECT_RECV_ID(recvCopy.DwID))
					// Send the copied message to the queue, respecting context cancellation
					select {
					case <-e.ctx.Done():
						e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
						release() // Return buffer to pool on early exit
						return
					case e.queue <- newMessage(recvCopy, size, err, dataCopy, release):
					}
				} else {
					// No data to send, release buffer
					release()
				}
			}
		}
	})

	return nil
}
