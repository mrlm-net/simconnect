package traffic

import "github.com/mrlm-net/simconnect/pkg/types"

// liftsOff tells whether a take-off roll rotating with the on-ground flag
// (strutsAhead) is sent clear of the runway: the height above the ground
// sent, above (its CG height blended, plus the rise of turning on its main
// gear), at or beyond its static CG height on its full struts. From there
// the flag is dropped: the simulator showed the height sent up to about
// that and flipped it airborne beyond, a frame back at its static pitch and
// the on-ground flag flickering for four (live LKPR 24, 2026-11-09, FKUJS
// PC-24 at 110 kt, 2.1° into its rotation: pitch 0.54 → 2.68 → 0.47).
// Without the flag, off its static pitch, the simulator shows the height
// sent, the same as the frame before. i.mu held.
func (i *Injector) liftsOff(o *injected, pose ApproachPose, onGround types.DWORD, above float64) bool {
	if onGround == 0 || !pose.OnGround || !pose.Unpinned || pose.Landing || pose.PitchDeg <= 0 {
		return false
	}
	s := i.strutsOf(o)
	return s > 0 && above >= o.cgFt+s
}
