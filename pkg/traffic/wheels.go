package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// wheelSpin gives the speed passed on INITPOSITION so an injected object's
// wheels turn, and the backwards speed written with it on a push.
//
// The simulator turns the wheels from INITPOSITION Airspeed, which is a
// DWORD (no negative speeds; -1 and -2 are special values): any speed
// passed spins them forward. The models animate the wheels from VELOCITY
// BODY Z (FSLTL_Wheel_Roll_Center/Main, the GSX vehicles alike), and a
// negative one written is kept (live, EZY311 2026-11-07: -1.2 m/s read
// back the whole push). So travelling backwards (a push tail first, a tug
// backing off the nose) Airspeed is 0 and VELOCITY BODY Z is written
// minus the ground speed with every placement: the wheels turn backwards,
// not forwards (user, live LKPR 2026-11-11: "wheels are not spinning
// properly on plane during pushback"; 2026-11-09 they spun forward,
// 2026-11-07 they stood still).
type wheelSpin struct {
	// ref: where it was when its way was last measured; back: it
	// travels backwards; written: a backwards speed was last written.
	ref     airport.LatLon
	haveRef bool
	back    bool
	written bool
}

// wheelMinMoveM: the way it travels is measured once moved this far.
const wheelMinMoveM = 0.05

// spin is the INITPOSITION Airspeed for an object at pos, heading hdg,
// moving at kts, and the VELOCITY BODY Z (m/s) to write with it, write
// false for none: minus its speed backwards every placement, 0 once when it
// stops or turns forward again.
func (w *wheelSpin) spin(pos airport.LatLon, hdg, kts float64) (types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED, float64, bool) {
	switch {
	case !w.haveRef:
		w.ref, w.haveRef = pos, true
	case localDist(w.ref, pos) > 50: // placed anew
		w.ref, w.back = pos, false
	case localDist(w.ref, pos) >= wheelMinMoveM:
		along := localDist(w.ref, pos) * math.Cos((localBearing(w.ref, pos)-hdg)*math.Pi/180)
		w.back = along < 0
		w.ref = pos
	}
	// Slowed (into the tug's swing) it keeps its way, and backwards any
	// speed turns the wheels back: a forward Airspeed of 1 at 0.6 kt spun
	// them forward. Stopped, 0 is written once.
	if w.back && kts > 0 {
		w.written = true
		return 0, -kts * 0.514444, true
	}
	if w.written {
		w.written = false
		return types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(math.Max(0, kts))), 0, true
	}
	return types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(math.Max(0, kts))), 0, false
}

// end is whether a backwards speed was last written (0 is to be written
// once), as the object leaves the ground placements.
func (w *wheelSpin) end() bool {
	was := w.written
	*w = wheelSpin{}
	return was
}
