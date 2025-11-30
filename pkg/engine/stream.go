//go:build windows
// +build windows

package engine

func (e *Engine) Stream() <-chan Message {
	if e.queue == nil {
		e.dispatch()
	}
	return e.queue
}
