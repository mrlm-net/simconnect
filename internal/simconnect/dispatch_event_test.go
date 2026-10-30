//go:build windows

package simconnect

import (
	"testing"
	"time"
)

// TestWaitDispatch: on a real auto-reset event, WaitDispatch returns as
// the event is set, and waits its time out when it is not; without one it
// sleeps (#855 E6).
func TestWaitDispatch(t *testing.T) {
	h := newDispatchEvent()
	if h == 0 {
		t.Fatal("no event")
	}
	sc := &SimConnect{event: h}
	defer closeEvent(h)
	setEvent := kernel32.NewProc("SetEvent")
	go func() {
		time.Sleep(20 * time.Millisecond)
		setEvent.Call(h)
	}()
	at := time.Now()
	sc.WaitDispatch(time.Second)
	if d := time.Since(at); d > 500*time.Millisecond {
		t.Errorf("set after 20 ms, returned after %s", d)
	}
	at = time.Now()
	sc.WaitDispatch(60 * time.Millisecond) // auto-reset: not set any more
	if d := time.Since(at); d < 40*time.Millisecond {
		t.Errorf("not set, returned after %s", d)
	}
	plain := &SimConnect{}
	at = time.Now()
	plain.WaitDispatch(30 * time.Millisecond)
	if d := time.Since(at); d < 25*time.Millisecond || plain.HasDispatchEvent() {
		t.Errorf("without an event: %s, has %v", d, plain.HasDispatchEvent())
	}
}
