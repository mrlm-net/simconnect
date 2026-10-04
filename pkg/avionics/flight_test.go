//go:build windows
// +build windows

package avionics

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeSetter struct {
	names map[uint32]string
	set   map[string]string
}

func (f *fakeSetter) AddToDataDefinition(def uint32, name, _ string, _ types.SIMCONNECT_DATATYPE, _ float32, _ uint32) error {
	f.names[def] = name
	return nil
}

func (f *fakeSetter) SetDataOnSimObject(def, _ uint32, _ types.SIMCONNECT_DATA_SET_FLAG, _ uint32, size uint32, data unsafe.Pointer) error {
	f.set[f.names[def]] = strings.TrimRight(string(unsafe.Slice((*byte)(data), size)), "\x00")
	return nil
}

// TestSetFlight: the airline and the flight number set as strings; "" left
// alone; too long refused (#680).
func TestSetFlight(t *testing.T) {
	f := &fakeSetter{names: map[uint32]string{}, set: map[string]string{}}
	if err := SetFlight(f, 0x7D00, "Czech Air Force", "007"); err != nil {
		t.Fatal(err)
	}
	if f.set["ATC AIRLINE"] != "Czech Air Force" || f.set["ATC FLIGHT NUMBER"] != "007" {
		t.Errorf("set %v", f.set)
	}
	f.set = map[string]string{}
	SetFlight(f, 0x7D00, "", "123")
	if _, ok := f.set["ATC AIRLINE"]; ok || f.set["ATC FLIGHT NUMBER"] != "123" {
		t.Errorf("number only: %v", f.set)
	}
	if err := SetFlight(f, 0x7D00, "", "12345678"); err == nil {
		t.Error("an 8-character flight number accepted")
	}
}
