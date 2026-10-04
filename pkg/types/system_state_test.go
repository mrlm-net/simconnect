package types

import (
	"testing"
	"unsafe"
)

// TestSystemStateLayout: fFloat is a 4-byte float, so the string starts at
// wire offset 24 (MSFS 2024 sends "SimObjects\..." for AircraftLoaded there).
func TestSystemStateLayout(t *testing.T) {
	var s SIMCONNECT_RECV_SYSTEM_STATE
	if off := unsafe.Offsetof(s.FFloatBytes); off != 20 {
		t.Errorf("fFloat at %d, want 20", off)
	}
	if off := unsafe.Offsetof(s.SzString); off != 24 {
		t.Errorf("szString at %d, want 24", off)
	}
}
