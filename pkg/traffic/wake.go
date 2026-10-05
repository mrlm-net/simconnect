package traffic

import (
	"fmt"
	"strings"
	"time"
)

// Wake turbulence separation (#389): the categories of the aircraft types,
// the spacing an arrival keeps behind the one landing before it, the
// interval between departures, and how long each occupies the runway. The
// approach sequencer, the runway controller and the go-around logic of
// v0.16 build on these.
//
// Sources: ICAO Doc 4444 (PANS-ATM) §5.8 and §8.7.3.4 for the ICAO wake
// turbulence categories (WTC) and their distance and time minima;
// EUROCONTROL RECAT-EU (2018) for the six categories A–F and their
// distance matrix. Types are assigned by MTOW (ICAO) and by the RECAT-EU
// category tables; types not listed take the category of their wing span.

// WakeCategory is the ICAO wake turbulence category.
type WakeCategory byte

const (
	WakeLight  WakeCategory = 'L' // MTOW 7 t or less
	WakeMedium WakeCategory = 'M' // 7–136 t
	WakeHeavy  WakeCategory = 'H' // 136 t or more
	WakeSuper  WakeCategory = 'J' // A380 (and An-225)
)

func (w WakeCategory) String() string { return string(w) }

// MarshalText makes the category its letter in JSON.
func (w WakeCategory) MarshalText() ([]byte, error) { return []byte{byte(w)}, nil }

// UnmarshalText reads the letter back (#768).
func (w *WakeCategory) UnmarshalText(b []byte) error {
	if len(b) != 1 {
		return fmt.Errorf("traffic: wake category %q", b)
	}
	*w = WakeCategory(b[0])
	return nil
}

// RecatCategory is the RECAT-EU wake category, A (super heavy) to F
// (light).
type RecatCategory byte

const (
	RecatA RecatCategory = 'A' // super heavy: A388
	RecatB RecatCategory = 'B' // upper heavy: B744, B748, B77W, A35K, A346
	RecatC RecatCategory = 'C' // lower heavy: B787, A330, A359, B767, A310
	RecatD RecatCategory = 'D' // upper medium: A320 family, B737, B757
	RecatE RecatCategory = 'E' // lower medium: E-Jets, CRJ, ATR, Dash 8
	RecatF RecatCategory = 'F' // light
)

func (r RecatCategory) String() string { return string(r) }

// MarshalText makes the category its letter in JSON.
func (r RecatCategory) MarshalText() ([]byte, error) { return []byte{byte(r)}, nil }

// UnmarshalText reads the letter back (#768).
func (r *RecatCategory) UnmarshalText(b []byte) error {
	if len(b) != 1 {
		return fmt.Errorf("traffic: RECAT category %q", b)
	}
	*r = RecatCategory(b[0])
	return nil
}

// Wake is a type's wake categories.
type Wake struct {
	ICAO  WakeCategory  `json:"icao"`
	Recat RecatCategory `json:"recat"`
}

// wakeTypes are the categories by ICAO type designator.
var wakeTypes = map[string]Wake{
	"A388": {WakeSuper, RecatA},
	"A124": {WakeHeavy, RecatB}, "B744": {WakeHeavy, RecatB}, "B748": {WakeHeavy, RecatB}, "B74F": {WakeHeavy, RecatB},
	"B77W": {WakeHeavy, RecatB}, "B77L": {WakeHeavy, RecatB}, "B772": {WakeHeavy, RecatB}, "B773": {WakeHeavy, RecatB},
	"A35K": {WakeHeavy, RecatB}, "A346": {WakeHeavy, RecatB}, "A345": {WakeHeavy, RecatB}, "MD11": {WakeHeavy, RecatB},
	"A359": {WakeHeavy, RecatC}, "A332": {WakeHeavy, RecatC}, "A333": {WakeHeavy, RecatC}, "A338": {WakeHeavy, RecatC}, "A339": {WakeHeavy, RecatC},
	"A343": {WakeHeavy, RecatC}, "A306": {WakeHeavy, RecatC}, "A310": {WakeHeavy, RecatC},
	"B788": {WakeHeavy, RecatC}, "B789": {WakeHeavy, RecatC}, "B78X": {WakeHeavy, RecatC},
	"B762": {WakeHeavy, RecatC}, "B763": {WakeHeavy, RecatC}, "B764": {WakeHeavy, RecatC},
	"A318": {WakeMedium, RecatD}, "A319": {WakeMedium, RecatD}, "A320": {WakeMedium, RecatD}, "A321": {WakeMedium, RecatD},
	"A19N": {WakeMedium, RecatD}, "A20N": {WakeMedium, RecatD}, "A21N": {WakeMedium, RecatD},
	"B736": {WakeMedium, RecatD}, "B737": {WakeMedium, RecatD}, "B738": {WakeMedium, RecatD}, "B739": {WakeMedium, RecatD},
	"B37M": {WakeMedium, RecatD}, "B38M": {WakeMedium, RecatD}, "B39M": {WakeMedium, RecatD},
	"B733": {WakeMedium, RecatD}, "B734": {WakeMedium, RecatD}, "B752": {WakeMedium, RecatD}, "B753": {WakeMedium, RecatD},
	"BCS1": {WakeMedium, RecatE}, "BCS3": {WakeMedium, RecatD},
	"E170": {WakeMedium, RecatE}, "E175": {WakeMedium, RecatE}, "E190": {WakeMedium, RecatE}, "E195": {WakeMedium, RecatE},
	"E290": {WakeMedium, RecatE}, "E295": {WakeMedium, RecatE},
	"CRJ7": {WakeMedium, RecatE}, "CRJ9": {WakeMedium, RecatE}, "CRJX": {WakeMedium, RecatE},
	"AT45": {WakeMedium, RecatE}, "AT46": {WakeMedium, RecatE}, "AT72": {WakeMedium, RecatE}, "AT75": {WakeMedium, RecatE}, "AT76": {WakeMedium, RecatE},
	"DH8A": {WakeMedium, RecatE}, "DH8C": {WakeMedium, RecatE}, "DH8D": {WakeMedium, RecatE}, "SF34": {WakeMedium, RecatE}, "SU95": {WakeMedium, RecatE},
	"C208": {WakeLight, RecatF}, "C172": {WakeLight, RecatF}, "SR22": {WakeLight, RecatF}, "PC12": {WakeLight, RecatF}, "BE20": {WakeLight, RecatF},
	"C25A": {WakeLight, RecatF}, "C25B": {WakeLight, RecatF}, "SF50": {WakeLight, RecatF}, "TBM9": {WakeLight, RecatF},
	"C152": {WakeLight, RecatF}, "P28A": {WakeLight, RecatF}, "DA40": {WakeLight, RecatF},
	// MTOW 7 t or less as well (live, OKFHP: a King Air 350 taken for a medium).
	"B350": {WakeLight, RecatF}, "DA62": {WakeLight, RecatF}, "BE58": {WakeLight, RecatF}, "C510": {WakeLight, RecatF},
	"E50P": {WakeLight, RecatF}, "C182": {WakeLight, RecatF}, "PA46": {WakeLight, RecatF},
}

// WakeFor is the wake categories of a type: an ICAO type designator, or a
// model title ProfileFor understands. Types not listed take the category of
// their wing span (below 15 m light, below 52 m medium, below 70 m heavy,
// super above); unknown altogether, medium.
func WakeFor(typ string) Wake {
	t := strings.ToUpper(strings.TrimSpace(typ))
	wake := wakeNow.Load()
	if w, ok := wake[t]; ok {
		return w
	}
	p := ProfileFor(typ)
	if w, ok := wake[p.Type]; ok {
		return w
	}
	switch s := p.WingspanM; {
	case p.Type == "" && s == DefaultAircraftProfile().WingspanM:
		return Wake{WakeMedium, RecatD} // nothing known
	case s < 15:
		return Wake{WakeLight, RecatF}
	case s < 32:
		return Wake{WakeMedium, RecatE}
	case s < 52:
		return Wake{WakeMedium, RecatD}
	case s < 70:
		return Wake{WakeHeavy, RecatC}
	}
	return Wake{WakeSuper, RecatA}
}

// MinRadarSeparationNM is the minimum radar separation on final when no
// wake minimum applies (ICAO: 3 NM; 2.5 NM under conditions the approach
// sequencer may choose).
const MinRadarSeparationNM = 3.0

// icaoArrivalNM are the ICAO wake minima on final (Doc 4444 §8.7.3.4), by
// leader and follower; pairs not listed take MinRadarSeparationNM.
var icaoArrivalNM = map[[2]WakeCategory]float64{
	{WakeSuper, WakeHeavy}: 6, {WakeSuper, WakeMedium}: 7, {WakeSuper, WakeLight}: 8,
	{WakeHeavy, WakeHeavy}: 4, {WakeHeavy, WakeMedium}: 5, {WakeHeavy, WakeLight}: 6,
	{WakeMedium, WakeLight}: 5,
}

// recatArrivalNM is the RECAT-EU distance matrix (leader row, follower
// column, A–F); 0 is the minimum radar separation.
var recatArrivalNM = [6][6]float64{
	/* A */ {3, 4, 5, 5, 6, 8},
	/* B */ {0, 3, 4, 4, 5, 7},
	/* C */ {0, 0, 3, 3, 4, 6},
	/* D */ {0, 0, 0, 0, 0, 5},
	/* E */ {0, 0, 0, 0, 0, 4},
	/* F */ {0, 0, 0, 0, 0, 3},
}

// SeparationScheme picks the wake minima.
type SeparationScheme uint8

const (
	SchemeICAO  SeparationScheme = iota // ICAO WTC (L/M/H/J)
	SchemeRecat                         // RECAT-EU (A–F)
)

// ArrivalSeparationNM is the spacing a follower keeps behind a leader
// landing on the same runway: the wake minimum of the pair, at least
// MinRadarSeparationNM.
func ArrivalSeparationNM(leader, follower Wake, scheme SeparationScheme) float64 {
	d := wakeArrivalNM(leader, follower, scheme)
	if d < MinRadarSeparationNM {
		return MinRadarSeparationNM
	}
	return d
}

// wakeArrivalNM is the wake turbulence minimum alone for the pair, 0 when
// none applies (the radar minimum governs).
func wakeArrivalNM(leader, follower Wake, scheme SeparationScheme) float64 {
	if scheme == SchemeRecat && leader.Recat >= RecatA && leader.Recat <= RecatF && follower.Recat >= RecatA && follower.Recat <= RecatF {
		return recatArrivalNM[leader.Recat-RecatA][follower.Recat-RecatA]
	}
	return icaoArrivalNM[[2]WakeCategory{leader.ICAO, follower.ICAO}]
}

// SeparationTime is a distance on final as time at the follower's ground
// speed (time-based separation keeps the time in a headwind).
func SeparationTime(distNM, followerKts float64) time.Duration {
	if followerKts <= 0 {
		followerKts = 140
	}
	return time.Duration(distNM / followerKts * float64(time.Hour))
}

// Departure intervals: DefaultDepartureInterval between departures on
// diverging routes, SameRouteDepartureInterval on the same SID; behind a
// heavy (ICAO: medium and light followers) WakeDepartureInterval, behind a
// super WakeSuperDepartureInterval (heavy followers: WakeDepartureInterval).
const (
	DefaultDepartureInterval   = time.Minute
	SameRouteDepartureInterval = 2 * time.Minute
	WakeDepartureInterval      = 2 * time.Minute
	WakeSuperDepartureInterval = 3 * time.Minute
)

// DepartureInterval is how long a follower waits to take off after a
// leader from the same runway: the wake time minimum of the pair (Doc 4444
// §5.8.3), and SameRouteDepartureInterval when both fly the same route.
func DepartureInterval(leader, follower Wake, sameRoute bool) time.Duration {
	d := DefaultDepartureInterval
	if sameRoute {
		d = SameRouteDepartureInterval
	}
	w := time.Duration(0)
	switch leader.ICAO {
	case WakeSuper:
		w = WakeDepartureInterval
		if follower.ICAO != WakeHeavy && follower.ICAO != WakeSuper {
			w = WakeSuperDepartureInterval
		}
	case WakeHeavy:
		if follower.ICAO == WakeMedium || follower.ICAO == WakeLight {
			w = WakeDepartureInterval
		}
	}
	if w > d {
		return w
	}
	return d
}

// RunwayOccupancy is a typical time on the runway: landing, from the
// threshold to clear of it; departing, from lining up to lift-off.
func RunwayOccupancy(w Wake, landing bool) time.Duration {
	sec := map[WakeCategory][2]float64{ // landing, departing
		WakeSuper: {70, 60}, WakeHeavy: {60, 50}, WakeMedium: {50, 45}, WakeLight: {45, 40},
	}[w.ICAO]
	if sec == [2]float64{} {
		sec = [2]float64{50, 45}
	}
	if landing {
		return time.Duration(sec[0] * float64(time.Second))
	}
	return time.Duration(sec[1] * float64(time.Second))
}

// Catch-up on the same route: a follower climbing CatchUpFromKts or more
// faster than the departure before it waits CatchUpPer40Kts more per 40 kt
// of the difference, at most CatchUpMax, so it does not close on it after
// take-off (live, LKPR: a B738 four minutes behind a C25C on VENO7D flew
// through it). The project's own rule: no source read gives the numbers.
const (
	CatchUpFromKts  = 20.0
	CatchUpPer40Kts = time.Minute
	CatchUpMax      = 3 * time.Minute
)

// DepartureIntervalSpeeds is DepartureInterval with the climb speeds of the
// two (TAS, 0 unknown): on the same route a faster follower waits longer.
func DepartureIntervalSpeeds(leader, follower Wake, sameRoute bool, leaderKts, followerKts float64) time.Duration {
	d := DepartureInterval(leader, follower, sameRoute)
	if !sameRoute || leaderKts <= 0 || followerKts <= 0 {
		return d
	}
	if diff := followerKts - leaderKts; diff >= CatchUpFromKts {
		d += min(CatchUpMax, time.Duration(diff/40*float64(CatchUpPer40Kts)))
	}
	return d
}

// DepartureFirstKts: at the holding points, a departure this much faster
// in the climb on the same route as the one before it, there no more than
// DepartureFirstWithin later, goes first.
const (
	DepartureFirstKts    = 40.0
	DepartureFirstWithin = 2 * time.Minute
)

// WithWake is t, an aircraft's initial call to an ATS unit, with "heavy"
// or "super" right after its call sign for an aircraft of that wake
// category (Doc 4444 4.9.2): "Ruzyne Radar, Speedbird 1367 heavy, ...".
// Other categories: t as it is.
func WithWake(t Transmission, w WakeCategory) Transmission {
	word := ""
	switch w {
	case WakeHeavy:
		word = "heavy"
	case WakeSuper:
		word = "super"
	}
	if word == "" || t.Callsign == "" {
		return t
	}
	t.Text = strings.Replace(t.Text, t.Callsign, t.Callsign+" "+word, 1)
	return t
}
