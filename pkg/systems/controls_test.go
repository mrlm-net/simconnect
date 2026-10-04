//go:build windows
// +build windows

package systems

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeControlClient struct {
	mapped map[uint32]string
	sent   []string  // "EVENT data"
	set    []float64 // values set, in order
	defs   map[uint32]string
	setVar []string
}

func (f *fakeControlClient) MapClientEventToSimEvent(id uint32, name string) error {
	f.mapped[id] = name
	return nil
}

func (f *fakeControlClient) TransmitClientEvent(_ uint32, id uint32, data uint32, _ uint32, _ types.SIMCONNECT_EVENT_FLAG) error {
	f.sent = append(f.sent, f.mapped[id]+" "+string(rune('0'+data)))
	return nil
}

func (f *fakeControlClient) AddToDataDefinition(def uint32, name, _ string, _ types.SIMCONNECT_DATATYPE, _ float32, _ uint32) error {
	f.defs[def] = name
	return nil
}

func (f *fakeControlClient) SetDataOnSimObject(def uint32, _ uint32, _ types.SIMCONNECT_DATA_SET_FLAG, _ uint32, _ uint32, data unsafe.Pointer) error {
	f.setVar = append(f.setVar, f.defs[def])
	f.set = append(f.set, *(*float64)(data))
	return nil
}

// TestControlsDefault: the standard key events, toggled only when the
// state differs; no chocks or GPU in the default (#667).
func TestControlsDefault(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c := NewControls(f, 0)
	c.Use(For(Aircraft{Title: "Asobo A320neo", ATCType: "A320"}))
	if c.Can(Chocks) || c.Can(GPU) || !c.Can(Door(0)) || !c.Can(ParkingBrake) {
		t.Fatalf("default: chocks %v gpu %v door %v brake %v", c.Can(Chocks), c.Can(GPU), c.Can(Door(0)), c.Can(ParkingBrake))
	}
	closed := State{Values: map[string]float64{Door(0): 0, ParkingBrake: 1}}
	if err := c.Set(Door(0), true, closed); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ParkingBrake, true, closed); err != nil { // set already: nothing sent
		t.Fatal(err)
	}
	if err := c.Set(ParkingBrake, false, closed); err != nil {
		t.Fatal(err)
	}
	if len(f.sent) != 2 || f.sent[0] != "TOGGLE_AIRCRAFT_EXIT 1" || f.sent[1] != "PARKING_BRAKES 0" {
		t.Errorf("sent %q, want the main exit toggled (1) and the brake toggled once", f.sent)
	}
	if err := c.Set(Chocks, true, closed); err == nil {
		t.Error("chocks on an aircraft without them: no error")
	}
}

// TestControlsFenix: the Fenix's own variables set, and its EFB (#667).
func TestControlsFenix(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	p := For(Aircraft{Package: "fnx-aircraft-320", Title: "FenixA319 CFM WF HD"})
	c := NewControls(f, 0)
	c.Use(p)
	now := State{Values: map[string]float64{Chocks: 1, GPU: 1}}
	for _, step := range []struct {
		name string
		on   bool
	}{{Chocks, false}, {GPU, false}, {ParkingBrake, true}} {
		if err := c.Set(step.name, step.on, now); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"L:B_CONFIG_CHOCKS", "L:B_CONFIG_GPU", "L:S_MIP_PARKING_BRAKE"}
	for i, w := range want {
		if i >= len(f.setVar) || f.setVar[i] != w {
			t.Fatalf("set %q %v, want %q", f.setVar, f.set, want)
		}
	}
	if f.set[0] != 0 || f.set[1] != 0 || f.set[2] != 1 {
		t.Errorf("values %v, want 0 0 1", f.set)
	}
	if p.EFB == nil || p.EFB.Port != 8083 {
		t.Errorf("EFB %+v, want port 8083", p.EFB)
	}
	if Default().EFB != nil {
		t.Error("the default has an EFB")
	}
	s := resolveState(p, map[varUnit]float64{{"L:B_CONFIG_CHOCKS", "number"}: 1})
	if !s.HasChocks || !s.Chocks || !s.HasGPU || s.GPU {
		t.Errorf("state chocks %v/%v gpu %v/%v", s.HasChocks, s.Chocks, s.HasGPU, s.GPU)
	}
}
