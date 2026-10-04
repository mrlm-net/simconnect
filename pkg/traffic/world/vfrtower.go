//go:build windows
// +build windows

package world

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// circuitPlace is a VFR arrival's place in the landing sequence as the
// tower gives it on its downwind report (#569, Doc 4444 12.3.4.14 b): its
// number and the traffic it follows as said, "the Airbus A320 on 4 mile
// final" ("" when number 1), and its call sign; 0 when it is not sequenced.
func (it *controlled) circuitPlace() (int, string, string) {
	if it.cc.sequencesAt == nil {
		return 0, "", ""
	}
	for _, seq := range it.cc.sequencesAt(it.ICAO) {
		for _, e := range seq {
			if e.Callsign != it.Tail {
				continue
			}
			if e.Leader == "" {
				return e.Number, "", ""
			}
			typ, inCircuit := "aircraft", false
			if l := it.cc.byTail(e.Leader); l != nil {
				l.mu.Lock()
				typ, inCircuit = typeSaid(traffic.ProfileFor(l.view.Model).Type), l.circuit != nil
				l.mu.Unlock()
			}
			where := ""
			for _, x := range seq {
				if x.Callsign != e.Leader {
					continue
				}
				switch d := x.DistanceToGoNM; {
				case d <= 1.5:
					where = " on short final"
				case inCircuit:
					where = " in the circuit"
				case d <= 12:
					where = fmt.Sprintf(" on %.0f mile final", d)
				}
			}
			return e.Number, "the " + typ + where, e.Leader
		}
	}
	return 0, "", ""
}

// fnv32 is a small stable hash of s (a call sign: the same choice each time).
func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h
}

// offsetLatLon is p moved meters along true bearing brg.
func offsetLatLon(p airport.LatLon, brg, meters float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, brg, meters)
	return airport.LatLon{Lat: lat, Lon: lon}
}
