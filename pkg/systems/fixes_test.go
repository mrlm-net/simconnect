//go:build windows
// +build windows

package systems

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/dict"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestControlsReset: after Reset (a new connection) events are mapped and
// variables defined again (#24); the IDs stop at ControlIDs.
func TestControlsReset(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c := NewControls(f, 0)
	c.Use(For(Aircraft{Title: "Asobo A320neo"}))
	if err := c.Set(ParkingBrake, true, State{}); err != nil {
		t.Fatal(err)
	}
	g := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c.Reset(g)
	if err := c.Set(ParkingBrake, true, State{}); err != nil {
		t.Fatal(err)
	}
	if len(g.mapped) != 1 || g.mapped[DefaultControlBase] != "PARKING_BRAKES" || len(g.sent) != 1 {
		t.Errorf("after Reset mapped %v, sent %v", g.mapped, g.sent)
	}
	c.mu.Lock()
	c.next = ControlIDs
	c.mu.Unlock()
	if err := c.Set(Jetway, true, State{}); !errors.Is(err, ErrNoIDs) {
		t.Errorf("beyond the IDs: %v", err)
	}
}

type fakeReaderClient struct{ adds, clears int }

func (f *fakeReaderClient) AddToDataDefinition(uint32, string, string, types.SIMCONNECT_DATATYPE, float32, uint32) error {
	f.adds++
	return nil
}
func (f *fakeReaderClient) ClearDataDefinition(uint32) error { f.clears++; return nil }
func (f *fakeReaderClient) RequestDataOnSimObject(uint32, uint32, uint32, types.SIMCONNECT_PERIOD, types.SIMCONNECT_DATA_REQUEST_FLAG, uint32, uint32, uint32) error {
	return nil
}

// data is a SIMOBJECT_DATA message of request req, definition def, with
// values.
func data(req, def uint32, values ...float64) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+8*len(values)+8)
	d := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	d.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	d.DwSize = types.DWORD(off + 8*len(values))
	d.DwRequestID, d.DwDefineID, d.DwDefineCount = types.DWORD(req), types.DWORD(def), types.DWORD(len(values))
	copy(unsafe.Slice((*float64)(unsafe.Pointer(&buf[off])), len(values)), values)
	return engine.Message{SIMCONNECT_RECV: &d.SIMCONNECT_RECV, Size: uint32(off + 8*len(values))}
}

// TestReaderStaleLayout: data of another layout (the last profile's, still
// coming after a switch) is not read with the new one (#25).
func TestReaderStaleLayout(t *testing.T) {
	f := &fakeReaderClient{}
	r := NewReader(f, 10, 11)
	p := Profile{Name: "p", Values: map[string]Value{Battery: {Vars: []string{"A"}}, Avionics: {Vars: []string{"B"}}}}
	r.Use(p)
	if err := r.Request(types.SIMCONNECT_PERIOD_SECOND); err != nil {
		t.Fatal(err)
	}
	if s, ok := r.Handle(data(11, 10, 1, 1)); !ok || !s.Battery || !s.Avionics {
		t.Fatalf("read %v %+v", ok, s)
	}
	r.Use(Profile{Name: "q", Values: map[string]Value{Battery: {Vars: []string{"A"}}, Avionics: {Vars: []string{"B"}}, Powered: {Vars: []string{"C"}}}})
	if _, ok := r.Handle(data(11, 10, 1, 1)); ok {
		t.Error("data read before the new profile was requested")
	}
	if err := r.Request(types.SIMCONNECT_PERIOD_SECOND); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Handle(data(11, 10, 1, 1)); ok {
		t.Error("the old layout's data read with the new one")
	}
	if _, ok := r.Handle(data(11, 10, 1, 1, 1)); !ok || f.clears != 1 {
		t.Errorf("the new layout not read, or not cleared (%d)", f.clears)
	}
}

// TestProfileOverPerValue: a host's profile for a shipped name replaces
// only the values it gives (#27).
func TestProfileOverPerValue(t *testing.T) {
	defer dict.Reset("systems.profiles")
	if err := dict.Use("systems.profiles", []byte(`[{"name":"Fenix A320 family","values":{"lightStrobe":{"vars":["L:S_OH_EXT_LT_STROBE"],"trueAt":[2]}}}]`)); err != nil {
		t.Fatal(err)
	}
	fx := For(Aircraft{Package: "fnx-aircraft-320"})
	if fx.Values[LightStrobe].Vars[0] != "L:S_OH_EXT_LT_STROBE" || fx.Values[Battery].Combine != "any" || fx.Actions[COM1Swap].Press != "L:S_PED_RMP1_XFER" || len(fx.Doors) != 6 {
		t.Errorf("Fenix with a host value: strobe %+v, battery %+v, swap %+v, doors %v", fx.Values[LightStrobe], fx.Values[Battery], fx.Actions[COM1Swap], fx.Doors)
	}
}

// TestExplicitDoorWins: with exits mapped, a door value the profile gives
// itself stays; an empty value reads 0, no panic (E12).
func TestExplicitDoorWins(t *testing.T) {
	own := Value{Vars: []string{"L:MY_DOOR"}}
	p := withDoors(Merge(Default(), Profile{Doors: []string{"L1", "L2"}, Exits: []int{1, 4}, Values: map[string]Value{Door(0): own}}))
	if v := p.Values[Door(0)]; len(v.Vars) != 1 || v.Vars[0] != "L:MY_DOOR" {
		t.Errorf("own door value replaced: %+v", v)
	}
	if v := p.Values[Door(1)]; v.Vars[0] != "EXIT OPEN:3" {
		t.Errorf("mapped door %+v", v)
	}
	if (Value{}).resolve(nil) != 0 {
		t.Error("empty value")
	}
}
