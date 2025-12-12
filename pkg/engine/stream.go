//go:build windows
// +build windows

package engine

func (e *Engine) Stream() <-chan Message {
	e.dispatchOnce.Do(func() {
		e.dispatch()
	})
	return e.queue
}
