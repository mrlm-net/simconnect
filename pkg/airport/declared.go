package airport

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Declared distances and runway end use (#1660, #1661). The sim sends
// neither: data repos fill them per end from the AIP (RawRunway's
// primary*/secondary* fields). Every value is optional and 0 keeps what
// the runway geometry gives (the behaviour before them).

// RunwayEndUse is what a runway end is used for.
type RunwayEndUse int32

const (
	// RunwayUseBoth: take-offs and landings (the default).
	RunwayUseBoth RunwayEndUse = 0
	// RunwayUseTakeoff: take-offs only (EDDF 18).
	RunwayUseTakeoff RunwayEndUse = 1
	// RunwayUseLanding: landings only (EDDF 07L, 25R).
	RunwayUseLanding RunwayEndUse = 2
	// RunwayUseNone: neither (EDDF 36): crossed and taxied on only.
	RunwayUseNone RunwayEndUse = 3
)

// String is "both", "takeoff", "landing" or "none".
// UnmarshalJSON reads a use from a layout's JSON; one out of 0-3 is left
// out (0: take-offs and landings), as a raw record's is (CheckDeclared).
func (u *RunwayEndUse) UnmarshalJSON(b []byte) error {
	var v int32
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v < int32(RunwayUseBoth) || v > int32(RunwayUseNone) {
		v = int32(RunwayUseBoth)
	}
	*u = RunwayEndUse(v)
	return nil
}

func (u RunwayEndUse) String() string {
	switch u {
	case RunwayUseBoth:
		return "both"
	case RunwayUseTakeoff:
		return "takeoff"
	case RunwayUseLanding:
		return "landing"
	case RunwayUseNone:
		return "none"
	}
	return fmt.Sprintf("RunwayEndUse(%d)", int32(u))
}

// ErrBadDeclared is what CheckDeclared reports for a declared distance or
// use the layout does not take (Build leaves that value out: 0).
var ErrBadDeclared = errors.New("airport: declared runway value not taken")

// ErrRunwayEndUse is returned for a take-off from an end used for landings
// only, or a landing on one used for take-offs only, or either on an end not
// used (RunwayEnd.Use).
var ErrRunwayEndUse = errors.New("airport: runway end not used for this")

// declaredSlackM: declared distances may run this much past the runway's
// length (rounding between the AIP and the sim's runway).
const declaredSlackM = 1.0

// ForTakeoff reports whether e is used for take-offs.
func (e RunwayEnd) ForTakeoff() bool { return e.Use == RunwayUseBoth || e.Use == RunwayUseTakeoff }

// ForLanding reports whether e is used for landings.
func (e RunwayEnd) ForLanding() bool { return e.Use == RunwayUseBoth || e.Use == RunwayUseLanding }

// TakeoffStart is where a take-off roll on e starts: Threshold, or
// TakeoffStartM further along Heading.
func (e RunwayEnd) TakeoffStart() LatLon {
	if e.TakeoffStartM <= 0 {
		return e.Threshold
	}
	lat, lon := calc.DisplaceByHeading(e.Threshold.Lat, e.Threshold.Lon, e.Heading, e.TakeoffStartM)
	return LatLon{Lat: lat, Lon: lon}
}

// TakeoffRunAhead is the take-off run available on e (of a runway
// runwayLength long) to a roll starting along meters from Threshold: to
// the end of its TORA (TakeoffStartM + TORAM) when declared, else to the
// pavement end; a start before TakeoffStart counts from there. 0 when none
// is left. Neither declared: runwayLength - along, as before them.
func (e RunwayEnd) TakeoffRunAhead(along, runwayLength float64) float64 {
	if e.TORAM <= 0 && e.TakeoffStartM <= 0 {
		return runwayLength - along
	}
	end := runwayLength
	if e.TORAM > 0 {
		end = math.Max(0, e.TakeoffStartM) + e.TORAM
		if runwayLength > 0 {
			end = math.Min(end, runwayLength)
		}
	}
	return math.Max(0, end-math.Max(along, math.Max(0, e.TakeoffStartM)))
}

// TORA is e's take-off run available: TORAM when declared, else from
// TakeoffStart to the pavement end of a runway runwayLength long.
func (e RunwayEnd) TORA(runwayLength float64) float64 {
	return e.TakeoffRunAhead(0, runwayLength)
}

// ASDA is e's accelerate-stop distance available: ASDAM when declared, else
// its TORA.
func (e RunwayEnd) ASDA(runwayLength float64) float64 {
	if e.ASDAM > 0 {
		return e.ASDAM
	}
	return e.TORA(runwayLength)
}

// LandingDistance is e's landing distance available from LandingThreshold:
// LDAM when declared, else to the pavement end of a runway runwayLength
// long.
func (e RunwayEnd) LandingDistance(runwayLength float64) float64 {
	if e.LDAM > 0 {
		return e.LDAM
	}
	return math.Max(0, runwayLength-e.DisplacedM)
}

// LandingEnd is where e's landing distance available ends: LandingDistance
// past LandingThreshold along Heading.
func (e RunwayEnd) LandingEnd(runwayLength float64) LatLon {
	t := e.LandingThreshold()
	lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, e.Heading, e.LandingDistance(runwayLength))
	return LatLon{Lat: lat, Lon: lon}
}

// endDeclared is one end's raw declared values.
type endDeclared struct {
	name                   string
	start, tora, asda, lda float32
	use                    int32
}

// applyDeclared sets e's declared values from d on a runway length long,
// leaving out (0) each one CheckDeclared refuses; the refusals.
func applyDeclared(e *RunwayEnd, d endDeclared, length float32) []error {
	var errs []error
	bad := func(field string, v any, why string) {
		errs = append(errs, fmt.Errorf("%w: runway %s %s %v: %s", ErrBadDeclared, d.name, field, v, why))
	}
	L := float64(length)
	start := float64(d.start)
	switch {
	case start < 0:
		// The paved start before the threshold (NZQN) is not part of the
		// runway the layout has: no line-up or roll there.
		bad("takeoffStartM", d.start, "negative (a start before the runway's end is not supported)")
		start = 0
	case start > 0 && start >= L:
		bad("takeoffStartM", d.start, "not on the runway")
		start = 0
	}
	e.TakeoffStartM = start
	if t := float64(d.tora); t < 0 || t > 0 && start+t > L+declaredSlackM {
		bad("toraM", d.tora, "negative or past the runway's end")
	} else {
		e.TORAM = t
	}
	if a := float64(d.asda); a < 0 || a > 0 && a+declaredSlackM < e.TORA(L) {
		bad("asdaM", d.asda, "negative or shorter than the take-off run available")
	} else {
		e.ASDAM = a
	}
	if l := float64(d.lda); l < 0 || l > 0 && e.DisplacedM+l > L+declaredSlackM {
		bad("ldaM", d.lda, "negative or past the runway's end")
	} else {
		e.LDAM = l
	}
	if u := RunwayEndUse(d.use); u < RunwayUseBoth || u > RunwayUseNone {
		bad("use", d.use, "not 0-3")
	} else {
		e.Use = u
	}
	return errs
}

// rawDeclared are r's two ends' declared values.
func rawDeclared(r RawRunway) (p, s endDeclared) {
	p = endDeclared{runwayEndName(r.PrimaryNumber, types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR(r.PrimaryDesignator)), r.PrimaryTakeoffStartM, r.PrimaryTORAM, r.PrimaryASDAM, r.PrimaryLDAM, r.PrimaryUse}
	s = endDeclared{runwayEndName(r.SecondaryNumber, types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR(r.SecondaryDesignator)), r.SecondaryTakeoffStartM, r.SecondaryTORAM, r.SecondaryASDAM, r.SecondaryLDAM, r.SecondaryUse}
	return p, s
}

// CheckDeclared reports each declared distance or use in raw's runways that
// Build leaves out (ErrBadDeclared): a negative or off-runway take-off
// start offset, a TORA or LDA past the runway's end, an ASDA shorter than
// the TORA, a use other than 0-3. For data repos to check what they fill;
// nil when everything is taken.
func CheckDeclared(raw RawAirport) []error {
	var errs []error
	for _, r := range raw.Runways {
		p, s := rawDeclared(r)
		var e RunwayEnd
		e.DisplacedM = displacedM(r.PrimaryDisplacedM, r.Length)
		errs = append(errs, applyDeclared(&e, p, r.Length)...)
		e = RunwayEnd{DisplacedM: displacedM(r.SecondaryDisplacedM, r.Length)}
		errs = append(errs, applyDeclared(&e, s, r.Length)...)
	}
	return errs
}
