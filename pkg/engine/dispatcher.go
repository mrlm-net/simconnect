//go:build windows
// +build windows

package engine

import "log"

func (e *Engine) dispatch() error {
	defer log.Println("[dispatcher] Starting dispatcher goroutine")
	e.sync.Go(func() {
		defer log.Println("[dispatcher] Exiting dispatcher goroutine")
		for {
			select {
			case <-e.ctx.Done():
				return
			default:
				recv, size, err := e.api.GetNextDispatch()

				if size > 0 || err != nil {
					e.queue <- Message{
						SIMCONNECT_RECV: recv,
						Size:            size,
						Err:             err,
					}
				}
			}
		}
	})

	return nil
}
