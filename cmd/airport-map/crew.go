//go:build windows
// +build windows

package main

import (
	"math/rand/v2"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The crew decides on its own (#621): our arrivals on final go around
// without being told when the landing clearance has not come by their
// decision point, and now and then from an approach that is not stable.
// The tower acknowledges and the arrival is sequenced again, as after a
// go-around the tower orders. Not with held gates: there the user is the
// tower and gives the landing clearances.
const (
	// crewDecisionNM: not cleared to land this close to the threshold
	// (about 200 ft on a 3° path), the crew goes around.
	crewDecisionNM = 0.6
	// crewUnstableShare: the share of approaches the crew finds not stable
	// and goes around from, decided once per approach.
	crewUnstableShare = 0.01
)

// crewDecides lets the crews of icao's arrivals in list decide.
func (t *towers) crewDecides(icao string, list []traffic.RunwayUser, ours map[string]*controlled) {
	for _, u := range list {
		if !u.Arrival || u.Other || u.Phase != traffic.RunwayFinal || !u.Established || u.DistanceNM > crewDecisionNM {
			continue
		}
		it := ours[u.Callsign]
		if it == nil || it.arr == nil || it.gates.Load() {
			continue
		}
		cs := u.Callsign
		t.mu.Lock()
		cleared, sent, checked := t.given[cs+" land"], t.given[cs+" goaround"], t.given[cs+" crew"]
		t.given[cs+" crew"] = true // the stability is judged once an approach
		t.mu.Unlock()
		if sent {
			continue
		}
		why := ""
		switch {
		case !cleared:
			why = "no landing clearance"
		case !checked && rand.Float64() < crewUnstableShare:
			why = "approach not stable"
		}
		if why == "" {
			continue
		}
		t.mu.Lock()
		t.given[cs+" goaround"] = true
		t.mu.Unlock()
		tlog.printf("%-6s crew: going around — %s", cs, why)
		it.say(traffic.GoingAround(cs))
		go func() {
			if err := t.cc.do(func() error { return it.act("goaround", 0) }); err != nil {
				tlog.printf("%-6s crew go-around: %v", cs, err)
			}
		}()
		it.call(traffic.PosTower, prioUrgent, func() { it.say(traffic.Acknowledge(traffic.PosTower, cs)) })
	}
}
