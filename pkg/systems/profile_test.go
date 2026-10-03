//go:build windows
// +build windows

package systems

import (
	"math"
	"strings"
	"testing"
)

func TestDefaultAndFenix(t *testing.T) {
	d := Default()
	if len(d.Values) < 25 {
		t.Fatalf("default has %d values", len(d.Values))
	}
	fnx := For(Aircraft{Package: "fnx-aircraft-319-321"})
	if fnx.Name != "Fenix A320 family" || len(fnx.Values[Battery].Vars) != 2 || fnx.Values[LightBeacon].Vars[0] != "LIGHT BEACON" {
		t.Fatalf("fenix profile %q: battery %v, beacon %v", fnx.Name, fnx.Values[Battery].Vars, fnx.Values[LightBeacon].Vars)
	}
	if For(Aircraft{Package: "asobo-c172"}).Name != "default" {
		t.Error("a stock aircraft matched an override")
	}
	if !strings.HasPrefix(fnx.Values[COM1Power].Note, "measured") || fnx.Actions[COM1Swap].Press != "L:S_PED_RMP1_XFER" || For(Aircraft{Package: "x"}).Actions[COM1Swap].Press != "" {
		t.Errorf("COM power %q, swap %+v", fnx.Values[COM1Power].Note, fnx.Actions[COM1Swap])
	}
}

// The Fenix read dark with the standard vars saying on; then with battery
// 2 on and external power feeding.
func TestResolve(t *testing.T) {
	p := For(Aircraft{Package: "fnx-aircraft-320"})
	read := map[varUnit]float64{
		{"ELECTRICAL MASTER BATTERY", "bool"}: 1, {"AVIONICS MASTER SWITCH", "bool"}: 1, {"ELECTRICAL MAIN BUS VOLTAGE", "volts"}: 28,
		{"TRANSPONDER CODE:1", "Bco16"}: 0x2000, {"LIGHT BEACON", "bool"}: 1, {"NUMBER OF ENGINES", "number"}: 2,
	}
	s := resolveState(p, read)
	if s.Battery || s.Powered || s.Avionics || s.COM1 {
		t.Errorf("dark Fenix read %+v", s)
	}
	if !s.Beacon || s.Squawk != "2000" || s.Engines != 2 {
		t.Errorf("standard values %+v", s)
	}
	for _, k := range []varUnit{{"L:S_OH_ELEC_BAT2", "number"}, {"L:B_ELEC_BUS_POWER_DC_ESS", "number"}, {"L:I_OH_ELEC_EXT_PWR_L", "number"}, {"L:B_ELEC_BUS_POWER_AC_ESS", "number"}, {"L:B_PED_RMP1_POWER", "number"}} {
		read[k] = 1
	}
	read[varUnit{"L:N_ELEC_VOLT_BAT_1", "number"}], read[varUnit{"L:N_ELEC_VOLT_BAT_2", "number"}] = 25.4, 27.8
	read[varUnit{"L:N_PED_RMP1_STDBY", "number"}] = 121805
	s = resolveState(p, read)
	if !s.Battery || !s.Powered || !s.Avionics || !s.ExtOn || !s.ExtAvailable || !s.COM1 || s.Volts != 27.8 || math.Abs(s.COM1Standby-121.805) > 1e-9 {
		t.Errorf("powered Fenix read %+v", s)
	}
}

// A three-position switch true only at a position, a threshold, and a local
// override winning per value.
func TestValuesAndOverride(t *testing.T) {
	strobeOn := Value{Vars: []string{"L:S_OH_EXT_LT_STROBE"}, TrueAt: []float64{2}}
	if strobeOn.resolve(map[varUnit]float64{{"L:S_OH_EXT_LT_STROBE", "number"}: 1}) != 0 {
		t.Error("strobe AUTO read on")
	}
	over, err := ReadProfile(strings.NewReader(`{"name":"my Fenix","match":{"packagePrefix":["fnx-aircraft"]},
		"values":{"lightStrobe":{"vars":["L:S_OH_EXT_LT_STROBE"],"trueAt":[2]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	p := For(Aircraft{Package: "fnx-aircraft-320"}, over)
	if p.Name != "my Fenix" || p.Values[LightStrobe].Vars[0] != "L:S_OH_EXT_LT_STROBE" || p.Values[Battery].Combine != "any" {
		t.Errorf("merged %q: strobe %v, battery %+v", p.Name, p.Values[LightStrobe], p.Values[Battery])
	}
	if _, err := ReadProfile(strings.NewReader(`{"name":"x","values":{"battery":{"vars":["A"],"combine":"sum"}}}`)); err == nil {
		t.Error("unknown combine accepted")
	}
	if squawkOf(0x7700) != "7700" {
		t.Error("squawk")
	}
}
