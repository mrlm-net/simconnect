package traffic

import "time"

// Real-world traffic in the manager (#841): flights of Observed aircraft
// are added as any (Add) and spawned at once (start); these keep them in
// step with later sightings.

// Flight returns the managed flight of kind ("arrival", "departure") and
// call sign.
func (m *TrafficManager) Flight(kind, callsign string) (ManagedFlight, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f := m.flights[kind+" "+callsign]; f != nil {
		return *f, true
	}
	return ManagedFlight{}, false
}

// Observe takes a later sighting of a real flight: one not spawned yet
// takes it all (it appears where the aircraft is then), one already
// flying only its registration, origin and destination (the engine flies
// it). False when there is no such flight.
func (m *TrafficManager) Observe(kind, callsign string, s Sighting, origin, destination string) bool {
	m.mu.Lock()
	defer m.unlock()
	f := m.flights[kind+" "+callsign]
	if f == nil || f.Observed == nil || f.Status == FlightDone || f.Status == FlightCancelled {
		return false
	}
	if f.Status == FlightScheduled {
		if len(s.Route) == 0 {
			s.Route = f.Observed.Route // a sighting without one keeps the route given
		}
		f.Observed = &s
	} else {
		o := *f.Observed
		o.Registration = s.Registration
		f.Observed = &o
	}
	if origin != "" && f.Arrival() {
		f.Origin = origin
	}
	if destination != "" && f.Departure() {
		f.Destination = destination
	}
	return true
}

// Retime moves a departure's STD (a real aircraft's push): the Spawner
// re-times its controller. False when there is no such departure still
// on its stand.
func (m *TrafficManager) Retime(callsign string, std time.Time) bool {
	m.mu.Lock()
	defer m.unlock()
	f := m.flights["departure "+callsign]
	if f == nil || f.Status > FlightBoarding {
		return false
	}
	f.STD = std
	return true
}

// Turn makes departure the next flight of arrival's aircraft (a real one
// seen again on the ground): the departure adopts it once it has parked.
func (m *TrafficManager) Turn(arrival, departure string) bool {
	m.mu.Lock()
	defer m.unlock()
	a, d := m.flights["arrival "+arrival], m.flights["departure "+departure]
	if a == nil || d == nil || d.Status != FlightScheduled || a.TurnTo != "" || d.TurnFrom != "" {
		return false
	}
	a.TurnTo, d.TurnFrom = d.Callsign, a.Callsign
	m.emit(EventTurnaround, d, time.Time{}, a.Callsign+" → "+d.Callsign)
	return true
}

// Drop ends a flight the feed no longer sees: not spawned yet, on its stand
// before the push or parked, it goes now; in progress it plays out (an
// arrival lands and parks, then goes; a departure leaves), never yanked
// off the final.
func (m *TrafficManager) Drop(kind, callsign string, now time.Time) {
	m.mu.Lock()
	f := m.flights[kind+" "+callsign]
	if f == nil || f.Status == FlightDone || f.Status == FlightCancelled {
		m.unlock()
		return
	}
	switch f.Status {
	case FlightScheduled, FlightBoarding, FlightParked:
		// This flight, by its kind: by call sign alone the other one of a
		// pair could go (#92).
		var remove []ManagedFlight
		if f.Status.active() {
			remove = append(remove, *f)
		}
		m.set(f, FlightDone, now)
		m.unpair(f)
		m.unlock()
		for _, r := range remove {
			m.removeNow(r)
		}
		return
	}
	f.dropped = true
	m.unlock()
}

// Replan asks the Source again for the hours ahead: after its flights
// were off (real-world traffic, #841), the hours asked meanwhile were
// empty.
func (m *TrafficManager) Replan() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.until = time.Time{}
}
