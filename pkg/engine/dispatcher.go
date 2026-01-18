//go:build windows
// +build windows

package engine

import (
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// byteSlicePool reduces GC pressure by reusing byte slices for message copying.
// Slices larger than maxPooledSliceSize are not pooled to prevent memory bloat.
var byteSlicePool = sync.Pool{
	New: func() any {
		return make([]byte, 0, 4096) // Pre-allocate 4KB capacity
	},
}

const maxPooledSliceSize = 65536 // 64KB - larger messages won't be pooled

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
					// No message available, sleep briefly to prevent CPU spinning
					time.Sleep(1 * time.Millisecond)
					continue
				}

				// Copy the received message before sending to the queue
				// Use pooled slice if size is reasonable, otherwise allocate fresh
				var dataCopy []byte
				if size <= maxPooledSliceSize {
					pooled := byteSlicePool.Get().([]byte)
					if cap(pooled) >= int(size) {
						dataCopy = pooled[:size]
					} else {
						// Pooled slice too small, allocate new one
						dataCopy = make([]byte, size)
					}
				} else {
					dataCopy = make([]byte, size)
				}
				copy(dataCopy, unsafe.Slice((*byte)(unsafe.Pointer(recv)), size))
				recvCopy := (*types.SIMCONNECT_RECV)(unsafe.Pointer(&dataCopy[0]))

				recvID := types.SIMCONNECT_RECV_ID(recvCopy.DwID)

				if recvID == types.SIMCONNECT_RECV_ID_EVENT {
					event := (*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(recvCopy))
					// Ignore those events to reduce noise (maybe consider making this configurable later)
					if event.UEventID == HEARTBEAT_EVENT_ID { // Heartbeat event ID
						e.logger.Debug("[dispatcher] Heartbeat event received")
						continue
					}

				}

				if recvID == types.SIMCONNECT_RECV_ID_OPEN {
					e.logger.Debug("[dispatcher] Connection to simulator established")
				}

				if recvID == types.SIMCONNECT_RECV_ID_QUIT {
					e.logger.Debug("[dispatcher] Received SIMCONNECT_RECV_ID_QUIT, simulator is closing the connection")
					// Sent message that simulator is quitting
					e.queue <- Message{
						SIMCONNECT_RECV: recvCopy,
						Size:            size,
						Err:             err,
						data:            dataCopy, // Keep reference to prevent GC
					}
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
					case e.queue <- Message{
						SIMCONNECT_RECV: recvCopy,
						Size:            size,
						Err:             err,
						data:            dataCopy, // Keep reference to prevent GC
					}:
					}
				}
			}
		}
	})

	return nil
}
