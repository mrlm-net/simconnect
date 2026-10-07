//go:build windows
// +build windows

package avionics

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/systems"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeClient struct {
	mapped map[uint32]string
	sent   [][2]uint32 // event, data
}

func (f *fakeClient) MapClientEventToSimEvent(id uint32, name string) error {
	if f.mapped == nil {
		f.mapped = map[uint32]string{}
	}
	f.mapped[id] = name
	return nil
}

func (f *fakeClient) TransmitClientEvent(obj, id, data, group uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	f.sent = append(f.sent, [2]uint32{id, data})
	return nil
}

func TestRadios(t *testing.T) {
	f := &fakeClient{}
	r := New(f, 0)
	if err := r.SetCOMStandby(1, 134.56); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCOMActive(2, 118.105); err != nil { // 8.33 kHz channel
		t.Fatal(err)
	}
	if err := r.SwapCOM(1); err != nil {
		t.Fatal(err)
	}
	if err := r.SetSquawk("4521"); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name string
		data uint32
	}{{"COM_STBY_RADIO_SET_HZ", 134560000}, {"COM2_RADIO_SET_HZ", 118105000}, {"COM1_RADIO_SWAP", 0}, {"XPNDR_SET", 0x4521}}
	if len(f.sent) != len(want) || len(f.mapped) != eventCount {
		t.Fatalf("sent %v, mapped %d", f.sent, len(f.mapped))
	}
	for i, w := range want {
		if got := f.mapped[f.sent[i][0]]; got != w.name || f.sent[i][1] != w.data {
			t.Errorf("event %d: %s %d, want %s %d", i, got, f.sent[i][1], w.name, w.data)
		}
	}
	for _, bad := range []error{r.SetCOMStandby(4, 120), r.SetCOMStandby(1, 108), r.SwapCOM(0), r.SetSquawk("7800"), r.SetSquawk("123")} {
		if bad == nil {
			t.Error("bad input accepted")
		}
	}
}

type pressClient struct {
	fakeClient
	defs map[uint32]string
	sets []float64
}

func (p *pressClient) AddToDataDefinition(def uint32, name, unit string, typ types.SIMCONNECT_DATATYPE, eps float32, id uint32) error {
	if p.defs == nil {
		p.defs = map[uint32]string{}
	}
	p.defs[def] = name
	return nil
}

func (p *pressClient) SetDataOnSimObject(def, obj uint32, flags types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, data unsafe.Pointer) error {
	p.sets = append(p.sets, *(*float64)(data))
	return nil
}

// The Fenix swaps with its RMP transfer key, pressed and released; others
// keep the key event.
func TestSwapByPress(t *testing.T) {
	p := &pressClient{}
	r := New(p, 0)
	r.Use(map[string]systems.Action{systems.COM1Swap: {Press: "L:S_PED_RMP1_XFER"}})
	if err := r.SwapCOM(1); err != nil {
		t.Fatal(err)
	}
	if len(p.sets) != 2 || p.sets[0] != 1 || p.sets[1] != 0 || len(p.sent) != 0 {
		t.Errorf("pressed %v, events %v", p.sets, p.sent)
	}
	for _, name := range p.defs {
		if name != "L:S_PED_RMP1_XFER" {
			t.Errorf("defined %s", name)
		}
	}
	if err := r.SwapCOM(2); err != nil || len(p.sent) != 1 {
		t.Errorf("COM 2 without an action: %v, events %v", err, p.sent)
	}
}

// A model's actions taken again (Use) keep the variables defined: a new
// variable gets a new definition, never one already holding another
// (#23); Reset (a new connection) defines them afresh.
func TestPressDefsAcrossUse(t *testing.T) {
	p := &pressClient{}
	r := New(p, 0)
	r.Use(map[string]systems.Action{systems.COM1Swap: {Press: "L:A"}})
	if err := r.SwapCOM(1); err != nil {
		t.Fatal(err)
	}
	r.Use(map[string]systems.Action{systems.COM1Swap: {Press: "L:A"}, systems.COM2Swap: {Press: "L:B"}})
	if err := r.SwapCOM(2); err != nil {
		t.Fatal(err)
	}
	if len(p.defs) != 2 || p.defs[DefaultEventBase] != "L:A" || p.defs[DefaultEventBase+1] != "L:B" {
		t.Fatalf("defined %v", p.defs)
	}
	r.Reset()
	p.defs = nil
	if err := r.SwapCOM(2); err != nil {
		t.Fatal(err)
	}
	if len(p.defs) != 1 || p.defs[DefaultEventBase] != "L:B" {
		t.Errorf("after Reset defined %v", p.defs)
	}
}
