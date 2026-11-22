package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// noHeading: a vehicle way end with no heading to face (VehicleRoutePose).
var noHeading = math.NaN()

// wayCost is the length of a vehicle's way pts, ending facing end (NaN:
// any way), with each turn priced as the vehicle roads price them
// (airport.VehicleRoutePose): its length on wayTurnRadiusM, and
// wayTurnBackM more for one beyond wayTurnBackDeg. The fuel truck takes
// the side of the aircraft whose way is cheaper by this (#1630: not by
// the last turn alone, which took it 78 m round for a 25 m way).
func wayCost(pts []airport.LatLon, end float64) float64 {
	turn := func(a, b float64) float64 {
		if math.IsNaN(a) || math.IsNaN(b) {
			return 0
		}
		t := math.Abs(headingDiff(a, b))
		c := wayTurnRadiusM * t * math.Pi / 180
		if t > wayTurnBackDeg {
			c += wayTurnBackM
		}
		return c
	}
	cost, last := 0.0, noHeading
	for i := 1; i < len(pts); i++ {
		d := localDist(pts[i-1], pts[i])
		if d < 0.5 {
			continue
		}
		h := localBearing(pts[i-1], pts[i])
		cost += d + turn(last, h)
		last = h
	}
	return cost + turn(last, end)
}

const (
	wayTurnRadiusM = 10.0
	wayTurnBackDeg = 120.0
	wayTurnBackM   = 60.0
)

// cleanWay is a vehicle's way pts without the knots its pieces leave
// where they meet (#1630): points within cleanWayMeters of the one before
// dropped, and points where the way comes straight back over its last leg
// (a turn above cleanWayTurnDeg) dropped, so the rounded path never turns
// back on itself. The ends stay.
func cleanWay(pts []airport.LatLon) []airport.LatLon {
	if len(pts) < 3 {
		return pts
	}
	out := []airport.LatLon{pts[0]}
	for i := 1; i < len(pts)-1; i++ {
		if localDist(out[len(out)-1], pts[i]) >= cleanWayMeters && localDist(pts[i], pts[len(pts)-1]) >= cleanWayMeters {
			out = append(out, pts[i])
		}
	}
	out = append(out, pts[len(pts)-1])
	for changed := true; changed; {
		changed = false
		for i := 1; i+1 < len(out); i++ {
			// A turn back, or more than a quarter turn a few metres in
			// from either end (a join's stub: LROP 202R's stairs went 2.5 m
			// on and turned back up), where the turn has no room.
			turn := math.Abs(headingDiff(localBearing(out[i-1], out[i]), localBearing(out[i], out[i+1])))
			stub := (i == 1 && localDist(out[0], out[1]) < wayTurnRadiusM) || (i+2 == len(out) && localDist(out[i], out[i+1]) < wayTurnRadiusM)
			if turn > cleanWayTurnDeg || (stub && turn > 90) {
				out = append(out[:i:i], out[i+1:]...)
				changed = true
				break
			}
		}
	}
	return out
}

const (
	cleanWayMeters  = 2.0
	cleanWayTurnDeg = 150.0
)

// wayHeading is the way pts leaves its start (or, last, comes into its
// end) heading: towards its first point wayHeadingMeters off it, not a
// join a metre beside it.
func wayHeading(pts []airport.LatLon, last bool) float64 {
	n := len(pts)
	for i := 1; i < n; i++ {
		if last {
			if localDist(pts[n-1-i], pts[n-1]) >= wayHeadingMeters || i == n-1 {
				return localBearing(pts[n-1-i], pts[n-1])
			}
		} else if localDist(pts[0], pts[i]) >= wayHeadingMeters || i == n-1 {
			return localBearing(pts[0], pts[i])
		}
	}
	return noHeading
}

// homeAheadMeters: a vehicle whose way this far straight ahead runs into
// the aircraft faces it.
const homeAheadMeters = 20.0

// homeFacingDeg: and its nose gear lies within this of its heading (not
// one beside the fuselage facing along it, the stairs backed round).
const homeFacingDeg = 45.0

// arriveTurnDeg: a way coming in more than this off the heading it must
// arrive on ends in a hook (LKPR N51's tug turned back on itself in front
// of the nose): it comes in by a loop or from the side instead.
const arriveTurnDeg = 90.0

// wayHeadingMeters: how far off a way's end its heading is taken.
const wayHeadingMeters = 10.0

// homeWay is the way home from p, facing h, to depot, round the aircraft
// at pose (#1630): joining the roads the way it faces
// (airport.VehicleRoutePose); where that still runs back the way it came,
// first round in a loop (FuelTurnMeters on, FuelTurnWidthMeters across, as
// far back) on the side away from the aircraft, never turning on the spot
// or back under it. It starts at p.
func homeWay(l *airport.Layout, p airport.LatLon, h float64, depot airport.LatLon, pose GroundPose, prof MotionProfile) ([]airport.LatLon, error) {
	f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	if !f.clear(p, offsetHeading(p, h, homeAheadMeters)) && math.Abs(headingDiff(localBearing(p, f.nose), h)) < homeFacingDeg {
		// Facing the aircraft (a tug backed off its nose): it turns away
		// first, so its heading picks no join; round the aircraft as before.
		route, err := l.VehicleRoute(p, depot)
		if err != nil {
			// The nearest join leads to no depot (LROP 131: the tug drove
			// 39 m off and vanished): by any other within reach.
			if route, err = l.VehicleRoutePose(p, noHeading, depot, noHeading); err != nil {
				return nil, err
			}
		}
		way := serviceWay(l, route, pose, prof)
		if wayAcrossStand(l, pose.Position, way) {
			// The nearest road lies over another stand (LKPR C26's tug
			// over C27): by a join that keeps off them.
			if route, err := l.VehicleRoutePose(p, noHeading, depot, noHeading); err == nil {
				if w := serviceWay(l, route, pose, prof); !wayAcrossStand(l, pose.Position, w) {
					way = w
				}
			}
		}
		return way, nil
	}
	route, err := l.VehicleRoutePose(p, h, depot, noHeading)
	if err != nil {
		return nil, err
	}
	direct := serviceWay(l, route, pose, prof)
	if len(route) < 2 {
		return direct, nil
	}
	direct = rejoinOffStands(l, direct, depot, pose, prof)
	ahead := math.Abs(headingDiff(wayHeading(route, false), h)) <= 90
	if ahead && !wayAcrossStand(l, pose.Position, direct) {
		return direct, nil
	}
	// The loop to the side whose way on is shorter, off the aircraft and
	// off the other stands; or, where a loop reaches the stand ahead (two
	// facing, LROP 120K and 120B), out to the side (a quarter turn) and on
	// from there.
	var best []airport.LatLon
	bestC := math.Inf(1)
	a := offsetHeading(p, h, FuelTurnMeters)
	for _, side := range []float64{90, -90} {
		b := offsetHeading(a, h+side, FuelTurnWidthMeters)
		for _, loop := range []bool{true, false} {
			if loop && ahead {
				// Facing its way: no turning back.
				continue
			}
			pts := []airport.LatLon{p, a, b}
			end, endH := b, h+side
			if loop {
				end, endH = offsetHeading(b, h+180, FuelTurnMeters), h+180
				pts = append(pts, end)
			}
			ok := true
			for i := 1; i < len(pts) && ok; i++ {
				ok = !f.inside(pts[i]) && f.clear(pts[i-1], pts[i]) && !l.VehicleAcrossStand(pose.Position, pts[i-1], pts[i], 0)
			}
			if !ok {
				continue
			}
			on, err := l.VehicleRoutePose(end, endH, depot, noHeading)
			if err != nil {
				continue
			}
			way := append(pts[:len(pts)-1:len(pts)-1], serviceWay(l, on, pose, prof)...)
			if cost := wayCost(way, noHeading); cost < bestC && !wayAcrossStand(l, pose.Position, way) && !wayTurnsBack(way) {
				best, bestC = way, cost
			}
		}
	}
	if best == nil {
		return direct, nil
	}
	return best, nil
}

// arriveWay is the way from depot to at, arriving facing h, for the
// aircraft at pose (#1630): by the roads, joined the way it will face
// (airport.VehicleRoutePose); where that still comes in the other way, it
// passes at and comes back in a loop (FuelTurnMeters, FuelTurnWidthMeters)
// on the side away from the aircraft, never turning on the spot.
func arriveWay(l *airport.Layout, depot, at airport.LatLon, h float64, pose GroundPose, prof MotionProfile) ([]airport.LatLon, error) {
	route, err := l.VehicleRoutePose(depot, noHeading, at, h)
	if err != nil || len(route) < 2 {
		return route, err
	}
	if math.Abs(headingDiff(wayHeading(route, true), h)) <= arriveTurnDeg {
		return route, nil
	}
	// The loop to the side whose way in is shorter, off the aircraft and
	// off the other stands; or in from the side (a quarter turn) where a
	// loop reaches another stand.
	f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	var best []airport.LatLon
	bestC := math.Inf(1)
	a := offsetHeading(at, h+180, FuelTurnMeters)
	for _, side := range []float64{90, -90} {
		b := offsetHeading(a, h+side, FuelTurnWidthMeters)
		for _, loop := range []bool{true, false} {
			pts := []airport.LatLon{b, a, at}
			start, startH := b, h-side
			if loop {
				start, startH = offsetHeading(b, h, FuelTurnMeters), h+180
				pts = append([]airport.LatLon{start}, pts...)
			}
			ok := true
			for i := 1; i < len(pts) && ok; i++ {
				ok = !f.inside(pts[i-1]) && f.clear(pts[i-1], pts[i]) && !l.VehicleAcrossStand(pose.Position, pts[i-1], pts[i], 0)
			}
			if !ok {
				continue
			}
			in, err := l.VehicleRoutePose(depot, noHeading, start, startH)
			if err != nil || len(in) < 2 {
				continue
			}
			way := append(in, pts[1:]...)
			if cost := wayCost(way, h); cost < bestC && !wayAcrossStand(l, pose.Position, way) && !wayTurnsBack(way) {
				best, bestC = way, cost
			}
		}
	}
	if best == nil {
		return route, nil
	}
	return best, nil
}

// rejoinOffStands is way with the roads joined again from the start of
// its first leg across another stand, where that is a corner round the
// aircraft (LROP 120A's stairs: round the tail, then straight on over
// stand 120 to the join the way from beside the aircraft had): the way as
// it was when that crosses too.
func rejoinOffStands(l *airport.Layout, way []airport.LatLon, depot airport.LatLon, pose GroundPose, prof MotionProfile) []airport.LatLon {
	for i := 2; i < len(way); i++ {
		if !l.VehicleAcrossStand(pose.Position, way[i-1], way[i], standOnWayMeters) {
			continue
		}
		on, err := l.VehicleRoutePose(way[i-1], localBearing(way[i-2], way[i-1]), depot, noHeading)
		if err != nil || len(on) < 2 {
			return way
		}
		w := append(way[:i-1:i-1], serviceWay(l, on, pose, prof)...)
		if wayAcrossStand(l, pose.Position, w) || wayTurnsBack(w) {
			return way
		}
		return w
	}
	return way
}

// standOnWayMeters: a way this near another stand's parking point drives
// over it.
const standOnWayMeters = 6.0

// wayAcrossStand reports whether a leg of pts crosses another stand than
// the aircraft's at own (airport.VehicleAcrossStand): a loop or side point
// joined across the stand beside it (LROP 109's fuel truck over 108).
func wayAcrossStand(l *airport.Layout, own airport.LatLon, pts []airport.LatLon) bool {
	for i := 1; i < len(pts); i++ {
		if l.VehicleAcrossStand(own, pts[i-1], pts[i], standOnWayMeters) {
			return true
		}
	}
	return false
}

// wayTurnsBack reports whether pts turns back (more than wayTurnBackDeg)
// at a point: a hook (LROP 14's stairs out to the side under the wing and
// back; 119K's bus forward and back down beside the fuselage).
func wayTurnsBack(pts []airport.LatLon) bool {
	for i := 1; i+1 < len(pts); i++ {
		if math.Abs(headingDiff(localBearing(pts[i-1], pts[i]), localBearing(pts[i], pts[i+1]))) > wayTurnBackDeg {
			return true
		}
	}
	return false
}

// nearWay is pts without points within cleanWayMeters of the one before
// (or of its end): the knots a way round an aircraft leaves where its
// corner falls beside a point of the way (#1630). The ends stay.
func nearWay(pts []airport.LatLon) []airport.LatLon {
	if len(pts) < 3 {
		return pts
	}
	out := []airport.LatLon{pts[0]}
	for i := 1; i < len(pts)-1; i++ {
		if localDist(out[len(out)-1], pts[i]) >= cleanWayMeters && localDist(pts[i], pts[len(pts)-1]) >= cleanWayMeters {
			out = append(out, pts[i])
		}
	}
	return append(out, pts[len(pts)-1])
}
