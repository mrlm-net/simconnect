package world

import (
	"sort"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// standNeed is a departure due soon that needs a stand of its own: its half
// span and airline.
type standNeed struct {
	half    float64
	airline string
}

// dueStandNeeds are the departures at icao due by now plus DepartureLead
// and DepartureLate that are still to come to a stand of their own: not
// spawned, no turnaround (its arrival's stand), no night stop or start
// stand (#1677).
func dueStandNeeds(flights []traffic.ManagedFlight, icao, except string, now time.Time, o traffic.ManagerOptions) []standNeed {
	var out []standNeed
	until := now.Add(o.DepartureLead + o.DepartureLate)
	for _, d := range flights {
		if !d.Departure() || d.Airport != icao || d.Callsign == except || d.Status != traffic.FlightScheduled ||
			d.TurnFrom != "" || d.NightStop || d.StartStand || d.Observed != nil || d.STD.After(until) {
			continue
		}
		out = append(out, standNeed{half: traffic.ProfileFor(d.Type).Motion.SpanMeters / 2, airline: airlineOf(d.Callsign)})
	}
	return out
}

// keepsStandsFor reports whether the departures due soon (needs) each
// still find a free suitable stand of their own, largest first, each on the
// smallest that fits: a start stand or a night stop must leave them those
// (#1677: stands filled 2-3 h ahead, the departures due now cancelled for
// no free stand).
func keepsStandsFor(alloc *traffic.StandAllocator, l *airport.Layout, needs []standNeed) bool {
	needs = append([]standNeed(nil), needs...)
	sort.SliceStable(needs, func(i, j int) bool { return needs[i].half > needs[j].half })
	used := map[int]bool{}
	for _, n := range needs {
		best := -1
		for _, i := range l.SuitableStands(n.half) {
			p := l.Parking[i]
			if used[i] || (len(p.Airlines) > 0 && n.airline != "" && !p.ServesAirline(n.airline)) || !alloc.Free(i, n.half) {
				continue
			}
			if best < 0 || p.Radius < l.Parking[best].Radius {
				best = i
			}
		}
		if best < 0 {
			return false
		}
		used[best] = true
	}
	return true
}
