package world

import (
	"context"
	"fmt"
	"math"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Arrivals on their way in when traffic begins (StartAirborne, stage
// "midway"): each appears on its STAR where it would be by now, so the
// first landings come minutes after the start, not ArrivalLead after (the
// user, live LKPR 2026-11-11: "arrivals are also spawned only at start of
// star ... not have long slot at start").
const (
	// midwayKts: the ground speed an arrival averages over its STAR and
	// approach, slowing from 250 kt to its landing speed.
	midwayKts = 220.0
	// midwayFtPerNM: its height above the field by the way still to fly,
	// a 3° path, when its plan's level is higher.
	midwayFtPerNM = 318.0
	// midwayFinalNM: without an initial approach fix in its plan, it
	// appears no nearer the runway than this along the plan.
	midwayFinalNM = 15.0
)

// midwayAlong is how far along fp (NM) an arrival with leftNM still to fly
// appears: on its STAR, from the STAR entry up to its initial approach fix
// (no straight-in from the start), false when the plan has no STAR or
// approach.
func midwayAlong(fp *nav.FlightPlan, leftNM float64) (float64, bool) {
	entry, iaf := -1.0, -1.0
	for _, w := range fp.Waypoints {
		if w.Phase != nav.PhaseSTAR && w.Phase != nav.PhaseApproach {
			continue
		}
		if entry < 0 {
			entry = w.DistanceNM
		}
		if w.IAF && iaf < 0 {
			iaf = w.DistanceNM
		}
	}
	if entry < 0 {
		return 0, false
	}
	if iaf < entry {
		iaf = math.Max(entry, fp.DistanceNM-midwayFinalNM)
	}
	return math.Min(iaf, math.Max(entry, fp.DistanceNM-leftNM)), true
}

// spawnMidway puts arrival f on its STAR where it would be by now: the
// way it still has to fly by its STA (or the landing flow's estimate) at
// midwayKts back from the runway,
// along its plan (midwayAlong), at its plan's level or a 3° path below
// it; MSFS AI flies it to the next point of the STAR, where it is handed
// to the arrival controller as a real arrival is (spawnArrivalAlong).
func (s *scheduler) spawnMidway(f traffic.ManagedFlight) error {
	cc, st := s.cc, s.st
	g, err := st.cache.Graph(f.Airport)
	if err != nil {
		return err
	}
	rwy := cc.pickRunway(g, true, -1)
	fp, err := planBetween(context.Background(), st, f.Origin, f.Destination, "", rwy, f.Type, f.Destination, variantOf(f.Callsign))
	if err != nil {
		return fmt.Errorf("flight plan %s → %s: %w", f.Origin, f.Destination, err)
	}
	// Due by its STA, or later where the landing flow moved it.
	due := f.STA
	if f.Estimated.After(due) {
		due = f.Estimated
	}
	left := due.Sub(cc.clock.Now()).Hours() * midwayKts
	d, ok := midwayAlong(fp, left)
	if !ok {
		return fmt.Errorf("its plan %s has no STAR", fp.Route)
	}
	pos, alt, _ := fp.PositionAt(d)
	field := g.Layout.Altitude / 0.3048
	if path := field + (fp.DistanceNM-d)*midwayFtPerNM; alt <= 0 || alt > path {
		alt = path
	}
	if who := s.nearPoint(pos, alt); who != "" {
		return fmt.Errorf("%w: %s near its position", traffic.ErrSpawnBlocked, who)
	}
	kts := fp.Performance.CruiseTASKts
	if kts < 200 {
		kts = 420
	}
	cc.log.printf("%-6s schedule: on its way in from the start: %.0f NM out on %s, %.0f ft, due %s", f.Callsign, fp.DistanceNM-d, fp.STAR, alt, due.Format("15:04"))
	return s.spawnArrivalAlong(f, pos, alt, kts, nil)
}
