//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

func TestConditionsFrom(t *testing.T) {
	w := nav.StaticWeather(240, 20, 9999, 15, 5, 1013)
	c := ConditionsFrom(w, 240)
	if math.Abs(c.HeadwindKts-20) > 0.01 || c.Surface != RunwayDry || c.LowVisibility() || !c.ReducedAllowed() {
		t.Errorf("fine day: %+v", c)
	}
	if c := ConditionsFrom(w, 60); c.HeadwindKts > -19 {
		t.Errorf("runway 06 in a westerly: headwind %.0f, want a tailwind", c.HeadwindKts)
	}
	w.Precip = nav.PrecipRain
	if c := ConditionsFrom(w, 240); c.Surface != RunwayWet || c.ReducedAllowed() {
		t.Errorf("rain: %v, reduced %v", c.Surface, c.ReducedAllowed())
	}
	w.TempC = -2
	if c := ConditionsFrom(w, 240); c.Surface != RunwayContaminated {
		t.Errorf("freezing rain: %v", c.Surface)
	}
	w.Precip, w.TempC = nav.PrecipSnow, 1
	if c := ConditionsFrom(w, 240); c.Surface != RunwayContaminated {
		t.Errorf("snow: %v", c.Surface)
	}
	fog := nav.StaticWeather(0, 0, 300, 5, 5, 1013)
	if c := ConditionsFrom(fog, 240); !c.LowVisibility() {
		t.Error("300 m: not low visibility")
	}
	low := ApproachConditions{VisibilityM: 3000, CeilingFt: 150}
	if !low.LowVisibility() || low.ReducedAllowed() {
		t.Error("150 ft ceiling")
	}
}

func TestArrivalSpacingInConditions(t *testing.T) {
	m, h, j, l := WakeFor("A320"), WakeFor("B77W"), WakeFor("A388"), WakeFor("C172")
	good := ApproachConditions{VisibilityM: 9999}
	wet := ApproachConditions{VisibilityM: 9999, Surface: RunwayWet}
	snow := ApproachConditions{VisibilityM: 3000, Surface: RunwayContaminated}
	lvp := ApproachConditions{VisibilityM: 300}
	for _, c := range []struct {
		name         string
		lead, follow Wake
		cond         ApproachConditions
		reduced      bool
		want         float64
		why          string
	}{
		{"good, not approved", m, m, good, false, 3, ""},
		{"good, reduced", m, m, good, true, 2.5, "reduced separation"},
		{"wake minimum not reduced", h, m, good, true, 5, ""},
		{"wet: no reduced", m, m, wet, true, 3, ""},
		{"contaminated", m, m, snow, true, 4, "contaminated runway"},
		{"LVP", m, m, lvp, true, 6, "low visibility procedures"},
		{"LVP behind a heavy", h, m, lvp, false, 6, "low visibility procedures"},
		{"LVP, wake more", j, l, lvp, false, 8, ""},
	} {
		nm, why := ArrivalSpacing(c.lead, c.follow, SchemeICAO, c.cond, c.reduced)
		if nm != c.want || why != c.why {
			t.Errorf("%s: %.1f NM %q, want %.1f %q", c.name, nm, why, c.want, c.why)
		}
	}
	if RunwayOccupancyIn(m, true, RunwayContaminated) <= RunwayOccupancyIn(m, true, RunwayWet) || RunwayOccupancyIn(m, true, RunwayWet) <= RunwayOccupancy(m, true) {
		t.Error("occupancy: dry < wet < contaminated")
	}
}

// TestSequencerWeather: the same pair lands farther apart in fog, and a
// headwind stretches distance-based spacing in time but not time-based.
func TestSequencerWeather(t *testing.T) {
	now := time.Now()
	pair := []ApproachAircraft{arr("CSA1", "A320", 30), arr("CSA2", "A320", 30.5)}
	gapIn := func(c ApproachConditions, tb bool) (time.Duration, SequenceEntry) {
		s := NewApproachSequencer("24", SequencerOptions{TimeBased: tb})
		s.SetConditions(c)
		seq := s.Update(now, pair)
		return seq[1].Landing.Sub(seq[0].Landing), seq[1]
	}
	calm, _ := gapIn(ApproachConditions{}, false)
	fog, e := gapIn(ApproachConditions{VisibilityM: 300}, false)
	if e.SpacingNM != 6 || e.SpacingWhy != "low visibility procedures" || fog < 2*calm-time.Second {
		t.Errorf("fog: %.0f NM %q, gap %v (calm %v)", e.SpacingNM, e.SpacingWhy, fog, calm)
	}
	wind, _ := gapIn(ApproachConditions{HeadwindKts: 30}, false)
	tbs, _ := gapIn(ApproachConditions{HeadwindKts: 30}, true)
	if wind <= calm || absDuration(tbs-calm) > time.Second {
		t.Errorf("30 kt headwind: distance-based %v, time-based %v, calm %v", wind, tbs, calm)
	}
}

// TestLandingFlowInFog: in low visibility the manager spaces its arrivals
// twice as far apart.
func TestLandingFlowInFog(t *testing.T) {
	var fs []ManagedFlight
	for i := 0; i < 3; i++ {
		fs = append(fs, ManagedFlight{Flight: flight("DLH"+string(rune('1'+i)), "EDDF", "LKPR", t0.Add(time.Duration(i)*time.Minute), time.Hour), Kind: "arrival", Airport: "LKPR"})
	}
	o := ManagerOptions{}
	o.defaults()
	var etas []time.Time
	for _, a := range CheckLandingFlow(0, 0)(Situation{Now: t0.Add(-time.Hour), Airport: "LKPR", Flights: fs, Options: o, Conditions: ApproachConditions{VisibilityM: 300}}) {
		if a.Action == AdviceEstimate {
			etas = append(etas, a.Until)
			if a.Reason != "landing sequence" {
				t.Errorf("reason %q", a.Reason)
			}
		}
	}
	if len(etas) != 2 || etas[0].Sub(t0.Add(time.Hour)) != 6*time.Minute || etas[1].Sub(etas[0]) != 6*time.Minute {
		t.Fatalf("estimates %v, want 6 min apart", etas)
	}
}

// RECAT-EU: a pair whose 3 NM is a wake minimum (C behind C) is not reduced;
// D behind D, where only the radar minimum applies, is.
func TestReducedSeparationRecat(t *testing.T) {
	good := ApproachConditions{VisibilityM: 10000, CeilingFt: 5000}
	c, d := Wake{Recat: RecatC}, Wake{Recat: RecatD}
	if nm, _ := ArrivalSpacing(c, c, SchemeRecat, good, true); nm != 3 {
		t.Errorf("C behind C: %.1f NM, want 3 (a wake minimum)", nm)
	}
	if nm, _ := ArrivalSpacing(d, d, SchemeRecat, good, true); nm != ReducedRadarSeparationNM {
		t.Errorf("D behind D: %.1f NM, want %.1f", nm, ReducedRadarSeparationNM)
	}
}
