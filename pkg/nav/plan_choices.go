package nav

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The player's choices in a plan (FlightPlanRequest): each followed when
// it fits, else chosen as without it and said in FlightPlan.Notes.

// note adds a note to the plan.
func (fp *FlightPlan) note(s string) { fp.Notes = append(fp.Notes, s) }

// chosenSIDs are the options of the SID named (and its transition), all
// of them when it is not one of the runway's (noted).
func chosenSIDs(fp *FlightPlan, opts []sidOption, name, transition, rwy string) []sidOption {
	if name == "" {
		return opts
	}
	var out []sidOption
	for _, o := range opts {
		if strings.EqualFold(o.name, name) && (transition == "" || strings.EqualFold(o.transition, transition)) {
			out = append(out, o)
		}
	}
	if len(out) > 0 {
		return out
	}
	if transition != "" && slices.ContainsFunc(opts, func(o sidOption) bool { return strings.EqualFold(o.name, name) }) {
		fp.note(fmt.Sprintf("SID %s has no transition %s: chosen as without it", name, transition))
		return chosenSIDs(fp, opts, name, "", rwy)
	}
	fp.note(fmt.Sprintf("SID %s is not one from runway %s: chosen as without it", name, rwy))
	return opts
}

// chosenSTARs are the options of the STAR named (and its transition), all
// of them when it is not one to the runway (noted).
func chosenSTARs(fp *FlightPlan, opts []arrivalOption, name, transition, rwy string) []arrivalOption {
	if name == "" {
		return opts
	}
	var out []arrivalOption
	for _, o := range opts {
		if strings.EqualFold(o.star, name) && (transition == "" || strings.EqualFold(o.starTransition, transition)) {
			out = append(out, o)
		}
	}
	if len(out) > 0 {
		return out
	}
	if transition != "" && slices.ContainsFunc(opts, func(o arrivalOption) bool { return strings.EqualFold(o.star, name) }) {
		fp.note(fmt.Sprintf("STAR %s has no transition %s: chosen as without it", name, transition))
		return chosenSTARs(fp, opts, name, "", rwy)
	}
	fp.note(fmt.Sprintf("STAR %s is not one to runway %s: chosen as without it", name, rwy))
	return opts
}

// routeOf is the request's Route (item 15) as enroute waypoints over g's
// airways; what is not known is noted (and flown direct). Procedure names
// and the airports in it are expected and passed over quietly.
func (fp *FlightPlan) routeOf(req FlightPlanRequest, g *AirwayGraph, from airport.LatLon, sids []sidOption, arrs []arrivalOption) []Waypoint {
	if g == nil {
		fp.note("no airways known: the route " + strings.TrimSpace(req.Route) + " is not flown")
		return nil
	}
	steps, skipped, err := g.ExpandRoute(req.Route, from)
	if err != nil {
		fp.note(fmt.Sprintf("route %s: %v: the route is not flown", strings.TrimSpace(req.Route), err))
		return nil
	}
	expected := map[string]bool{strings.ToUpper(req.Departure.ICAO): true, strings.ToUpper(req.Arrival.ICAO): true, "SID": true, "STAR": true}
	for _, s := range sids {
		expected[strings.ToUpper(s.name)] = true
	}
	for _, a := range arrs {
		expected[strings.ToUpper(a.star)] = true
	}
	var unknown []string
	for _, t := range skipped {
		if !expected[t] {
			unknown = append(unknown, t)
		}
	}
	if len(unknown) > 0 {
		fp.note("not known, flown direct: " + strings.Join(unknown, " "))
	}
	out := make([]Waypoint, 0, len(steps))
	for _, st := range steps {
		out = append(out, Waypoint{Ident: st.Fix.Ident, Region: st.Fix.Region, Kind: st.Fix.Kind.String(), Position: st.Position, Airway: st.Airway, Phase: PhaseEnroute})
	}
	return out
}

// routeNM is the length along wps.
func routeNM(wps []Waypoint) float64 {
	nm := 0.0
	for i := 1; i < len(wps); i++ {
		nm += dist(wps[i-1].Position, wps[i].Position)
	}
	return nm
}

// approachNamed is the approach named ("ILS 24", any case and spacing).
func approachNamed(p *airport.Procedures, name string) (airport.Approach, bool) {
	want := strings.Join(strings.Fields(strings.ToUpper(name)), " ")
	for _, a := range p.Approaches {
		if strings.ToUpper(a.Name) == want {
			return a, true
		}
	}
	return airport.Approach{}, false
}

func hasTransition(ts []airport.Transition, name string) bool {
	return slices.ContainsFunc(ts, func(t airport.Transition) bool { return strings.EqualFold(t.Name, name) })
}
