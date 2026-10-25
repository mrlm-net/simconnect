package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestSpeedSpareCountsItsCost: told 190 kt 40 NM out, an arrival landing
// a minute more than its spacing behind its leader keeps the speed: at its
// normal speed it would land sooner and be short again (peak bed, RYR1616
// slowed and resumed twice, then sent around on the final). With room for
// the speed's cost as well it resumes.
func TestSpeedSpareCountsItsCost(t *testing.T) {
	now := time.Now()
	lead := traffic.SequenceEntry{Callsign: "OKDME", Landing: now}
	e := traffic.SequenceEntry{Callsign: "RYR1616", Leader: "OKDME", SpacingNM: 6, DistanceToGoNM: 40}
	gap := time.Duration(e.SpacingNM / traffic.ProcedureApproachSpeedKts * float64(time.Hour))
	e.Landing = now.Add(gap + resumeAfter + 15*time.Second)
	if speedSpare(e, lead, 190) {
		t.Error("speed resumed with 15 s spare: at its normal speed it lands short again")
	}
	cost := time.Duration(e.DistanceToGoNM * (1/190.0 - 1/traffic.ProcedureSpeedKts) * float64(time.Hour))
	e.Landing = now.Add(gap + resumeAfter + cost + 5*time.Second)
	if !speedSpare(e, lead, 190) {
		t.Errorf("speed kept with %v spare, its cost %v", resumeAfter+cost+5*time.Second, cost)
	}
}

// TestSpeedSpareToTheFinal: the speed's cost counts to the final only
// (speedSpareFinalNM): on it both speeds come down to the approach speed,
// so a slowed arrival with room for the cost of its last 30 NM resumes
// (reviewer: the cost over all 40 NM kept it slow for nothing).
func TestSpeedSpareToTheFinal(t *testing.T) {
	now := time.Now()
	lead := traffic.SequenceEntry{Callsign: "OKDME", Landing: now}
	e := traffic.SequenceEntry{Callsign: "RYR1616", Leader: "OKDME", SpacingNM: 6, DistanceToGoNM: 40}
	gap := time.Duration(e.SpacingNM / traffic.ProcedureApproachSpeedKts * float64(time.Hour))
	cost := time.Duration((e.DistanceToGoNM - speedSpareFinalNM) * (1/190.0 - 1/traffic.ProcedureSpeedKts) * float64(time.Hour))
	e.Landing = now.Add(gap + resumeAfter + cost + 2*time.Second)
	if !speedSpare(e, lead, 190) {
		t.Errorf("speed kept with room for its cost to the final (%v)", cost)
	}
	e.DistanceToGoNM = 8 // on the final: no cost
	e.Landing = now.Add(gap + resumeAfter + time.Second)
	if !speedSpare(e, lead, 190) {
		t.Error("on the final the speed costs nothing more")
	}
}
