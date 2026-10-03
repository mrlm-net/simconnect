//go:build windows
// +build windows

package airport

// Tracker follows one aircraft from airport to airport: Locate on the
// ground, and in the air what Locate alone cannot tell where corridors
// overlap — climbing out it stays with the airport it took off from,
// approaching it keeps to its destination (SetDestination) or to the
// approach it was already flying. Feed it every position (a few a second
// or once a second; the order matters, not the rate).
type Tracker struct {
	from string   // the airport it was last on the ground at
	dest string   // the airport it is flying to ("" unknown)
	last Location // the last airborne location, for sticking to it
	air  bool     // last was in the air
}

// NewTracker returns a Tracker that knows nothing yet.
func NewTracker() *Tracker { return &Tracker{} }

// SetDestination tells the tracker where the aircraft is going (its flight
// plan's destination, "" none): an approach there wins over a neighbour's
// that overlaps it.
func (t *Tracker) SetDestination(icao string) { t.dest = icao }

// From is the airport the aircraft was last on the ground at ("" none yet).
func (t *Tracker) From() string { return t.from }

// Update locates q among layouts (the airports around it) with what the
// tracker knows, and remembers the answer.
func (t *Tracker) Update(q LocateQuery, layouts []*Layout) (Location, bool) {
	if q.OnGround {
		t.air = false
		loc, ok := Locate(q, layouts)
		if ok && loc.Meters <= locateGroundOnMeters {
			t.from = loc.ICAO
		}
		return loc, ok
	}
	only := func(icao string) []*Layout {
		for _, l := range layouts {
			if l != nil && l.ICAO == icao {
				return []*Layout{l}
			}
		}
		return nil
	}
	pick := func(icao string, feat LocateFeature) (Location, bool) {
		if icao == "" {
			return Location{}, false
		}
		ls := only(icao)
		if ls == nil {
			return Location{}, false
		}
		loc, ok := locateAir(q, ls, feat)
		if !ok {
			return Location{}, false
		}
		return loc, true
	}
	var loc Location
	ok := false
	// Climbing out of where it took off: that airport's departure.
	if q.VerticalFpm >= -locateLevelFpm {
		loc, ok = pick(t.from, OnDeparture)
	}
	// Going to its destination: that airport's approach.
	if !ok && q.VerticalFpm <= locateLevelFpm {
		loc, ok = pick(t.dest, OnApproach)
	}
	// Still in the corridor it was in.
	if !ok && t.air && t.last.Feature != NearAirport {
		loc, ok = pick(t.last.ICAO, t.last.Feature)
	}
	if !ok {
		loc, ok = Locate(q, layouts)
	}
	// Off every corridor but still near the airport it left: that one.
	if ok && loc.Feature == NearAirport && t.from != "" && loc.ICAO != t.from {
		if ls := only(t.from); ls != nil {
			if s := surface(ls[0], q.Position); s.Meters <= LocateNearMeters {
				loc = Location{ICAO: t.from, Feature: NearAirport, Meters: s.Meters}
			}
		}
	}
	if ok {
		t.last, t.air = loc, true
	}
	return loc, ok
}

// locateGroundOnMeters: on the ground within this of an airport's surface,
// the aircraft is at it (a departure's origin).
const locateGroundOnMeters = 100.0
