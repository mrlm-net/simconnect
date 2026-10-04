package airport

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// RouteOptions controls which edges a route may use.
type RouteOptions struct {
	// UseRunwayPaths allows taxiing along RUNWAY paths (back-tracking or
	// using a runway as a taxiway). Crossing a runway on a taxiway is always
	// allowed and reported in Route.RunwayCrossings.
	UseRunwayPaths bool
	// IntersectionTolerance selects hold-short points for RouteToRunway: any
	// hold-short within this many meters (along the runway) of the one nearest
	// the threshold is a candidate, and the shortest route among them wins.
	// Zero means DefaultIntersectionTolerance.
	IntersectionTolerance float64
	// TurnPenalty is the extra cost, in meters of route length, of a 90°
	// turn at a taxiway junction (proportional above TurnFreeAngle), and
	// TaxiwayChangePenalty that of turning onto another taxiway: routes with
	// fewer turns win even when a little longer. Zero means
	// DefaultTurnPenalty / DefaultTaxiwayChangePenalty; negative disables.
	TurnPenalty          float64
	TaxiwayChangePenalty float64
	// RunwayCrossingPenalty is the extra cost of each runway crossing. Zero
	// means DefaultRunwayCrossingPenalty; negative disables.
	RunwayCrossingPenalty float64
	// ApronPenalty is the extra cost, as a fraction of the length, of edges
	// at taxi points where a stand connects, so routes keep to taxiways
	// without stands. Zero means DefaultApronPenalty; negative disables.
	ApronPenalty float64
	// StandTurnAroundPenalty is the extra cost of entering a stand through a
	// lead-in ahead of it (turning round on the apron) and PushbackPenalty of
	// leaving through one behind it (a pushback), so arrivals enter nose-in
	// and departures leave forward where the stand allows. Zero means the
	// default; negative disables.
	StandTurnAroundPenalty float64
	PushbackPenalty        float64
	// HalfSpan is half the aircraft's wing span in meters: the route keeps
	// to taxiway edges whose Clearance leaves WingtipMargin (zero:
	// DefaultWingtipMargin) beyond it, so a large aircraft stays off apron
	// taxilanes it does not fit. OwnStands (the stands the aircraft leaves or
	// enters) do not count as obstacles. When no route fits, the route is
	// found without the check and marked Route.Tight. Zero disables.
	HalfSpan      float64
	WingtipMargin float64
	OwnStands     []int
	// TaxiwayMaxSpan limits taxiways by the largest wing span allowed on
	// them, in meters (published restrictions the scenery does not carry,
	// e.g. code C taxilanes: 36 m), by taxiway name; nil uses the airport's
	// entry in KnownTaxiwayMaxSpan. Applies with HalfSpan.
	TaxiwayMaxSpan map[string]float64
	// OwnApronMeters waives ApronPenalty within this distance of the start:
	// an aircraft leaving its own apron uses its taxilanes (LKPR C17 leaves by
	// JB, the nearest). Zero means DefaultOwnApronMeters; negative disables.
	OwnApronMeters float64
	// Via makes the route pass through these nodes in order (a custom route,
	// #340). Each leg is found by the same search and costs, and the route
	// goes on from a via point the way it arrived: it never turns back there.
	// A via point the route cannot reach returns a *RouteError wrapping
	// ErrViaUnreachable.
	Via []NodeID
	// Taxiways makes the route follow these taxiways in order ("via A, L"):
	// the names must appear in Route.Taxiways in this order. Until the last
	// is joined, other taxiways stay usable to connect them at
	// OffTaxiwaysFactor times their length. A route that cannot follow them
	// returns a *RouteError wrapping ErrTaxiwaysNotFollowed. Names match
	// case-insensitively.
	//
	// With Via or Taxiways a route that does not fit the aircraft (HalfSpan)
	// is never returned Tight: the error wraps ErrTooNarrow instead.
	Taxiways []string
	// CurrentTaxiway is the listed taxiway the route starts on, already
	// followed (RemainingOptions sets it after a pushback or runway exit
	// onto it): going on along it costs no penalty.
	CurrentTaxiway string
}

// DefaultIntersectionTolerance is the RouteOptions.IntersectionTolerance used
// when none is set.
const DefaultIntersectionTolerance = 300.0

// Route is a taxi route through the graph.
type Route struct {
	Nodes  []NodeID `json:"nodes"`
	Points []LatLon `json:"points"`
	Edges  []Edge   `json:"edges"`  // Edges[i] leads from Nodes[i] to Nodes[i+1]
	Length float64  `json:"length"` // meters
	// Cost is what the search minimised: length plus the turn, crossing and
	// apron penalties (RouteOptions); 0 for routes not found by a search.
	Cost float64 `json:"cost,omitempty"`
	// Taxiways is the sequence of taxiway names along the route with
	// consecutive repeats and unnamed segments removed, e.g. ["C", "L", "D"].
	Taxiways []string `json:"taxiways"`
	// RunwayCrossings names each runway the route crosses, in order.
	RunwayCrossings []string `json:"runwayCrossings"`
	// Runway and RunwayEnd are set by RouteToRunway.
	Runway    string `json:"runway,omitempty"`
	RunwayEnd string `json:"runwayEnd,omitempty"`
	// Entry is the entry taxiway set by RouteToRunwayEntry ("B" in "24 at B").
	Entry string `json:"entry,omitempty"`
	// Tight is set when no route fits the aircraft (RouteOptions.HalfSpan)
	// and this one was found without the span check.
	Tight bool `json:"tight,omitempty"`
	// HoldShort is the final node's hold-short data when the route ends at one.
	HoldShort *HoldShort `json:"holdShort,omitempty"`
}

// ParkingIndex resolves a parking label such as "C22" to a parking index.
// It returns ErrUnknownParking when no spot matches and ErrAmbiguousParking
// when several do.
func (l *Layout) ParkingIndex(label string) (int, error) {
	m := l.ParkingByLabel(label)
	switch len(m) {
	case 0:
		return 0, fmt.Errorf("%w: %q", ErrUnknownParking, label)
	case 1:
		return m[0].Index, nil
	}
	idx := make([]int, len(m))
	for i, p := range m {
		idx[i] = p.Index
	}
	return 0, fmt.Errorf("%w: %q (indexes %v)", ErrAmbiguousParking, label, idx)
}

// Route returns the shortest route from one node to another.
func (g *Graph) Route(from, to NodeID, opts RouteOptions) (*Route, error) {
	if !g.valid(from) || !g.valid(to) {
		return nil, fmt.Errorf("%w: node out of range", ErrNoRoute)
	}
	return g.fitOrTight(opts, func(o RouteOptions) (*Route, error) { return g.routeVia(from, -1, to, o) })
}

// routeVia is Route for an aircraft that arrived at from via prev (-1 if
// its heading is free).
func (g *Graph) routeVia(from, prev, to NodeID, opts RouteOptions) (*Route, error) {
	s := g.shortestPaths(from, prev, opts)
	if math.IsInf(s.dist[to], 1) {
		if err := s.failure(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: node %d to node %d", ErrNoRoute, from, to)
	}
	return s.route(g, to)
}

// RouteToRunway returns a departure taxi route from a parking spot to a
// hold-short point of the given runway end (e.g. "24"). Runway holding points
// are preferred over ILS critical area holds; among holds within
// IntersectionTolerance of the one nearest the threshold, the shortest route
// wins, so aircraft depart from (or near) the full runway length.
func (g *Graph) RouteToRunway(parking int, runwayEnd string, opts RouteOptions) (*Route, error) {
	from, ok := g.ParkingNode(parking)
	if !ok {
		return nil, fmt.Errorf("%w: index %d", ErrUnknownParking, parking)
	}
	opts.OwnStands = append(slices.Clone(opts.OwnStands), parking)
	return g.fitOrTight(opts, func(o RouteOptions) (*Route, error) { return g.runwayRoute(from, -1, runwayEnd, o) })
}

// RouteToRunwayFrom is RouteToRunwayEntry for an aircraft at node from that
// arrived there from prev (-1 if its heading is free): the route does not
// turn back into prev. A departure pushed back from its stand onto a
// taxiway branch starts its taxi-out this way, from the junction facing away
// from that branch. An empty entry means full length.
func (g *Graph) RouteToRunwayFrom(from, prev NodeID, runwayEnd, entry string, opts RouteOptions) (*Route, error) {
	if !g.valid(from) || (prev >= 0 && !g.valid(prev)) {
		return nil, fmt.Errorf("%w: node out of range", ErrNoRoute)
	}
	return g.fitOrTight(opts, func(o RouteOptions) (*Route, error) {
		if entry == "" {
			return g.runwayRoute(from, prev, runwayEnd, o)
		}
		return g.entryRoute(from, prev, runwayEnd, entry, o)
	})
}

// RouteFromNodes assembles a Route along consecutive, adjacent nodes, e.g. to
// join a pushback onto a taxi-out planned with RouteToRunwayFrom.
func (g *Graph) RouteFromNodes(nodes []NodeID) (*Route, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("%w: no nodes", ErrNoRoute)
	}
	for i, id := range nodes {
		if !g.valid(id) {
			return nil, fmt.Errorf("%w: node %d out of range", ErrNoRoute, id)
		}
		if i > 0 && !g.adjacent(nodes[i-1], id) {
			return nil, fmt.Errorf("%w: nodes %d and %d are not connected", ErrNoRoute, nodes[i-1], id)
		}
	}
	return g.routeFromNodes(nodes), nil
}

// adjacent reports whether an edge leads from a to b.
func (g *Graph) adjacent(a, b NodeID) bool {
	for _, e := range g.Adj[a] {
		if e.To == b {
			return true
		}
	}
	return false
}

// runwayRoute is RouteToRunway from a node reached via prev.
func (g *Graph) runwayRoute(from, prev NodeID, runwayEnd string, opts RouteOptions) (*Route, error) {
	rwy, end, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return nil, fmt.Errorf("%w: %q at %s", ErrUnknownRunway, runwayEnd, g.Layout.ICAO)
	}
	holds := g.HoldShortNodes(rwy.Index)
	if len(holds) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoHoldShort, rwy.Name())
	}
	s := g.shortestPaths(from, prev, opts)
	dist := s.dist

	type cand struct {
		id         NodeID
		fromThresh float64
		route      float64
		ils        bool
	}
	var cands []cand
	for _, id := range holds {
		if math.IsInf(dist[id], 1) {
			continue
		}
		h := g.Nodes[id].HoldShort
		along := h.FromPrimary
		if end.Name == rwy.Secondary.Name {
			along = rwy.Length - h.FromPrimary
		}
		cands = append(cands, cand{id: id, fromThresh: along, route: dist[id], ils: h.ILS})
	}
	if len(cands) == 0 {
		if err := s.failure(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: no reachable hold-short for runway %s", ErrNoRoute, end.Name)
	}
	// Prefer runway holding points; fall back to ILS holds only if none is reachable.
	var runwayHolds []cand
	for _, c := range cands {
		if !c.ils {
			runwayHolds = append(runwayHolds, c)
		}
	}
	if len(runwayHolds) > 0 {
		cands = runwayHolds
	}
	tol := opts.IntersectionTolerance
	if tol <= 0 {
		tol = DefaultIntersectionTolerance
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].fromThresh < cands[j].fromThresh })
	best := cands[0]
	for _, c := range cands[1:] {
		if c.fromThresh-cands[0].fromThresh > tol {
			break
		}
		if c.route < best.route {
			best = c
		}
	}

	r, err := s.route(g, best.id)
	if err != nil {
		return nil, err
	}
	r.Runway, r.RunwayEnd = rwy.Name(), end.Name
	return r, nil
}

// RouteToParking returns a taxi-in route from any node to a parking spot.
func (g *Graph) RouteToParking(from NodeID, parking int, opts RouteOptions) (*Route, error) {
	to, ok := g.ParkingNode(parking)
	if !ok {
		return nil, fmt.Errorf("%w: index %d", ErrUnknownParking, parking)
	}
	opts.OwnStands = append(slices.Clone(opts.OwnStands), parking)
	return g.Route(from, to, opts)
}

// usable reports whether a route may traverse e: RUNWAY paths only with
// RouteOptions.UseRunwayPaths. Taxiway edges along a runway surface are
// usable but expensive (AlongRunwayFactor), so a route can cross a runway
// where the taxiway runs a short way along it (LROP) but never backtracks
// along a runway while a taxiway will do.
func usable(e Edge, opts RouteOptions) bool {
	return e.Type != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY || opts.UseRunwayPaths
}

// Turn costs (#307): pilots and ATC prefer routes with fewer and gentler
// turns, even when they are a little longer. The route search adds these to
// the length (Route.Length stays the real length).
const (
	// DefaultTurnPenalty is the extra cost, in meters, of a 90° turn at a
	// taxiway junction, proportional to the angle above TurnFreeAngle.
	DefaultTurnPenalty = 60.0
	// DefaultTaxiwayChangePenalty is the extra cost of turning onto a
	// differently named taxiway (going straight on where the name changes is
	// free). (100 m saved only 0.03 letters a route at LKPR and made some
	// turn more than the shortest route; a clearance leaves out short stubs
	// instead: Route.SpokenTaxiways.)
	DefaultTaxiwayChangePenalty = 40.0
	// TurnFreeAngle is the largest heading change at a junction that costs
	// nothing (taxiways bend).
	TurnFreeAngle = 15.0
	// UTurnAngle is the smallest heading change counted as turning back;
	// it costs UTurnPenalty, so a route only turns back when it must.
	UTurnAngle   = 150.0
	UTurnPenalty = 2000.0
	// DefaultRunwayCrossingPenalty is the extra cost of each runway
	// crossing: a crossing needs a clearance and blocks the runway.
	DefaultRunwayCrossingPenalty = 1000.0
	// AlongRunwayFactor multiplies the cost of taxiway edges that run along
	// a runway surface (unless RouteOptions.UseRunwayPaths).
	AlongRunwayFactor = 20.0
	// DefaultApronPenalty is the extra cost, as a fraction of the length, of
	// edges at taxi points where a stand connects (apron taxilanes).
	DefaultApronPenalty = 0.5
	// DefaultOwnApronMeters is the RouteOptions.OwnApronMeters used when none
	// is set.
	DefaultOwnApronMeters = 250.0
	// DefaultStandTurnAroundPenalty and DefaultPushbackPenalty: see
	// RouteOptions.
	DefaultStandTurnAroundPenalty = 3000.0
	DefaultPushbackPenalty        = 200.0
)

func (o RouteOptions) turnPenalty() float64 {
	switch {
	case o.TurnPenalty < 0:
		return 0
	case o.TurnPenalty == 0:
		return DefaultTurnPenalty
	}
	return o.TurnPenalty
}

func (o RouteOptions) apronPenalty() float64 {
	switch {
	case o.ApronPenalty < 0:
		return 0
	case o.ApronPenalty == 0:
		return DefaultApronPenalty
	}
	return o.ApronPenalty
}

func (o RouteOptions) crossingPenalty() float64 {
	switch {
	case o.RunwayCrossingPenalty < 0:
		return 0
	case o.RunwayCrossingPenalty == 0:
		return DefaultRunwayCrossingPenalty
	}
	return o.RunwayCrossingPenalty
}

func (o RouteOptions) changePenalty() float64 {
	switch {
	case o.TaxiwayChangePenalty < 0:
		return 0
	case o.TaxiwayChangePenalty == 0:
		return DefaultTaxiwayChangePenalty
	}
	return o.TaxiwayChangePenalty
}

// turnCost is the extra cost of going on from node onto out, having arrived
// from prev on taxiway inName.
func (g *Graph) turnCost(prev, node NodeID, inName string, out Edge, opts RouteOptions) float64 {
	angle := g.turnAngle(prev, node, out.To)
	cost := 0.0
	if angle >= UTurnAngle {
		cost += UTurnPenalty
	}
	junction := len(g.Adj[node]) > 2
	if junction && angle > TurnFreeAngle {
		cost += opts.turnPenalty() * (angle - TurnFreeAngle) / 90
	}
	if junction && angle > TurnFreeAngle && inName != "" && out.Name != "" && inName != out.Name {
		cost += opts.changePenalty()
	}
	return cost
}

// turnAngle is the heading change, in degrees 0–180, of going on from node
// to next having arrived from prev.
func (g *Graph) turnAngle(prev, node, next NodeID) float64 {
	ax, az := g.local.xz(g.Nodes[prev].Position)
	bx, bz := g.local.xz(g.Nodes[node].Position)
	cx, cz := g.local.xz(g.Nodes[next].Position)
	h1, h2 := math.Atan2(bx-ax, bz-az), math.Atan2(cx-bx, cz-bz)
	return math.Abs(math.Mod(math.Abs(h2-h1)*180/math.Pi+180, 360) - 180)
}

// search is a turn-aware Dijkstra over (node, arrived-from) states. With
// RouteOptions.Via and Taxiways a state also carries how many via points
// and taxiways the route has passed, and only states that passed them all
// count towards dist.
type search struct {
	dist []float64 // cheapest cost per node
	best []int     // state with that cost, -1 if unreached
	node []NodeID  // per state
	from []int     // per state: previous state, -1 at the source
	name []string  // per state: taxiway arrived on (unnamed connectors inherit the previous name)
	cost []float64 // per state

	via      []NodeID // RouteOptions.Via
	taxiways []string // RouteOptions.Taxiways
	current  string   // RouteOptions.CurrentTaxiway
	maxVia   int      // most via points any state passed
	maxTw    int      // most taxiways any state past all via points followed
}

// stateKey identifies a search state: the node, the node it was reached
// from, and the via points and taxiways passed.
type stateKey struct {
	node, prev NodeID
	via, tw    int
}

// OffTaxiwaysFactor multiplies the cost of named taxiway edges that are
// neither the taxiway a RouteOptions.Taxiways route is on nor the next one
// in the list: the route may use them to connect, but only where it must.
const OffTaxiwaysFactor = 10.0

// route assembles the route to to. With RouteOptions.Taxiways it checks the
// route names them in order (parallel edges may carry other names than the
// search followed) and returns ErrTaxiwaysNotFollowed if not.
func (s *search) route(g *Graph, to NodeID) (*Route, error) {
	r := g.routeFromNodes(s.path(to))
	r.Cost = s.dist[to]
	if n := followedTaxiways(r.Taxiways, s.taxiways); n < len(s.taxiways) {
		return nil, &RouteError{Err: ErrTaxiwaysNotFollowed, Via: -1, Node: -1, Taxiway: s.taxiways[n]}
	}
	return r, nil
}

// failure explains why a search with RouteOptions.Via or Taxiways reached
// no destination; nil without them.
func (s *search) failure() error {
	switch {
	case s.maxVia < len(s.via):
		return &RouteError{Err: ErrViaUnreachable, Via: s.maxVia, Node: s.via[s.maxVia]}
	case len(s.taxiways) > 0:
		return &RouteError{Err: ErrTaxiwaysNotFollowed, Via: -1, Node: -1, Taxiway: s.taxiways[min(s.maxTw, len(s.taxiways)-1)]}
	case len(s.via) > 0:
		return &RouteError{Err: ErrNoRoute, Via: len(s.via), Node: -1}
	}
	return nil
}

// passVia advances the count of via points passed on arriving at node.
func passVia(via []NodeID, n int, node NodeID) int {
	for n < len(via) && via[n] == node {
		n++
	}
	return n
}

// offTaxiways reports whether e is a named taxiway edge a Taxiways route
// that followed tw of them should keep off: neither the current nor the next
// taxiway in the list. Past the last one the route goes on freely to its
// destination ("via B" ends where B meets the way to the holding point).
// Runway paths and stand lead-ins never count.
func offTaxiways(e Edge, taxiways []string, tw int, current string) bool {
	if tw >= len(taxiways) || e.Name == "" ||
		e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY || e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING {
		return false
	}
	if strings.EqualFold(e.Name, taxiways[tw]) {
		return false
	}
	if tw > 0 {
		current = taxiways[tw-1]
	}
	return current == "" || !strings.EqualFold(e.Name, current)
}

// followedTaxiways counts how many of want names contains in order.
func followedTaxiways(names, want []string) int {
	i := 0
	for _, n := range names {
		if i < len(want) && strings.EqualFold(n, want[i]) {
			i++
		}
	}
	return i
}

// path returns the node sequence from the source to to.
func (s *search) path(to NodeID) []NodeID {
	var nodes []NodeID
	for st := s.best[to]; st != -1; st = s.from[st] {
		nodes = append(nodes, s.node[st])
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return nodes
}

// shortestPaths runs the search from src; srcPrev, if valid, is the node the
// aircraft arrived at src from (its heading), otherwise any first direction
// is free. Parking nodes other than src are dead ends: a route may end at a
// stand but never pass through one.
func (g *Graph) shortestPaths(src, srcPrev NodeID, opts RouteOptions) *search {
	if opts.TaxiwayMaxSpan == nil {
		opts.TaxiwayMaxSpan = KnownTaxiwayMaxSpan[g.Layout.ICAO]
	}
	n := len(g.Nodes)
	s := &search{dist: make([]float64, n), best: make([]int, n), via: opts.Via, taxiways: opts.Taxiways, current: opts.CurrentTaxiway}
	own := opts.ownApron()
	srcPos := g.Nodes[src].Position
	nearSrc := func(id NodeID) bool {
		p := g.Nodes[id].Position
		return own > 0 && calc.HaversineMeters(srcPos.Lat, srcPos.Lon, p.Lat, p.Lon) < own
	}
	for i := range s.dist {
		s.dist[i] = math.Inf(1)
		s.best[i] = -1
	}
	index := map[stateKey]int{}
	crossed := map[[2]NodeID]int{} // runway crossings per edge
	var vias, tws []int            // per state: via points passed, taxiways followed
	state := func(node, prev NodeID, via, tw int) int {
		k := stateKey{node, prev, via, tw}
		if id, ok := index[k]; ok {
			return id
		}
		id := len(s.node)
		index[k] = id
		s.node, s.from, s.name, s.cost = append(s.node, node), append(s.from, -1), append(s.name, ""), append(s.cost, math.Inf(1))
		vias, tws = append(vias, via), append(tws, tw)
		if via > s.maxVia {
			s.maxVia = via
		}
		if via == len(s.via) && tw > s.maxTw {
			s.maxTw = tw
		}
		return id
	}
	// done reports whether a state passed every via point and taxiway.
	done := func(st int) bool { return vias[st] == len(s.via) && tws[st] == len(s.taxiways) }
	start := state(src, -1, passVia(s.via, 0, src), 0)
	s.cost[start] = 0
	if done(start) {
		s.dist[src], s.best[src] = 0, start
	}
	if g.valid(srcPrev) {
		s.name[start] = g.edge(srcPrev, src).Name
	}
	pq := &nodeQueue{{id: NodeID(start)}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(queued)
		st := int(cur.id)
		if cur.dist > s.cost[st] {
			continue
		}
		node := s.node[st]
		if node != src && g.Nodes[node].Kind == NodeParking {
			continue
		}
		prev := srcPrev
		if s.from[st] != -1 {
			prev = s.node[s.from[st]]
		}
		// At a via point the route goes on the way it arrived.
		atVia := vias[st] > 0 && s.via[vias[st]-1] == node && g.valid(prev)
		for _, e := range g.Adj[node] {
			if !usable(e, opts) || !opts.fits(e) {
				continue
			}
			if atVia && g.turnAngle(prev, node, e.To) >= UTurnAngle {
				continue
			}
			d := cur.dist + e.Length
			tw := tws[st]
			if offTaxiways(e, s.taxiways, tw, s.current) {
				d += (OffTaxiwaysFactor - 1) * e.Length
			}
			// A taxiway counts on joining it, as in Route.Taxiways: going on
			// along it does not follow it again.
			if tw < len(s.taxiways) && strings.EqualFold(e.Name, s.taxiways[tw]) && (st == start || !strings.EqualFold(e.Name, s.name[st])) {
				tw++
			}
			if e.AlongRunway && !opts.UseRunwayPaths {
				d += (AlongRunwayFactor - 1) * e.Length
			}
			// Stands: enter nose-in through a lead-in behind the stand; one
			// ahead of it means turning round on the apron. Leave forward
			// through a lead-in ahead when there is one; one behind means a
			// pushback.
			if e.To != src && g.Nodes[e.To].Kind == NodeParking && g.LeadInAhead(e.To, node) {
				d += penalty(opts.StandTurnAroundPenalty, DefaultStandTurnAroundPenalty)
			}
			if node == src && g.Nodes[src].Kind == NodeParking && !g.LeadInAhead(src, e.To) {
				d += penalty(opts.PushbackPenalty, DefaultPushbackPenalty)
			}
			// Apron taxilanes (a stand connects at either end) cost extra, so
			// through traffic keeps to taxiways without stands (LROP: N, not M).
			// An aircraft leaving its own apron uses its taxilanes freely
			// (OwnApronMeters around the start).
			if pen := opts.apronPenalty(); pen > 0 && g.stands != nil && (g.stands[node] || g.stands[e.To]) && !nearSrc(node) {
				d += pen * e.Length
			}
			if pen := opts.crossingPenalty(); pen > 0 {
				k := [2]NodeID{node, e.To}
				c, ok := crossed[k]
				if !ok {
					c = g.crossings(node, e.To)
					crossed[k] = c
				}
				d += pen * float64(c)
			}
			if g.valid(prev) {
				d += g.turnCost(prev, node, s.name[st], e, opts)
			}
			next := state(e.To, node, passVia(s.via, vias[st], e.To), tw)
			if d < s.cost[next] {
				nm := e.Name
				if nm == "" {
					nm = s.name[st]
				}
				s.cost[next], s.from[next], s.name[next] = d, st, nm
				if done(next) && d < s.dist[e.To] {
					s.dist[e.To], s.best[e.To] = d, next
				}
				heap.Push(pq, queued{id: NodeID(next), dist: d})
			}
		}
	}
	return s
}

// routeFromNodes assembles a Route along consecutive, adjacent nodes.
func (g *Graph) routeFromNodes(nodes []NodeID) *Route {
	r := &Route{Nodes: nodes, Taxiways: []string{}, RunwayCrossings: []string{}}
	for i, id := range nodes {
		r.Points = append(r.Points, g.Nodes[id].Position)
		if i == 0 {
			continue
		}
		e := g.edge(nodes[i-1], id)
		r.Edges = append(r.Edges, e)
		r.Length += e.Length
		if e.Name != "" && (len(r.Taxiways) == 0 || r.Taxiways[len(r.Taxiways)-1] != e.Name) {
			r.Taxiways = append(r.Taxiways, e.Name)
		}
	}
	r.RunwayCrossings = g.runwayCrossings(r.Points)
	r.HoldShort = g.Nodes[nodes[len(nodes)-1]].HoldShort
	return r
}

// edge returns the shortest edge from a to b.
func (g *Graph) edge(a, b NodeID) Edge {
	best := Edge{To: b, Length: math.Inf(1)}
	for _, e := range g.Adj[a] {
		if e.To == b && e.Length < best.Length {
			best = e
		}
	}
	return best
}

// runwayCrossings lists the runways whose surface the polyline enters, in
// order, sampling every few meters.
func (g *Graph) runwayCrossings(pts []LatLon) []string {
	out := []string{}
	inside := -1
	for i := 1; i < len(pts); i++ {
		d := g.distance(pts[i-1], pts[i])
		steps := int(d/5) + 1
		for s := 1; s <= steps; s++ {
			f := float64(s) / float64(steps)
			p := LatLon{Lat: pts[i-1].Lat + (pts[i].Lat-pts[i-1].Lat)*f, Lon: pts[i-1].Lon + (pts[i].Lon-pts[i-1].Lon)*f}
			on := -1
			for _, r := range g.Layout.Runways {
				along, off := g.runwayCoords(r, p)
				if along >= 0 && along <= r.Length && off <= r.Width/2 {
					on = r.Index
					break
				}
			}
			if on != -1 && on != inside {
				out = append(out, g.Layout.Runways[on].Name())
			}
			inside = on
		}
	}
	return out
}

func (g *Graph) valid(id NodeID) bool { return id >= 0 && int(id) < len(g.Nodes) }

type queued struct {
	id   NodeID
	dist float64
}

type nodeQueue []queued

func (q nodeQueue) Len() int           { return len(q) }
func (q nodeQueue) Less(i, j int) bool { return q[i].dist < q[j].dist }
func (q nodeQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *nodeQueue) Push(x any)        { *q = append(*q, x.(queued)) }
func (q *nodeQueue) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}

// onRunway returns the index of the runway whose surface contains p, or -1.
func (g *Graph) onRunway(p LatLon) int { return g.RunwayAt(p, 0) }

// RunwayAt returns the index of the runway whose surface, widened by margin
// meters on every side, contains p; -1 if none.
func (g *Graph) RunwayAt(p LatLon, margin float64) int {
	for _, r := range g.Layout.Runways {
		along, off := g.runwayCoords(r, p)
		if along >= -margin && along <= r.Length+margin && off <= r.Width/2+margin {
			return r.Index
		}
	}
	return -1
}

// crossings counts the runways the edge a → b enters, other than one a is
// already on (vacating or lining up is not a crossing).
func (g *Graph) crossings(a, b NodeID) int {
	on := g.onRunway(g.Nodes[a].Position)
	n := 0
	for _, name := range g.runwayCrossings([]LatLon{g.Nodes[a].Position, g.Nodes[b].Position}) {
		if on == -1 || name != g.Layout.Runways[on].Name() {
			n++
		}
	}
	return n
}

// penalty resolves an optional cost: zero means def, negative disables.
func penalty(v, def float64) float64 {
	switch {
	case v < 0:
		return 0
	case v == 0:
		return def
	}
	return v
}

// LeadInAhead reports whether the lead-in junction of a stand (a neighbour
// of its parking node) lies ahead of an aircraft parked there, i.e. on the
// side it faces. Such a stand is left forward without a pushback and
// entered by turning round on the apron.
func (g *Graph) LeadInAhead(parkingNode, junction NodeID) bool {
	if !g.valid(parkingNode) || !g.valid(junction) || g.Nodes[parkingNode].Kind != NodeParking {
		return false
	}
	p := g.Layout.Parking[g.Nodes[parkingNode].Index]
	px, pz := g.local.xz(p.Position)
	jx, jz := g.local.xz(g.Nodes[junction].Position)
	h := p.Heading * math.Pi / 180
	return (jx-px)*math.Sin(h)+(jz-pz)*math.Cos(h) > 0
}

func (o RouteOptions) ownApron() float64 {
	switch {
	case o.OwnApronMeters < 0:
		return 0
	case o.OwnApronMeters == 0:
		return DefaultOwnApronMeters
	}
	return o.OwnApronMeters
}

// DefaultWingtipMargin is the RouteOptions.WingtipMargin used when none is
// set: the wingtip clearance to a stand's circle, in meters.
const DefaultWingtipMargin = 3.0

// fits reports whether the aircraft (HalfSpan) fits beside e.
func (o RouteOptions) fits(e Edge) bool {
	if o.HalfSpan <= 0 {
		return true
	}
	if max, ok := o.TaxiwayMaxSpan[e.Name]; ok && e.Name != "" && 2*o.HalfSpan > max {
		return false
	}
	free := e.Clearance
	if e.ClearanceStand >= 0 && slices.Contains(o.OwnStands, e.ClearanceStand) {
		free = e.Clearance2
	}
	margin := o.WingtipMargin
	switch {
	case margin < 0:
		margin = 0
	case margin == 0:
		margin = DefaultWingtipMargin
	}
	return free >= o.HalfSpan+margin
}

// fitOrTight runs a route search with the span check and, when no route
// fits, again without it, marking the result Tight. A custom route (Via,
// Taxiways) is never Tight: when only the loose search finds it, the error
// names where the aircraft does not fit (ErrTooNarrow).
func (g *Graph) fitOrTight(opts RouteOptions, find func(RouteOptions) (*Route, error)) (*Route, error) {
	if err := g.ValidateRouteOptions(opts); err != nil {
		return nil, err
	}
	r, err := find(opts)
	if err == nil {
		return g.fewerStands(r, opts, find), nil
	}
	if !errors.Is(err, ErrNoRoute) {
		return r, err
	}
	if opts.HalfSpan > 0 {
		loose := opts
		loose.HalfSpan = 0
		if r2, err2 := find(loose); err2 == nil {
			if opts.custom() {
				return nil, g.tooNarrow(r2, opts)
			}
			r2.Tight = true
			return r2, nil
		}
	}
	// Blame Via or Taxiways only when the destination is reachable at all.
	if opts.custom() {
		plain := opts
		plain.Via, plain.Taxiways, plain.HalfSpan = nil, nil, 0
		if _, perr := find(plain); perr != nil {
			return nil, perr
		}
	}
	return r, err
}

// The last word between routes of about the same length: the one past
// fewer stands. A second search prices apron taxilanes at
// FewerStandsApronPenalty; its route wins when it passes fewer stands and
// is no more than FewerStandsTolerance (at least FewerStandsMinMeters)
// longer. Stands within OwnApronMeters of either end do not count.
const (
	FewerStandsApronPenalty = 4.0
	FewerStandsTolerance    = 0.15
	FewerStandsMinMeters    = 150.0
)

// fewerStandsOff turns the choice off (tests comparing routes with and
// without it).
var fewerStandsOff bool

// fewerStands is r, or the route found with apron taxilanes priced
// FewerStandsApronPenalty when that passes fewer stands within the
// tolerance. Not for custom routes (Via, Taxiways: as asked) nor with the
// apron penalty off.
func (g *Graph) fewerStands(r *Route, opts RouteOptions, find func(RouteOptions) (*Route, error)) *Route {
	if fewerStandsOff || opts.custom() || opts.apronPenalty() <= 0 || opts.apronPenalty() >= FewerStandsApronPenalty {
		return r
	}
	n := g.StandsPassed(r, opts)
	if n == 0 {
		return r
	}
	alt := opts
	alt.ApronPenalty = FewerStandsApronPenalty
	r2, err := find(alt)
	// Across no runway r does not cross (EGLL 09R exit S5W: back over the
	// one vacated instead of another).
	if err != nil || g.StandsPassed(r2, opts) >= n || !crossesOnly(r2, r) ||
		r2.Length > r.Length+math.Max(FewerStandsTolerance*r.Length, FewerStandsMinMeters) {
		return r
	}
	// The plain route's cost: the choice between routes this near is a
	// tie-break, which must not move choices made by comparing costs (an
	// exit, a push; EGLL 09R took S5W, back over the runway, when the
	// other exits' routes grew dearer).
	r2.Cost = r.Cost
	return r2
}

// StandsPassed counts the stands a route passes: parking spots connected to
// its nodes, leaving out those within OwnApronMeters of its start and end
// (its own stand and apron).
func (g *Graph) StandsPassed(r *Route, opts RouteOptions) int {
	if r == nil || len(r.Nodes) == 0 {
		return 0
	}
	own := opts.ownApron()
	first, last := g.Nodes[r.Nodes[0]].Position, g.Nodes[r.Nodes[len(r.Nodes)-1]].Position
	seen := map[NodeID]bool{}
	for _, id := range r.Nodes {
		if !g.stands[id] {
			continue
		}
		p := g.Nodes[id].Position
		if calc.HaversineMeters(p.Lat, p.Lon, first.Lat, first.Lon) < own || calc.HaversineMeters(p.Lat, p.Lon, last.Lat, last.Lon) < own {
			continue
		}
		for _, e := range g.Adj[id] {
			if g.Nodes[e.To].Kind == NodeParking {
				seen[e.To] = true
			}
		}
	}
	return len(seen)
}

// KnownTaxiwayMaxSpan are published taxiway span limits by airport ICAO and
// taxiway name, in meters: a first seed of the airport limits (#335).
var KnownTaxiwayMaxSpan = map[string]map[string]float64{
	// Apron taxilanes of the B/C apron: code C (A320, B737) only; wide-bodies
	// use J.
	"LKPR": {"JO": 36, "JB": 36},
}

// Fits reports whether an aircraft routed with opts (HalfSpan, OwnStands,
// TaxiwayMaxSpan — the airport's KnownTaxiwayMaxSpan when nil) fits beside
// e, as a route search would check it: e.g. before pushing a tail onto e.
func (g *Graph) Fits(e Edge, opts RouteOptions) bool {
	if opts.TaxiwayMaxSpan == nil {
		opts.TaxiwayMaxSpan = KnownTaxiwayMaxSpan[g.Layout.ICAO]
	}
	return opts.fits(e)
}

// SpokenMinMeters: a taxiway the route follows for less than this, only
// to lead onto the next one, is left out of a spoken clearance.
const SpokenMinMeters = 150.0

// SpokenTaxiways is the route up to edge upto (all when upto < 0) as a
// controller says it: the taxiways in order, without the short stubs that
// only lead onto the next one (SpokenMinMeters; at LKPR from N58 "H, L, G,
// F" is 270 m of three stubs curving onto F: "F"). The last taxiway is kept
// whatever its length: it is where the aircraft goes.
func (r *Route) SpokenTaxiways(upto int) []string {
	edges := r.Edges
	if upto >= 0 && upto < len(edges) {
		edges = edges[:upto]
	}
	type run struct {
		name   string
		meters float64
	}
	var runs []run
	for _, e := range edges {
		if e.Name == "" {
			continue
		}
		if len(runs) == 0 || runs[len(runs)-1].name != e.Name {
			runs = append(runs, run{name: e.Name})
		}
		runs[len(runs)-1].meters += e.Length
	}
	var out []string
	for i, rn := range runs {
		if rn.meters < SpokenMinMeters && i < len(runs)-1 {
			continue
		}
		if len(out) == 0 || out[len(out)-1] != rn.name {
			out = append(out, rn.name)
		}
	}
	return out
}

// crossesOnly reports whether r crosses only runways other crosses, each no
// more often.
func crossesOnly(r, other *Route) bool {
	left := map[string]int{}
	for _, c := range other.RunwayCrossings {
		left[c]++
	}
	for _, c := range r.RunwayCrossings {
		if left[c] == 0 {
			return false
		}
		left[c]--
	}
	return true
}
