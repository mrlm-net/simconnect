package world

import "testing"

// A panic in one part of the World's second is caught and logged; the
// caller goes on (E27).
func TestRecoverTick(t *testing.T) {
	ran := false
	recoverTick("test", func() { panic("boom") })
	recoverTick("test", func() { ran = true })
	if !ran {
		t.Error("the next part did not run after a panic")
	}
}
