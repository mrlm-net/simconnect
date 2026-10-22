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
// full-length departures (measured, 262 runway ends at 40 large airports:
// the nearest entry within 59 m at half of them, 186 m at nine in ten,
// 265 m at 95 in 100; LKPR 24 at 201 m, EDDL 23L at 269 m).
const FullLengthMeters = 300.0

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
	// Entries as far from the threshold (LKPR 24: A and Z both 201 m) by
	// taxiway name, then node: the same order every time.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.FromThreshold != b.FromThreshold {
			return a.FromThreshold < b.FromThreshold
		}
		if a.Taxiway != b.Taxiway {
			return a.Taxiway < b.Taxiway
		}
		return a.Node < b.Node
	})
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
		// The same runway ahead (entries meeting the runway at one node):
		// the nearer one, not whichever came first (#43: LKPR "30 at D"
		// went either way, once across a runway).
		if best == nil || e.Remaining > best.Remaining || e.Remaining == best.Remaining && s.dist[t] < s.dist[target] {
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

// EndEntranceSpanMeters: entries this much further in than the nearest one
// (that one within FullLengthMeters)
// still enter at the runway's end, side by side (EDDM 08L: A1, A2 at 56 m,
// A3 at 146 m); beyond, an intersection.
const EndEntranceSpanMeters = 120.0

// EndEntrances are the entries onto runwayEnd at its end, side by side
// (LKPR 24: A and Z), each with a holding point of its own, one per
// taxiway (the nearest of a name), nearest the
// threshold first: the departures' holding points for full length, each a
// queue of its own. Intersections further in (LKPR 24: B, E) are not.
func (g *Graph) EndEntrances(runwayEnd string) ([]RunwayEntry, error) {
	entries, err := g.RunwayEntries(runwayEnd)
	if err != nil || len(entries) == 0 || entries[0].FromThreshold > FullLengthMeters {
		return nil, err // none at its end: every entry an intersection
	}
	first := entries[0].FromThreshold
	seen := map[string]bool{}
	var out []RunwayEntry
	for _, e := range entries {
		if e.FromThreshold > first+EndEntranceSpanMeters {
			break
		}
		if e.HoldShort < 0 {
			continue // no holding point of its own: no queue (LKPR 12: L, D)
		}
		k := strings.ToUpper(e.Taxiway)
		if k == "" {
			k = fmt.Sprint("#", e.Node)
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out, nil
}
