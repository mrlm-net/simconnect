//go:build windows
// +build windows

package main

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// circuitPlace is a VFR arrival's place in the landing sequence as the
// tower gives it on its downwind report (#569, Doc 4444 12.3.4.14 b): its
// number and the traffic it follows as said, "the Airbus A320 on 4 mile
// final" ("" when number 1); 0 when it is not sequenced.
func (it *controlled) circuitPlace() (int, string) {
	if it.cc.sequencesAt == nil {
		return 0, ""
	}
	for _, seq := range it.cc.sequencesAt(it.ICAO) {
		for _, e := range seq {
			if e.Callsign != it.Tail {
				continue
			}
			if e.Leader == "" {
				return e.Number, ""
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
			return e.Number, "the " + typ + where
		}
	}
	return 0, ""
}

// touchAndGosFor is how many touch-and-goes a scheduled VFR arrival makes
// (#569): by its call sign, two in five fly two to four training circuits,
// the rest land straight away.
func touchAndGosFor(cs string) int {
	h := 0
	for _, r := range cs {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	if h%5 >= 2 {
		return 0
	}
	return 2 + h%3
}
