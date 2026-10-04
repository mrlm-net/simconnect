package airport

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

// RunwayEntry is a taxiway onto a runway for aircraft departing from a given
// runway end — the "B" in "runway 24 at B" (#306).
type RunwayEntry struct {
	// RunwayNode is where the entry meets the runway centreline; Node is the
	// last taxiway node before the runway surface. Path runs from Node to
	// RunwayNode.
	RunwayNode NodeID   `json:"runwayNode"`
	Node       NodeID   `json:"node"`
	Path       []NodeID `json:"path"`
	// FromThreshold is the distance from the departure threshold to
	// RunwayNode; Remaining is the runway length left ahead from there (the
	// take-off run available from this entry), in meters.
	FromThreshold float64 `json:"fromThreshold"`
	Remaining     float64 `json:"remaining"`
	// Angle is the turn from the entry taxiway onto the runway heading,
	// 0–MaxEntryAngle; entries needing a sharper turn are left out.
	Angle float64 `json:"angle"`
	// Taxiway is the entry taxiway name, "" if unnamed.
	Taxiway string `json:"taxiway"`
	// HoldShort is the hold-short node on the entry, -1 if none was found.
	HoldShort NodeID `json:"holdShort"`
}

// FullLengthMeters: entries within this distance of the threshold count as
// full-length departures.
const FullLengthMeters = 150.0

// RunwayEntries returns the entries onto runwayEnd for departures, nearest
// the threshold (full length) first. An entry onto runway 24 is an exit for
// aircraft landing on 06 driven backwards, and its turn onto the runway is
// that exit's turn-off angle; entries pointing back along the runway (more
// than MaxEntryAngle) are left out.
func (g *Graph) RunwayEntries(runwayEnd string) ([]RunwayEntry, error) {
	rwy, end, ok := g.Layout.RunwayEnd(runwayEnd)
	if !ok {
		return nil, fmt.Errorf("%w: %q at %s", ErrUnknownRunway, runwayEnd, g.Layout.ICAO)
	}
	opposite := rwy.Primary.Name
	if end.Name == rwy.Primary.Name {
		opposite = rwy.Secondary.Name
	}
	exits, err := g.turnoffs(opposite, MaxEntryAngle)
	if err != nil {
		return nil, err
	}
	out := make([]RunwayEntry, 0, len(exits))
	for _, x := range exits {
		path := make([]NodeID, len(x.Path))
		for i, id := range x.Path {
			path[len(path)-1-i] = id
		}
		out = append(out, RunwayEntry{
			RunwayNode: x.RunwayNode, Node: x.Node, Path: path,
			FromThreshold: math.Max(0, rwy.Length-x.Along), Remaining: x.Along,
			Angle: x.Angle, Taxiway: x.Taxiway, HoldShort: x.HoldShort,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FromThreshold < out[j].FromThreshold })
	return out, nil
}

// RouteToRunwayEntry returns a departure route from a parking spot to the
// hold-short of runwayEnd at the entry taxiway named entry ("24 at B"). An
// empty entry means full length: the same as RouteToRunway. When several
// entries share the name, the reachable one with the most runway ahead wins.
// It returns ErrUnknownEntry when no entry of that name exists.
func (g *Graph) RouteToRunwayEntry(parking int, runwayEnd, entry string, opts RouteOptions) (*Route, error) {
	if entry == "" {
		return g.RouteToRunway(parking, runwayEnd, opts)
	}
	from, ok := g.ParkingNode(parking)
	if !ok {
		return nil, fmt.Errorf("%w: index %d", ErrUnknownParking, parking)
	}
	opts.OwnStands = append(slices.Clone(opts.OwnStands), parking)
	r, err := g.fitOrTight(opts, func(o RouteOptions) (*Route, error) { return g.entryRoute(from, -1, runwayEnd, entry, o) })
	if errors.Is(err, ErrNoRoute) {
		return nil, fmt.Errorf("%w from parking %d", err, parking)
	}
	return r, err
}

// entryRoute is RouteToRunwayEntry from a node reached via prev.
func (g *Graph) entryRoute(from, prev NodeID, runwayEnd, entry string, opts RouteOptions) (*Route, error) {
	rwy, end, _ := g.Layout.RunwayEnd(runwayEnd)
	entries, err := g.RunwayEntries(runwayEnd)
	if err != nil {
		return nil, err
	}
	s := g.shortestPaths(from, prev, opts)
	var best *RunwayEntry
	var target NodeID
	for i := range entries {
		e := &entries[i]
		if !strings.EqualFold(e.Taxiway, entry) {
			continue
		}
		t := e.HoldShort
		if t < 0 {
			t = e.Node
		}
		if math.IsInf(s.dist[t], 1) {
			continue
		}
		if best == nil || e.Remaining > best.Remaining {
			best, target = e, t
		}
	}
	if best == nil {
		if err := s.failure(); err != nil {
			return nil, err
		}
		for _, e := range entries {
			if strings.EqualFold(e.Taxiway, entry) {
				return nil, fmt.Errorf("%w: runway %s at %s", ErrNoRoute, end.Name, entry)
			}
		}
		return nil, fmt.Errorf("%w: %s at %s", ErrUnknownEntry, end.Name, entry)
	}
	r, err := s.route(g, target)
	if err != nil {
		return nil, err
	}
	r.Runway, r.RunwayEnd, r.Entry = rwy.Name(), end.Name, best.Taxiway
	return r, nil
}
