package traffic

import (
	"math"
	"slices"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// groundMap is the ground elevation the simulator gave under our aircraft,
// by position, in cells of about a metre: an aircraft is placed on the
// ground where it goes, not where the last sample was taken. Placed on the
// sample a frame old, it rode a step in the runway mesh a frame late
// (live, every take-off on LKPR 24 at 76 kt: +0.31 ft for a frame).
type groundMap struct {
	cells map[groundCell]groundSeen
}

// groundSeen is a cell's ground elevation (feet), the object whose sample
// it came from (0 unknown) and when (UnixNano, 0 unknown). The simulator
// gives each aircraft the ground at the scenery's detail around it, which
// changes with the camera: a cell another aircraft wrote may stand a few
// tenths off what this one is shown now (cellBias).
type groundSeen struct {
	ft float32
	by uint32
	at int64
}

type groundCell struct{ lat, lon int32 }

// groundCellDeg is a cell's size: about 1.1 m north–south.
const groundCellDeg = 1e-5

// groundMapMax: the cells kept at most; beyond it the map starts again (a
// long session over many airports).
const groundMapMax = 1 << 20

func cellOf(p airport.LatLon) groundCell {
	return groundCell{int32(math.Round(p.Lat / groundCellDeg)), int32(math.Round(p.Lon / groundCellDeg))}
}

// at is the cell's centre.
func (c groundCell) at() airport.LatLon {
	return airport.LatLon{Lat: float64(c.lat) * groundCellDeg, Lon: float64(c.lon) * groundCellDeg}
}

// groundSpikeFt: a sample this far from the median of the last three is a
// spike, not kept in the map.
const groundSpikeFt = 0.1

// groundSampleResetM: a sample this far from the last one starts again (placed
// elsewhere, or sampled rarely): the ground there has nothing to do with it.
const groundSampleResetM = 50.0

// groundSample is the ground height to use for a new sample ft at p: the median
// of the last three samples. The simulator reports a wrong ground for a single
// frame at some spots, the same ones each time, and the aircraft placed on it
// hopped a frame later (live, LKPR: 0.15 to 0.39 ft, ground and height the same
// amount apart). A real change of the ground passes a sample later.
func (o *injected) groundSample(p airport.LatLon, ft float64) float64 {
	if o.groundSeen > 0 && localDist(o.groundPrevAt, p) > groundSampleResetM {
		o.groundSeen = 0
	}
	a, b := o.groundPrev[0], o.groundPrev[1]
	seen := o.groundSeen
	o.groundPrev, o.groundPrevAt = [2]float64{b, ft}, p
	o.groundSeen = min(seen+1, 2)
	if seen < 2 {
		return ft
	}
	return max(min(a, b), min(max(a, b), ft))
}

// put records the ground elevation ft at p, from no object in particular.
func (m *groundMap) put(p airport.LatLon, ft float64) { m.putBy(p, ft, 0, time.Time{}) }

// putBy records the ground elevation ft at p as sampled under object by at
// at.
func (m *groundMap) putBy(p airport.LatLon, ft float64, by uint32, at time.Time) {
	if p == (airport.LatLon{}) {
		return
	}
	if m.cells == nil || len(m.cells) >= groundMapMax {
		m.cells = make(map[groundCell]groundSeen, 4096)
	}
	s := groundSeen{ft: float32(ft), by: by}
	if !at.IsZero() {
		s.at = at.UnixNano()
	}
	m.cells[cellOf(p)] = s
}

// at is the ground elevation at p, when seen there.
func (m *groundMap) at(p airport.LatLon) (float64, bool) {
	v, ok := m.cells[cellOf(p)]
	return float64(v.ft), ok
}

// seen is the cell at p as recorded.
func (m *groundMap) seen(p airport.LatLon) (groundSeen, bool) {
	v, ok := m.cells[cellOf(p)]
	return v, ok
}

// ownFresh: whether cell s was written by object obj within
// groundOwnFreshFor before now: the ground at the detail it is shown now.
func (s groundSeen) ownFresh(obj uint32, now time.Time) bool {
	return obj != 0 && s.by == obj && s.at != 0 && now.UnixNano()-s.at < int64(groundOwnFreshFor)
}

// groundOwnFreshFor: a cell an aircraft wrote itself this recently is its
// own ground as shown now: taken without its cellBias.
const groundOwnFreshFor = 5 * time.Second

// noteCellBias records, before a sample ft at p of o is put
// into the map, how far the sample stands from the cell written there
// before by another aircraft: the simulator's ground for o against the
// map's (cellBias). Live, LKPR 24: cells along the runway stood up to 0.26
// ft above what CSA1156 was shown on its take-off roll, and placed on them
// it flickered between its height and ours frame after frame. Its own
// cells are not measured (own). i.mu held.
func (i *Injector) noteCellBias(o *injected, p airport.LatLon, ft float64) {
	now := i.now()
	c := cellOf(p)
	s, ok := i.ground.cells[c]
	if !ok || s.ownFresh(o.id, now) {
		return
	}
	// Measured against the cells around it, not a single odd one (oddCell).
	b := ft - i.ground.oddCell(c, float64(s.ft), func(n groundSeen) (float64, bool) {
		return float64(n.ft), !n.ownFresh(o.id, now)
	})
	if math.Abs(b) >= groundMapTrustFt {
		return // a bridge, not the same ground
	}
	if w := o.cellBiasWeight(p, now); w > 0 {
		b = o.cellBias*w + (b-o.cellBias*w)*cellBiasFollow
	}
	o.cellBias, o.cellBiasPos, o.cellBiasAt = b, p, now
}

// cellBiasFollow: the share of a new cell's difference taken at once (the
// rest from the ones before): a cell's own bump does not show.
const cellBiasFollow = 0.5

// cellBiasWeight is how much of o's cellBias holds at p now: fading with
// the distance (groundOffFadeM) and the time (cellBiasFadeSec) since it was
// measured.
func (o *injected) cellBiasWeight(p airport.LatLon, now time.Time) float64 {
	if o.cellBiasAt.IsZero() {
		return 0
	}
	return math.Max(0, 1-localDist(o.cellBiasPos, p)/groundOffFadeM-now.Sub(o.cellBiasAt).Seconds()/cellBiasFadeSec)
}

// cellBiasFadeSec: the scenery's detail under an aircraft holds while it
// stands on its own cells (nothing to measure it by): its cellBias fades
// over this long, not groundOffFadeSec (lined up 2 s, it rolled onto the
// cells ahead without it).
const cellBiasFadeSec = 60.0

// cellUnder is the ground at p for o from the map: the cell,
// moved by o's cellBias unless o wrote it itself (own). i.mu held.
func (i *Injector) cellUnder(o *injected, p airport.LatLon) (float64, bool) {
	return i.cellFt(o, cellOf(p), i.now())
}

// cellFt is cell c's ground for o at now, as cellUnder. i.mu held.
func (i *Injector) cellFt(o *injected, c groundCell, now time.Time) (float64, bool) {
	s, ok := i.ground.cells[c]
	if !ok {
		return 0, false
	}
	if s.ownFresh(o.id, now) {
		return float64(s.ft), true
	}
	w := o.cellBiasWeight(c.at(), now)
	ft := float64(s.ft) + o.cellBias*w
	// Another aircraft's cell standing off the ones around it: theirs.
	return i.ground.oddCell(c, ft, func(n groundSeen) (float64, bool) {
		if n.ownFresh(o.id, now) {
			return float64(n.ft), true
		}
		return float64(n.ft) + o.cellBias*w, true
	}), true
}

// oddCell is cell c's ground ft, unless it stands alone: fewer than
// oddCellAgree of the cells around it (each as value gives it, those it
// refuses left out; at least oddCellMin of them) within oddCellFt of it.
// Then the median of those. Along a runway most cells were written by the
// last departure, at the detail of the scenery it was shown; where its
// samples skipped one (a frame's way is about a cell at 90 kt) the cell kept
// an older aircraft's ground, a few tenths off. Rolling over it the aircraft
// dropped that much for a frame, and measured there its cellBias took half
// of it on to the cells after, halving cell by cell (live LKPR 24,
// 2026-11-06: AFR704, OKLVO, WZZ626, CSA354, -0.22 → +0.11 → +0.05 → +0.03
// ft at 17 to 92 kt, the simulator's ground smooth). A step in the runway
// mesh moves all the cells past it: there a cell has neighbours alike, even
// where most of them lie before the step. i.mu held.
func (m *groundMap) oddCell(c groundCell, ft float64, value func(groundSeen) (float64, bool)) float64 {
	var vs [8]float64
	n, agree := 0, 0
	for dl := int32(-1); dl <= 1; dl++ {
		for dn := int32(-1); dn <= 1; dn++ {
			if dl == 0 && dn == 0 {
				continue
			}
			s, ok := m.cells[groundCell{c.lat + dl, c.lon + dn}]
			if !ok {
				continue
			}
			v, ok := value(s)
			if !ok {
				continue
			}
			if math.Abs(v-ft) <= oddCellFt {
				agree++
			}
			vs[n] = v
			n++
		}
	}
	if n < oddCellMin || agree >= oddCellAgree {
		return ft
	}
	return cellMedian(vs[:n])
}

// oddCellFt: a cell around within this of another agrees with it. A
// runway's slope (at most about 1.5 %) moves the ground about 0.07 ft from
// one cell to the one diagonally next to it.
const oddCellFt = 0.08

// oddCellMin: with fewer cells around than this, none is judged odd;
// oddCellAgree: a cell with this many around agreeing is not (two odd
// cells side by side are still odd).
const (
	oddCellMin   = 3
	oddCellAgree = 2
)

// cellMedian is the median of vs (sorted in place).
func cellMedian(vs []float64) float64 {
	slices.Sort(vs)
	n := len(vs)
	if n%2 == 1 {
		return vs[n/2]
	}
	return (vs[n/2-1] + vs[n/2]) / 2
}

// groundUnder is the ground under o placed at p: as seen there before, else
// its last sample, moved by what the last cell read differed from the sample
// then. The sample is some frames old: on a slope it is off by a steady amount,
// and between cells seen and not seen the aircraft flipped by that amount, frame
// after frame (live, TVS148 at 117 kt on LKPR 06: 0.11 ft). The difference fades
// with distance and time (groundOffFadeM, groundOffFadeSec). i.mu held.
func (i *Injector) groundUnder(o *injected, p airport.LatLon) float64 {
	now := i.now()
	if ft, ok := i.cellUnder(o, p); ok {
		if math.Abs(ft-o.groundFt) < groundMapTrustFt {
			o.groundOff, o.groundOffFrom, o.groundOffPos, o.groundOffAt = ft-o.groundFt, o.groundFt, p, now
			return ft
		}
		o.groundOff = 0 // a bridge, a stale cell: nothing to go by
	}
	if math.Abs(o.groundFt-o.groundOffFrom) >= groundMapTrustFt {
		o.groundOff = 0 // the sample itself moved on: a bridge
	}
	if o.groundOff != 0 {
		w := 1 - localDist(o.groundOffPos, p)/groundOffFadeM - now.Sub(o.groundOffAt).Seconds()/groundOffFadeSec
		if w > 0 {
			return o.groundFt + w*o.groundOff
		}
		o.groundOff = 0
	}
	return o.groundFt
}

// A cell's difference from the sample fades out over this distance and this
// time, the two added.
const (
	groundOffFadeM   = 100.0
	groundOffFadeSec = 2.0
)

// groundMapTrustFt: a cell further than this from the aircraft's own
// sample is not taken (a bridge, a stale cell).
const groundMapTrustFt = 3.0

// groundShown is the ground height to place o at for pose: the ground under
// it (groundUnder), eased in at groundEaseFtPerSec while it stands still.
// Under a standing aircraft the ground does not change: a new value there
// is the scenery's ground swapping detail, and followed at once the
// aircraft dropped with it in a frame (live, OKSLS 0.48 ft at 0 kt).
// Rolling, it follows at up to groundRollFtPerSec: a step in the ground, or
// a cell read before at another detail, took it in a frame too (live, LKPR
// 06: EZY1403 and TVS1326 0.3 to 0.5 ft at 54 and 75 kt).
func (i *Injector) groundShown(o *injected, pose GroundPose) float64 {
	g := i.groundUnder(o, pose.Position)
	now := i.now()
	if o.groundShownAt.IsZero() || math.Abs(g-o.groundShownFt) > groundMapTrustFt {
		o.groundShownFt, o.groundShownPos, o.groundShownAt = g, pose.Position, now
		return g
	}
	// Standing it is placed a couple of times a second (Detail): the step
	// is of one frame's time, whatever the gap, so the new ground comes in
	// over a few seconds, never in one placement (live, LOT1111 holding
	// short: 0.46 ft in one).
	rate, dt := groundEaseFtPerSec, math.Min(math.Max(0, now.Sub(o.groundShownAt).Seconds()), standStepMaxSec)
	if pose.GroundSpeedKts >= groundEaseMaxKts || localDist(o.groundShownPos, pose.Position) > groundEaseStillM {
		rate, dt = groundFollowFtPerSec(pose.GroundSpeedKts), math.Max(0, now.Sub(o.groundShownAt).Seconds())
		// Rolling onto ground well above the one shown (a step not seen
		// before, no ramp to it): the simulator has put it on its
		// static height over it already; taken at once, not eased up from
		// below it (live LKPR 2026-11-08, CSA1763 at 15 kt over the seam at
		// 50.10845,14.26300 heading 028: +0.37 ft for 4 frames, 0.33 back
		// down, then up again).
		g += i.groundAlong(o, pose)
		if g-o.groundShownFt > groundStepUpFt {
			o.groundShownFt, o.groundShownPos, o.groundShownAt = g, pose.Position, now
			return g
		}
	}
	step := rate * dt
	o.groundShownFt += math.Max(-step, math.Min(step, g-o.groundShownFt))
	o.groundShownPos, o.groundShownAt = pose.Position, now
	return o.groundShownFt
}

// groundAlong is how far the ground averaged over a few metres before and
// after o along its heading at pose stands from the cell under it (the
// map's cells, cellFt, no side effects): a step in the ground is met as a
// ramp centred on it, at most half the step off on either side, a steady
// slope unchanged. 0 when the cell under it is not known. Followed late, a
// step left a rolling aircraft below the simulator's own static height over
// the higher side, and the simulator put it there for a few frames, then
// ours again (live LKPR 2026-11-08, TVS1774 lining up at 4 kt over the seam
// at 50.11487,14.27405: +0.44 ft for 3 frames, then 0.40 down). i.mu held.
func (i *Injector) groundAlong(o *injected, pose GroundPose) float64 {
	now := i.now()
	c0, ok := i.cellFt(o, cellOf(pose.Position), now)
	if !ok {
		return 0
	}
	// Only in pairs, as far behind as ahead: ahead not seen yet (the first
	// over it), the cells behind alone would hold it back.
	L := math.Max(groundAlongMinM, math.Min(groundAlongMaxM, groundAlongSec*pose.GroundSpeedKts*ktsToMS))
	sum, n := 0.0, 0
	for k := 1; k <= groundAlongN; k++ {
		d := L * float64(k) / groundAlongN
		a, okA := i.cellFt(o, cellOf(offsetHeading(pose.Position, pose.Heading, d)), now)
		b, okB := i.cellFt(o, cellOf(offsetHeading(pose.Position, pose.Heading, -d)), now)
		if !okA || !okB || math.Abs(a-c0) >= groundAlongMaxFt || math.Abs(b-c0) >= groundAlongMaxFt {
			continue
		}
		sum += a + b - 2*c0
		n += 2
	}
	if n == 0 {
		return 0
	}
	// The cell under it left out: over a step it stands half the step off
	// at most, the cell under it on either side.
	return sum / float64(n)
}

// groundAlongSec, groundAlongMinM, groundAlongMaxM: the ground under a
// rolling aircraft averaged over the way it rolls in groundAlongSec behind
// and ahead, at least groundAlongMinM and at most groundAlongMaxM, so a
// step is ramped no steeper than it is followed (groundFollowFtPerSec);
// groundAlongN samples on either side. groundAlongMaxFt: a cell further
// than this from the one under it is left out (a bridge, a stale cell).
const (
	groundAlongSec   = 0.2
	groundAlongMinM  = 1.5
	groundAlongMaxM  = 5.0
	groundAlongN     = 3
	groundAlongMaxFt = 1.0
)

// A standing aircraft (under groundEaseMaxKts, moved less than
// groundEaseStillM since it was last placed) takes a new ground height at
// groundEaseFtPerSec.
const (
	groundEaseMaxKts   = 1.0
	groundEaseStillM   = 0.3
	groundEaseFtPerSec = 1.0
)

// standStepMaxSec: standing, a placement moves the height by at most this
// much time's worth of its blend rate, however long since the last one
// (placed every Detail.StillInterval+1 frames, or after a pause).
const standStepMaxSec = 2.0 / InjectHz

// groundRollFtPerSec: well above what a runway's slope asks at take-off speed
// (1% at 140 kt is about 2.4 ft/s), about 0.13 ft a frame at 60 Hz.
const groundRollFtPerSec = 8.0

// groundStepUpFt: rolling onto ground this much above the one shown (a
// step met without a ramp, groundAlong), the new ground is taken at once.
const groundStepUpFt = 0.2

// Taxiing, the ground under an aircraft is followed as steeply as
// groundFollowGrade over the distance it rolls, at least groundTaxiFtPerSec
// and at most groundRollFtPerSec: a step in the simulator's ground is
// ridden over a few metres, not in a frame (live LKPR 2026-11-08, the
// seam at 50.1110,14.2657 heading 035: the ground 0.39 ft up between two
// frames at 16 kt, every aircraft lifted 0.38 ft in 4 frames, 0.1 ft a
// frame; now 0.04).
const (
	groundFollowGrade  = 0.04
	groundTaxiFtPerSec = 2.0
)

// groundFollowFtPerSec is how fast the ground under an aircraft rolling at
// kts is followed (groundFollowGrade).
func groundFollowFtPerSec(kts float64) float64 {
	return math.Max(groundTaxiFtPerSec, math.Min(groundRollFtPerSec, groundFollowGrade*kts*ktsToMS/0.3048))
}
