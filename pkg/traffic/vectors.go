package traffic

import (
	"fmt"
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Vector is a radar vector an arrival off its STAR is given (#661): a
// heading at a corner of its path with the reason, or back onto the STAR
// direct to a fix. A dog-leg or an extended downwind is flown on vectors:
// "fly heading 060, for spacing" leaving the STAR, "turn left heading 150,
// for base" (or "resume own navigation direct GOLOP" after a dog-leg), then
// the intercept with the approach clearance (InterceptHeading).
type Vector struct {
	// At is the corner the vector is given before, as the arrival comes
	// within cornerAhead's reach of it; zero: at once.
	At airport.LatLon `json:"at"`
	// HeadingDeg is the true heading to fly; Turn the way to it ("left",
	// "right"; "" fly heading, leaving the STAR).
	HeadingDeg float64 `json:"headingDeg"`
	Turn       string  `json:"turn,omitempty"`
	// For is the reason said: "spacing", "base" (Doc 4444 12.4.1.5 note).
	For string `json:"for,omitempty"`
	// Fix: resume own navigation direct to it (12.4.1.4 b), no heading.
	Fix string `json:"fix,omitempty"`
}

// vectorsFor is what an arrival off its STAR is told along plain (its
// corners from pos, names alike): leaving the STAR before baseAt (an
// extended downwind) or apexAt (a dog-leg), and the turn there; none for
// neither (-1).
func vectorsFor(pos airport.LatLon, plain []types.SIMCONNECT_DATA_WAYPOINT, names []string, baseAt, apexAt int) []Vector {
	ll := func(i int) airport.LatLon {
		if i < 0 {
			return airport.LatLon{} // where it is: at once
		}
		return airport.LatLon{Lat: plain[i].Latitude, Lon: plain[i].Longitude}
	}
	into := func(i int) float64 { // the heading of the leg into corner i
		from := pos
		if i > 0 {
			from = ll(i - 1)
		}
		to := ll(i)
		return calc.BearingDegrees(from.Lat, from.Lon, to.Lat, to.Lon)
	}
	at := baseAt
	if at < 0 {
		at = apexAt
	}
	if at < 0 || at+1 >= len(plain) {
		return nil
	}
	out := []Vector{{At: ll(at - 1), HeadingDeg: into(at), For: "spacing"}}
	turn := Vector{At: ll(at), HeadingDeg: into(at + 1)}
	switch {
	case baseAt >= 0:
		turn.For = "base"
	default: // back onto the STAR, direct to its next named fix (not the final's points)
		for j := at + 1; j < len(names) && j < len(plain)-2; j++ {
			if names[j] != "" {
				turn.Fix = names[j]
				break
			}
		}
	}
	return append(out, turn)
}

// VectorDue is the next radar vector to say, once the arrival is about to
// turn at its corner (or at once), and takes it; false when none is due.
// The turn is the way from its heading now to the vector's.
func (c *ArrivalController) VectorDue() (Vector, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.holding != nil {
		return Vector{}, false // in the hold: no vectors
	}
	for len(c.vectors) > 0 {
		v := c.vectors[0]
		if v.At != (airport.LatLon{}) {
			i := c.cornerIndex(v.At)
			if i < 0 {
				c.vectors = c.vectors[1:] // its corner is gone (a shortcut, a re-plan)
				continue
			}
			if c.cornerAhead() <= i {
				return Vector{}, false
			}
		}
		c.vectors = c.vectors[1:]
		if v.For != "spacing" && v.Fix == "" {
			v.Turn = TurnTo(c.last.Heading, v.HeadingDeg)
		}
		return v, true
	}
	return Vector{}, false
}

// InterceptHeading is the true heading onto the final an arrival on
// vectors is given with its approach clearance ("turn left heading 245 to
// intercept"); false when it flies its procedure on its own navigation.
func (c *ArrivalController) InterceptHeading() (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.corners)
	if !c.vectored || n < 3 {
		return 0, false
	}
	a, b := c.corners[n-3], c.corners[n-2]
	return calc.BearingDegrees(a.Latitude, a.Longitude, b.Latitude, b.Longitude), true
}

// cornerIndex is the index of the corner at p, -1 none. c.mu held.
func (c *ArrivalController) cornerIndex(p airport.LatLon) int {
	for i, w := range c.corners {
		if math.Abs(w.Latitude-p.Lat) < 1e-9 && math.Abs(w.Longitude-p.Lon) < 1e-9 {
			return i
		}
	}
	return -1
}

// TurnTo is the way to turn from heading from to heading to: "left" or
// "right" (right for a reversal).
func TurnTo(from, to float64) string {
	if math.Mod(to-from+540, 360)-180 < 0 {
		return "left"
	}
	return "right"
}

// HeadingSaid is a true heading as ATC says it: magnetic (magVar:
// airport.Procedures.MagVar, magnetic = true + magVar), three digits, 360
// for north.
func HeadingSaid(trueDeg, magVar float64) string {
	h := math.Mod(math.Round(trueDeg+magVar)+720, 360)
	if h == 0 {
		h = 360
	}
	return fmt.Sprintf("%03.0f", h)
}

// Vectored says a radar vector to cs (#661): "CSA1, fly heading 060, for
// spacing", "CSA1, turn left heading 150, for base", "CSA1, resume own
// navigation direct GOLOP". magVar turns the true heading magnetic
// (airport.Procedures.MagVar).
func Vectored(cs string, v Vector, magVar float64) Transmission {
	p := map[string]string{ParamFix: v.Fix, ParamTurn: v.Turn, ParamFor: v.For}
	if v.Fix == "" {
		p[ParamHeading] = HeadingSaid(v.HeadingDeg, magVar)
	}
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentVector, Params: p})
}

// WithVector adds radar vector v to transmission t, one call instead of
// two in a row: "KLM868, number 3, for spacing reduce speed to 210 knots,
// fly heading 142"; its readback reads back both (live, KLM868 heard the
// two 0.1 s apart).
func WithVector(t Transmission, v Vector, magVar float64) Transmission {
	return Joined(t, Vectored(t.Callsign, v, magVar))
}

// Joined is t with u (another instruction to the same aircraft) said in
// the same call, read back together: "CSA1, identified, climb to flight
// level 240, cleared direct to ARTUP" (live, LOT924 two calls, #707).
func Joined(t, u Transmission) Transmission {
	said := strings.TrimPrefix(u.Text, u.Callsign+", ")
	rb := ""
	if r, ok := Readback(u); ok {
		rb = strings.TrimSuffix(r.Text, ", "+u.Callsign)
		rb = strings.ToLower(rb[:1]) + rb[1:]
	}
	p := map[string]string{}
	for k, x := range t.Params {
		p[k] = x
	}
	if s := p[ParamAlsoSaid]; s != "" {
		said = s + ", " + said
	}
	if r := p[ParamAlsoReadback]; r != "" && rb != "" {
		rb = r + ", " + rb
	} else if r != "" {
		rb = r
	}
	p[ParamAlsoSaid], p[ParamAlsoReadback] = said, rb
	t.Params = p
	t.Text += ", " + strings.TrimPrefix(u.Text, u.Callsign+", ")
	return t
}
