package nav

import (
	"testing"
	"time"
)

// TestRunwayClosures: LKPR with the wind on 24: closing 06/24 (by name or
// by an end) moves the choice to 12/30 at once, also from a configuration
// held through wind shifts; all runways closed, none is chosen.
func TestRunwayClosures(t *testing.T) {
	l := loadLKPR(t)
	w := Weather{WindDirTrue: 240, WindKts: 10}
	if u := ActiveRunways(l, w, RunwayLimits{}); u.Departure.Name != "24" {
		t.Fatalf("open: %s", u.Departure.Name)
	}
	for _, closed := range [][]string{{"06/24"}, {"24"}} {
		u := ActiveRunways(l, w, RunwayLimits{Closed: closed})
		if n := u.Departure.Name; n != "30" && n != "12" {
			t.Errorf("%v closed: %s, want 12 or 30", closed, n)
		}
	}
	s := &RunwaySelector{}
	now := time.Now()
	if u := s.Choose(now, l, w, RunwayLimits{}); u.Departure.Name != "24" {
		t.Fatalf("held: %s", u.Departure.Name)
	}
	if u := s.Choose(now.Add(time.Second), l, w, RunwayLimits{Closed: []string{"06/24"}}); u.Departure.Name == "24" || u.Arrival.Name == "24" {
		t.Errorf("24 closed while held: still %s/%s", u.Departure.Name, u.Arrival.Name)
	}
	var all []string
	for _, r := range l.Runways {
		all = append(all, r.Name())
	}
	if u := ActiveRunways(l, w, RunwayLimits{Closed: all}); u.Departure.Name != "" || u.Arrival.Name != "" {
		t.Errorf("all closed: %s/%s", u.Departure.Name, u.Arrival.Name)
	}
}
