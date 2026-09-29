//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

type holdSpawner struct {
	fakeSpawner
	holds map[string]bool
}

func (s *holdSpawner) Hold(f ManagedFlight, on bool) { s.holds[f.Callsign] = on }

func find(m *TrafficManager, key string) ManagedFlight {
	for _, f := range m.Flights() {
		if f.Key() == key {
			return f
		}
	}
	return ManagedFlight{}
}

func TestCheckStuck(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, Checks: []SituationCheck{CheckStuck(nil)}}, "LKPR")
	m.Add([]Flight{flight("CSA100", "LKPR", "EDDF", t0.Add(10*time.Minute), time.Hour)})
	m.Tick(t0)
	m.Update("CSA100", FlightTaxiing, t0.Add(10*time.Minute))
	m.Tick(t0.Add(39 * time.Minute))
	if len(sp.removed) != 0 {
		t.Fatal("removed while taxiing 29 min")
	}
	m.Tick(t0.Add(41 * time.Minute))
	if f := find(m, "departure CSA100"); len(sp.removed) != 1 || f.Status != FlightCancelled || f.Err == "" {
		t.Fatalf("taxiing 31 min: removed %d, %v %q", len(sp.removed), f.Status, f.Err)
	}
}

func TestCheckLandingFlow(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, ArrivalSpacing: time.Second, Checks: []SituationCheck{CheckLandingFlow(0, 0)}}, "LKPR")
	sp.onSpawn = func(f ManagedFlight) { m.Update(f.Callsign, FlightApproaching, f.Since) }
	// Three arrivals a minute apart: they must land 3 min apart.
	m.Add([]Flight{
		flight("DLH1", "EDDF", "LKPR", t0, time.Hour),
		flight("DLH2", "EDDF", "LKPR", t0.Add(time.Minute), time.Hour),
		flight("DLH3", "EDDF", "LKPR", t0.Add(2*time.Minute), time.Hour),
	})
	spawnAt := map[string]time.Time{}
	for s := 30 * 60; s <= 60*60; s += 5 {
		now := t0.Add(time.Duration(s) * time.Second)
		n := len(sp.spawned)
		m.Tick(now)
		for _, f := range sp.spawned[n:] {
			spawnAt[f.Callsign] = now
		}
	}
	if len(spawnAt) != 3 {
		t.Fatalf("%d arrivals spawned", len(spawnAt))
	}
	if d := spawnAt["DLH2"].Sub(spawnAt["DLH1"]); d < 3*time.Minute-5*time.Second {
		t.Errorf("DLH2 %s behind DLH1", d)
	}
	if d := spawnAt["DLH3"].Sub(spawnAt["DLH2"]); d < 3*time.Minute-5*time.Second {
		t.Errorf("DLH3 %s behind DLH2", d)
	}
	if f := find(m, "arrival DLH3"); f.Estimated.IsZero() || f.Note == "" {
		t.Errorf("DLH3 delayed without an estimate (%v, %q)", f.Estimated, f.Note)
	}
}

func TestCheckGroundCongestion(t *testing.T) {
	sp := &holdSpawner{holds: map[string]bool{}}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, DepartureSpacing: time.Second, Checks: []SituationCheck{CheckGroundCongestion(2)}}, "LKPR")
	m.Add([]Flight{
		flight("CSA1", "LKPR", "EDDF", t0.Add(10*time.Minute), time.Hour),
		flight("CSA2", "LKPR", "EDDF", t0.Add(10*time.Minute), time.Hour),
		flight("CSA3", "LKPR", "EDDF", t0.Add(10*time.Minute), time.Hour),
	})
	for s := 0; s < 10; s++ {
		m.Tick(t0.Add(time.Duration(s) * time.Second))
	}
	for _, cs := range []string{"CSA1", "CSA2", "CSA3"} {
		m.Update(cs, FlightBoarding, t0.Add(10*time.Second))
	}
	m.Update("CSA1", FlightTaxiing, t0.Add(10*time.Minute))
	m.Update("CSA2", FlightTaxiing, t0.Add(10*time.Minute))
	m.Tick(t0.Add(10 * time.Minute))
	if !sp.holds["CSA3"] || !find(m, "departure CSA3").Held {
		t.Fatal("CSA3 not held with 2 taxiing")
	}
	m.Update("CSA1", FlightDeparting, t0.Add(12*time.Minute))
	m.Tick(t0.Add(12 * time.Minute))
	if sp.holds["CSA3"] || find(m, "departure CSA3").Held {
		t.Fatal("CSA3 still held with 1 taxiing")
	}
}

func TestCheckTurnaroundEstimate(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{Checks: []SituationCheck{CheckTurnaround(0)}}, "LKPR")
	// STA 09:00, STD 09:45; the inbound parks at 09:30: ETD 09:55.
	m.Add([]Flight{flight("CSA101", "EDDF", "LKPR", t0, time.Hour), flight("CSA102", "LKPR", "EDDF", t0.Add(105*time.Minute), time.Hour)})
	m.Tick(t0.Add(35 * time.Minute))
	m.Update("CSA101", FlightParked, t0.Add(90*time.Minute))
	m.Tick(t0.Add(90 * time.Minute))
	f := find(m, "departure CSA102")
	if want := t0.Add(115 * time.Minute); !f.Estimated.Equal(want) {
		t.Fatalf("ETD %v, want %v (%q)", f.Estimated.Format("15:04"), want.Format("15:04"), f.Note)
	}
}

func TestSpawnBlockedIsNoAttempt(t *testing.T) {
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, Checks: []SituationCheck{}}, "LKPR")
	blocked := 5
	sp.onSpawn = func(f ManagedFlight) {
		if blocked > 0 {
			blocked--
			m.Failed(f.Callsign, ErrSpawnBlocked, f.Since)
			return
		}
		m.Update(f.Callsign, FlightApproaching, f.Since)
	}
	m.Add([]Flight{flight("DLH1", "EDDF", "LKPR", t0.Add(30*time.Minute), time.Hour)})
	for s := 0; s <= 10*60; s += 5 {
		m.Tick(t0.Add(65*time.Minute + time.Duration(s)*time.Second))
	}
	if f := find(m, "arrival DLH1"); f.Status != FlightApproaching || f.Attempts != 1 {
		t.Fatalf("after 5 blocked spawns: %v, attempt %d", f.Status, f.Attempts)
	}
}

func TestManagerEvents(t *testing.T) {
	sp := &fakeSpawner{}
	var got []ManagerEvent
	var m *TrafficManager
	m = NewTrafficManager(sp, ManagerOptions{OnEvent: func(e ManagerEvent) {
		got = append(got, e)
		if e.Kind == EventStatus && e.Flight.Status == FlightSpawning {
			m.Update(e.Flight.Callsign, FlightBoarding, e.Time) // re-entrant: outside the lock
		}
	}}, "LKPR")
	m.Add([]Flight{flight("CSA101", "EDDF", "LKPR", t0, time.Hour), flight("CSA102", "LKPR", "EDDF", t0.Add(2*time.Hour), time.Hour)})
	m.SetEnabled(false)
	m.SetEnabled(true)
	m.Tick(t0.Add(35 * time.Minute))
	kinds := map[ManagerEventKind]int{}
	for _, e := range got {
		kinds[e.Kind]++
	}
	if kinds[EventAdded] != 2 || kinds[EventTurnaround] != 1 || kinds[EventDisabled] != 1 || kinds[EventEnabled] != 1 || kinds[EventStatus] != 2 {
		t.Fatalf("events %v", kinds)
	}
	if n := len(m.Events()); n != len(got) {
		t.Fatalf("channel has %d events, hook saw %d", n, len(got))
	}
	m.Remove("CSA101", t0.Add(36*time.Minute))
	if last := got[len(got)-1]; last.Kind != EventRemoved || last.Flight.Callsign != "CSA101" {
		t.Fatalf("last event %v %s, want removed CSA101", last.Kind, last.Flight.Callsign)
	}
}

func TestOtherTrafficRespected(t *testing.T) {
	pic := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 150})
	pic.SetAirports([]AirportRef{pictureLKPR})
	near := func(d float64) airport.LatLon { return offsetHeading(pictureLKPR.Position, 45, d) }
	now := t0.Add(10 * time.Minute)
	pic.Observe(now, []Observation{
		{ObjectID: 11, Tail: "AI1", Position: near(800), OnGround: true, GroundKts: 15},
		{ObjectID: 12, Tail: "AI2", Position: near(900), OnGround: true, GroundKts: 12},
	})
	for _, mode := range []OtherTrafficMode{OtherRespect, OtherIgnore} {
		sp := &holdSpawner{holds: map[string]bool{}}
		m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, Picture: pic, Others: mode, Checks: []SituationCheck{CheckGroundCongestion(2)}}, "LKPR")
		m.Add([]Flight{flight("CSA1", "LKPR", "EDDF", t0.Add(15*time.Minute), time.Hour)})
		m.Tick(t0.Add(5 * time.Minute))
		m.Update("CSA1", FlightBoarding, t0.Add(5*time.Minute))
		m.Tick(now)
		if held := find(m, "departure CSA1").Held; held != (mode == OtherRespect) {
			t.Errorf("%s: held %v with 2 other aircraft taxiing", mode, held)
		}
		if n := len(m.Others("LKPR")); n != map[OtherTrafficMode]int{OtherRespect: 2, OtherIgnore: 0}[mode] {
			t.Errorf("%s: %d others", mode, n)
		}
	}
}

func TestOtherArrivalTakesSlot(t *testing.T) {
	pic := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 150})
	pic.SetAirports([]AirportRef{pictureLKPR})
	now := t0.Add(25 * time.Minute)
	// An MSFS AI aircraft 25 NM out at 150 kt: lands in 10 min, 08:35.
	pic.Observe(now, []Observation{{ObjectID: 14, Tail: "AI3", Position: offsetHeading(pictureLKPR.Position, 45, 25*1852), AGLFt: 6000, VSFpm: -800, GroundKts: 150}})
	sp := &fakeSpawner{}
	m := NewTrafficManager(sp, ManagerOptions{MinTurn: -1, Picture: pic, ArrivalLead: 15 * time.Minute, Checks: []SituationCheck{CheckLandingFlow(0, 0)}}, "LKPR")
	// Ours would land 08:35 too (STA 08:40 − 5, appearing now, 10 min to
	// fly): moved behind AI3.
	m.Add([]Flight{flight("DLH1", "EDDF", "LKPR", t0.Add(-20*time.Minute), time.Hour)})
	m.Tick(now)
	if len(sp.spawned) != 0 {
		t.Fatal("spawned into the other arrival's slot")
	}
	if f := find(m, "arrival DLH1"); f.Estimated.IsZero() {
		t.Fatal("no later ETA")
	}
}

// TestEstimateMovesByMinutes: an estimate that drifts by seconds each tick
// is not announced again.
func TestEstimateMovesByMinutes(t *testing.T) {
	n := 0
	drift := time.Duration(0)
	check := func(s Situation) []Advice {
		var out []Advice
		for _, f := range s.Flights {
			out = append(out, Advice{Key: f.Key(), Action: AdviceEstimate, Until: f.STA.Add(10*time.Minute + drift)})
		}
		return out
	}
	m := NewTrafficManager(&fakeSpawner{}, ManagerOptions{MinTurn: -1, Checks: []SituationCheck{check}, OnEvent: func(e ManagerEvent) {
		if e.Kind == EventEstimated {
			n++
		}
	}}, "LKPR")
	m.Add([]Flight{flight("DLH1", "EDDF", "LKPR", t0.Add(3*time.Hour), time.Hour)})
	for i := 0; i < 20; i++ {
		drift = time.Duration(i) * 2 * time.Second
		m.Tick(t0.Add(time.Duration(i) * time.Second))
	}
	if n != 1 {
		t.Fatalf("%d estimate events for a 40 s drift, want 1", n)
	}
	drift = 2 * time.Minute
	m.Tick(t0.Add(time.Minute))
	if n != 2 {
		t.Fatalf("%d estimate events after a 2 min move, want 2", n)
	}
}

// TestLandingFlowOneDepartureGap: with departures waiting, one gap is
// opened in the arrival stream, not one between every two arrivals.
func TestLandingFlowOneDepartureGap(t *testing.T) {
	var fs []ManagedFlight
	for i := 0; i < 5; i++ {
		fs = append(fs, ManagedFlight{Flight: flight("DLH"+string(rune('1'+i)), "EDDF", "LKPR", t0.Add(time.Duration(i)*time.Minute), time.Hour), Kind: "arrival", Airport: "LKPR"})
	}
	for i := 0; i < 2; i++ {
		fs = append(fs, ManagedFlight{Flight: flight("CSA"+string(rune('1'+i)), "LKPR", "EDDF", t0, time.Hour), Kind: "departure", Airport: "LKPR", Status: FlightTaxiing})
	}
	o := ManagerOptions{}
	o.defaults()
	now := t0.Add(-time.Hour)
	var etas []time.Time
	for _, a := range CheckLandingFlow(0, 0)(Situation{Now: now, Airport: "LKPR", Flights: fs, Options: o}) {
		if a.Action == AdviceEstimate {
			etas = append(etas, a.Until)
		}
	}
	if len(etas) != 4 {
		t.Fatalf("%d estimates for 4 arrivals behind the first", len(etas))
	}
	// Gaps: 6 min (the departure gap) once, then 3 min.
	sta0 := t0.Add(time.Hour) // DLH1's STA
	want := []time.Duration{6, 9, 12, 15}
	for i, e := range etas {
		if d := e.Sub(sta0); d != want[i]*time.Minute {
			t.Errorf("arrival %d estimated %v after the first, want %v min", i+2, d, want[i])
		}
	}
}
