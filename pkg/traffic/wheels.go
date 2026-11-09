package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// wheelSpin gives the speed passed on INITPOSITION so an injected object's
// wheels turn.
//
// The simulator turns the wheels from INITPOSITION Airspeed, which is a
// DWORD (no negative speeds; -1 and -2 are special values): any speed
// passed spins them forward. Pushed back tail first, or a tug backing off
// the nose, they spin forward too: turning the wrong way looked better than
// standing still (user, live LKPR 2026-11-09: "CSA is not spinning wheels
// while pushed back"; 2026-11-07 they had been held still backwards).
type wheelSpin struct{}

// airspeed is the INITPOSITION Airspeed for an object moving at kts,
// whichever way: kts rounded.
func (wheelSpin) airspeed(_ airport.LatLon, _, kts float64) types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED {
	return types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(math.Max(0, kts)))
}
