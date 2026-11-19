package world

import (
	"slices"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The overflight and transit traffic's settings (#1483–#1487), each off
// when not given: the zero value keeps the traffic over the airport as it
// was. Part of ScheduleSettings and of POST and GET /api/schedule; a
// setting not given keeps what is set.
type OverflightTraffic struct {
	// OverflightFlows: the overflights by the area's streams and volume
	// from the flows table (traffic.overflightFlows, #1483), else the
	// generic fallback; overflightsPerHour, when set, is the area's peak
	// volume instead.
	OverflightFlows *bool `json:"overflightFlows,omitempty"`
	// OverflightRoutes: the overflights cross by the airspace's route
	// structure (nav.routeStructure, #1484): direct in free route
	// airspace, along the airways elsewhere (with no entry: the airways),
	// each at a strategic offset of 0 to 2 NM right of its route.
	OverflightRoutes *bool `json:"overflightRoutes,omitempty"`
	// Transits: the departures and arrivals of the airports in and near the
	// area cross it on their climb or descent (traffic.Transits, #1485),
	// beside the overflights.
	Transits *bool `json:"transits,omitempty"`
	// CrossingKeepClear: the overflights and transits keep clear of the
	// terminal airspace, the focus airports' and (with the real airspaces)
	// the other airports' in the area: above each layer inside its
	// outline, or round it (#1486, overflight_keepclear.go).
	CrossingKeepClear *bool `json:"crossingKeepClear,omitempty"`
	// OverflightRadio: the overflights and transits talk to the centre
	// station whose sector holds them: a check-in on entering, a hand-off
	// on leaving, on the frequency the user has tuned only (#1487,
	// overflight_radio.go). CentreCallsPerMinute caps the transmissions
	// on a centre frequency a minute, readbacks counted (0: 6).
	OverflightRadio      *bool `json:"overflightRadio,omitempty"`
	CentreCallsPerMinute *int  `json:"centreCallsPerMinute,omitempty"`
	// OverflightArea and OverflightPeakPerHour (GET only, with the flows
	// on): the flows table's area the overflights cross ("" the generic
	// fallback) and its peak volume at density 1.
	OverflightArea        string  `json:"overflightArea,omitempty"`
	OverflightPeakPerHour float64 `json:"overflightPeakPerHour,omitempty"`
	// TransitFlights (GET only): the call signs of the transits planned.
	TransitFlights []string `json:"transitFlights,omitempty"`
}

// overflightTraffic is what is set of OverflightTraffic. s.mu held.
type overflightTraffic struct {
	flows, routes, transits, keepClear, radio bool
	callsPerMinute                            int
	// crossing: the overflights' and transits' ends by call sign.
	crossing map[string][2]string
	// cr: the radio (overflight_radio.go).
	cr centreRadio
	// transitCS: the transits' call signs (overflight_transit.go).
	transitCS map[string]bool
}

// setOverflightTraffic takes the settings given.
func (s *scheduler) setOverflightTraffic(o OverflightTraffic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := func(p *bool, to *bool) {
		if p != nil {
			*to = *p
		}
	}
	set(o.OverflightFlows, &s.over.flows)
	set(o.OverflightRoutes, &s.over.routes)
	set(o.Transits, &s.over.transits)
	set(o.CrossingKeepClear, &s.over.keepClear)
	set(o.OverflightRadio, &s.over.radio)
	if o.CentreCallsPerMinute != nil {
		s.over.callsPerMinute = max(0, *o.CentreCallsPerMinute)
	}
}

// overflightTrafficView is what is set, for GET /api/schedule.
func (s *scheduler) overflightTrafficView() OverflightTraffic {
	s.mu.Lock()
	o, perHour, focus := s.over, s.overPerHour, s.focus
	s.mu.Unlock()
	v := OverflightTraffic{OverflightFlows: &o.flows, OverflightRoutes: &o.routes, Transits: &o.transits, CrossingKeepClear: &o.keepClear, OverflightRadio: &o.radio, CentreCallsPerMinute: &o.callsPerMinute}
	s.mu.Lock()
	for cs := range s.over.transitCS {
		v.TransitFlights = append(v.TransitFlights, cs)
	}
	s.mu.Unlock()
	slices.Sort(v.TransitFlights)
	if c, ok := s.cc.world.Centre(); ok && o.flows {
		v.OverflightPeakPerHour = perHour
		if a, ok := traffic.OverflightAreaAt(c); ok {
			v.OverflightArea = a.String()
			if perHour <= 0 {
				v.OverflightPeakPerHour = a.PeakPerHour
			}
		} else if perHour <= 0 {
			v.OverflightPeakPerHour = traffic.FallbackOverflightsPerHour(s.config(), c, overflightRadiusNM, focus)
		}
	}
	return v
}

// overflightSettings is what is set.
func (s *scheduler) overflightSettings() overflightTraffic {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.over
}

// crossingTraffic is the traffic crossing the area in an hour: the
// overflights (traffic.Overflights with o) and, when on, the transits.
func (s *scheduler) crossingTraffic(o traffic.OverflightOptions, from, to time.Time) []traffic.Flight {
	out := traffic.Overflights(s.config(), o, from, to)
	if s.overflightSettings().transits {
		used := map[string]bool{}
		for _, f := range out {
			used[f.Callsign] = true
		}
		out = append(out, s.transits(o, used, from, to)...)
	}
	s.mu.Lock()
	if s.over.crossing == nil {
		s.over.crossing = map[string][2]string{}
	}
	for _, f := range out {
		s.over.crossing[f.Callsign] = [2]string{f.Origin, f.Destination}
	}
	s.mu.Unlock()
	return out
}
