//go:build windows
// +build windows

package engine

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	HEARTBEAT_EVENT_ID types.DWORD = 999999999 // SimConnect_SystemState_6Hz ID
)

func (e *Engine) dispatch() error {
	e.logger.Debug("[dispatcher] Starting dispatcher goroutine")
	e.queue = make(chan Message, e.config.BufferSize)
	// Subscribe to a system event to receive regular updates about the simulator connection state
	e.api.SubscribeToSystemEvent(uint32(HEARTBEAT_EVENT_ID), e.config.Heartbeat) // SimConnect_SystemState_6Hz
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
					e.logger.Error(fmt.Sprintf("[dispatcher] Error: %v\n", err))
					select {
					case <-e.ctx.Done():
						e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
						return
					case e.queue <- Message{Err: err}:
						continue
					}

				}

				if recv == nil {
					// No message available, continue the loop
					continue
				}

				// Copy the received message before sending to the queue
				dataCopy := make([]byte, size)
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
					e.logger.Error(fmt.Sprintf("[dispatcher] Exception received - ID: %d, Error: %d\n", exception.DwException, exception.DwSendID))
				}

				if size > 0 {
					e.logger.Debug(fmt.Sprintf("[dispatcher] Message received - %v", types.SIMCONNECT_RECV_ID(recvCopy.DwID)))
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
