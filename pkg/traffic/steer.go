package traffic

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Nose wheel steering (#1446). Models animate the nose wheel's steering from
// the aircraft's yaw rate (ROTATION VELOCITY BODY Y; FSLTL: the
// FSLTL_Steering and FSLTL_Alt_Steering templates), as MSFS AI steering
// itself sets it. Ours are placed frame by frame and the simulator shows
// them at a yaw rate of 0: the nose wheel stayed straight through every
// turn, the aircraft sliding round it (live LKPR 2026-11-04). So each
// placement on the ground writes the yaw rate it turns at, as it moves:
// never more than its main wheels roll with (the simulator turns each
// wheel from the yaw rate too), none standing still. Written as a nose
// wheel angle's rate at a fixed taxi speed, and held standing, it was more
// than the aircraft turned: the wheels on the inside of the turn stood
// still or turned back, and the nose came up (live LKPR, CSA1822 BCS3).
const (
	// steerSmoothSec: the yaw rate follows the turn this quickly (the
	// aircraft's own heading steps frame by frame).
	steerSmoothSec = 0.25
	// steerMinMoveM: the yaw rate is measured once moved this far.
	steerMinMoveM = 0.02
	// steerHalfTrackM: the yaw rate is kept under the speed over this
	// (about half the widest main gear track): the main wheels on the
	// inside of the turn never stand or turn back.
	steerHalfTrackM = 6.0
	// steerWriteScale: the simulator stores a written ROTATION VELOCITY
	// BODY Y this many times what was written (live: 0.05 rad/s read back
	// 0.164), so what is written is the yaw rate over it.
	steerWriteScale = 1 / 0.3048
	// steerSendDelta: a yaw rate is written again once it changed this
	// much (rad/s); one that is not written falls back to 0.
	steerSendDelta = 0.0005
)

// nosewheel is a ground placement's nose wheel steering: the yaw rate
// (rad/s, right positive, the steering's way: reversed on a push) it goes
// to and is at, what was last written, and where the aircraft last moved
// from (ref): measured from there, not frame to frame, a push at 2 kt
// moved under steerMinMoveM a frame and never steered.
type nosewheel struct {
	target, yaw float64
	written     float64
	at          time.Time
	ref         airport.LatLon
	refHdg      float64
	refAt       time.Time
}

// steerYaw is the yaw rate (rad/s, right positive) of an aircraft moved
// from (from, fromHdg) to (to, hdg) in dt seconds, the way its nose wheel
// steers it: reversed backwards (a push turning right steers left), at
// most its speed over steerHalfTrackM. False when it did not move.
func steerYaw(from airport.LatLon, fromHdg float64, to airport.LatLon, hdg, dt float64) (float64, bool) {
	d := localDist(from, to)
	if d < steerMinMoveM || dt <= 0 {
		return 0, false
	}
	along := d * math.Cos((localBearing(from, to)-hdg)*math.Pi/180) // negative backwards
	if math.Abs(along) < steerMinMoveM {
		return 0, false
	}
	rate := math.Remainder(hdg-fromHdg, 360) * math.Pi / 180 / dt
	limit := math.Abs(along) / dt / steerHalfTrackM
	rate = math.Max(-limit, math.Min(limit, rate))
	if along < 0 {
		rate = -rate
	}
	return rate, true
}

// update is the yaw rate to write for a placement at pos, heading hdg,
// kts, at now, ok when it is to be written now: the rate it turned at since
// it last moved, eased in; 0 at once placed standing (under steerStandKts,
// or not moved for steerStandAfter).
func (n *nosewheel) update(pos airport.LatLon, hdg, kts float64, now time.Time) (float64, bool) {
	dt := 0.0
	if !n.at.IsZero() {
		dt = math.Max(0, now.Sub(n.at).Seconds())
	}
	n.at = now
	stood := false
	switch since := now.Sub(n.refAt); {
	case n.refAt.IsZero() || since > steerResetAfter || localDist(n.ref, pos) > steerResetM:
		n.ref, n.refHdg, n.refAt, n.target = pos, hdg, now, 0 // placed anew
		stood = true
	case localDist(n.ref, pos) >= steerMinMoveM:
		y, _ := steerYaw(n.ref, n.refHdg, pos, hdg, since.Seconds())
		n.ref, n.refHdg, n.refAt, n.target = pos, hdg, now, y
	case since >= steerStandAfter:
		stood = true
	}
	if math.Abs(kts) < steerStandKts {
		stood = true
	}
	switch {
	case stood:
		n.target, n.yaw = 0, 0
	case dt > 0 && dt < steerSmoothSec:
		n.yaw += dt / steerSmoothSec * (n.target - n.yaw)
	default:
		n.yaw = n.target
	}
	v := n.yaw / steerWriteScale
	if math.Abs(v) < steerSendDelta {
		v = 0 // straight: written once, then left at 0
	}
	if v == 0 && n.written == 0 {
		return 0, false
	}
	n.written = v // turning: every frame, or it falls back to 0
	return v, true
}

// steerStandAfter: moved less than steerMinMoveM for this long, or placed
// under steerStandKts, the aircraft stands. steerResetAfter, steerResetM: not placed for this long,
// or placed this far from where it last moved, it starts again.
const (
	steerStandAfter = 500 * time.Millisecond
	steerStandKts   = 0.1
	steerResetAfter = 2 * time.Second
	steerResetM     = 50.0
)

// handOff ends the steering as the aircraft leaves the ground placements
// (PlaceAir: the take-off roll, the air): true when a yaw rate other than 0
// was last written, so 0 is written once; placed on the ground again it
// starts anew. No yaw rate is written on the roll or in the air.
func (n *nosewheel) handOff() bool {
	straighten := n.written != 0
	*n = nosewheel{}
	return straighten
}
