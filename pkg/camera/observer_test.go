package camera

import (
	"math"
	"testing"
	"time"
)

// observerLeg is a leg from a camera n metres north of the spot (at the
// test's origin), the subject standing 100 m east of the spot.
func observerLeg(n float64) Leg {
	clock := time.Unix(1000, 0)
	subj := geo(100, 0, 0)
	aim := NewAim(7, func() (SpotSample, bool) { return SpotSample{At: clock, Pose: subj}, true }, func() time.Time { return clock })
	return Leg{
		From: Pose{Eye: world(0, n, 20), Target: world(0, n-100, 0), FovDeg: 40},
		Have: true,
		To:   Spot{Name: "s", Eye: At(spotLat, spotLon, spotAlt)},
		Aim:  aim, Size: 36, D: 20 * time.Second,
	}
}

// TestObserverPlan: the zero Observer cuts; within CutM a cut, between it
// and TravelMaxM a travel ending at the spot's eye with the spot shot joined,
// farther too far; the first shot a cut.
func TestObserverPlan(t *testing.T) {
	o := Observer{TravelMaxM: 1000}
	if m := (Observer{}).Plan(observerLeg(1500)); m.Kind != MoveCut || len(m.Shots) != 1 {
		t.Errorf("zero Observer: %s, %d shots, want a cut", m.Kind, len(m.Shots))
	}
	first := observerLeg(800)
	first.Have = false
	if m := o.Plan(first); m.Kind != MoveCut || len(m.Shots) != 1 || m.M != 0 {
		t.Errorf("first shot: %s %.0f m, want a cut", m.Kind, m.M)
	}
	if m := o.Plan(observerLeg(200)); m.Kind != MoveCut || len(m.Shots) != 1 || math.Abs(m.M-200) > 1 || m.Travel != 0 {
		t.Errorf("200 m: %s %.0f m, want a cut", m.Kind, m.M)
	}
	m := o.Plan(observerLeg(800))
	if m.Kind != MoveTravel || len(m.Shots) != 2 || math.Abs(m.M-800) > 1 {
		t.Fatalf("800 m: %s %.0f m, %d shots, want a travel", m.Kind, m.M, len(m.Shots))
	}
	if m.Travel != m.Shots[0].Length() || m.Travel <= 0 {
		t.Errorf("travel %v, shot %v", m.Travel, m.Shots[0].Length())
	}
	if p := m.Shots[0].PoseAt(m.Travel); local(p.Eye).len() > 0.01 {
		t.Errorf("travel ends %.2f m from the spot's eye", local(p.Eye).len())
	}
	if p := m.Shots[0].PoseAt(0); local(p.Eye).sub(vec3{0, 800, 20}).len() > 0.01 {
		t.Errorf("travel starts %.2f m from the camera", local(p.Eye).sub(vec3{0, 800, 20}).len())
	}
	if m := o.Plan(observerLeg(1500)); m.Kind != MoveTooFar || m.Shots != nil {
		t.Errorf("1500 m: %s, %d shots, want too far", m.Kind, len(m.Shots))
	}
}

// TestObserverRailStart: a rail spot's shot starts abeam the subject, so a
// cut is measured to there, not to its eye.
func TestObserverRailStart(t *testing.T) {
	l := observerLeg(900)
	end := At(spotLat+1000/mPerDegLat, spotLon, spotAlt)
	l.To.RailEnd = &end
	clock := time.Unix(1000, 0)
	subj := geo(100, 850, 0)
	l.Aim = NewAim(7, func() (SpotSample, bool) { return SpotSample{At: clock, Pose: subj}, true }, func() time.Time { return clock })
	if m := (Observer{TravelMaxM: 1000}).Plan(l); m.Kind != MoveCut || math.Abs(m.M-50) > 1 {
		t.Errorf("rail abeam 50 m away: %s %.0f m, want a cut", m.Kind, m.M)
	}
}

// TestObserverDefaults: CutM 300 and at most TravelMaxM, the cruise 15 m/s
// kept to 10–20, the margin 15 m; Seconds the way at the cruise and a ramp.
func TestObserverDefaults(t *testing.T) {
	o := Observer{TravelMaxM: 1000}.WithDefaults()
	if o.CutM != DefaultObserverCutM || o.SpeedMps != DefaultTravelSpeed || o.MarginM != DefaultTravelMargin {
		t.Errorf("defaults: %+v", o)
	}
	if o := (Observer{TravelMaxM: 1000, CutM: 1200}).WithDefaults(); o.CutM != 1000 {
		t.Errorf("CutM %.0f past TravelMaxM 1000", o.CutM)
	}
	for in, want := range map[float64]float64{0: DefaultTravelSpeed, 4: 10, 12: 12, 40: 20} {
		if got := (Observer{TravelMaxM: 1000, SpeedMps: in}).WithDefaults().SpeedMps; got != want {
			t.Errorf("speed %.0f: %.0f, want %.0f", in, got, want)
		}
	}
	if z := (Observer{}).WithDefaults(); z != (Observer{}) {
		t.Errorf("zero Observer filled: %+v", z)
	}
	if s := o.Seconds(true, 300); s != 0 {
		t.Errorf("300 m: %.1f s, want a cut", s)
	}
	if s, want := o.Seconds(true, 600), 600/DefaultTravelSpeed+TravelRamp.Seconds(); s != want {
		t.Errorf("600 m: %.1f s, want %.1f", s, want)
	}
	if s := o.Seconds(false, 600); s != 0 {
		t.Errorf("no pose: %.1f s, want a cut", s)
	}
	if k, w := (Observer{TravelMaxM: 1000}).Decide(true, 400, 1001); k != MoveTooFar || w != 1001 {
		t.Errorf("way: %s %.0f", k, w)
	}
}
