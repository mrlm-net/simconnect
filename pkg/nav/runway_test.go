package nav

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestRunwaySelectorHolds: the runway in use does not flip with a wind
// shift near a limit; it changes when out of limits, or after the better
// choice held for ChangeAfter.
func TestRunwaySelectorHolds(t *testing.T) {
	l := lkprInfo(t).Layout
	var s RunwaySelector
	now := time.Now()
	// 120°/11 kt: 06 has the headwind.
	calm := StaticWeather(120, 11, 9999, 15, 5, 1013)
	if u := s.Choose(now, l, calm, RunwayLimits{}); u.Arrival.Name != "06" && u.Arrival.Name != "12" {
		t.Fatalf("first choice %s", u.Arrival.Name)
	}
	first := s.Choose(now, l, calm, RunwayLimits{}).Arrival.Name
	// The wind veers so another runway is better, within 06's limits: kept.
	shift := StaticWeather(220, 6, 9999, 15, 5, 1013)
	fresh := ActiveRunways(l, shift, RunwayLimits{}).Arrival.Name
	if fresh == first {
		t.Skip("the shift does not change the choice at this airport")
	}
	for i := 1; i <= 9; i++ {
		if u := s.Choose(now.Add(time.Duration(i)*time.Minute), l, shift, RunwayLimits{}); u.Arrival.Name != first {
			t.Fatalf("changed to %s after %d min", u.Arrival.Name, i)
		}
	}
	if u := s.Choose(now.Add(11*time.Minute), l, shift, RunwayLimits{}); u.Arrival.Name != fresh {
		t.Fatalf("still %s after 11 min, want %s", u.Arrival.Name, fresh)
	}
	// Out of limits: at once.
	var s2 RunwaySelector
	s2.Choose(now, l, calm, RunwayLimits{})
	strong := StaticWeather(300, 20, 9999, 15, 5, 1013) // a strong tailwind on 06/12
	if u := s2.Choose(now.Add(time.Second), l, strong, RunwayLimits{}); u.Arrival.Name == first {
		t.Fatalf("kept %s in a 20 kt tailwind", first)
	}
}

// The ATIS says the runway the traffic keeps, not a fresh choice at every
// wind shift (#454; LKPR, live: wind 100° at 5–10 kt, at 24's tailwind
// limit, and the ATIS broadcast 06, 24, 06, 24… a minute apart while the
// traffic held 06).
func TestATISHoldsTheRunway(t *testing.T) {
	info := lkprInfo(t)
	l := info.Layout
	lim := RunwayLimitsFrom(airport.LimitsFor(l, loadLKPRProcedures(t)))
	now := time.Date(2026, 9, 30, 10, 39, 0, 0, time.UTC)
	light, fresh := StaticWeather(100, 5, 9999, 18, 10, 1025), StaticWeather(100, 10, 9999, 18, 10, 1025)
	if ActiveRunways(l, light, lim).Arrival.Name == ActiveRunways(l, fresh, lim).Arrival.Name {
		t.Skip("the two winds choose the same runway with these limits")
	}
	for _, shared := range []bool{false, true} {
		var opts []ATISOption
		sel := &RunwaySelector{}
		if shared {
			opts = append(opts, ATISWithSelector(sel))
		}
		svc := NewATISService("Ruzyne", l, lim, 5000, opts...)
		changes, last := 0, ""
		for i := 0; i < 12; i++ {
			w := light
			if i%2 == 1 {
				w = fresh
			}
			at := now.Add(time.Duration(i) * time.Minute)
			if shared {
				sel.Choose(at, l, w, lim) // the traffic asks too
			}
			a, _ := svc.Update(w, at)
			if rwy := a.Use.Arrival.Name; rwy != last {
				if last != "" {
					changes++
				}
				last = rwy
			}
		}
		if changes > 1 {
			t.Errorf("shared %v: the ATIS changed runway %d times in 12 minutes", shared, changes)
		}
	}
}

// A runway is chosen only with a margin within its wind limits, and kept up
// to the limits (LKPR, live: at 110°/6 kt, 4.2 kt of tailwind on 24, the
// selector chose 24 and dropped it at the next gust).
func TestRunwayChoiceMargin(t *testing.T) {
	l := lkprInfo(t).Layout
	lim := RunwayLimitsFrom(airport.LimitsFor(l, loadLKPRProcedures(t)))
	now := time.Now()
	calm, edge := StaticWeather(110, 1, 9999, 18, 10, 1024), StaticWeather(110, 6, 9999, 18, 10, 1024)
	if ActiveRunways(l, calm, lim).Departure.Name != "24" || ActiveRunways(l, edge, lim).Departure.Name != "24" {
		t.Skip("24 is not the preferred choice in these winds")
	}
	var fresh RunwaySelector
	if u := fresh.Choose(now, l, edge, lim); u.Departure.Name != "06" {
		t.Errorf("chose %s at 4.2 kt of tailwind on 24, want 06", u.Departure.Name)
	}
	var kept RunwaySelector
	kept.Choose(now, l, calm, lim) // 24 in calm wind
	if u := kept.Choose(now.Add(time.Minute), l, edge, lim); u.Departure.Name != "24" {
		t.Errorf("dropped 24 within its limits for %s", u.Departure.Name)
	}
}

// A change that is due waits for its Ready moment (a gap in the traffic), at
// most MaxChangeWait; Pending shows it coming; Seed starts from the runway
// in use already.
func TestRunwaySelectorReadyAndSeed(t *testing.T) {
	l := lkprInfo(t).Layout
	now := time.Now()
	calm := StaticWeather(120, 11, 9999, 15, 5, 1013)
	shift := StaticWeather(220, 6, 9999, 15, 5, 1013)
	ready := false
	s := RunwaySelector{Ready: func(from, to RunwayUse) bool { return ready }}
	first := s.Choose(now, l, calm, RunwayLimits{}).Arrival.Name
	fresh := ActiveRunways(l, shift, RunwayLimits{}).Arrival.Name
	if fresh == first {
		t.Skip("the shift does not change the choice at this airport")
	}
	s.Choose(now.Add(time.Minute), l, shift, RunwayLimits{})
	if u, _, ok := s.Pending(); !ok || u.Arrival.Name != fresh {
		t.Fatalf("pending %v %s, want %s", ok, u.Arrival.Name, fresh)
	}
	if u := s.Choose(now.Add(12*time.Minute), l, shift, RunwayLimits{}); u.Arrival.Name != first {
		t.Fatalf("changed to %s with no gap", u.Arrival.Name)
	}
	ready = true
	if u := s.Choose(now.Add(13*time.Minute), l, shift, RunwayLimits{}); u.Arrival.Name != fresh {
		t.Fatalf("still %s at the gap, want %s", u.Arrival.Name, fresh)
	}
	if _, _, ok := s.Pending(); ok {
		t.Error("still pending after the change")
	}
	// No gap at all: changed once MaxChangeWait has gone too.
	ready = false
	s2 := RunwaySelector{Ready: func(from, to RunwayUse) bool { return ready }}
	s2.Choose(now, l, calm, RunwayLimits{})
	s2.Choose(now.Add(time.Minute), l, shift, RunwayLimits{})
	if u := s2.Choose(now.Add(time.Minute+RunwayChangeAfter+RunwayChangeMaxWait), l, shift, RunwayLimits{}); u.Arrival.Name != fresh {
		t.Errorf("still %s after the longest wait", u.Arrival.Name)
	}
	// Seeded with the other runway: kept, as one in use is.
	var s3 RunwaySelector
	seeded := ActiveRunways(l, calm, RunwayLimits{})
	s3.Seed(seeded)
	if u := s3.Choose(now, l, shift, RunwayLimits{}); u.Arrival.Name != seeded.Arrival.Name {
		t.Errorf("seeded with %s, chose %s at once", seeded.Arrival.Name, u.Arrival.Name)
	}
}

// Near calm the runway in use stays with no change pending; a better choice
// gone for a moment keeps its pending change (live LKPR, 083/2–3 kt:
// Pending came and went every few seconds).
func TestRunwaySelectorCalmAndFlicker(t *testing.T) {
	l := lkprInfo(t).Layout
	now := time.Now()
	west := StaticWeather(250, 8, 9999, 15, 5, 1013)
	east := StaticWeather(70, 4, 9999, 15, 5, 1013)
	var s RunwaySelector
	first := s.Choose(now, l, west, RunwayLimits{}).Arrival.Name
	if u := s.Choose(now.Add(time.Second), l, StaticWeather(83, 2, 9999, 15, 5, 1013), RunwayLimits{}); u.Arrival.Name != first {
		t.Fatalf("calm: changed to %s", u.Arrival.Name)
	}
	if _, _, ok := s.Pending(); ok {
		t.Fatal("calm: a change pending")
	}
	s.Choose(now.Add(10*time.Second), l, east, RunwayLimits{})
	if _, _, ok := s.Pending(); !ok {
		t.Skip("the east wind does not change the choice at this airport")
	}
	s.Choose(now.Add(20*time.Second), l, west, RunwayLimits{})
	if _, _, ok := s.Pending(); !ok {
		t.Fatal("pending dropped at once")
	}
	s.Choose(now.Add(2*time.Minute), l, west, RunwayLimits{})
	if _, _, ok := s.Pending(); ok {
		t.Error("still pending after the better choice was gone a minute")
	}
}

// TestRunwaySelectorCrosswindKept: a near pure crosswind with half a knot of
// headwind either way keeps the runway in use (live KSAN, 010°/3–6 kt
// flipped 09 and 27 every ten minutes); a real headwind still changes it.
func TestRunwaySelectorCrosswindKept(t *testing.T) {
	l := &airport.Layout{ICAO: "TEST", Runways: []airport.Runway{{
		Heading: 90, Length: 2800,
		Primary:   airport.RunwayEnd{Name: "09", Heading: 90},
		Secondary: airport.RunwayEnd{Name: "27", Heading: 270},
	}}}
	now := time.Now()
	var s RunwaySelector
	if u := s.Choose(now, l, StaticWeather(10, 5, 9999, 21, 15, 1006), RunwayLimits{}); u.Arrival.Name != "09" {
		t.Fatalf("start on %s, want 09", u.Arrival.Name)
	}
	for i := range 10 {
		at := now.Add(time.Duration(i+1) * 5 * time.Minute)
		if u := s.Choose(at, l, StaticWeather(355, 6, 9999, 21, 15, 1006), RunwayLimits{}); u.Arrival.Name != "09" {
			t.Fatalf("crosswind: changed to %s", u.Arrival.Name)
		}
	}
	if _, _, ok := s.Pending(); ok {
		t.Error("crosswind: a change pending")
	}
	west := StaticWeather(270, 8, 9999, 21, 15, 1006)
	s.Choose(now.Add(time.Hour), l, west, RunwayLimits{})
	if u := s.Choose(now.Add(2*time.Hour), l, west, RunwayLimits{}); u.Arrival.Name != "27" {
		t.Errorf("west 8 kt: still on %s", u.Arrival.Name)
	}
}
