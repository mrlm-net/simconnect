//go:build windows
// +build windows

package traffic

import "time"

// Enroute traffic (#369): arrivals appear en route on their flight plan
// EnrouteLead before they would appear at the STAR entry, flown by MSFS AI,
// and are handed to the arrival controller at the entry (the Spawner does
// the handover and reports FlightApproaching); overflights cross the area
// between airports outside it; departures fly on after their SID. Airborne
// aircraft of ours leave with the area: once the picture no longer sees
// them, they are removed.

// start is when f is spawned, and in which stage ("enroute" or "").
func (m *TrafficManager) start(f *ManagedFlight, now time.Time) (time.Time, string) {
	o := m.opts
	switch {
	case f.Departure():
		return f.STD.Add(-o.DepartureLead), ""
	case f.Overflight():
		return f.Enter, "enroute"
	}
	direct := f.STA.Add(-m.arrivalLead(&f.Flight))
	if f.Rules == "VFR" {
		return direct, "" // near the airport, joining the circuit
	}
	if o.EnrouteLead > 0 && !f.noEnroute && now.Before(direct.Add(-2*time.Minute)) {
		return direct.Add(-o.EnrouteLead), "enroute"
	}
	return direct, ""
}

// leaving removes an airborne aircraft of ours once it left the area: seen
// in the picture, then not for LeftAfter. Without a picture a departed
// aircraft goes after RemoveDepartedAfter, an overflight after its exit.
func (m *TrafficManager) leaving(f *ManagedFlight, now time.Time, remove *[]ManagedFlight) {
	o := m.opts
	gone := func(why string) {
		f.Note = why
		m.set(f, FlightDone, now)
		*remove = append(*remove, *f)
	}
	if p := o.Picture; p != nil && f.ObjectID != 0 {
		if p.has(f.ObjectID) {
			f.seenAt = now
		} else if !f.seenAt.IsZero() && now.Sub(f.seenAt) > o.LeftAfter {
			gone("left the area")
			return
		}
	}
	switch {
	case f.Status == FlightDeparted && now.Sub(f.Since) >= o.RemoveDepartedAfter:
		gone("")
	case f.Overflight() && now.After(f.Exit.Add(10*time.Minute)):
		gone("left the area")
	}
}

// extendOverflights asks the Overflights source for the hours up to
// now+Horizon not asked yet, while spawning is on (the area may not be
// known before).
func (m *TrafficManager) extendOverflights(now time.Time) {
	if m.opts.Overflights == nil || !m.enabled {
		return
	}
	from := now.Truncate(time.Hour)
	if m.overUntil.After(from) {
		from = m.overUntil
	}
	for ; from.Before(now.Add(m.opts.Horizon)); from = from.Add(time.Hour) {
		for _, fl := range m.opts.Overflights(from, from.Add(time.Hour)) {
			if now.After(fl.Exit.Add(-5 * time.Minute)) {
				continue // already through
			}
			mf := &ManagedFlight{Flight: fl, Kind: "overflight"}
			if _, ok := m.flights[mf.key()]; !ok {
				m.flights[mf.key()] = mf
				m.emit(EventAdded, mf, time.Time{}, "")
			}
		}
		m.overUntil = from.Add(time.Hour)
	}
}

// Attach tells the manager which aircraft flies a flight (its SimConnect
// object ID), so it can follow it in the picture.
func (m *TrafficManager) Attach(callsign string, objectID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f := m.find(callsign); f != nil {
		f.ObjectID = objectID
	}
}

// has reports whether the picture tracks an object.
func (p *TrafficPicture) has(objectID uint32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.aircraft[objectID]
	return ok
}

// arrivalLead is how long before its STA arrival f appears: VFRLead for a
// VFR flight, ArrivalLead for the rest.
func (m *TrafficManager) arrivalLead(f *Flight) time.Duration {
	if f.Rules == "VFR" {
		return m.opts.VFRLead
	}
	return m.opts.ArrivalLead
}
