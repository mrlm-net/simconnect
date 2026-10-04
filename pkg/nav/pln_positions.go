//go:build windows
// +build windows

package nav

import (
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// fixLoader is the part of a NavLoader a PLNResolver drives.
type fixLoader interface {
	Free() int
	Request(key FixKey) error
	Handle(msg engine.Message) (NavResult, bool)
	Expire(now time.Time) []NavResult
}

// PLNResolver fills in the positions of a .pln's waypoints that have none
// (the MSFS 2024 layout gives only ident and region, #679): intersections,
// VORs and NDBs through a NavLoader (the facility API), airports through
// the caller's lookup (an airport list, loaded layouts). Start it, feed
// every message to Handle and call Expire now and then until Done.
type PLNResolver struct {
	loader    fixLoader
	plan      *PLNPlan
	airportAt func(icao string) (airport.LatLon, bool)
	queue     []FixKey
	waiting   map[FixKey][]int // a fix → the waypoints at it
	missing   []string
}

// NewPLNResolver resolves plan's positions with loader (NewNavLoader on the
// connection) and airportAt (nil: airports stay without one).
func NewPLNResolver(loader *NavLoader, plan *PLNPlan, airportAt func(icao string) (airport.LatLon, bool)) *PLNResolver {
	return newPLNResolver(loader, plan, airportAt)
}

func newPLNResolver(loader fixLoader, plan *PLNPlan, airportAt func(string) (airport.LatLon, bool)) *PLNResolver {
	return &PLNResolver{loader: loader, plan: plan, airportAt: airportAt, waiting: map[FixKey][]int{}}
}

// plnKind is the facility kind of a .pln waypoint type; false for airports,
// user points and others.
func plnKind(t string) (FixKind, bool) {
	switch strings.ToLower(t) {
	case "intersection", "waypoint":
		return KindWaypoint, true
	case "vor":
		return KindVOR, true
	case "ndb":
		return KindNDB, true
	}
	return 0, false
}

// Start fills the airports and requests the fixes, as many as the loader
// takes at a time (the rest as answers come in).
func (r *PLNResolver) Start() error {
	for i, w := range r.plan.Waypoints {
		if w.Position != (airport.LatLon{}) || w.Ident == "" {
			continue // the file has it, or a user point
		}
		if strings.EqualFold(w.Type, "Airport") {
			if r.airportAt != nil {
				if p, ok := r.airportAt(strings.ToUpper(w.Ident)); ok {
					r.plan.Waypoints[i].Position = p
					continue
				}
			}
			r.missing = append(r.missing, w.Ident)
			continue
		}
		kind, ok := plnKind(w.Type)
		if !ok {
			r.missing = append(r.missing, w.Ident)
			continue
		}
		k := Key(w.Ident, w.Region, kind)
		if _, queued := r.waiting[k]; !queued {
			r.queue = append(r.queue, k)
		}
		r.waiting[k] = append(r.waiting[k], i)
	}
	return r.next()
}

// next requests queued fixes while the loader has room.
func (r *PLNResolver) next() error {
	for len(r.queue) > 0 && r.loader.Free() > 0 {
		k := r.queue[0]
		r.queue = r.queue[1:]
		if err := r.loader.Request(k); err != nil {
			return err
		}
	}
	return nil
}

// Handle feeds msg to the loader; true when it was the loader's.
func (r *PLNResolver) Handle(msg engine.Message) bool {
	res, ok := r.loader.Handle(msg)
	if !ok {
		return false
	}
	r.take(res)
	return true
}

// Expire ends fixes the simulator never answered (missing).
func (r *PLNResolver) Expire(now time.Time) {
	for _, res := range r.loader.Expire(now) {
		r.take(res)
	}
}

func (r *PLNResolver) take(res NavResult) {
	at, ok := r.waiting[res.Key]
	if !ok {
		return
	}
	delete(r.waiting, res.Key)
	for _, i := range at {
		if res.Found {
			r.plan.Waypoints[i].Position = res.Fix.Position
		}
	}
	if !res.Found {
		r.missing = append(r.missing, res.Key.String())
	}
	r.next()
}

// Done reports whether every fix has been answered (found or not).
func (r *PLNResolver) Done() bool { return len(r.waiting) == 0 }

// Missing are the waypoints left without a position: unknown fixes, user
// points, airports the lookup does not know.
func (r *PLNResolver) Missing() []string { return append([]string(nil), r.missing...) }
