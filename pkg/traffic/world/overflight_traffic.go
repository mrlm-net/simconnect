package world

import (
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
	// OverflightArea and OverflightPeakPerHour (GET only, with the flows
	// on): the flows table's area the overflights cross ("" the generic
	// fallback) and its peak volume at density 1.
	OverflightArea        string  `json:"overflightArea,omitempty"`
	OverflightPeakPerHour float64 `json:"overflightPeakPerHour,omitempty"`
}

// overflightTraffic is what is set of OverflightTraffic. s.mu held.
type overflightTraffic struct {
	flows bool
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
}

// overflightTrafficView is what is set, for GET /api/schedule.
func (s *scheduler) overflightTrafficView() OverflightTraffic {
	s.mu.Lock()
	o, perHour, focus := s.over, s.overPerHour, s.focus
	s.mu.Unlock()
	v := OverflightTraffic{OverflightFlows: &o.flows}
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
// overflights (traffic.Overflights with o).
func (s *scheduler) crossingTraffic(o traffic.OverflightOptions, from, to time.Time) []traffic.Flight {
	return traffic.Overflights(s.config(), o, from, to)
}
