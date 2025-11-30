//go:build windows
// +build windows

package engine

import (
	"log"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

const HEARTBEAT_EVENT_ID = ^uint32(0)

func (e *Engine) dispatch() error {
	log.Println("[dispatcher] Starting dispatcher goroutine")
	e.queue = make(chan Message, e.config.BufferSize)
	// Subscribe to a system event to receive regular updates about the simulator connection state
	e.api.SubscribeToSystemEvent(HEARTBEAT_EVENT_ID, "6Hz") // SimConnect_SystemState_6Hz
	e.sync.Go(func() {
		defer log.Println("[dispatcher] Exiting dispatcher goroutine")
		for {
			select {
			case <-e.ctx.Done():
				log.Println("[dispatcher] Context cancelled, stopping dispatcher")
				return
			default:
				recv, size, err := e.api.GetNextDispatch()

				if err != nil {
					log.Printf("[dispatcher] Error: %v\n", err)
					select {
					case <-e.ctx.Done():
						log.Println("[dispatcher] Context cancelled, stopping dispatcher")
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
						log.Printf("[dispatcher] %s\n", "Heartbeat event received")
						continue
					}
				}

				if recvID == types.SIMCONNECT_RECV_ID_OPEN {
					log.Println("[dispatcher] Connection to simulator established")
				}

				if recvID == types.SIMCONNECT_RECV_ID_QUIT {
					log.Println("[dispatcher] Simulator has closed the connection")
					e.cancel()
					close(e.queue)
					return
				}

				if size > 0 {
					select {
					case <-e.ctx.Done():
						log.Println("[dispatcher] Context cancelled, stopping dispatcher")
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
