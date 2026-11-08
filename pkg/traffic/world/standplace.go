package world

import "github.com/mrlm-net/simconnect/pkg/traffic"

// standPlacer is an arrival whose stop on its stand can be moved before it
// lands (traffic.ArrivalController.SetStandPlace).
type standPlacer interface {
	SetStandPlace(nose, side float64) (bool, error)
}

// recheckArrivalStandPlaces moves the stop of an arrival not yet landed to
// where standPlace puts it now. Its place was worked out when it spawned:
// at a jetway gate before the jetways were ready it stood as with no
// jetway, and the helper docked later met a door metres away (live, LKPR
// C22: CSA1894, nose gear 5.4 m into the circle, the "B748 D-A320"
// helper's door 9 m ahead of its own). Runs in the connection goroutine,
// after recheckArrivalStands (a new stand, its own place).
func (cc *controlCenter) recheckArrivalStandPlaces() {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	for _, it := range items {
		if it.arr == nil || it.graph == nil || it.arr.State() > traffic.ArrivalLanding {
			continue
		}
		sp, ok := it.arr.(standPlacer)
		if !ok {
			continue // a director's: placed as it spawned
		}
		it.mu.Lock()
		stand, tail, model := it.stand, it.Tail, it.view.Model
		it.mu.Unlock()
		if stand < 0 || stand >= len(it.graph.Layout.Parking) {
			continue
		}
		nose, side := cc.standPlace(it.graph, stand, model)
		if nose <= 0 {
			continue // the profile's, as it spawned (traffic.ArrivalRequest)
		}
		if changed, err := sp.SetStandPlace(nose, side); err == nil && changed {
			cc.log.printf("%-6s arrival: stop on stand %s now %.2f m in, %.2f m right (its jetway's helper known)", tail, it.graph.Layout.Parking[stand].Label(), nose, side)
		}
	}
}
