package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// wheelsDecideM: how far an aircraft must have moved since the last decision
// before its direction of travel is judged again. Below it, position rounding
// and a heading swing about the reference point could read either way.
const wheelsDecideM = 0.03

// wheelsJumpM: a move longer than this between placements is a jump (a
// new spawn, a teleport), not travel: the direction is judged afresh.
const wheelsJumpM = 30.0

// wheelSpin tracks which way an injected object travels, from how its
// placed position moves against its heading, so the speed passed on
// INITPOSITION spins its wheels the right way.
//
// The simulator turns the wheels from INITPOSITION Airspeed, which is a
// DWORD (no negative speeds; -1 and -2 are special values): any speed
// passed spins them forward. Pushed back tail first, forward-spinning
// wheels look wrong (user report 2026-11-07), so backwards the speed passed
// is 0 and the wheels stand still.
type wheelSpin struct {
	anchor   airport.LatLon // where the last decision was made
	have     bool
	backward bool
}

// airspeed is the INITPOSITION Airspeed for an object placed at pos facing
// hdg (true degrees) at kts: kts rounded, 0 while it moves backwards.
func (w *wheelSpin) airspeed(pos airport.LatLon, hdg, kts float64) types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED {
	if !w.have {
		w.anchor, w.have = pos, true
	} else if d := localDist(w.anchor, pos); d > wheelsJumpM {
		w.anchor, w.backward = pos, false
	} else if d >= wheelsDecideM {
		along := d * math.Cos((localBearing(w.anchor, pos)-hdg)*math.Pi/180)
		if math.Abs(along) >= d/2 { // mostly along the fuselage, not sideways
			w.backward = along < 0
		}
		w.anchor = pos
	}
	if w.backward {
		return 0
	}
	return types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(math.Max(0, kts)))
}
