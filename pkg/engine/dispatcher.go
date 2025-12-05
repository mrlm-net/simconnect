//go:build windows
// +build windows

package engine

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

const HEARTBEAT_EVENT_ID types.DWORD = 4294967295 // SimConnect_SystemState_6Hz ID

func (e *Engine) dispatch() error {
	e.logger.Debug("[dispatcher] Starting dispatcher goroutine")
	e.queue = make(chan Message, e.config.BufferSize)
	e.state.SetAvailable(true)
	// Subscribe to a system event to receive regular updates about the simulator connection state
	e.api.SubscribeToSystemEvent(uint32(HEARTBEAT_EVENT_ID), "6Hz") // SimConnect_SystemState_6Hz
	e.sync.Go(func() {
		defer e.logger.Debug("[dispatcher] Exiting dispatcher goroutine")
		defer e.state.Reset()
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

				recvID := types.SIMCONNECT_RECV_ID(recv.DwID)

				if recvID == types.SIMCONNECT_RECV_ID_EVENT {
					event := (*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(recv))
					if event.UEventID == HEARTBEAT_EVENT_ID { // Heartbeat event ID
						e.logger.Debug("[dispatcher] Heartbeat event received")
						continue
					}
				}

				if recvID == types.SIMCONNECT_RECV_ID_OPEN {
					e.logger.Debug("[dispatcher] Connection to simulator established")
					e.state.SetReady(true)
				}

				if recvID == types.SIMCONNECT_RECV_ID_QUIT {
					e.logger.Debug("[dispatcher] Received SIMCONNECT_RECV_ID_QUIT, simulator is closing the connection")
					// Sent message that simulator is quitting
					e.queue <- Message{
						SIMCONNECT_RECV: recv,
						Size:            size,
						Err:             err,
					}
					e.state.Reset()
					e.cancel()
					close(e.queue)
					return
				}

				if recvID == types.SIMCONNECT_RECV_ID_EXCEPTION {
					exception := (*types.SIMCONNECT_RECV_EXCEPTION)(unsafe.Pointer(recv))
					e.logger.Error(fmt.Sprintf("[dispatcher] Exception received - ID: %d, Error: %d\n", exception.DwException, exception.DwSendID))
				}

				if size > 0 {
					select {
					case <-e.ctx.Done():
						e.logger.Debug("[dispatcher] Context cancelled, stopping dispatcher")
						return
					case e.queue <- Message{
						SIMCONNECT_RECV: recv,
						Size:            size,
						Err:             err,
					}:
					}
				}
			}
		}
	})

	return nil
}
