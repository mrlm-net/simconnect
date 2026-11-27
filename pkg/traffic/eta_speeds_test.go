package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestETAFinalSpeeds: an arrival is predicted at its own speed to the join
// and at the injected final's speeds from there, as the ApproachMover flies
// them: a TBM9 18 NM out at 180 kt was predicted 1m20s early on an average
// of 180 and its approach speed over the last 10 NM, and the B738 behind it
// planned that much too close (offline LKPR, OKBJZ and LOT229). Its chain
// slows it before the join (#1682): from the final's outer end (FinalNM)
// to the join at about the align and join points' speeds, not 180 kt.
func TestETAFinalSpeeds(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{})
	now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	for _, model := range []string{"FSLTL_GA_TBM9_ZZZ", "FSLTL_B738_RYR"} {
		prof := ProfileFor(model)
		p := withDefaults(prof.Approach, DefaultApproachProfile())
		flown := NewApproachMover(airport.LatLon{Lat: 50.1, Lon: 14.26}, 244, ProcedureJoinNm*1852, p).timeToGo()
		for _, dtg := range []float64{18, 8} {
			want := flown
			if dtg > ProcedureJoinNm {
				_, align, join := ChainSpeeds(p.StartKts, p.TransitionMaxKts)
				outer := math.Max(0, dtg-10)
				want += time.Duration((outer/180 + (dtg-outer-ProcedureJoinNm)/((align+join)/2)) * float64(time.Hour))
			}
			got := s.eta(now, ApproachAircraft{Callsign: "X", DistanceToGoNM: dtg, GroundKts: 180, FinalKts: prof.Approach.ApproachKts, TopKts: prof.Approach.TransitionMaxKts}, ApproachConditions{}).Sub(now)
			if d := got - want; d < -15*time.Second || d > 15*time.Second {
				t.Errorf("%s %.0f NM out at 180 kt: predicted %s, flown %s", model, dtg, got.Round(time.Second), want.Round(time.Second))
			}
		}
	}
}
