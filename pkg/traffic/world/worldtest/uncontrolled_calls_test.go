package worldtest

import (
	"net/http"
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

// TestUncontrolledMixedCalls: airliners and light aircraft together, with
// room for both, say all the calls (#1307). At a limit the airliners fill,
// the light aircraft waits for a slot.
func TestUncontrolledMixedCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: uncontrolledData(t), ICAO: "LKPR", Manual: true, FrameRate: 5})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.SetWeather(&nav.Weather{WindDirTrue: 240, WindKts: 5, VisibilityM: 9999, QNHhPa: 1013, TempC: 15})
	yes := true
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 40, IFR: &yes, VFR: &yes}); err != nil {
		t.Fatal(err)
	}
	calls := []string{"released for departure", "cancel IFR", " downwind runway 24", " base runway 24", "final runway 24", "clear of runway"}
	h.Until(3*time.Hour, func() bool {
		for _, c := range calls {
			if !heardCall(h, c) {
				return false
			}
		}
		return true
	})
	for _, c := range calls {
		if !heardCall(h, c) {
			t.Errorf("%q not said in 3 simulated hours", c)
		}
	}
	nobodyLocal(t, h)
}

// TestUncontrolledNoWeatherCancels: no weather known: an IFR arrival cancels
// (#1307).
func TestUncontrolledNoWeatherCancels(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: uncontrolledData(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	yes, no := true, false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, IFR: &yes, VFR: &no}); err != nil {
		t.Fatal(err)
	}
	if !h.Until(3*time.Hour, func() bool { return heardCall(h, "cancel IFR") }) {
		t.Error("no cancellation in 3 simulated hours")
	}
}

// TestUncontrolledTouchAndGo: a training circuit reports every leg, on each
// circuit, and only the upwind has no side (#1307).
func TestUncontrolledTouchAndGo(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	start := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	h, err := Start(Options{DataDir: uncontrolledData(t), ICAO: "LKPR", Manual: true, FrameRate: 10, Start: start})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.World.SetWeather(&nav.Weather{WindDirTrue: 240, WindKts: 5, VisibilityM: 9999, QNHhPa: 1013, TempC: 15})
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, IFR: &no, VFR: &no, Generator: &no}); err != nil {
		t.Fatal(err)
	}
	sta := start.Add(15 * time.Minute)
	flights := []traffic.Flight{{Callsign: "OKTNG", Origin: "LKKB", Destination: "LKPR", Type: "DA62", Rules: "VFR", STD: sta.Add(-8 * time.Minute), STA: sta, TouchAndGos: 1}}
	if _, err := h.World.Do(http.MethodPost, "/api/flights", flights); err != nil {
		t.Fatal(err)
	}
	legs := map[string]int{}
	count := func() {
		clear(legs)
		for _, tx := range h.Heard() {
			if tx.Callsign == "OKTNG" && tx.Params[traffic.ParamSelf] == "pattern" {
				legs[tx.Params[traffic.ParamCircuit]]++
			}
		}
	}
	h.Until(90*time.Minute, func() bool {
		count()
		return heardCall(h, "clear of runway")
	})
	count()
	var said []string
	for _, tx := range h.Heard() {
		if tx.Callsign == "OKTNG" {
			said = append(said, tx.Text)
		}
	}
	t.Log(strings.Join(said, "\n"))
	if legs["upwind"] != 1 {
		t.Errorf("upwind said %d times, want 1: %v", legs["upwind"], legs)
	}
	for _, want := range []struct {
		leg string
		n   int
	}{{" crosswind", 1}, {" downwind", 2}, {" base", 2}} {
		n := 0
		for leg, c := range legs {
			if strings.HasSuffix(leg, want.leg) && (strings.HasPrefix(leg, "left ") || strings.HasPrefix(leg, "right ")) {
				n += c
			}
		}
		if n != want.n {
			t.Errorf("%s said %d times, want %d: %v", want.leg, n, want.n, legs)
		}
	}
	nobodyLocal(t, h)
}

// wakeBed is a medium airliner and a light VFR arrival appearing as the
// airliner lands, on data's LKPR, flown until the light one is off the runway.
func wakeBed(t *testing.T, data string) *Harness {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	start := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	h, err := Start(Options{DataDir: data, ICAO: "LKPR", Manual: true, FrameRate: 10, Start: start})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.World.SetWeather(&nav.Weather{WindDirTrue: 240, WindKts: 5, VisibilityM: 9999, QNHhPa: 1013, TempC: 15})
	yes, no := true, false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, IFR: &yes, VFR: &yes, Generator: &no}); err != nil {
		t.Fatal(err)
	}
	sta := start.Add(35 * time.Minute)
	flights := []traffic.Flight{{Callsign: "CSA100", Origin: "LKKB", Destination: "LKPR", Type: "A320", STD: sta.Add(-time.Hour), STA: sta}}
	at := start.Add(11 * time.Minute)
	flights = append(flights, traffic.Flight{Callsign: wakeVFR, Origin: "LKKB", Destination: "LKPR", Type: "DA62", Rules: "VFR", STD: at.Add(-6 * time.Minute), STA: at})
	if _, err := h.World.Do(http.MethodPost, "/api/flights", flights); err != nil {
		t.Fatal(err)
	}
	h.Until(90*time.Minute, func() bool {
		for _, tx := range h.Heard() {
			if tx.Callsign == wakeVFR && (strings.Contains(tx.Text, "when vacated") || strings.Contains(tx.Text, "clear of runway")) {
				return true
			}
		}
		return false
	})
	return h
}

// wakeVFR is wakeBed's light aircraft.
const wakeVFR = "OKWAK"

// TestWakeCautionVFR: the tower cautions the light aircraft behind the
// airliner, in the sequence or just landed; once, and only it (#1307).
func TestWakeCautionVFR(t *testing.T) {
	h := wakeBed(t, dataDir(t))
	each := map[string]int{}
	told := map[string]time.Time{}
	var log []string
	for _, tx := range h.Heard() {
		if tx.Intent == traffic.IntentFollow && strings.Contains(tx.Text, "follow the Airbus") && told[tx.Callsign].IsZero() {
			told[tx.Callsign] = tx.At
		}
		if at := told[tx.Callsign]; tx.Intent == traffic.IntentWakeCaution && !at.IsZero() && tx.At.Sub(at) > 30*time.Second {
			t.Errorf("%s told to follow the airliner at %v, cautioned at %v", tx.Callsign, at, tx.At)
		}
		if tx.Intent == traffic.IntentWakeCaution || tx.Intent == traffic.IntentLanding || tx.Intent == traffic.IntentFollow {
			log = append(log, tx.At.Format("15:04:05")+" "+tx.Text)
		}
		if tx.Intent != traffic.IntentWakeCaution {
			continue
		}
		each[tx.Callsign]++
		if tx.Callsign != wakeVFR || tx.Position != traffic.PosTower {
			t.Errorf("%s on %s: %q", tx.Callsign, tx.Position, tx.Text)
		}
	}
	t.Log(strings.Join(log, "\n"))
	if len(each) == 0 {
		t.Error("no caution")
	}
	for cs, n := range each {
		if n != 1 {
			t.Errorf("%s cautioned %d times", cs, n)
		}
	}
}

// TestWakeCautionNoTower: nobody to say it at a field without a tower.
func TestWakeCautionNoTower(t *testing.T) {
	h := wakeBed(t, uncontrolledData(t))
	if heardCall(h, "wake turbulence") {
		t.Error("a wake caution at a field without a tower")
	}
	if !heardCall(h, "clear of runway") {
		t.Error("nothing landed")
	}
}
