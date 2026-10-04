package airport

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Custom route errors (#340): RouteOptions.Via and Taxiways. Those wrapping
// ErrNoRoute come as a *RouteError that says where the route fails.
var (
	ErrViaUnreachable      = fmt.Errorf("%w: via point not reachable", ErrNoRoute)
	ErrTaxiwaysNotFollowed = fmt.Errorf("%w: taxiways cannot be followed in order", ErrNoRoute)
	ErrTooNarrow           = fmt.Errorf("%w: aircraft does not fit", ErrNoRoute)
	ErrUnknownTaxiway      = errors.New("airport: no taxiway of that name")
)

// RouteError is why a custom route (RouteOptions.Via, Taxiways) failed, for
// a UI to point at: errors.As(err, &re) and re.Via, re.Taxiway.
type RouteError struct {
	// Err is ErrViaUnreachable, ErrTaxiwaysNotFollowed, ErrTooNarrow,
	// ErrUnknownTaxiway or ErrNoRoute.
	Err error
	// Via is the index into RouteOptions.Via of the via point the failing
	// leg leads to, len(Via) for the leg from the last via point to the
	// destination, -1 if the failure is not about a leg. Node is that via
	// point, -1 for the destination or none.
	Via  int
	Node NodeID
	// Taxiway is the requested taxiway not followed, or the taxiway the
	// aircraft does not fit (ErrTooNarrow); "" if none.
	Taxiway string
}

func (e *RouteError) Error() string {
	msg := e.Err.Error()
	switch {
	case e.Via >= 0 && e.Node >= 0:
		msg += fmt.Sprintf(": via point %d (node %d)", e.Via, e.Node)
	case e.Via >= 0:
		msg += ": from the last via point to the destination"
	}
	if e.Taxiway != "" {
		msg += ": taxiway " + e.Taxiway
	}
	return msg
}

func (e *RouteError) Unwrap() error { return e.Err }

// custom reports whether opts asks for a custom route.
func (o RouteOptions) custom() bool { return len(o.Via) > 0 || len(o.Taxiways) > 0 }

// ValidateRouteOptions checks a custom route request before routing: every
// Via node exists (else ErrViaUnreachable) and every Taxiways name is a
// taxiway of the airport (else ErrUnknownTaxiway), both as a *RouteError.
// The routing functions call it; a UI may call it to check its input.
func (g *Graph) ValidateRouteOptions(opts RouteOptions) error {
	for i, id := range opts.Via {
		if !g.valid(id) {
			return &RouteError{Err: ErrViaUnreachable, Via: i, Node: id}
		}
	}
	if len(opts.Taxiways) == 0 {
		return nil
	}
	names := g.TaxiwayNames()
	for _, name := range opts.Taxiways {
		if !slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, name) }) {
			return &RouteError{Err: ErrUnknownTaxiway, Via: -1, Node: -1, Taxiway: name}
		}
	}
	return nil
}

// TaxiwayNames returns the names of the taxiways in the graph, sorted.
func (g *Graph) TaxiwayNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, edges := range g.Adj {
		for _, e := range edges {
			if e.Name != "" && !seen[e.Name] {
				seen[e.Name] = true
				out = append(out, e.Name)
			}
		}
	}
	slices.Sort(out)
	return out
}

// RemainingOptions returns opts for a route that goes on from the end of the
// walk nodes: the Via points and Taxiways the walk already passed, in order,
// are dropped. A pushback, or an exit off the runway, passes some of a
// custom route before the route search starts.
func (g *Graph) RemainingOptions(opts RouteOptions, walked []NodeID) RouteOptions {
	if !opts.custom() {
		return opts
	}
	via, tw, last := 0, 0, ""
	for i, id := range walked {
		via = passVia(opts.Via, via, id)
		if i == 0 || !g.valid(walked[i-1]) || !g.valid(id) {
			continue
		}
		name := g.edge(walked[i-1], id).Name
		if name == "" || strings.EqualFold(name, last) {
			continue
		}
		if last = name; tw < len(opts.Taxiways) && strings.EqualFold(name, opts.Taxiways[tw]) {
			tw++
		}
	}
	if tw > 0 && strings.EqualFold(last, opts.Taxiways[tw-1]) {
		opts.CurrentTaxiway = last // still on it: going on along it is on the route
	}
	opts.Via, opts.Taxiways = opts.Via[via:], opts.Taxiways[tw:]
	return opts
}

// tooNarrow explains a custom route found only without the span check: the
// first taxiway along loose the aircraft does not fit, and the via point the
// leg there leads to.
func (g *Graph) tooNarrow(loose *Route, opts RouteOptions) error {
	re := &RouteError{Err: ErrTooNarrow, Via: -1, Node: -1}
	found := false
	via := passVia(opts.Via, 0, loose.Nodes[0])
	for i, e := range loose.Edges {
		if !g.Fits(e, opts) && (!found || e.Name != "") { // prefer a named taxiway
			re.Taxiway = e.Name
			if len(opts.Via) > 0 {
				re.Via, re.Node = via, -1
				if via < len(opts.Via) {
					re.Node = opts.Via[via]
				}
			}
			if found = true; e.Name != "" {
				break
			}
		}
		via = passVia(opts.Via, via, loose.Nodes[i+1])
	}
	return re
}
