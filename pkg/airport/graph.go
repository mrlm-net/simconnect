//go:build windows
// +build windows

package airport

import (
	"errors"
	"math"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Sentinel errors returned by graph building and routing.
var (
	ErrNoTaxiNetwork    = errors.New("airport: layout has no taxi network")
	ErrUnknownParking   = errors.New("airport: unknown parking spot")
	ErrAmbiguousParking = errors.New("airport: parking label matches more than one spot")
	ErrUnknownRunway    = errors.New("airport: unknown runway end")
	ErrNoHoldShort      = errors.New("airport: no hold-short point for runway")
	ErrNoRoute          = errors.New("airport: no route")
	ErrUnknownEntry     = errors.New("airport: no runway entry of that name")
)

// NodeID identifies a graph node. Taxi point i is node i; parking spot k is
// node len(TaxiPoints)+k.
type NodeID int

// NodeKind classifies a graph node.
type NodeKind uint8

const (
	NodeTaxiPoint NodeKind = iota // an ordinary taxi point
	NodeHoldShort                 // a runway hold-short taxi point
	NodeParking                   // a parking spot
)

// Node is a vertex of the taxi graph.
type Node struct {
	ID       NodeID   `json:"id"`
	Kind     NodeKind `json:"kind"`
	Position LatLon   `json:"position"`
	// Index is the taxi point index, or the parking index for NodeParking.
	Index int `json:"index"`
	// HoldShort describes the runway a NodeHoldShort protects; nil otherwise,
	// and nil for a hold-short point not near any runway.
	HoldShort *HoldShort `json:"holdShort,omitempty"`
}

// HoldShort relates a hold-short point to the runway it protects.
type HoldShort struct {
	Runway int    `json:"runway"` // index into Layout.Runways
	Name   string `json:"name"`   // runway name, e.g. "06/24"
	ILS    bool   `json:"ils"`    // ILS critical area hold rather than the runway holding point
	// FromPrimary is the distance along the runway from the primary threshold
	// to the point abeam the hold-short, in meters.
	FromPrimary float64 `json:"fromPrimary"`
	// Offset is the distance from the runway centreline, in meters.
	Offset float64 `json:"offset"`
}

// Edge is a directed half of an undirected taxi path.
type Edge struct {
	To     NodeID                                   `json:"to"`
	Length float64                                  `json:"length"` // meters
	Type   types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE `json:"type"`
	Name   string                                   `json:"name"` // taxiway name, "" if none
	Path   int                                      `json:"path"` // index into Layout.TaxiPaths
	// AlongRunway marks a taxiway edge that lies on a runway surface and runs
	// along it (MSFS data has such segments, e.g. where a taxiway crosses at a
	// runway end). Routes may use it at AlongRunwayFactor times its length.
	AlongRunway bool `json:"alongRunway,omitempty"`
	// Clearance is the free half-width beside the edge: the distance from
	// its centreline to the nearest stand circle (Parking.Radius, the space
	// of the largest aircraft the stand takes), in meters; ClearanceStand is
	// that stand (-1: none) and Clearance2 the same without it. A route for
	// an aircraft (RouteOptions.HalfSpan) keeps to edges it fits: LKPR JO has
	// 17.8 m, JB 21.5 m, J 35.9 m, so a 777 leaves B14 by J. Stand lead-ins
	// and runways are not limited (+Inf).
	Clearance      float64 `json:"-"`
	Clearance2     float64 `json:"-"`
	ClearanceStand int     `json:"-"`
}

// clearanceHorizon caps the stand search: stands further than this from an
// edge never limit it.
const clearanceHorizon = 150.0

// setClearances fills Edge.Clearance for every taxiway edge.
func (g *Graph) setClearances() {
	type stand struct {
		x, z, r float64
		i       int
	}
	var stands []stand
	for _, p := range g.Layout.Parking {
		if p.Size() == StandNone {
			continue
		}
		x, z := g.local.xz(p.Position)
		stands = append(stands, stand{x, z, p.Radius, p.Index})
	}
	for a := range g.Adj {
		ax, az := g.local.xz(g.Nodes[a].Position)
		for k := range g.Adj[a] {
			e := &g.Adj[a][k]
			e.Clearance, e.Clearance2, e.ClearanceStand = math.Inf(1), math.Inf(1), -1
			if e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING || e.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY ||
				g.Nodes[a].Kind == NodeParking || g.Nodes[e.To].Kind == NodeParking {
				continue
			}
			bx, bz := g.local.xz(g.Nodes[e.To].Position)
			for _, s := range stands {
				d := segmentDistance(ax, az, bx, bz, s.x, s.z)
				if d > clearanceHorizon+s.r {
					continue
				}
				free := d - s.r
				switch {
				case free < e.Clearance:
					e.Clearance2, e.Clearance, e.ClearanceStand = e.Clearance, free, s.i
				case free < e.Clearance2:
					e.Clearance2 = free
				}
			}
		}
	}
}

// segmentDistance is the distance from (px, pz) to the segment a–b.
func segmentDistance(ax, az, bx, bz, px, pz float64) float64 {
	dx, dz := bx-ax, bz-az
	u := 0.0
	if l2 := dx*dx + dz*dz; l2 > 0 {
		u = math.Max(0, math.Min(1, ((px-ax)*dx+(pz-az)*dz)/l2))
	}
	return math.Hypot(px-ax-u*dx, pz-az-u*dz)
}

// Graph is the routable taxi network of a Layout.
type Graph struct {
	Layout *Layout
	Nodes  []Node
	Adj    [][]Edge
	local  localFrame
	// stands marks taxi points with a PARKING path to a stand: apron
	// taxilanes, which routes avoid when a through taxiway will do.
	stands []bool
}

// Taxiway, runway and parking path types taken into the graph. CLOSED,
// VEHICLE, ROAD and PAINTEDLINE paths are not routable for aircraft.
func routable(t types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE) bool {
	switch t {
	case types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_TAXI,
		types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PATH,
		types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY,
		types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING:
		return true
	}
	return false
}

// holdShortMaxOffset is how far from a runway centreline a hold-short point
// may lie and still be associated with that runway. ICAO runway holding
// positions are 30–90 m from the centreline; ILS critical area holds further.
const holdShortMaxOffset = 300.0

// holdShortMaxOverrun is how far beyond a runway end a hold-short point may
// lie along the runway axis and still be associated with it.
const holdShortMaxOverrun = 400.0

// BuildGraph builds the taxi graph of l. Taxiway edges come from TAXI and
// PATH paths, RUNWAY paths become runway edges, and each PARKING path joins
// its START taxi point to the parking spot at END. Hold-short points are
// associated with the nearest runway.
func BuildGraph(l *Layout) (*Graph, error) {
	np := len(l.TaxiPoints)
	g := &Graph{
		Layout: l,
		Nodes:  make([]Node, np+len(l.Parking)),
		Adj:    make([][]Edge, np+len(l.Parking)),
		local:  newLocalFrame(l.Latitude, l.Longitude),
	}
	for i, t := range l.TaxiPoints {
		g.Nodes[i] = Node{ID: NodeID(i), Kind: NodeTaxiPoint, Position: t.Position, Index: i}
		if t.IsHoldShort() {
			g.Nodes[i].Kind = NodeHoldShort
			g.Nodes[i].HoldShort = g.associateRunway(t)
		}
	}
	for k, p := range l.Parking {
		id := NodeID(np + k)
		g.Nodes[id] = Node{ID: id, Kind: NodeParking, Position: p.Position, Index: k}
	}

	edges := 0
	for _, p := range l.TaxiPaths {
		if !routable(p.Type) || p.Start < 0 || int(p.Start) >= np || p.End < 0 {
			continue
		}
		a := NodeID(p.Start)
		var b NodeID
		if p.EndsAtParking() {
			if int(p.End) >= len(l.Parking) {
				continue
			}
			b = NodeID(np + int(p.End))
		} else {
			if int(p.End) >= np || p.Start == p.End {
				continue
			}
			b = NodeID(p.End)
		}
		length := g.distance(g.Nodes[a].Position, g.Nodes[b].Position)
		name := l.PathName(p)
		along := p.Type != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY && !p.EndsAtParking() && g.alongRunway(g.Nodes[a].Position, g.Nodes[b].Position)
		g.Adj[a] = append(g.Adj[a], Edge{To: b, Length: length, Type: p.Type, Name: name, Path: p.Index, AlongRunway: along})
		g.Adj[b] = append(g.Adj[b], Edge{To: a, Length: length, Type: p.Type, Name: name, Path: p.Index, AlongRunway: along})
		edges++
	}
	if edges == 0 {
		return nil, ErrNoTaxiNetwork
	}
	g.stands = make([]bool, len(g.Nodes))
	for id, es := range g.Adj {
		for _, e := range es {
			if g.Nodes[id].Kind != NodeParking && g.Nodes[e.To].Kind == NodeParking {
				g.stands[id] = true
			}
		}
	}
	g.setClearances()
	return g, nil
}

// ParkingNode returns the node of a parking index.
func (g *Graph) ParkingNode(parking int) (NodeID, bool) {
	if parking < 0 || parking >= len(g.Layout.Parking) {
		return 0, false
	}
	return NodeID(len(g.Layout.TaxiPoints) + parking), true
}

// Apron reports whether node id is on an apron taxilane: a taxi point with
// a parking path to a stand.
func (g *Graph) Apron(id NodeID) bool {
	return id >= 0 && int(id) < len(g.stands) && g.stands[id]
}

// HoldShortNodes returns the hold-short nodes associated with runway index
// rwy.
func (g *Graph) HoldShortNodes(rwy int) []NodeID {
	var out []NodeID
	for _, n := range g.Nodes {
		if n.HoldShort != nil && n.HoldShort.Runway == rwy {
			out = append(out, n.ID)
		}
	}
	return out
}

// associateRunway finds the runway a hold-short point protects: the one with
// the nearest centreline, within holdShortMaxOffset.
func (g *Graph) associateRunway(t TaxiPoint) *HoldShort {
	var best *HoldShort
	for _, r := range g.Layout.Runways {
		along, offset := g.runwayCoords(r, t.Position)
		if along < -holdShortMaxOverrun || along > r.Length+holdShortMaxOverrun || offset > holdShortMaxOffset {
			continue
		}
		if best == nil || offset < best.Offset {
			best = &HoldShort{Runway: r.Index, Name: r.Name(), ILS: t.IsILSHoldShort(), FromPrimary: along, Offset: offset}
		}
	}
	return best
}

// runwayCoords returns p's distance along runway r from its primary threshold
// and its distance from the centreline, in meters.
func (g *Graph) runwayCoords(r Runway, p LatLon) (along, offset float64) {
	x0, z0 := g.local.xz(r.Primary.Threshold)
	x1, z1 := g.local.xz(r.Secondary.Threshold)
	px, pz := g.local.xz(p)
	dx, dz := x1-x0, z1-z0
	length := math.Hypot(dx, dz)
	if length == 0 {
		return 0, math.Hypot(px-x0, pz-z0)
	}
	ux, uz := dx/length, dz/length
	along = (px-x0)*ux + (pz-z0)*uz
	offset = math.Abs((px-x0)*uz - (pz-z0)*ux)
	return along, offset
}

// distance is the ground distance between two positions near the airport.
func (g *Graph) distance(a, b LatLon) float64 {
	ax, az := g.local.xz(a)
	bx, bz := g.local.xz(b)
	return math.Hypot(bx-ax, bz-az)
}

// localFrame converts positions near an airport to meters east (x) and north
// (z) of its reference point. The equirectangular approximation is accurate to
// well under 0.1% across an airport.
type localFrame struct {
	lat0, lon0       float64
	mPerLat, mPerLon float64
}

func newLocalFrame(lat, lon float64) localFrame {
	const r = 6371008.8
	rad := lat * math.Pi / 180
	return localFrame{lat0: lat, lon0: lon, mPerLat: r * math.Pi / 180, mPerLon: r * math.Pi / 180 * math.Cos(rad)}
}

func (f localFrame) xz(p LatLon) (x, z float64) {
	return (p.Lon - f.lon0) * f.mPerLon, (p.Lat - f.lat0) * f.mPerLat
}

// alongRunwayMaxAngle is the largest angle between a taxiway edge and a
// runway for the edge to count as running along it. High-speed exits leave
// at 20–45°, so they are not affected.
const alongRunwayMaxAngle = 10.0

// alongRunway reports whether the segment a–b lies on a runway surface and
// runs along it.
func (g *Graph) alongRunway(a, b LatLon) bool {
	for _, r := range g.Layout.Runways {
		aAlong, aOff := g.runwayCoords(r, a)
		bAlong, bOff := g.runwayCoords(r, b)
		if aOff > r.Width/2 || bOff > r.Width/2 || aAlong < 0 || bAlong < 0 || aAlong > r.Length || bAlong > r.Length {
			continue
		}
		dAlong := math.Abs(bAlong - aAlong)
		dOff := math.Abs(bOff - aOff)
		if dAlong < 1e-6 {
			continue
		}
		if math.Atan(dOff/dAlong)*180/math.Pi <= alongRunwayMaxAngle {
			return true
		}
	}
	return false
}
