//go:build windows

package simconnect

import (
	"syscall"
	"time"
)

// The dispatch event (Config.DispatchEvent, #855 E6): a Win32 auto-reset
// event handed to SimConnect_Open as hEventHandle, which SimConnect sets
// when a message is waiting. The dispatcher waits on it instead of
// sleeping a fixed time between polls: woken as a message comes, it idles
// without spinning.

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procCreateEventW    = kernel32.NewProc("CreateEventW")
	procWaitForSingleOb = kernel32.NewProc("WaitForSingleObject")
)

// newDispatchEvent is a new auto-reset event, not set; 0 when it could not
// be made (the dispatcher then sleeps between polls as without it).
func newDispatchEvent() uintptr {
	h, _, _ := procCreateEventW.Call(0, 0, 0, 0) // no security, auto-reset, not set, no name
	return h
}

// closeEvent closes dispatch event h (0 none).
func closeEvent(h uintptr) {
	if h != 0 {
		syscall.CloseHandle(syscall.Handle(h))
	}
}

// WaitDispatch waits at most d for a message: returned early when
// SimConnect sets the dispatch event, a plain sleep without one. It never
// says whether a message came; GetNextDispatch tells.
func (sc *SimConnect) WaitDispatch(d time.Duration) {
	sc.sync.RLock()
	h := sc.event
	sc.sync.RUnlock()
	if h == 0 {
		time.Sleep(d)
		return
	}
	ms := max(d.Milliseconds(), 1)
	procWaitForSingleOb.Call(h, uintptr(ms))
}

// HasDispatchEvent reports whether SimConnect sets an event for messages:
// the connection was opened with one.
func (sc *SimConnect) HasDispatchEvent() bool {
	sc.sync.RLock()
	defer sc.sync.RUnlock()
	return sc.event != 0
}
