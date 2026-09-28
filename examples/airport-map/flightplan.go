//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Traffic with generated flight plans (#331): a departure given a
// destination flies the planned SID, airways and levels after the take-off;
// an arrival given an origin flies the STAR and approach the plan from
// there chooses, appearing at the STAR's entry.

// planned is a resolved flight plan for a spawn.
type planned struct {
	route  []airport.NavPoint
	name   string // SID or STAR
	expect string // arrival: approach type
	plan   *nav.FlightPlan
}

// planFor loads the other airport (waiting for the simulator) and plans
// the flight between it and g's airport for r.
func planFor(ctx context.Context, st *state, g *airport.Graph, r SpawnRequest) (*planned, error) {
	other := strings.ToUpper(strings.TrimSpace(r.Other))
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ol, err := st.load(ctx, other, false, st.requests)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", other, err)
	}
	info := func(l *airport.Layout) nav.AirportInfo {
		a := nav.AirportInfo{ICAO: l.ICAO, Name: l.Name, Layout: l}
		st.mu.Lock()
		if p, ok := st.procedures[l.ICAO]; ok {
			a.Procedures = &p
		}
		st.mu.Unlock()
		return a
	}
	here, there := info(g.Layout), info(ol)
	req := nav.FlightPlanRequest{Type: typeOf(r.Model)}
	if r.Kind == "departure" {
		req.Departure, req.Arrival, req.DepartureRunway = here, there, r.Runway
	} else {
		req.Departure, req.Arrival, req.ArrivalRunway = there, here, r.Runway
	}
	st.mu.Lock()
	graph := st.airways
	st.mu.Unlock()
	fp, err := nav.Plan(req, graph)
	if err != nil {
		return nil, err
	}
	out := &planned{plan: fp}
	for _, w := range fp.Waypoints {
		if w.Kind == nav.PointRunway || w.Kind == nav.PointAirport || w.Kind == nav.PointProfile {
			continue
		}
		n := airport.NavPoint{Ident: w.Ident, Kind: w.Kind, Position: w.Position, IAF: w.IAF, FAF: w.FAF, MAP: w.MAP,
			Vectors: w.Vectors, FlyOver: w.FlyOver, SpeedMax: w.SpeedMaxKts}
		if r.Kind == "departure" {
			// The whole flight: planned levels, held exactly.
			n.AltMin, n.AltMax = w.AltFt*0.3048, w.AltFt*0.3048
			out.route = append(out.route, n)
			continue
		}
		if w.Phase != nav.PhaseSTAR && w.Phase != nav.PhaseApproach {
			continue // an arrival appears at the STAR entry
		}
		n.AltMin, n.AltMax = w.AltMinFt*0.3048, w.AltMaxFt*0.3048
		out.route = append(out.route, n)
	}
	if len(out.route) == 0 {
		return nil, fmt.Errorf("the plan %s has no route to fly", fp.Route)
	}
	if r.Kind == "departure" {
		out.name = fp.SID
	} else {
		out.name = fp.STAR
		out.expect, _, _ = strings.Cut(fp.Approach, " ")
		if out.name == "" {
			out.name = "direct"
		}
	}
	return out, nil
}

// typeOf guesses the ICAO type designator from a model title ("FSLTL A320
// Air France SL" → A320); "" when none is recognised.
func typeOf(model string) string {
	for _, f := range strings.Fields(strings.ToUpper(model)) {
		if nav.PerformanceFor(f).Type == f {
			return f
		}
	}
	return ""
}
