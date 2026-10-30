package worldtest

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// runCalls runs data's LKPR with only IFR or only VFR traffic, in visM metres
// of visibility, until all of calls are said or 3 simulated hours; the
// harness.
func runCalls(t *testing.T, data string, visM float64, ifr bool, calls []string) *Harness {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: data, ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.World.SetWeather(&nav.Weather{WindDirTrue: 240, WindKts: 5, VisibilityM: visM, QNHhPa: 1013, TempC: 15})
	vfr := !ifr
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, IFR: &ifr, VFR: &vfr}); err != nil {
		t.Fatal(err)
	}
	h.Until(3*time.Hour, func() bool {
		for _, c := range calls {
			if !heardCall(h, c) {
				return false
			}
		}
		return true
	})
	return h
}

// heardCall: sub was said by anyone.
func heardCall(h *Harness, sub string) bool {
	for _, tx := range h.Heard() {
		if strings.Contains(tx.Text, sub) {
			return true
		}
	}
	return false
}

// nobodyLocal: nothing on tower, ground or delivery.
func nobodyLocal(t *testing.T, h *Harness) {
	t.Helper()
	for _, tx := range h.Heard() {
		switch tx.Position {
		case traffic.PosTower, traffic.PosGround, traffic.PosDelivery:
			t.Errorf("said on %s: %q", tx.Position, tx.Text)
		}
	}
}

// TestUncontrolledIFRCalls: at a field without a tower approach releases IFR
// departures, and arrivals announce themselves inbound and cancel IFR in
// good visibility (#1305).
func TestUncontrolledIFRCalls(t *testing.T) {
	calls := []string{"inbound for landing", "released for departure, report airborne", "cancel IFR", "IFR cancellation received"}
	h := runCalls(t, uncontrolledData(t), 9999, true, calls)
	for _, c := range calls {
		if !heardCall(h, c) {
			t.Errorf("%q not said in 3 simulated hours", c)
		}
	}
	nobodyLocal(t, h)
}

// TestUncontrolledVFRCalls: a VFR arrival at a field without a tower
// announces itself inbound, and its downwind and base with side and runway
// (#1305).
func TestUncontrolledVFRCalls(t *testing.T) {
	calls := []string{"inbound for landing", " downwind runway 24", " base runway 24"}
	h := runCalls(t, uncontrolledData(t), 9999, false, calls)
	for _, c := range calls {
		if !heardCall(h, c) {
			t.Errorf("%q not said in 3 simulated hours", c)
		}
	}
	nobodyLocal(t, h)
	for _, tx := range h.Heard() {
		if leg := tx.Params[traffic.ParamCircuit]; tx.Params[traffic.ParamSelf] == "pattern" && leg != "final" && !strings.HasPrefix(leg, "left ") && !strings.HasPrefix(leg, "right ") {
			t.Errorf("no side: %q", tx.Text)
		}
		if strings.Contains(tx.Text, "cancel IFR") || strings.Contains(tx.Text, "released for departure") {
			t.Errorf("VFR: %q", tx.Text)
		}
	}
}

// TestUncontrolledNoCancelInPoorVisibility: IFR arrivals do not cancel when
// the field cannot be seen, and are still released and land.
func TestUncontrolledNoCancelInPoorVisibility(t *testing.T) {
	calls := []string{"released for departure", "clear of runway"}
	h := runCalls(t, uncontrolledData(t), 2000, true, calls)
	for _, c := range calls {
		if !heardCall(h, c) {
			t.Errorf("%q not said in 3 simulated hours", c)
		}
	}
	if heardCall(h, "cancel IFR") {
		t.Error("cancelled IFR in 2000 m visibility")
	}
}

// TestTowerCallsUnchanged: at a towered field none of those calls is said,
// and a report in the circuit is as it was.
func TestTowerCallsUnchanged(t *testing.T) {
	data := dataDir(t)
	vfr := runCalls(t, data, 9999, false, []string{", downwind"})
	if !heardCall(vfr, ", downwind") {
		t.Error("no downwind report in 3 simulated hours")
	}
	ifr := runCalls(t, data, 9999, true, []string{"cleared to land", "cleared for take-off"})
	for _, h := range []*Harness{vfr, ifr} {
		for _, c := range []string{"inbound for landing", "released for departure", "cancel IFR"} {
			if heardCall(h, c) {
				t.Errorf("%q at a towered field", c)
			}
		}
		for _, tx := range h.Heard() {
			if tx.Intent == traffic.IntentCircuitReport && !strings.HasSuffix(tx.Params[traffic.ParamCircuit], "downwind") {
				t.Errorf("report changed: %q", tx.Text)
			}
		}
	}
}
