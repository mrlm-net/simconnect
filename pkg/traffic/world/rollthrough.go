package world

import (
	"slices"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A departure taxiing up to its runway, the runway next and close
// (TaxiController.RunwayNext), may be cleared for take-off before it gets
// to the holding point and roll through it onto the runway without a stop
// (#1568): a standard way to keep the runway moving. The clearance is
// offered only while every check the tower makes at the holding point
// passes for it — the runway free, the departure interval behind the last
// one (wake), the next arrival far enough out — decided as for one holding
// short there, its taxi to the holding point counted in its line-up time,
// on a copy of the runway's state (as PlayerRunway). "lineupbehind" is
// offered while the next arrival is all it waits for.

// rollThroughLineUp: the line-up counted after the taxi to the holding
// point (the tower's own LineUpTime).
const rollThroughLineUp = 60 * time.Second

// rollThroughOffer drops from a taxiing departure's offered actions a
// take-off or a line-up behind the tower's checks do not pass for now.
func (it *controlled) rollThroughOffer(actions []string) []string {
	return slices.DeleteFunc(actions, func(a string) bool {
		return a == "takeoff" && !it.rollTakeoff.Load() || a == "lineupbehind" && !it.rollBehind.Load()
	})
}

// rollThrough decides, each tower look, the take-off and the line-up
// behind for every departure taxiing up to its runway.
func (t *towers) rollThrough(now time.Time, items []*controlled, layout func(string) *airport.Layout) {
	for _, it := range items {
		takeoff, behind := t.rollThroughChecks(now, it, layout)
		it.rollTakeoff.Store(takeoff)
		it.rollBehind.Store(behind)
	}
}

// rollThroughChecks: the take-off may be given to it now, rolling through
// its holding point; the line-up behind the next arrival may be.
func (t *towers) rollThroughChecks(now time.Time, it *controlled, layout func(string) *airport.Layout) (bool, bool) {
	if it.dep == nil {
		return false, false
	}
	it.mu.Lock()
	v, atc := it.view, it.atc
	it.mu.Unlock()
	// The tower's to clear: on its frequency already, not still on ground's.
	if v.Done || atc != traffic.PosTower || v.State != traffic.TaxiTaxiing.String() || !v.OnGround || !it.dep.RunwayNext() {
		return false, false
	}
	l := layout(it.ICAO)
	if l == nil {
		return false, false
	}
	r, ok := runwayOf(l, v.Runway)
	if !ok {
		return false, false
	}
	// Closed, or held for the user aircraft: not onto it.
	shut := nav.RunwayLimits{Closed: t.cc.core.shut(it.ICAO)}
	if slices.ContainsFunc(strings.Split(r.Name(), "/"), shut.ClosedEnd) || t.cc.core.playerBlocks(it.ICAO, r.Name(), l) {
		return false, false
	}
	k := it.ICAO + " " + r.Name()
	t.mu.Lock()
	rc := t.ctl[k]
	users := append([]traffic.RunwayUser(nil), t.users[k]...)
	t.mu.Unlock()
	if rc == nil {
		rc = traffic.NewRunwayController(traffic.RunwayControllerOptions{}) // nobody on it yet
	} else {
		rc = rc.Clone()
	}
	list := slices.DeleteFunc(users, func(u traffic.RunwayUser) bool { return u.Callsign == v.Tail })
	// As at its holding point, ours to clear, its taxi there and its line-up
	// still to come.
	taxi := time.Duration(v.TaxiRemainingM / (traffic.TaxiSpeedKts * 1852 / 3600) * float64(time.Second))
	u := traffic.RunwayUser{Callsign: v.Tail, Wake: traffic.WakeFor(v.Model), Route: v.Procedure, End: v.Runway,
		Phase: traffic.RunwayHoldingShort, ClimbKts: climbKts(v.Model), LineUpIn: taxi + rollThroughLineUp}
	d := rc.Decide(now, append(list, u))
	takeoff := slices.Contains(d.Takeoff, v.Tail)
	if takeoff {
		// A runway crossing this one in use: not through it.
		if x, _ := t.crossingBusy(it.ICAO, r, l.Runways); x != "" {
			takeoff = false
		}
	}
	return takeoff, d.LineUpBehind[v.Tail] != ""
}

// behindOf: departure tail is told to line up behind (lineUpBehind).
func (t *towers) behindOf(tail string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.behind[tail]
	return ok
}
