//go:build windows
// +build windows

package client

func (e *Engine) bootstrap() error {
	// We need to load the procedures from the SimConnect DLL.
	e.lazyloadProcedures()
	return nil
}
