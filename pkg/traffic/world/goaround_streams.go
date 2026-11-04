package world

import (
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// goAround sends it (an arrival) around, its circuit on the side of the
// runway the other arrivals coming in to it fly through least (#1458;
// offline LKPR 06: DLH299 on VLM6T sent round on the south side into
// TVS947 coming in on APRA2T from the south).
func (cc *controlCenter) goAround(it *controlled) error {
	streams := cc.arrivalStreams(it)
	it.arr.SetGoAroundStreams(streams)
	cc.log.printf("%-6s go-around: %d other arrivals coming in for its circuit side", it.Tail, len(streams))
	return it.arr.GoAround()
}

// arrivalStreams are the ways ahead of our other arrivals on their
// procedures to me's runway end.
func (cc *controlCenter) arrivalStreams(me *controlled) [][]airport.LatLon {
	p := me.arr.Plan()
	if p == nil {
		return nil
	}
	var out [][]airport.LatLon
	for _, it := range cc.snapshotItems() {
		if it == me || it.arr == nil || it.ICAO != me.ICAO || it.finished.Load() || it.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		if q := it.arr.Plan(); q == nil || q.End.Name != p.End.Name {
			continue
		}
		plan := it.arr.ProcedurePlan()
		if len(plan) < 2 {
			continue
		}
		route := make([]airport.LatLon, len(plan))
		for i, r := range plan {
			route[i] = r.Position
		}
		out = append(out, route)
	}
	return out
}
