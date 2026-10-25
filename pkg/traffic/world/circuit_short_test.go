package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestFinalClearBehind: a VFR circuit arrival's downwind goes out to
// CircuitWideExtendNM only while no arrival landing after it is within
// circuitWideClearNM: its base would cross the final in front of that one.
func TestFinalClearBehind(t *testing.T) {
	vfr := traffic.SequenceEntry{Callsign: "OKCMZ", Number: 2, DistanceToGoNM: 6}
	far := traffic.SequenceEntry{Callsign: "TVS917", Number: 3, DistanceToGoNM: 25}
	ahead := traffic.SequenceEntry{Callsign: "CSA1154", Number: 1, DistanceToGoNM: 4}
	if !finalClearBehind(vfr, []traffic.SequenceEntry{ahead, vfr, far}) {
		t.Error("the one behind 25 NM out: the final is clear")
	}
	near := traffic.SequenceEntry{Callsign: "TVS917", Number: 3, DistanceToGoNM: 8}
	if finalClearBehind(vfr, []traffic.SequenceEntry{ahead, vfr, near}) {
		t.Error("the one behind 8 NM out: its base would cross in front of it")
	}
}

// TestLeaderTakes: a VFR in the circuit with time left over behind an IFR
// arrival still far out lands ahead of it, the IFR losing the time with
// speed (cheaper than an orbit, which is never given); not ahead of one on
// the final (fixed) nor one too close to lose it.
func TestLeaderTakes(t *testing.T) {
	now := time.Now()
	lead := traffic.SequenceEntry{Callsign: "TVS917", Number: 1, DistanceToGoNM: 40, Landing: now.Add(12 * time.Minute)}
	vfr := traffic.SequenceEntry{Callsign: "OKCMZ", Number: 2, Leader: "TVS917", DistanceToGoNM: 6, Landing: now.Add(15 * time.Minute)}
	cs, need, ok := leaderTakes(vfr, []traffic.SequenceEntry{lead, vfr}, 4*time.Minute)
	if !ok || cs != "TVS917" || need != 2*time.Minute {
		t.Errorf("leader takes %q %v %v, want TVS917 losing 2m0s", cs, need, ok)
	}
	lead.Fixed = true
	if _, _, ok := leaderTakes(vfr, []traffic.SequenceEntry{lead, vfr}, 4*time.Minute); ok {
		t.Error("ahead of an established arrival")
	}
	lead.Fixed, lead.DistanceToGoNM = false, 11
	if _, _, ok := leaderTakes(vfr, []traffic.SequenceEntry{lead, vfr}, 4*time.Minute); ok {
		t.Error("ahead of one 11 NM out: no room for the speed")
	}
}
