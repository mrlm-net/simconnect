package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TCAS II for our traffic (#450). The thresholds and the behaviour are
// the FAA's "Introduction to TCAS II Version 7.1" (2011): Table 2
// (sensitivity levels and alarm thresholds), Table 3 (initial RAs), Table
// 4 (aural annunciations), the RA sense selection (non-crossing first when
// it gives ALIM), the pilot response model (0.25 g within 5 s to 1500
// fpm), the inhibits (increase descent below 1450 ft AGL, descend below
// 1100, all RAs below 1000, TA only) and TCAS/TCAS coordination
// (complementary senses).

// TCASLevel is a sensitivity level's thresholds (booklet Table 2).
type TCASLevel struct {
	SL                 int
	TATau, RATau       float64 // seconds; 0: no such advisory
	TADMODNM, RADMODNM float64
	TAZThrFt, RAZThrFt float64
	ALIMFt             float64
}

// TCASSensitivity is the sensitivity level for own altitude altFt (MSL,
// pressure) and aglFt (radio): SL2 (TA only) below 1000 ft AGL, SL3 to
// 2350 ft AGL, then by altitude (booklet Table 2).
func TCASSensitivity(altFt, aglFt float64) TCASLevel {
	switch {
	case aglFt < 1000:
		return TCASLevel{SL: 2, TATau: 20, TADMODNM: 0.30, TAZThrFt: 850}
	case aglFt < 2350:
		return TCASLevel{SL: 3, TATau: 25, RATau: 15, TADMODNM: 0.33, RADMODNM: 0.20, TAZThrFt: 850, RAZThrFt: 600, ALIMFt: 300}
	case altFt < 5000:
		return TCASLevel{SL: 4, TATau: 30, RATau: 20, TADMODNM: 0.48, RADMODNM: 0.35, TAZThrFt: 850, RAZThrFt: 600, ALIMFt: 300}
	case altFt < 10000:
		return TCASLevel{SL: 5, TATau: 40, RATau: 25, TADMODNM: 0.75, RADMODNM: 0.55, TAZThrFt: 850, RAZThrFt: 600, ALIMFt: 350}
	case altFt < 20000:
		return TCASLevel{SL: 6, TATau: 45, RATau: 30, TADMODNM: 1.00, RADMODNM: 0.80, TAZThrFt: 850, RAZThrFt: 600, ALIMFt: 400}
	case altFt < 42000:
		return TCASLevel{SL: 7, TATau: 48, RATau: 35, TADMODNM: 1.30, RADMODNM: 1.10, TAZThrFt: 850, RAZThrFt: 700, ALIMFt: 600}
	}
	return TCASLevel{SL: 7, TATau: 48, RATau: 35, TADMODNM: 1.30, RADMODNM: 1.10, TAZThrFt: 1200, RAZThrFt: 800, ALIMFt: 700}
}

// TCAS RA inhibits by height (booklet p. 30).
const (
	TCASNoRABelowAGLFt              = 1000.0
	TCASNoDescendBelowAGLFt         = 1100.0
	TCASNoIncreaseDescentBelowAGLFt = 1450.0 // SelectRA issues no Increase Descent: met by construction
)

// The pilot response the RA sense selection models (booklet p. 29): an
// initial RA flown within TCASResponseDelay at 0.25 g to TCASRAFpm.
const (
	TCASResponseDelaySec = 5.0
	TCASResponseG        = 0.25
	TCASRAFpm            = 1500.0
)

// TCASTrack is an aircraft as TCAS sees it.
type TCASTrack struct {
	ID        uint32
	Callsign  string
	Lat, Lon  float64
	AltFt     float64 // pressure altitude
	AGLFt     float64
	GroundKts float64
	TrackDeg  float64
	VSFpm     float64
	OnGround  bool
}

// Advisory is what TCAS says about an intruder.
type Advisory int

const (
	AdvisoryNone Advisory = iota
	AdvisoryTA
	AdvisoryRA
)

// RAKind is a resolution advisory as annunciated (booklet Table 4,
// Version 7.1).
type RAKind string

const (
	RAClimb           RAKind = "climb"            // "Climb, Climb"
	RADescend         RAKind = "descend"          // "Descend, Descend"
	RACrossingClimb   RAKind = "crossing climb"   // "Climb, Crossing Climb; …"
	RACrossingDescend RAKind = "crossing descend" // "Descend, Crossing Descend; …"
	RALevelOff        RAKind = "level off"        // "Level Off, Level Off" (reduce climb/descent, 7.1)
	RAMonitor         RAKind = "monitor"          // "Monitor Vertical Speed" (preventive)
)

// RA is a resolution advisory against one intruder: its sense (+1 up, -1
// down), the vertical rate to fly (TargetFpm; for a preventive one the
// limit), and whether it crosses the intruder's altitude.
type RA struct {
	Kind      RAKind  `json:"kind"`
	Sense     int     `json:"sense"`
	TargetFpm float64 `json:"targetFpm"`
	Crossing  bool    `json:"crossing,omitempty"`
}

// Aural is the RA's annunciation (booklet Table 4, Version 7.1).
func (r RA) Aural() string {
	switch r.Kind {
	case RAClimb:
		return "Climb, Climb"
	case RADescend:
		return "Descend, Descend"
	case RACrossingClimb:
		return "Climb, Crossing Climb; Climb, Crossing Climb"
	case RACrossingDescend:
		return "Descend, Crossing Descend; Descend, Crossing Descend"
	case RALevelOff:
		return "Level Off, Level Off"
	case RAMonitor:
		return "Monitor Vertical Speed"
	}
	return ""
}

// TCASGeometry is an encounter now: slant range (NM, horizontal here),
// its rate (kt, negative closing), the altitude difference intruder minus
// own (ft) and its rate (fpm).
type TCASGeometry struct {
	RangeNM, RangeRateKts float64
	DZFt, DZRateFpm       float64
}

// Geometry of intruder b against own a.
func Geometry(a, b TCASTrack) TCASGeometry {
	r := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
	// Relative velocity along the line of sight, from both ground tracks.
	brg := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon) * math.Pi / 180
	va := [2]float64{a.GroundKts * math.Sin(a.TrackDeg*math.Pi/180), a.GroundKts * math.Cos(a.TrackDeg*math.Pi/180)}
	vb := [2]float64{b.GroundKts * math.Sin(b.TrackDeg*math.Pi/180), b.GroundKts * math.Cos(b.TrackDeg*math.Pi/180)}
	los := [2]float64{math.Sin(brg), math.Cos(brg)}
	rdot := (vb[0]-va[0])*los[0] + (vb[1]-va[1])*los[1]
	return TCASGeometry{RangeNM: r, RangeRateKts: rdot, DZFt: b.AltFt - a.AltFt, DZRateFpm: b.VSFpm - a.VSFpm}
}

// rangeTest: within DMOD, or closing with the range tau, modified to
// converge on DMOD at slow closure (booklet Figure 12), under tau seconds.
func rangeTest(g TCASGeometry, tau, dmod float64) bool {
	if tau <= 0 {
		return false
	}
	if g.RangeNM <= dmod {
		return true
	}
	if g.RangeRateKts >= 0 {
		return false
	}
	closing := -g.RangeRateKts / 3600 // NM per second
	tauMod := (g.RangeNM*g.RangeNM - dmod*dmod) / (g.RangeNM * closing)
	return tauMod <= tau
}

// verticalTest: within ZTHR, or closing in altitude to co-altitude
// within tau seconds (the vertical tau, booklet p. 23).
func verticalTest(g TCASGeometry, tau, zthr float64) bool {
	if math.Abs(g.DZFt) <= zthr {
		return true
	}
	// Closing: the intruder above and coming down relative to own, or below
	// and coming up.
	if g.DZFt*g.DZRateFpm >= 0 {
		return false
	}
	return math.Abs(g.DZFt)/math.Abs(g.DZRateFpm)*60 <= tau
}

// Evaluate is the advisory own a has against intruder b: an RA when both
// the range and the vertical tests pass at the RA thresholds of its
// sensitivity level, a TA at the TA thresholds; none on the ground.
func Evaluate(a, b TCASTrack) (Advisory, TCASLevel, TCASGeometry) {
	lv := TCASSensitivity(a.AltFt, a.AGLFt)
	g := Geometry(a, b)
	if a.OnGround || b.OnGround {
		return AdvisoryNone, lv, g
	}
	if lv.RATau > 0 && a.AGLFt >= TCASNoRABelowAGLFt && rangeTest(g, lv.RATau, lv.RADMODNM) && verticalTest(g, lv.RATau, lv.RAZThrFt) {
		return AdvisoryRA, lv, g
	}
	if rangeTest(g, lv.TATau, lv.TADMODNM) && verticalTest(g, lv.TATau, lv.TAZThrFt) {
		return AdvisoryTA, lv, g
	}
	return AdvisoryNone, lv, g
}

// timeToCPA is the time (s) to the closest point of approach, at least
// one second: the range over the closing speed.
func timeToCPA(g TCASGeometry) float64 {
	if g.RangeRateKts >= 0 {
		return 1
	}
	return math.Max(1, g.RangeNM/(-g.RangeRateKts/3600))
}

// ownAltAt is own altitude change (ft) after t seconds flying to fpm from
// vs, starting after delay seconds at accel g.
func ownAltAt(vs, fpm, t, delay, g float64) float64 {
	if t <= delay {
		return vs / 60 * t
	}
	acc := g * 9.80665 * 196.85 // fpm gained a second: m/s² × 196.85 is ft/min per s
	dv := fpm - vs
	ramp := math.Abs(dv) / acc
	d := vs / 60 * delay
	tt := t - delay
	if tt <= ramp {
		return d + (vs*tt+math.Copysign(acc, dv)*tt*tt/2)/60
	}
	return d + (vs*ramp+math.Copysign(acc, dv)*ramp*ramp/2)/60 + fpm/60*(tt-ramp)
}

// SelectRA chooses the RA for own a against intruder b (booklet p. 29):
// the sense with ALIM at CPA, the non-crossing one first; then the least
// disruptive strength: monitor (already safe), level off (a reduction
// gives ALIM), else climb or descend at TCASRAFpm, crossing when it passes
// the intruder's altitude. forced (+1/-1) is a sense the intruder's TCAS
// coordinated (its complement); 0 none. Descend RAs are inhibited low.
func SelectRA(a, b TCASTrack, lv TCASLevel, forced int) RA {
	g := Geometry(a, b)
	t := timeToCPA(g)
	intrAt := g.DZFt + b.VSFpm/60*t // intruder relative to own's start, at CPA
	sepWith := func(fpm, delay float64) float64 {
		return math.Abs(intrAt - ownAltAt(a.VSFpm, fpm, t, delay, TCASResponseG))
	}
	crosses := func(sense int) bool { return float64(sense)*g.DZFt > 0 } // the intruder on that side now
	noDescend := a.AGLFt < TCASNoDescendBelowAGLFt
	senses := []int{1, -1}
	switch {
	case forced < 0 && noDescend:
		// The intruder climbs, own may not descend this low (#103): it stops
		// its climb rather than climb into it.
		return RA{Kind: RALevelOff, Sense: -1}
	case forced != 0:
		senses = []int{forced}
	case noDescend:
		senses = []int{1}
	}
	best, bestSep := senses[0], -1.0
	for _, s := range senses {
		sep := sepWith(float64(s)*TCASRAFpm, TCASResponseDelaySec)
		if !crosses(s) && sep >= lv.ALIMFt {
			best, bestSep = s, sep
			break // the non-crossing sense that gives ALIM
		}
		if sep > bestSep {
			best, bestSep = s, sep
		}
	}
	// Strength: least disruptive that still gives ALIM.
	switch {
	case float64(best)*a.VSFpm >= 0 && sepWith(a.VSFpm, 0) >= lv.ALIMFt:
		return RA{Kind: RAMonitor, Sense: best, TargetFpm: a.VSFpm}
	case float64(best)*a.VSFpm < 0 && sepWith(0, TCASResponseDelaySec) >= lv.ALIMFt:
		return RA{Kind: RALevelOff, Sense: best}
	}
	kind := RAClimb
	if best < 0 {
		kind = RADescend
	}
	cross := crosses(best)
	if cross {
		kind = RACrossingClimb
		if best < 0 {
			kind = RACrossingDescend
		}
	}
	return RA{Kind: kind, Sense: best, TargetFpm: float64(best) * TCASRAFpm, Crossing: cross}
}
