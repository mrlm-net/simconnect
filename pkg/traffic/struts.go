package traffic

import "math"

// Struts at rotation and as the nose comes down after landing. On the
// ground at exactly its static pitch the simulator shows an aircraft at
// its static CG height on compressed struts; at any other pitch, however
// little, on its extended struts, a fixed height above it whatever height
// was sent (live LKPR 2026-11-07/08, merge1 and push builds: the first
// frame 0.05° off its static pitch at rotation B738 8.48 → 8.97 ft, A320
// 12.25 → 12.87, A321 12.46 → 13.23, A20N 12.00 → 13.13 or 12.07 →
// 12.40, the same back down in the frame the nose came down to it on the
// rollout; 40 of them in three sessions). Between the two, with the flag
// set at its static pitch, it shows the height sent as long as its struts
// reach the ground: up to that height above static (DLH368's A20N, 0.33 ft
// of struts: 799 frames held up to 0.32 ft, every other frame snapped back
// beyond it; ground_reground_test.go). So the struts are let out ahead of
// the rotation, up to strutsMarginFt short of them, and let down after the
// nose came down from there: what is left steps.

const (
	// strutsMarginFt: the struts let out this much short of their full
	// extension, so the simulator never takes the aircraft for airborne
	// (its gear clear of the ground) and snaps it back.
	strutsMarginFt = 0.1
	// strutsRampKts: let out over the last this many knots before the
	// rotation speed.
	strutsRampKts = 12.0
	// strutsLearnMinDeg, strutsLearnMaxDeg: a sample on the ground this far
	// off its static pitch shows its extension, the turn on its gear
	// negligible (0.1° over 20 m is 0.1 ft).
	strutsLearnMinDeg = 0.02
	strutsLearnMaxDeg = 0.12
	// strutsLearnMinKts: only from a roll (rotation, rollout), never a
	// stand or a push.
	strutsLearnMinKts = 40.0
	// strutsMinFt, strutsMaxFt: an extension outside these is not one.
	strutsMinFt = 0.15
	strutsMaxFt = 2.5
)

// strutsSeed: measured extensions (feet) by static CG height (hundredths
// of a foot), the model's fingerprint whatever its livery or title (live
// LKPR 2026-11-07/08, glitch-watch). One ICAO type can be two models of
// very different struts (A20N 1.13 and 0.33), so never by type. Learnt
// ones override these.
var strutsSeed = map[int]float64{
	848:  0.49, // B738
	1225: 0.62, // A320
	1246: 0.77, // A321, A21N
	1200: 1.13, // A20N (FSLTL easyJet)
	1207: 0.33, // A20N (DLH)
	705:  0.42, // BCS3
	490:  0.41, // C56X
	485:  0.41,
	715:  0.30,
	540:  0.26,
	1330: 1.04, // E195
	850:  0.57, // B738 (live THY1039: 8.49 → 9.07 ft as the nose came up)
	342:  0.53, // GA (live OKLOZ: 3.42 → 3.95 ft)
	410:  2.38, // GA (live DZTPU rollout: held 6.48 nose up, 4.10 at static pitch)
}

// strutsKey is a model's fingerprint: its static CG height in hundredths
// of a foot, 0 when not known.
func strutsKey(cgFt float64) int {
	if cgFt <= 0 {
		return 0
	}
	return int(math.Round(cgFt * 100))
}

// noteStruts learns o's struts from a sample g (rest: its height above the
// ground there) on the ground just off its static pitch, for its model.
// i.mu held.
func (i *Injector) noteStruts(o *injected, g *injectGround, rest float64) {
	k := strutsKey(g.CGFt)
	if k == 0 || g.OnGround == 0 || g.GS < strutsLearnMinKts {
		return
	}
	d := math.Abs(g.PlanePitch - g.StaticPitch)
	// Raised by us at its static pitch with the flag and shown back at its
	// static height: beyond its struts' reach. Held short of it from now.
	if sent := o.cgShown - g.CGFt; (o.strutsUp || o.strutsDown) && d < strutsLearnMinDeg && o.sentGround == 1 &&
		sent >= strutsMinFt && rest-g.CGFt < strutsSnappedFt {
		r := math.Max(0, sent-strutsReachBackFt)
		if old, ok := i.reach[k]; !ok || r < old {
			i.reach[k] = r
			i.strutsGen++
		}
		return
	}
	if d < strutsLearnMinDeg || d > strutsLearnMaxDeg {
		return
	}
	if s := rest - g.CGFt; s >= strutsMinFt && s <= strutsMaxFt {
		if old, ok := i.struts[k]; !ok || math.Abs(old-s) >= 0.01 {
			i.strutsGen++
		}
		i.struts[k] = s
	}
}

// strutsOf is o's struts extension: learnt for its model, else seeded
// (within 0.01 ft of its static CG), else 0. i.mu held.
func (i *Injector) strutsOf(o *injected) float64 {
	k := strutsKey(o.cgFt)
	if k == 0 {
		return 0
	}
	if s, ok := i.struts[k]; ok {
		return s
	}
	for _, d := range [...]int{0, -1, 1} {
		if s, ok := strutsSeed[k+d]; ok {
			return s
		}
	}
	return 0
}

// strutsHeld is how far above its static CG height o can be shown on the
// ground at its static pitch, its struts let out short of their full
// extension: 0 when they are not known. i.mu held.
func (i *Injector) strutsHeld(o *injected) float64 {
	h := math.Max(0, i.strutsOf(o)-strutsMarginFt)
	if r, ok := i.reachOf(o); ok {
		h = math.Min(h, r)
	}
	return h
}

// strutsReachSeed: how far above its static CG height a model's struts hold
// it at its static pitch with the flag, when well short of their
// extension (live LKPR 2026-11-08: AFR284, BAW709 A321, 0.77 ft of struts,
// held at 0.62 and back at static beyond; OKBZI CJ3, 0.41 ft, flickering
// between 0.23 and static). By static CG height as strutsSeed; learnt ones
// (noteStruts) override these.
var strutsReachSeed = map[int]float64{
	1246: 0.60, // A321
	485:  0.21, // CJ3
	1330: 0.60, // E195 (LOT452: 1.04 ft of struts, shown at 0.65, back at static from 0.94)
}

const (
	// strutsSnappedFt: shown this close to its static height while raised
	// by us, it snapped back.
	strutsSnappedFt = 0.05
	// strutsReachBackFt: its reach taken this far below where it snapped.
	strutsReachBackFt = 0.05
)

// reachOf is how far o's struts hold it above its static height at its
// static pitch, learnt or seeded; false when not known (their extension
// short of the margin then). i.mu held.
func (i *Injector) reachOf(o *injected) (float64, bool) {
	k := strutsKey(o.cgFt)
	if k == 0 {
		return 0, false
	}
	if r, ok := i.reach[k]; ok {
		return r, true
	}
	for _, d := range [...]int{0, -1, 1} {
		if r, ok := strutsReachSeed[k+d]; ok {
			return r, true
		}
	}
	return 0, false
}

// strutsAhead is how far o is raised on its take-off roll at pose, its
// struts let out over the last strutsRampKts before its rotation speed, 0
// before or when its struts or rotation speed are not known. Once the nose
// comes up, still on the runway: held there, the flag kept on, until it
// lifts off. The simulator takes the flag and height a frame before the
// pitch: dropping both at rotation it showed the aircraft at its static
// height for that frame (live LKPR 24, CSA1883 BCS3: 7.39 → 7.05 → 7.47
// ft; B738s 8.48 → 8.97), and raised to its full extension in that frame,
// still at its static pitch, beyond its struts' reach: back at its static
// height until lift-off (AFR284, BAW709 A321: 13.08 → 12.46). i.mu held.
func (i *Injector) strutsAhead(o *injected, pose ApproachPose) float64 {
	o.strutsUp = false
	if pose.RotateKts <= 0 || pose.Landing {
		return 0
	}
	if pose.PitchDeg > 0 {
		return i.strutsHeld(o)
	}
	f := (pose.GroundSpeedKts - (pose.RotateKts - strutsRampKts)) / strutsRampKts
	r := i.strutsHeld(o) * math.Max(0, math.Min(1, f))
	o.strutsUp = r > 0
	return r
}
