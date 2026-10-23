package nav

import (
	"math"
	"strings"
)

// Cruising levels by direction of flight (the semicircular or hemispheric
// rule), as Wikipedia's "Flight level" gives them:
//
//   - IFR below FL290: magnetic track 000–179° odd thousands (FL250, 270),
//     180–359° even thousands (FL260, 280).
//   - IFR in RVSM airspace, FL290–FL410: 000–179° odd thousands (FL290,
//     310, 330), 180–359° even (FL300, 320, 340).
//   - IFR at FL410 and above: 4,000 ft apart, 000–179° FL410, 450, 490;
//     180–359° FL430, 470, 510.
//   - IFR from FL290 without RVSM: 4,000 ft apart, 000–179° FL290, 330,
//     370; 180–359° FL310, 350, 390.
//   - Where the airways run north and south the split is north/south: in
//     Italy, France, Portugal and Spain southbound traffic uses odd levels,
//     in New Zealand even ones.
//
// VFR levels are the IFR ones plus 500 ft (FL35, 55 eastbound; FL45, 65
// westbound), below FL200: the ICAO table of cruising levels, which the
// article lists without stating it in words.

// LevelRule is the cruising-level scheme of an airspace.
type LevelRule struct {
	// NorthSouth splits by north and south (southbound: track 090–269°)
	// instead of east and west; SouthOdd gives the southbound odd levels.
	NorthSouth, SouthOdd bool
	// NoRVSM: no reduced vertical separation; from FL290 levels are 4,000 ft
	// apart.
	NoRVSM bool
}

// LevelRuleFor is the scheme of the country of airport icao (its ICAO
// prefix): Italy (LI), France (LF), Portugal (LP) and Spain (LE) north
// and south, southbound odd; New Zealand (NZ) north and south, southbound
// even; elsewhere east and west, RVSM.
func LevelRuleFor(icao string) LevelRule {
	switch p := strings.ToUpper(icao); {
	case strings.HasPrefix(p, "LI"), strings.HasPrefix(p, "LF"), strings.HasPrefix(p, "LP"), strings.HasPrefix(p, "LE"):
		return LevelRule{NorthSouth: true, SouthOdd: true}
	case strings.HasPrefix(p, "NZ"):
		return LevelRule{NorthSouth: true}
	}
	return LevelRule{}
}

// oddLevels reports whether a flight on magTrack (degrees magnetic) takes
// the odd levels under r.
func (r LevelRule) oddLevels(magTrack float64) bool {
	t := math.Mod(magTrack+360, 360)
	if r.NorthSouth {
		south := t >= 90 && t < 270
		return south == r.SouthOdd
	}
	return t < 180
}

// Valid reports whether fl (hundreds of feet) is a cruising level for a
// flight on magTrack under r, IFR or VFR.
func (r LevelRule) Valid(fl int, magTrack float64, vfr bool) bool {
	odd := r.oddLevels(magTrack)
	if fl <= 0 {
		return false
	}
	if vfr {
		return fl < 200 && fl%10 == 5 && (fl/10)%2 == 1 == odd
	}
	if fl%10 != 0 {
		return false
	}
	if fl < 290 || fl <= 410 && !r.NoRVSM {
		return (fl/10)%2 == 1 == odd
	}
	// 4,000 ft apart: odd FL290, 330, 370, 410, 450 …; even FL310, 350,
	// 390, 430, 470 … (above FL410 with RVSM the same sequences).
	if odd {
		return (fl-290)%40 == 0
	}
	return (fl-310)%40 == 0
}

// Below is the highest cruising level at or below fl under r for the
// flight (0 none).
func (r LevelRule) Below(fl int, magTrack float64, vfr bool) int {
	for ; fl > 0; fl-- {
		if r.Valid(fl, magTrack, vfr) {
			return fl
		}
	}
	return 0
}

// Above is the lowest cruising level at or above fl under r for the
// flight.
func (r LevelRule) Above(fl int, magTrack float64, vfr bool) int {
	for ; fl <= 600; fl++ {
		if r.Valid(fl, magTrack, vfr) {
			return fl
		}
	}
	return 0
}
