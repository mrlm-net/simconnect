package world

import (
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

type stateDep struct {
	departureCtl
	s traffic.TaxiState
}

func (d *stateDep) State() traffic.TaxiState { return d.s }

// At a runway change only the departures rolling, lining up or lined up and
// the first one held short of the old runway finish on it; the queue held
// behind it and every one taxiing go to the new runway (live, LKPR 24 to
// 06: nine departures left taxiing to 24 into the 06 arrivals).
func TestRunwayChangeKeepsOnlyTheFirstHeldShort(t *testing.T) {
	cc := &controlCenter{core: &core{}}
	now := traffic.Now()
	var items []*controlled
	add := func(tail string, s traffic.TaxiState, remaining float64, heldFor time.Duration) {
		it := &controlled{Tail: tail, dep: &stateDep{s: s}}
		it.view = ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: s.String(), TaxiRemainingM: remaining, HoldingShortOf: ""}
		if s == traffic.TaxiHoldingShort {
			it.view.HoldingShortOf = "24"
			it.heldShortAt = now.Add(-heldFor)
		}
		items = append(items, it)
	}
	add("ROLL", traffic.TaxiDeparting, 0, 0)
	add("LINED", traffic.TaxiLinedUp, 0, 0)
	add("LINING", traffic.TaxiLiningUp, 0, 0)
	add("HELD2", traffic.TaxiHoldingShort, 0, 30*time.Second)
	add("HELD1", traffic.TaxiHoldingShort, 0, 2*time.Minute)
	add("QUEUE", traffic.TaxiTaxiing, 120, 0)
	add("NEAR", traffic.TaxiTaxiing, 400, 0)
	add("FAR", traffic.TaxiTaxiing, 1500, 0)
	add("STAND", traffic.TaxiAwaitingPushback, 0, 0)
	keep := cc.lastOnOldRunway(&airport.Graph{Layout: &airport.Layout{ICAO: "LKPR"}}, items, []string{"06"})
	var got []string
	for tail := range keep {
		got = append(got, tail)
	}
	slices.Sort(got)
	if want := []string{"HELD1", "LINED", "LINING", "ROLL"}; !slices.Equal(got, want) {
		t.Errorf("finish on 24: %v, want %v", got, want)
	}
}

// A departure still for the old runway goes to the new one past the
// changeover's deadline, and once there is no deadline any more after the
// change has settled (live, LKPR: PHVUZ lined up on 24 against the 06
// arrivals 20 minutes after the change, three sent around).
func TestChangeOverdueWithoutDeadline(t *testing.T) {
	changed := time.Date(2026, 11, 4, 10, 17, 28, 0, time.UTC)
	cases := []struct {
		name string
		by   time.Time
		ok   bool
		now  time.Time
		want bool
	}{
		{"before the deadline", changed.Add(10 * time.Minute), true, changed.Add(5 * time.Minute), false},
		{"past the deadline", changed.Add(10 * time.Minute), true, changed.Add(11 * time.Minute), true},
		{"just changed, deadline not set yet", time.Time{}, false, changed.Add(10 * time.Second), false},
		{"deadline gone, 18 min after the change", time.Time{}, false, changed.Add(18 * time.Minute), true},
	}
	for _, c := range cases {
		if got := changeOverdueNow(c.by, c.ok, changed, c.now); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if changeOverdueNow(time.Time{}, false, time.Time{}, changed) {
		t.Error("never changed: overdue")
	}
}
