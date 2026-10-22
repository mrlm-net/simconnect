package world

import (
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Composition first: a shot is a slow glide between two frames picked from
// the composition sweep (every move at three distances and three moments,
// held still and looked at), never a move found only in motion. A frame is
// named <move>_dist<back>_t<moment>: revealRise_dist1.6_t50 is the middle
// of revealRise, 1.6 times farther from what it looks at.

// sceneCompositions are the frames picked (LKPR C30, an A320, 2026-10-22);
// the close details (cockpit, lights, main gear, hero push-in, head-on) were
// all too tight.
var sceneCompositions = []string{
	"chaseRise_dist1.6_t00", "chaseRise_dist1.6_t100", "chaseRise_dist1.6_t50", "chaseRise_dist1_t00",
	"chaseRise_dist1_t100", "chaseRise_dist1_t50", "chaseRise_dist2.4_t00", "chaseRise_dist2.4_t100",
	"chaseRise_dist2.4_t50", "crane_dist1.6_t00", "crane_dist1.6_t100", "crane_dist1.6_t50",
	"crane_dist1_t00", "crane_dist1_t100", "crane_dist1_t50", "crane_dist2.4_t00",
	"crane_dist2.4_t100", "crane_dist2.4_t50", "engine_dist1.6_t00", "flyover_dist1.6_t00",
	"flyover_dist1.6_t100", "flyover_dist1.6_t50", "flyover_dist1_t00", "flyover_dist1_t100",
	"flyover_dist1_t50", "flyover_dist2.4_t00", "flyover_dist2.4_t50", "leadChase_dist1.6_t00",
	"noseGear_dist1.6_t00", "noseGear_dist2.4_t00", "noseGear_dist2.4_t100", "noseGear_dist2.4_t50",
	"parallaxTrack_dist1.6_t50", "parallaxTrack_dist2.4_t00", "parallaxTrack_dist2.4_t100", "parallaxTrack_dist2.4_t50",
	"pullBackReveal_dist1.6_t00", "pullBackReveal_dist1_t100", "pullBackReveal_dist2.4_t00", "pullBackReveal_dist2.4_t100",
	"pullBackReveal_dist2.4_t50", "revealRise_dist1.6_t00", "revealRise_dist1.6_t100", "revealRise_dist1.6_t50",
	"revealRise_dist1_t00", "revealRise_dist1_t100", "revealRise_dist1_t50", "revealRise_dist2.4_t00",
	"revealRise_dist2.4_t100", "revealRise_dist2.4_t50", "sideDolly_dist1.6_t00", "sideDolly_dist1.6_t100",
	"sideDolly_dist1.6_t50", "sideDolly_dist1_t00", "sideDolly_dist1_t100", "sideDolly_dist1_t50",
	"sideDolly_dist2.4_t00", "sideDolly_dist2.4_t100", "sideDolly_dist2.4_t50", "spiralDescend_dist1.6_t00",
	"spiralDescend_dist1.6_t100", "spiralDescend_dist1.6_t50", "spiralDescend_dist1_t00", "spiralDescend_dist1_t100",
	"spiralDescend_dist1_t50", "spiralDescend_dist2.4_t00", "spiralDescend_dist2.4_t100", "spiralDescend_dist2.4_t50",
	"tail_dist1.6_t00", "tail_dist1.6_t100", "tail_dist1.6_t50", "tail_dist1_t00",
	"tail_dist1_t100", "tail_dist1_t50", "tail_dist2.4_t00", "tail_dist2.4_t100",
	"tail_dist2.4_t50", "topDown_dist1.6_t00", "topDown_dist1.6_t100", "topDown_dist1.6_t50",
	"topDown_dist1_t00", "topDown_dist1_t100", "topDown_dist1_t50", "topDown_dist2.4_t00",
	"topDown_dist2.4_t100", "topDown_dist2.4_t50", "topOrbit_dist1.6_t00", "topOrbit_dist1.6_t100",
	"topOrbit_dist1.6_t50", "topOrbit_dist1_t00", "topOrbit_dist1_t100", "topOrbit_dist1_t50",
	"topOrbit_dist2.4_t00", "topOrbit_dist2.4_t100", "topOrbit_dist2.4_t50", "wingtip_dist1.6_t00",
	"wingtip_dist1.6_t100", "wingtip_dist1.6_t50", "wingtip_dist1_t00", "wingtip_dist1_t100",
	"wingtip_dist1_t50", "wingtip_dist2.4_t00", "wingtip_dist2.4_t100", "wingtip_dist2.4_t50",
}

// glideSpeed is the fastest a glide moves the camera (m/s, its mean): slow,
// a film camera, not a drone; a glide longer than that in a shot is not
// played.
const glideSpeed = 1.2

// composition is one frame: a move, its distance (1 as the move is
// written) and its moment (0 its start, 1 its end).
type composition struct {
	move     string
	back, at float64
}

// parseComposition reads <move>_dist<back>_t<00|50|100>.
func parseComposition(s string) (composition, bool) {
	move, rest, ok := strings.Cut(s, "_dist")
	if !ok {
		return composition{}, false
	}
	back, t, ok := strings.Cut(rest, "_t")
	if !ok {
		return composition{}, false
	}
	b, err1 := strconv.ParseFloat(back, 64)
	at, err2 := strconv.ParseFloat(t, 64)
	if err1 != nil || err2 != nil || b <= 0 {
		return composition{}, false
	}
	return composition{move, b, at / 100}, true
}

// dollyTo is the To of a dolly: the frame itself slid along the aircraft,
// the camera beside it tracking (from a side frame only).
const dollyTo = "dolly"

// dollyAlong is how far a dolly slides, in lengths of the aircraft (an
// A320: some 5 m over a shot, 0.5 m/s).
const dollyAlong = 0.14

// compositionGlides are the glides between picked frames: a dolly along
// each side frame, one moment on along the same move, one distance step in
// at the same moment; two dollies to each other glide. Never from behind:
// a frame with the camera behind the tail is left out.
var compositionGlides = func() [][2]string {
	use := map[string]bool{}
	var side []string
	for _, c := range sceneCompositions {
		p, ok := nominalPose(c)
		if !ok {
			continue
		}
		e, t := p.Eye.Offset, p.Target.Offset
		if e.Forward < t.Forward-0.4*nominalSize.Length {
			continue // from behind
		}
		use[c] = true
		if math.Abs(e.Right-t.Right) >= math.Abs(e.Forward-t.Forward) {
			side = append(side, c)
		}
	}
	moments := []string{"00", "50", "100"}
	dists := []string{"1", "1.6", "2.4"}
	var moves [][2]string
	for _, c := range sceneCompositions {
		if !use[c] {
			continue
		}
		move, rest, _ := strings.Cut(c, "_dist")
		d, t, _ := strings.Cut(rest, "_t")
		for i, m := range moments[:2] {
			if t == m {
				if to := move + "_dist" + d + "_t" + moments[i+1]; use[to] {
					moves = append(moves, [2]string{c, to})
				}
			}
		}
		for i, x := range dists[1:] {
			if d == x {
				if to := move + "_dist" + dists[i] + "_t" + t; use[to] {
					moves = append(moves, [2]string{c, to}) // pushing in
				}
			}
		}
	}
	var out [][2]string
	for i, j := 0, 0; i < len(side) || j < len(moves); {
		for range 2 {
			if i < len(side) {
				out = append(out, [2]string{side[i], dollyTo})
				i++
			}
		}
		if j < len(moves) {
			out = append(out, moves[j])
			j++
		}
	}
	return out
}()

// nominalSize is an A320's, for sorting the frames.
var nominalSize = camera.Size{Span: 36, Length: 38}

// nominalPose is frame name on an A320 (its right side).
func nominalPose(name string) (camera.Pose, bool) {
	c, ok := parseComposition(name)
	if !ok {
		return camera.Pose{}, false
	}
	d := time.Duration(9 * scenePace * float64(time.Second))
	var s camera.Shot
	if c.move == "crane" {
		s = craneShot(0, nominalSize, 1, d)
	} else if f, ok := sceneMoves[c.move]; ok {
		s = f(0, nominalSize, 1, d)
	} else {
		return camera.Pose{}, false
	}
	if c.back != 1 {
		s = tuned{s, c.back, 0}
	}
	return s.PoseAt(time.Duration(c.at * float64(s.Length()))), true
}

// compositionPose is frame c on it, side its right (+1) or left (-1).
func compositionPose(g *airport.Graph, it *controlled, c composition, side float64) (camera.Pose, bool) {
	shots := castShots(g, map[string]*controlled{"it": it}, []SceneShot{{Move: c.move, Who: "it", Side: side, Sec: 9}}, "composition")
	if len(shots) == 0 {
		return camera.Pose{}, false
	}
	s := shots[0]
	if c.back != 1 {
		s = tuned{s, c.back, 0}
	}
	return s.PoseAt(time.Duration(c.at * float64(s.Length()))), true
}

// glideShot glides from frame from to frame to on it, at most glideSpeed
// (over d, or longer up to MaxShot; a glide farther than that is not
// played), the camera clear of the buildings all the way; to empty holds
// from.
func glideShot(g *airport.Graph, it *controlled, from, to string, side float64, d time.Duration) (camera.Shot, bool) {
	if to == "" {
		to = from
	}
	if to == dollyTo {
		return dollyShot(g, it, from, side, d)
	}
	a, ok1 := parseComposition(from)
	b, ok2 := parseComposition(to)
	if !ok1 || !ok2 {
		return nil, false
	}
	pa, ok1 := compositionPose(g, it, a, side)
	pb, ok2 := compositionPose(g, it, b, side)
	if !ok1 || !ok2 {
		return nil, false
	}
	m := eyeTravel(pa, pb) / glideSpeed
	if m > camera.MaxShot.Seconds() {
		return nil, false // too far to glide slowly
	}
	d = max(d, time.Duration(m*float64(time.Second)))
	l := lengthOf(it)
	if !clearEye(g, it, pa, l) || !clearEye(g, it, midPose(pa, pb), l) || !clearEye(g, it, pb, l) {
		return nil, false
	}
	name := from
	if to != from {
		name += " → " + to
	}
	return camera.Path(name, d, nil, camera.Key{At: 0, Pose: pa}, camera.Key{At: 1, Pose: pb}), true
}

// dollyShot slides frame from along it, camera and the point looked at
// together: tracking beside it, slowly. It ends clear: when one end is
// hidden (by a building, by the jetway) the slide starts there and comes
// out, showing the aircraft, never covering it.
func dollyShot(g *airport.Graph, it *controlled, from string, side float64, d time.Duration) (camera.Shot, bool) {
	c, ok := parseComposition(from)
	if !ok {
		return nil, false
	}
	p, ok := compositionPose(g, it, c, side)
	if !ok || p.Eye.Frame != camera.Object || p.Target.Frame != camera.Object {
		return nil, false
	}
	l := lengthOf(it)
	slide := func(by float64) camera.Pose {
		q := p
		q.Eye.Offset.Forward += by
		q.Target.Offset.Forward += by
		return q
	}
	half := dollyAlong * l / 2
	a, b := slide(-half), slide(half) // tail to nose
	ca, cb := clearEye(g, it, a, l), clearEye(g, it, b, l)
	switch {
	case !ca && !cb:
		return nil, false
	case !cb:
		a, b = b, a // out of the hidden end
	}
	return camera.Path(from+" dolly", max(d, camera.MaxShot), camera.Linear,
		camera.Key{At: 0, Pose: a}, camera.Key{At: 1, Pose: b}), true
}

// clearAboveM: a camera this high above the aircraft clears the buildings
// (a terminal's roof); clearNodeM: lower, it must be this near the taxi
// graph (over the taxiways and aprons, not in a building).
const (
	clearAboveM = 30.0
	clearNodeM  = 35.0
)

// clearEye reports whether p's eye is clear of the buildings round it: high
// enough, or over the pavement; at a stand never ahead of the nose (the
// terminal) nor at its left (the jetway).
func clearEye(g *airport.Graph, it *controlled, p camera.Pose, l float64) bool {
	if p.Eye.Frame != camera.Object {
		return true
	}
	e := p.Eye.Offset
	if e.Up >= clearAboveM {
		return true
	}
	it.mu.Lock()
	v := it.view
	it.mu.Unlock()
	if phaseOf(v) == "stand" && (e.Forward > l*0.45 || e.Right < -l*0.15) {
		return false
	}
	if g == nil || v.Position.Lat == 0 && v.Position.Lon == 0 {
		return true
	}
	lat, lon := calc.DisplaceByHeading(v.Position.Lat, v.Position.Lon, v.Heading, e.Forward)
	lat, lon = calc.DisplaceByHeading(lat, lon, math.Mod(v.Heading+90, 360), e.Right)
	_, ok := g.NearestNode(airport.LatLon{Lat: lat, Lon: lon}, clearNodeM)
	return ok
}

// midPose is halfway between two poses on one aircraft (a, when not).
func midPose(a, b camera.Pose) camera.Pose {
	if a.Eye.Frame != camera.Object || b.Eye.Frame != camera.Object {
		return a
	}
	m := a
	m.Eye.Offset = camera.Offset{
		Right:   (a.Eye.Offset.Right + b.Eye.Offset.Right) / 2,
		Up:      (a.Eye.Offset.Up + b.Eye.Offset.Up) / 2,
		Forward: (a.Eye.Offset.Forward + b.Eye.Offset.Forward) / 2,
	}
	return m
}

// lengthOf is it's length (meters), as the shots size it.
func lengthOf(it *controlled) float64 {
	it.mu.Lock()
	model := it.view.Model
	it.mu.Unlock()
	if l := traffic.ProfileFor(strings.SplitN(model, liverySep, 2)[0]).Motion.SpanMeters * 1.05; l > 0 {
		return l
	}
	return nominalSize.Length
}

// eyeTravel is how far the eye moves between two poses on one aircraft
// (meters; 0 when not both on it).
func eyeTravel(a, b camera.Pose) float64 {
	if a.Eye.Frame != camera.Object || b.Eye.Frame != camera.Object {
		return 0
	}
	x, y, z := a.Eye.Offset.Right-b.Eye.Offset.Right, a.Eye.Offset.Up-b.Eye.Offset.Up, a.Eye.Offset.Forward-b.Eye.Offset.Forward
	return math.Sqrt(x*x + y*y + z*z)
}

// fillCursor walks the glides for the filler: each tried once a round.
var fillCursor atomic.Int64

// fillerGlides are the next two glides of the picked ones that play on it
// (clear of the buildings, on its right side or its left), none again until
// all have been tried.
func fillerGlides(g *airport.Graph, it *controlled, n int) []camera.Shot {
	k := len(compositionGlides)
	var out []camera.Shot
	for range k {
		if len(out) == 2 {
			break
		}
		c := int(fillCursor.Add(1) - 1)
		p := compositionGlides[c*37%k] // 37 prime to the count: spread, every glide once a round
		if k%37 == 0 {
			p = compositionGlides[c%k]
		}
		side := 1.0
		if (n+len(out))%2 == 1 {
			side = -1
		}
		for _, sd := range []float64{side, -side} {
			if s, ok := glideShot(g, it, p[0], p[1], sd, camera.MaxShot); ok {
				out = append(out, named{s, it.Tail + " " + s.Name()})
				break
			}
		}
	}
	return out
}
