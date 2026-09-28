//go:build windows
// +build windows

package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// pushPlan fits the pushback to the stand's surroundings (#322). The main
// gear goes straight back along the stand axis, turns onto the taxiway on
// the widest arc the real distances allow (the push line up to the taxiway
// and the straight run of the taxiway beyond it), then follows the taxiway
// centreline until the aircraft is aligned. A radius that would swing the
// tail or a wingtip deeper into a neighbouring stand than the parked
// aircraft already is gives way to a tighter one. It returns the gear
// points after gear; tail is the taxiway centreline past the junction jp.
func pushPlan(g *airport.Graph, own int, gear airport.LatLon, standHdg float64, jp airport.LatLon, tail []airport.LatLon, prof MotionProfile) []airport.LatLon {
	line := append([]airport.LatLon{jp}, tail...)
	cum := make([]float64, len(line))
	for i := 1; i < len(line); i++ {
		cum[i] = cum[i-1] + localDist(line[i-1], line[i])
	}
	total := cum[len(cum)-1]
	settle := prof.WheelbaseMeters + PushTailMeters
	if len(tail) == 0 { // dead end: straight back along the stand axis
		return []airport.LatLon{offsetHeading(gear, standHdg+180, math.Max(10, alongHeading(gear, standHdg+180, jp)))}
	}
	// The taxiway line: the longest chord from the junction the centreline
	// stays within pushLineToleranceMeters of, so a jog at the junction does
	// not hide a long straight taxiway. The push ends on that straight part,
	// aligned, and never in the bend after it.
	run := total
	for d := total; d >= 10; d -= 2 {
		dd, straight := localBearing(jp, pointAlong(line, cum, d)), true
		for i := 1; i < len(line) && cum[i] < d; i++ {
			if math.Abs(alongHeading(jp, dd+90, line[i])) > pushLineToleranceMeters {
				straight = false
				break
			}
		}
		if run = d; straight {
			break
		}
	}
	dir := localBearing(jp, pointAlong(line, cum, run))
	chord := func() []airport.LatLon { // unfitted: through the junction
		return append([]airport.LatLon{jp}, cutLine(line, cum, 0.5, math.Min(total, math.Max(10, math.Min(settle, run))))...)
	}
	pushDir := math.Mod(standHdg+180, 360)
	turn := headingDiff(pushDir, dir)
	if math.Abs(turn) < 3 {
		return chord()
	}
	// Corner V where the stand axis meets the taxiway line: s along the push
	// from the gear, a along the taxiway from the junction.
	px, py := alongHeading(gear, 90, jp), alongHeading(gear, 0, jp)
	u1x, u1y := math.Sin(pushDir*math.Pi/180), math.Cos(pushDir*math.Pi/180)
	u2x, u2y := math.Sin(dir*math.Pi/180), math.Cos(dir*math.Pi/180)
	den := u1x*u2y - u1y*u2x
	s := (px*u2y - py*u2x) / den
	a := (px*u1y - py*u1x) / den
	if s < 2 || s > 150 || a > run {
		return chord()
	}
	v := offsetHeading(gear, pushDir, s)
	k := math.Tan(math.Abs(turn) / 2 * math.Pi / 180)

	build := func(r float64) []airport.LatLon {
		t := math.Min(r*k, s)
		r = t / k
		var out []airport.LatLon
		if s-t > 0.5 {
			out = append(out, offsetHeading(gear, pushDir, s-t))
		}
		arc := fillet([]airport.LatLon{offsetHeading(v, pushDir+180, 2*t), v, offsetHeading(v, dir, 2*t)}, r)
		out = append(out, arc[1:len(arc)-1]...)
		end := math.Min(total, math.Max(a+t, math.Min(run, math.Max(a+t+PushAlignMeters, settle))))
		// On along the fitted taxiway line, which the centreline stays within
		// pushLineToleranceMeters of (stepping back onto it would jog).
		if end > a+t+1 {
			out = append(out, offsetHeading(jp, dir, end))
		}
		return out
	}

	// The widest radius the distances allow, then tighter while the swing
	// reaches into a neighbouring stand.
	widest := math.Min(s-PushStraightMeters, run-a) / k
	widest = math.Max(PushbackMinArcMeters, math.Min(widest, PushbackArcMeters))
	base := standIntrusion(g, own, []airport.LatLon{offsetHeading(gear, standHdg, 1), gear}, prof)
	best, bestIn := []airport.LatLon(nil), math.Inf(1)
	for r := widest; r >= PushbackMinArcMeters-0.01; r -= 2 {
		pts := build(r)
		in := standIntrusion(g, own, append([]airport.LatLon{gear}, pts...), prof)
		if in <= base+pushClearanceSlackMeters {
			return pts
		}
		if in < bestIn {
			best, bestIn = pts, in
		}
	}
	return best
}

// standIntrusion is how deep (meters) the tail or a wingtip reaches into
// another stand's circle anywhere along a pushback of the main gear through
// pts (the nose faces away from the direction of travel).
func standIntrusion(g *airport.Graph, own int, pts []airport.LatLon, prof MotionProfile) float64 {
	span, tailLen := prof.SpanMeters, prof.TailMeters
	if span <= 0 {
		span = 35.8
	}
	if tailLen <= 0 {
		tailLen = 20.5
	}
	var near []airport.Parking
	for _, p := range g.Layout.Parking {
		if p.Index != own && localDist(p.Position, pts[0]) < 200 {
			near = append(near, p)
		}
	}
	worst := math.Inf(-1)
	for i := 1; i < len(pts); i++ {
		gear, hdg := pts[i], localBearing(pts[i], pts[i-1])
		wing := offsetHeading(gear, hdg, 2)
		for _, q := range []airport.LatLon{offsetHeading(gear, hdg+180, tailLen), offsetHeading(wing, hdg-90, span/2), offsetHeading(wing, hdg+90, span/2)} {
			for _, p := range near {
				worst = math.Max(worst, p.Radius-localDist(q, p.Position))
			}
		}
	}
	return worst
}

// pointAlong is the point d meters along a polyline with cumulative
// distances cum.
func pointAlong(line []airport.LatLon, cum []float64, d float64) airport.LatLon {
	for i := 1; i < len(line); i++ {
		if cum[i] >= d {
			seg := cum[i] - cum[i-1]
			if seg <= 0 {
				return line[i]
			}
			return offsetHeading(line[i-1], localBearing(line[i-1], line[i]), d-cum[i-1])
		}
	}
	return line[len(line)-1]
}

// cutLine is the part of a polyline from just after from to to meters
// along it, ending exactly at to.
func cutLine(line []airport.LatLon, cum []float64, from, to float64) []airport.LatLon {
	var out []airport.LatLon
	for i := range line {
		if cum[i] > from && cum[i] < to-0.5 {
			out = append(out, line[i])
		}
	}
	if to > from {
		out = append(out, pointAlong(line, cum, to))
	}
	return out
}
