package world

import (
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Crossing runways depend on each other: no take-off or landing on one
// while one of ours uses a runway crossing it and has not passed where the
// two cross — a departure cleared, rolling or climbing out, an arrival on a
// short final, on its landing roll or gone around (live, LKPR after a
// change back to 24: TVS833 rolled on 30 behind TVS652 off 24, CSA522
// cleared for 24 with TVS833 on its roll).
const (
	// crossArrivalNM: an arrival this close in on a crossing runway holds
	// take-offs (it lands before they are clear of the crossing).
	crossArrivalNM = 3.0
	// crossPastM: this far beyond the crossing runway's centerline, it is
	// clear of it.
	crossPastM = 300.0
)

// crossUse is one of ours using a runway (end) that may cross others.
type crossUse struct {
	icao, end, tail string
	pos             airport.LatLon
	// arrival on a final (not yet over the runway): it holds take-offs on
	// a crossing runway, not landings.
	final bool
}

// crossUseOf is u as a crossUse, false when it does not use its runway in
// a way a crossing runway waits for. takeoff: its take-off is cleared.
func crossUseOf(icao string, u traffic.RunwayUser, pos airport.LatLon, departure, takeoff bool) (crossUse, bool) {
	c := crossUse{icao: icao, end: u.End, tail: u.Callsign, pos: pos}
	switch {
	case u.Crossing || u.End == "":
		return c, false
	case departure && (u.Phase == traffic.RunwayRolling || u.Phase == traffic.RunwayAirborne):
	case departure && u.Phase == traffic.RunwayLinedUp && takeoff:
	case !departure && (u.Phase == traffic.RunwayRolling || u.GoingAround):
	case !departure && u.Phase == traffic.RunwayFinal && u.DistanceNM <= crossArrivalNM:
		c.final = true
	default:
		return c, false
	}
	return c, true
}

// crossAlong is how far along end's runway, from its threshold, the
// centerline of runway r crosses it; false when it does not.
func crossAlong(l *airport.Layout, end string, r airport.Runway) (float64, bool) {
	own, ok := runwayOf(l, end)
	if !ok || own.Name() == r.Name() {
		return 0, false
	}
	from, to := own.Primary.Threshold, own.Secondary.Threshold
	if own.Secondary.Name == end {
		from, to = to, from
	}
	a, b := r.Primary.Threshold, r.Secondary.Threshold
	ca := calc.CrossTrackMeters(from.Lat, from.Lon, to.Lat, to.Lon, a.Lat, a.Lon)
	cb := calc.CrossTrackMeters(from.Lat, from.Lon, to.Lat, to.Lon, b.Lat, b.Lon)
	if ca*cb > 0 || ca == cb {
		return 0, false // both ends on one side: no crossing
	}
	k := ca / (ca - cb)
	aa := calc.AlongTrackMeters(from.Lat, from.Lon, to.Lat, to.Lon, a.Lat, a.Lon)
	ab := calc.AlongTrackMeters(from.Lat, from.Lon, to.Lat, to.Lon, b.Lat, b.Lon)
	x := aa + k*(ab-aa)
	if x < -crossPastM || x > own.Length+crossPastM {
		return 0, false // the lines cross off the runway
	}
	return x, true
}

// crossBlocked names one of uses on a runway crossing rwy at l, not past
// the crossing yet, that a take-off (landing: false, a landing) on rwy
// waits for; "" none.
func crossBlocked(l *airport.Layout, rwy string, uses []crossUse, takeoff bool) string {
	r, ok := runwayOf(l, rwy)
	if !ok {
		return ""
	}
	for _, u := range uses {
		if u.icao != l.ICAO || u.final && !takeoff {
			continue
		}
		x, ok := crossAlong(l, u.end, r)
		if !ok {
			continue
		}
		own, _ := runwayOf(l, u.end)
		from, to := own.Primary.Threshold, own.Secondary.Threshold
		if own.Secondary.Name == u.end {
			from, to = to, from
		}
		if calc.AlongTrackMeters(from.Lat, from.Lon, to.Lat, to.Lon, u.pos.Lat, u.pos.Lon) < x+crossPastM {
			return u.tail
		}
	}
	return ""
}

// crossFilter is tails (cleared to take off, takeoff, or to land on rwy)
// less those a crossing runway's use holds, said waiting in c; those left
// are added to cross for the runways crossing rwy.
func (t *towers) crossFilter(l *airport.Layout, rwy string, tails []string, c *traffic.RunwayClearances, cross *[]crossUse, pos map[string]airport.LatLon, ours map[string]*controlled, takeoff bool) []string {
	var keep []string
	for _, tail := range tails {
		if by := crossBlocked(l, rwy, *cross, takeoff); by != "" {
			if c.Waiting == nil {
				c.Waiting = map[string]string{}
			}
			c.Waiting[tail] = by + " on the crossing runway"
			continue
		}
		keep = append(keep, tail)
		if it := ours[tail]; it != nil {
			it.mu.Lock()
			end := it.view.Runway
			it.mu.Unlock()
			*cross = append(*cross, crossUse{icao: l.ICAO, end: end, tail: tail, pos: pos[tail]})
		}
	}
	return keep
}

// givenTakeoff reports whether tail has been cleared for take-off.
func (t *towers) givenTakeoff(tail string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.given[tail+" takeoff"]
}
