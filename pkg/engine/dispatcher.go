//go:build windows
// +build windows

package engine

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	HEARTBEAT_EVENT_ID   types.DWORD = 9999 // SimConnect_SystemState_6Hz ID
	PAUSE_EVENT_ID       types.DWORD = 9998
	SIM_EVENT_ID         types.DWORD = 9997
	SOUND_EVENT_ID       types.DWORD = 9996
	STATE_DATE_DEFINE_ID uint32      = 9000
)

func (e *Engine) dispatch() error {
	e.logger.Debug("[dispatcher] Starting dispatcher goroutine")
	e.queue = make(chan Message, e.config.BufferSize)
	// Subscribe to a system event to receive regular updates about the simulator connection state
	e.api.SubscribeToSystemEvent(uint32(HEARTBEAT_EVENT_ID), "6Hz") // SimConnect_SystemState_6Hz
	e.api.SubscribeToSystemEvent(uint32(PAUSE_EVENT_ID), "Pause")
	e.api.SubscribeToSystemEvent(uint32(SIM_EVENT_ID), "Sim")
	e.api.SubscribeToSystemEvent(uint32(SOUND_EVENT_ID), "Sound")
	e.api.AddToDataDefinition(STATE_DATE_DEFINE_ID, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
	e.api.AddToDataDefinition(STATE_DATE_DEFINE_ID, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1)
	e.api.RequestDataOnSimObject(STATE_DATE_DEFINE_ID, STATE_DATE_DEFINE_ID, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_VISUAL_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)
	// Set engine state to available
	e.state.SetAvailable(true)
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
					// Ignore those events to reduce noise (maybe consider making this configurable later)
					if event.UEventID == HEARTBEAT_EVENT_ID { // Heartbeat event ID
						e.logger.Debug("[dispatcher] Heartbeat event received")
						continue
					}

					if event.UEventID == PAUSE_EVENT_ID { // Pause event ID
						paused := event.DwData == 1
						e.state.SetPaused(paused)
						if paused {
							e.logger.Debug("[dispatcher] Simulator is PAUSED")
						} else {
							e.logger.Debug("[dispatcher] Simulator is UNPAUSED")
						}
						//continue
					}

					if event.UEventID == SIM_EVENT_ID { // Sim event ID
						running := event.DwData == 1
						e.state.SetSimRunning(running)
						if running {
							e.logger.Debug("[dispatcher] Simulator SIM STARTED")
						} else {
							e.logger.Debug("[dispatcher] Simulator SIM STOPPED")
						}
						//continue
					}

					if event.UEventID == SOUND_EVENT_ID { // Sound event ID
						soundOn := event.DwData == 1
						e.state.SetSoundOn(soundOn)
						if soundOn {
							e.logger.Debug("[dispatcher] Simulator SOUND ON")
						} else {
							e.logger.Debug("[dispatcher] Simulator SOUND OFF")
						}
						//continue
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
