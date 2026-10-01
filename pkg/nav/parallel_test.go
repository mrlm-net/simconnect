//go:build windows
// +build windows

package nav

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

func loadLayout(t testing.TB, icao string) *airport.Layout {
	t.Helper()
	b, err := os.ReadFile("../airport/testdata/" + icao + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// Parallel runways in use together, the mode from their spacing; crossing
// runways never.
func TestParallelRunways(t *testing.T) {
	west := Weather{WindDirTrue: 260, WindKts: 10}
	for _, tc := range []struct {
		icao string
		lim  RunwayLimits
		mode ParallelMode
		n    int // arrival runways in use
	}{
		{"EDDM", RunwayLimits{}, ParallelIndependent, 2},
		{"EGLL", RunwayLimits{Preferred: []string{"27R", "27L"}}, ParallelIndependent, 2},
		{"EGLL", RunwayLimits{Preferred: []string{"27R", "27L"}, Parallel: ParallelSegregated}, ParallelSegregated, 1},
		{"EDDM", RunwayLimits{Parallel: ParallelNone}, ParallelNone, 1},
		{"LKPR", lkprLimits, ParallelNone, 1}, // 06/24 and 12/30 cross
	} {
		l := loadLayout(t, tc.icao)
		u := ActiveRunways(l, west, tc.lim)
		t.Logf("%s %s: departures %v, arrivals %v, %.0f m", tc.icao, u.Parallel, Names(u.Departures), Names(u.Arrivals), u.SpacingM)
		if u.Parallel != tc.mode || len(u.Arrivals) != tc.n {
			t.Errorf("%s: %s with %d arrival runways, want %s with %d", tc.icao, u.Parallel, len(u.Arrivals), tc.mode, tc.n)
		}
		if u.Departure.Name != u.Departures[0].Name || u.Arrival.Name != u.Arrivals[0].Name {
			t.Errorf("%s: Departure/Arrival not the first in use", tc.icao)
		}
		if tc.mode == ParallelSegregated && (len(u.Departures) != 1 || u.Departures[0].Name == u.Arrivals[0].Name) {
			t.Errorf("%s segregated: departures %v, arrivals %v", tc.icao, Names(u.Departures), Names(u.Arrivals))
		}
		for _, e := range append(slices.Clone(u.Departures), u.Arrivals...) {
			if d := headingDiff(e.Heading, u.Departure.Heading); d > 15 || d < -15 {
				t.Errorf("%s: %s not in the same direction", tc.icao, e.Name)
			}
		}
	}
	if m := ParallelModeFor(800); m != ParallelSegregated {
		t.Errorf("800 m: %s", m)
	}
	if m := ParallelModeFor(950); m != ParallelDependent {
		t.Errorf("950 m: %s", m)
	}
	if m := ParallelModeFor(500); m != ParallelNone {
		t.Errorf("500 m: %s", m)
	}
}

// A stand flies from and to the parallel nearest it.
func TestNearestParallel(t *testing.T) {
	l := loadLayout(t, "EDDM")
	u := ActiveRunways(l, Weather{WindDirTrue: 260, WindKts: 10}, RunwayLimits{})
	if len(u.Arrivals) < 2 {
		t.Skip("no parallels")
	}
	for _, e := range u.Arrivals {
		r, _ := runwayOfEnd(l, e.Name)
		near := airport.LatLon{Lat: r.Center.Lat, Lon: r.Center.Lon}
		if got := Nearest(l, u.Arrivals, near); got.Name != e.Name {
			t.Errorf("at %s's centre: %s", e.Name, got.Name)
		}
	}
}

// The ATIS names every runway in use.
func TestATISParallels(t *testing.T) {
	w := Weather{WindDirTrue: 260, WindKts: 10, QNHhPa: 1013, VisibilityM: 10000}
	for _, tc := range []struct {
		icao string
		lim  RunwayLimits
		want string
	}{
		{"EDDM", RunwayLimits{}, "runways in use 26L and 26R"},
		{"EGLL", RunwayLimits{Preferred: []string{"27R", "27L"}, Parallel: ParallelSegregated}, "landing runway 27R, departure runway 27L"},
		{"LKPR", lkprLimits, "runway in use 24"},
	} {
		u := ActiveRunways(loadLayout(t, tc.icao), w, tc.lim)
		if txt := NewATIS(tc.icao, 'A', time.Time{}, w, u, 0, 0).Text(); !strings.Contains(txt, tc.want) {
			t.Errorf("%s: %q, want %q", tc.icao, txt, tc.want)
		}
	}
}
