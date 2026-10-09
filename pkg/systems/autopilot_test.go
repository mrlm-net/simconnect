//go:build windows

package systems

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/registry"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type dataClient struct {
	mapped map[uint32]string
	sent   []struct {
		event string
		data  uint32
	}
}

func (f *dataClient) MapClientEventToSimEvent(id uint32, name string) error {
	f.mapped[id] = name
	return nil
}

func (f *dataClient) TransmitClientEvent(_ uint32, id uint32, data uint32, _ uint32, _ types.SIMCONNECT_EVENT_FLAG) error {
	f.sent = append(f.sent, struct {
		event string
		data  uint32
	}{f.mapped[id], data})
	return nil
}

func (f *dataClient) AddToDataDefinition(uint32, string, string, types.SIMCONNECT_DATATYPE, float32, uint32) error {
	return nil
}

func (f *dataClient) SetDataOnSimObject(uint32, uint32, types.SIMCONNECT_DATA_SET_FLAG, uint32, uint32, unsafe.Pointer) error {
	return nil
}

// TestAutopilotDefault (#962): every autopilot value is a known SimVar in
// the default profile; the values set travel as their event's data (a
// negative vertical speed as two's complement, Mach × 100, an axis
// scaled); push and pull set the slot; modes on and off by their events.
func TestAutopilotDefault(t *testing.T) {
	p := Default()
	for _, k := range []string{APMaster, FD, ATHR, ATHRActive, APHeadingSel, APAltitudeSel, APVSSel, APSpeedSel, APMachSel,
		APHeadingManaged, APSpeedManaged, APAltitudeManaged, APVSManaged, APHeadingHold, APAltitudeHold, APVSHold, APFLC,
		APSpeedHold, APMachHold, APNav, APApproach, APGlideslope, APApproachArmed, APGSArmed, APAltitudeArmed} {
		v, ok := p.Values[k]
		if !ok {
			t.Errorf("%s: no value", k)
			continue
		}
		for _, name := range v.Vars {
			if err := registry.Validate(name, v.Unit); err != nil {
				t.Errorf("%s: %v", k, err)
			}
		}
	}
	f := &dataClient{mapped: map[uint32]string{}}
	c := NewControls(f, 0)
	c.Use(p)
	var none State
	steps := []struct {
		name string
		v    float64
		want string
		data uint32
	}{
		{APHeadingSel, 247, "HEADING_BUG_SET", 247},
		{APAltitudeSel, 5000, "AP_ALT_VAR_SET_ENGLISH", 5000},
		{APVSSel, -1000, "AP_VS_VAR_SET_ENGLISH", uint32(0xFFFFFC18)},
		{APSpeedSel, 250, "AP_SPD_VAR_SET", 250},
		{APMachSel, 0.78, "AP_MACH_VAR_SET", 78},
		{Throttle, 50, "AXIS_THROTTLE_SET", 8192},
		{ThrottleN(2), 100, "AXIS_THROTTLE2_SET", 16383},
		{Aileron, -100, "AXIS_AILERONS_SET", uint32(0xFFFFC001)},
	}
	for _, s := range steps {
		if err := c.SetValue(s.name, s.v, none); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		got := f.sent[len(f.sent)-1]
		if got.event != s.want || got.data != s.data {
			t.Errorf("%s %v: sent %s %d (%#x), want %s %d", s.name, s.v, got.event, got.data, got.data, s.want, s.data)
		}
	}
	for _, s := range []struct {
		name string
		on   bool
		want string
		data uint32
	}{
		{APSpeedManaged, true, "SPEED_SLOT_INDEX_SET", 2},
		{APSpeedManaged, false, "SPEED_SLOT_INDEX_SET", 1},
		{APMaster, true, "AUTOPILOT_ON", 1},
		{APMaster, false, "AUTOPILOT_OFF", 0},
		{APApproach, true, "AP_APR_HOLD_ON", 1},
		{GearDown, false, "GEAR_UP", 0},
		{SpoilersArmed, true, "SPOILERS_ARM_ON", 1},
	} {
		if err := c.Set(s.name, s.on, none); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got := f.sent[len(f.sent)-1]; got.event != s.want || got.data != s.data {
			t.Errorf("%s %v: sent %s %d, want %s %d", s.name, s.on, got.event, got.data, s.want, s.data)
		}
	}
}

// TestAutopilotState: read values give State.AP, the slot index 2 managed.
func TestAutopilotState(t *testing.T) {
	p := Default()
	read := map[varUnit]float64{
		{"AUTOPILOT MASTER", "bool"}: 1, {"AUTOPILOT ALTITUDE LOCK VAR", "feet"}: 7000,
		{"AUTOPILOT SPEED SLOT INDEX", "number"}: 2, {"AUTOPILOT HEADING SLOT INDEX", "number"}: 1,
		{"AUTOPILOT APPROACH ARM", "bool"}: 1,
	}
	s := resolveState(p, read)
	if !s.AP.Has || !s.AP.Master || s.AP.AltitudeSel != 7000 || !s.AP.SpeedManaged || s.AP.HeadingManaged || !s.AP.ApproachArmed || s.AP.Approach {
		t.Errorf("AP %+v", s.AP)
	}
}
