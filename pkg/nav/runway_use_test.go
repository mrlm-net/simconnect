package nav

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// withUse is l with end name used only for u (synthetic, #1661).
func withUse(l *airport.Layout, name string, u airport.RunwayEndUse) *airport.Layout {
	c := *l
	c.Runways = slices.Clone(l.Runways)
	for i := range c.Runways {
		if c.Runways[i].Primary.Name == name {
			c.Runways[i].Primary.Use = u
		}
		if c.Runways[i].Secondary.Name == name {
			c.Runways[i].Secondary.Use = u
		}
	}
	return &c
}

// Departures only from ends used for take-offs, arrivals only onto ends
// used for landings; both uses as before.
func TestActiveRunwaysEndUse(t *testing.T) {
	l := loadLKPR(t)
	w := Weather{WindDirTrue: 240, WindKts: 8}
	base := ActiveRunways(l, w, lkprLimits)
	if base.Departure.Name != "24" || base.Arrival.Name != "24" {
		t.Fatalf("LKPR 240/8: %s/%s, want 24/24", base.Departure.Name, base.Arrival.Name)
	}
	if both := ActiveRunways(withUse(l, "24", airport.RunwayUseBoth), w, lkprLimits); both.key() != base.key() {
		t.Errorf("use 0 written: %s, want %s", both.key(), base.key())
	}
	for _, tc := range []struct {
		use      airport.RunwayEndUse
		dep, arr bool // 24 for departures, arrivals
	}{
		{airport.RunwayUseTakeoff, true, false},
		{airport.RunwayUseLanding, false, true},
		{airport.RunwayUseNone, false, false},
	} {
		u := ActiveRunways(withUse(l, "24", tc.use), w, lkprLimits)
		t.Logf("24 %s: departures %v, arrivals %v", tc.use, Names(u.Departures), Names(u.Arrivals))
		if (u.Departure.Name == "24") != tc.dep || (u.Arrival.Name == "24") != tc.arr {
			t.Errorf("24 %s: departure %s, arrival %s", tc.use, u.Departure.Name, u.Arrival.Name)
		}
		if u.Departure.Name == "" || u.Arrival.Name == "" {
			t.Errorf("24 %s: none chosen (%q/%q)", tc.use, u.Departure.Name, u.Arrival.Name)
		}
	}
}

// A landing-only parallel entered at its threshold (EDDF 25R's shape,
// synthetic on EDDM) takes no departures, whether the ends are shared out
// by threshold entry or by spacing; segregated, with no parallel taking
// departures, the mode says one runway, not segregated (#1661).
func TestParallelEndUseThresholdEntry(t *testing.T) {
	l := loadLayout(t, "EDDM")
	w := Weather{WindDirTrue: 260, WindKts: 10}
	base := ActiveRunways(l, w, RunwayLimits{})
	if base.Parallel != ParallelIndependent || len(base.Departures) != 2 {
		t.Fatalf("EDDM: %s, departures %v", base.Parallel, Names(base.Departures))
	}
	own, other := base.Departures[0].Name, base.Departures[1].Name
	lu := withUse(l, other, airport.RunwayUseLanding)
	entry := func(n string) bool { return n == other }
	u := ActiveRunways(lu, w, RunwayLimits{ThresholdEntry: entry})
	t.Logf("%s landings only, entered at its threshold: %s, departures %v, arrivals %v", other, u.Parallel, Names(u.Departures), Names(u.Arrivals))
	if slices.Contains(Names(u.Departures), other) || !slices.Contains(Names(u.Departures), own) {
		t.Errorf("%s landings only: departures %v", other, Names(u.Departures))
	}
	seg := ActiveRunways(lu, w, RunwayLimits{Parallel: ParallelSegregated})
	t.Logf("segregated, %s landings only: %s, departures %v, arrivals %v", other, seg.Parallel, Names(seg.Departures), Names(seg.Arrivals))
	if slices.Contains(Names(seg.Departures), other) || seg.Parallel == ParallelSegregated && len(seg.Departures) == 1 && seg.Departures[0].Name == seg.Arrivals[0].Name {
		t.Errorf("segregated, %s landings only: %s, departures %v, arrivals %v", other, seg.Parallel, Names(seg.Departures), Names(seg.Arrivals))
	}
	ok := ActiveRunways(l, w, RunwayLimits{Parallel: ParallelSegregated})
	if ok.Parallel != ParallelSegregated || ok.Departures[0].Name == ok.Arrivals[0].Name {
		t.Errorf("segregated, both used for all: %s, departures %v, arrivals %v", ok.Parallel, Names(ok.Departures), Names(ok.Arrivals))
	}
}

// Parallels in use together: an end used for landings only takes no
// departures, one for take-offs only no arrivals.
func TestParallelEndUse(t *testing.T) {
	l := loadLayout(t, "EDDM")
	w := Weather{WindDirTrue: 260, WindKts: 10}
	base := ActiveRunways(l, w, RunwayLimits{})
	if base.Parallel != ParallelIndependent || len(base.Departures) != 2 {
		t.Fatalf("EDDM: %s, departures %v", base.Parallel, Names(base.Departures))
	}
	other := base.Departures[1].Name
	u := ActiveRunways(withUse(l, other, airport.RunwayUseLanding), w, RunwayLimits{})
	t.Logf("%s landings only: departures %v, arrivals %v", other, Names(u.Departures), Names(u.Arrivals))
	if slices.Contains(Names(u.Departures), other) || !slices.Contains(Names(u.Arrivals), other) {
		t.Errorf("%s landings only: departures %v, arrivals %v", other, Names(u.Departures), Names(u.Arrivals))
	}
	u = ActiveRunways(withUse(l, other, airport.RunwayUseTakeoff), w, RunwayLimits{})
	t.Logf("%s take-offs only: departures %v, arrivals %v", other, Names(u.Departures), Names(u.Arrivals))
	if !slices.Contains(Names(u.Departures), other) || slices.Contains(Names(u.Arrivals), other) {
		t.Errorf("%s take-offs only: departures %v, arrivals %v", other, Names(u.Departures), Names(u.Arrivals))
	}
}
