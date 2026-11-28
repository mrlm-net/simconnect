package world

import (
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// plainExit is whether arrival it is planned off its runway by an exit
// ahead, not back along it (ArrivalPlan.EndTurn, Backtrack): its way off
// is that exit (#1719).
func plainExit(it *controlled) bool {
	plan := it.arr.Plan()
	return plan != nil && !(plan.EndTurn || plan.Backtrack)
}

// lookable is whether arrival it's way off is looked at (keepWayOffClear):
// before it touches down, and one off by an exit ahead on its roll too,
// which may still be sent off by another one ahead (#1719).
func lookable(it *controlled) bool {
	if beforeTouchdown(it) {
		return true
	}
	return it.arr.State() == traffic.ArrivalRollout && plainExit(it)
}

// takenFor is where arrival it's way off must keep clear of: where
// departures hold short already for one off by an exit ahead (plain; one
// taxiing there stops short of it, GroundPicture.ReportWayOff), where they
// hold short or will otherwise (wayOffTaken).
func takenFor(it *controlled, items []*controlled, plain bool) []airport.Occupied {
	if plain {
		return holdingTaken(it, items)
	}
	return wayOffTaken(it, items)
}

// holdingTaken are the places departures at it's airport hold short.
func holdingTaken(it *controlled, items []*controlled) []airport.Occupied {
	var occ []airport.Occupied
	for _, o := range items {
		if o == it || o.dep == nil || o.ICAO != it.ICAO || o.dep.State() != traffic.TaxiHoldingShort {
			continue
		}
		o.mu.Lock()
		pos, model := o.view.Position, o.view.Model
		o.mu.Unlock()
		if pos == (airport.LatLon{}) {
			continue
		}
		occ = append(occ, airport.Occupied{Points: []airport.LatLon{pos}, HalfSpan: traffic.MotionProfileFor(model).SpanMeters / 2})
	}
	return occ
}
