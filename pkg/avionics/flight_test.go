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

type clearingSetter struct {
	fakeSetter
	adds map[uint32]int // datums in each definition
}

func (c *clearingSetter) AddToDataDefinition(def uint32, name, u string, typ types.SIMCONNECT_DATATYPE, e float32, id uint32) error {
	c.adds[def]++
	return c.fakeSetter.AddToDataDefinition(def, name, u, typ, e, id)
}

func (c *clearingSetter) ClearDataDefinition(def uint32) error {
	delete(c.adds, def)
	return nil
}

// TestSetFlightAgain: a second call defines the datum afresh, not twice in
// one definition (#22).
func TestSetFlightAgain(t *testing.T) {
	c := &clearingSetter{fakeSetter: fakeSetter{names: map[uint32]string{}, set: map[string]string{}}, adds: map[uint32]int{}}
	for _, n := range []string{"007", "008"} {
		if err := SetFlight(c, 0x7D00, "Czech Air Force", n); err != nil {
			t.Fatal(err)
		}
	}
	if c.adds[0x7D00] != 1 || c.adds[0x7D01] != 1 || c.set["ATC FLIGHT NUMBER"] != "008" {
		t.Errorf("datums %v, set %v", c.adds, c.set)
	}
}

// countingClearer counts the clears.
type countingClearer struct {
	clearingSetter
	clears int
}

func (c *countingClearer) ClearDataDefinition(def uint32) error {
	c.clears++
	return c.clearingSetter.ClearDataDefinition(def)
}

// TestSetFlightClearsOnlyWhatItAdded: the first SetFlight of a connection
// clears nothing (a definition never added raised exception 3), a second
// clears what the first added, and after Reset (a new connection) nothing
// again.
func TestSetFlightClearsOnlyWhatItAdded(t *testing.T) {
	c := &countingClearer{clearingSetter: clearingSetter{fakeSetter: fakeSetter{names: map[uint32]string{}, set: map[string]string{}}, adds: map[uint32]int{}}}
	if err := SetFlight(c, 0x7E00, "", "007"); err != nil {
		t.Fatal(err)
	}
	if c.clears != 0 {
		t.Errorf("first call cleared %d definitions, want none", c.clears)
	}
	if err := SetFlight(c, 0x7E00, "", "008"); err != nil {
		t.Fatal(err)
	}
	if c.clears != 1 || c.adds[0x7E01] != 1 {
		t.Errorf("second call: %d clears, datums %v", c.clears, c.adds)
	}
	Reset(c)
	if err := SetFlight(c, 0x7E00, "", "009"); err != nil {
		t.Fatal(err)
	}
	if c.clears != 1 {
		t.Errorf("after Reset: %d clears, want still 1", c.clears)
	}
}
