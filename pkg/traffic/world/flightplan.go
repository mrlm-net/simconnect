package world

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
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
	// adopt: an en route arrival handed over flies on as this object (#643).
	adopt uint32
	// runway: the arrival runway route was picked for, without a plan (a
	// real aircraft's join, #841).
	runway string
}

// planFor loads the other airport (waiting for the simulator) and plans
// the flight between it and g's airport for r.
func planFor(ctx context.Context, st *state, g *airport.Graph, r SpawnRequest) (*planned, error) {
	other := strings.ToUpper(strings.TrimSpace(r.Other))
	dep, arr, depRwy, arrRwy := g.Layout.ICAO, other, r.Runway, ""
	if r.Kind != "departure" {
		dep, arr, depRwy, arrRwy = other, g.Layout.ICAO, "", r.Runway
	}
	fp, err := planBetween(ctx, st, dep, arr, depRwy, arrRwy, typeOf(r.Model), g.Layout.ICAO)
	if err != nil {
		return nil, err
	}
	return plannedFrom(fp, r.Kind)
}

// planBetween plans a flight from dep to arr (runways "" chosen by the
// plan, #369), loading in full (waiting for the simulator) only local, the
// airport whose procedures it flies here; the other end is its layout if
// loaded already, else where it is (the worldwide list): loading it,
// LFPG's or KJFK's whole airport for a flight to or past LKPR, stood
// every aircraft still meanwhile (#898). local "": both loaded.
func planBetween(ctx context.Context, st *state, dep, arr, depRwy, arrRwy, typ, local string) (*nav.FlightPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	info := func(icao string) (nav.AirportInfo, error) {
		if local != "" && !strings.EqualFold(icao, local) {
			if _, ok := st.cache.Layout(icao); !ok {
				st.mu.Lock()
				r, ok := st.airportRefs[strings.ToUpper(icao)]
				st.mu.Unlock()
				if ok {
					return nav.AirportInfo{ICAO: strings.ToUpper(r.ICAO), Position: r.Position, ElevationM: r.AltM}, nil
				}
			}
		}
		l, err := st.load(ctx, icao, false, st.requests)
		if err != nil {
			return nav.AirportInfo{}, fmt.Errorf("loading %s: %w", icao, err)
		}
		a := nav.AirportInfo{ICAO: l.ICAO, Name: l.Name, Layout: l}
		st.mu.Lock()
		if p, ok := st.procedures[l.ICAO]; ok {
			a.Procedures = &p
		}
		st.mu.Unlock()
		return a, nil
	}
	d, err := info(dep)
	if err != nil {
		return nil, err
	}
	a, err := info(arr)
	if err != nil {
		return nil, err
	}
	st.mu.Lock()
	graph := st.airways
	st.mu.Unlock()
	return nav.Plan(nav.FlightPlanRequest{Type: typ, Departure: d, Arrival: a, DepartureRunway: depRwy, ArrivalRunway: arrRwy}, graph)
}

// PlanRequest asks PlanFlight for a flight: from Departure to Arrival
// (ICAO codes) for aircraft Type (ICAO designator; "" an A320's
// performance). The runways are ends ("24", "06L"); "" lets the plan
// choose them from the airports' weather.
type PlanRequest struct {
	Departure       string `json:"departure"`
	Arrival         string `json:"arrival"`
	Type            string `json:"type,omitempty"`
	DepartureRunway string `json:"departureRunway,omitempty"`
	ArrivalRunway   string `json:"arrivalRunway,omitempty"`
}

// PlanFlight plans an IFR flight the way the World plans its own traffic's
// (nav.Plan): both airports loaded from the simulator (waiting for it, at
// most 30 s; an error without a connection), their SIDs, STARs and
// approaches, the airways the World knows between (its -airways graph and
// those read around loaded airports; direct where it has none). The plan's
// PLN gives a .pln file for the simulator (FlightPlanLoad). It must not be
// called from a Do function or the World's hooks: it waits on the
// connection loop.
func (w *World) PlanFlight(ctx context.Context, r PlanRequest) (*nav.FlightPlan, error) {
	dep, arr := strings.ToUpper(strings.TrimSpace(r.Departure)), strings.ToUpper(strings.TrimSpace(r.Arrival))
	if dep == "" || arr == "" {
		return nil, fmt.Errorf("world: plan: departure and arrival needed")
	}
	return planBetween(ctx, w.st, dep, arr, r.DepartureRunway, r.ArrivalRunway, r.Type, "")
}

// planOverflight plans an overflight from dep to arr from where the two
// airports are (the worldwide list), without loading them: it flies none
// of their runways, procedures or taxiways. One not in the list (not asked
// yet): planned as any flight (planBetween).
func planOverflight(ctx context.Context, st *state, dep, arr, typ string) (*nav.FlightPlan, error) {
	st.mu.Lock()
	d, okD := st.airportRefs[strings.ToUpper(dep)]
	a, okA := st.airportRefs[strings.ToUpper(arr)]
	graph := st.airways
	st.mu.Unlock()
	if !okD || !okA {
		return planBetween(ctx, st, dep, arr, "", "", typ, "")
	}
	info := func(r traffic.AirportRef) nav.AirportInfo {
		return nav.AirportInfo{ICAO: strings.ToUpper(r.ICAO), Position: r.Position, ElevationM: r.AltM}
	}
	return nav.Plan(nav.FlightPlanRequest{Type: typ, Departure: info(d), Arrival: info(a)}, graph)
}

// plannedFrom is what a spawn flies of a plan: a departure the whole
// flight at the planned levels, an arrival its STAR and approach (it
// appears at the STAR entry).
func plannedFrom(fp *nav.FlightPlan, kind string) (*planned, error) {
	out := &planned{plan: fp}
	for _, w := range fp.Waypoints {
		if w.Kind == nav.PointRunway || w.Kind == nav.PointAirport || w.Kind == nav.PointProfile {
			continue
		}
		n := airport.NavPoint{Ident: w.Ident, Kind: w.Kind, Position: w.Position, IAF: w.IAF, FAF: w.FAF, MAP: w.MAP,
			Vectors: w.Vectors, FlyOver: w.FlyOver, SpeedMax: w.SpeedMaxKts}
		if kind == "departure" {
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
	if kind == "departure" {
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
