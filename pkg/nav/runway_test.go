//go:build windows
// +build windows

package nav

import (
	"testing"
	"time"
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
	shift := StaticWeather(200, 4, 9999, 15, 5, 1013)
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
