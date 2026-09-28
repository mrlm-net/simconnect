//go:build windows
// +build windows

package traffic

import (
	"math"
	"strings"
)

// AircraftCategory is the broad class of an aircraft type.
type AircraftCategory string

const (
	CategoryJet       AircraftCategory = "jet"
	CategoryTurboprop AircraftCategory = "turboprop"
	CategoryPiston    AircraftCategory = "piston"
)

// FlapSchedule is how an injected aircraft sets its flaps, as percent of
// the flap handle travel (100 is the last detent).
type FlapSchedule struct {
	// TakeoffPct is set while taxiing out and retracted from RetractFt in
	// the climb.
	TakeoffPct float64
	// ApproachPct is set on final and runs to LandingPct when passing
	// FullFt (the stabilised-approach gate).
	ApproachPct, LandingPct float64
	RetractFt, FullFt       float64
}

// LightUsage is how a type differs from the common light rules.
type LightUsage struct {
	// NoLogo keeps the logo light off on approach (types without one).
	NoLogo bool
}

// AircraftProfile bundles every per-type figure injected traffic uses
// (#324): the airframe, the ground motion, take-off, approach and rollout,
// the stand stop, flaps, pushback and lights. ProfileFor resolves one from
// a model title or type designator; Refine adjusts it with the figures the
// simulator reports for the spawned aircraft (#325).
type AircraftProfile struct {
	// Type is the ICAO type designator (A20N, B77W), "" for a generic
	// profile.
	Type     string
	Category AircraftCategory
	// WingspanM, LengthM and WheelbaseM are the airframe in meters;
	// ICAOCode is the aerodrome reference code letter from the span.
	WingspanM, LengthM, WheelbaseM float64
	ICAOCode                       byte
	// CGHeightM is the reference point's height above the ground on the
	// wheels (STATIC CG TO GROUND).
	CGHeightM float64
	// TakeoffDistanceM is the published take-off distance at maximum
	// take-off weight, sea level ISA; a reference figure (the run an
	// injected take-off needs is RequiredTakeoffRun).
	TakeoffDistanceM float64

	Motion   MotionProfile
	Takeoff  TakeoffProfile
	Approach ApproachProfile
	Rollout  RolloutProfile
	// NoseOffsetM is the distance from the reference point to the nose,
	// placing the nose at the stand's stop mark.
	NoseOffsetM float64
	Flaps       FlapSchedule
	// PushbackKts is the pushback speed.
	PushbackKts float64
	Lights      LightUsage
}

// ICAOCodeFor is the aerodrome reference code letter of a wing span in
// meters (ICAO Annex 14): A below 15 m, B below 24, C below 36, D below 52,
// E below 65, F below 80; 0 for no span.
func ICAOCodeFor(spanM float64) byte {
	switch {
	case spanM <= 0:
		return 0
	case spanM < 15:
		return 'A'
	case spanM < 24:
		return 'B'
	case spanM < 36:
		return 'C'
	case spanM < 52:
		return 'D'
	case spanM < 65:
		return 'E'
	}
	return 'F'
}

// DefaultAircraftProfile is the A320 family profile every per-type default
// in this package stands for: DefaultMotionProfile, DefaultTakeoffProfile,
// DefaultApproachProfile, DefaultRolloutProfile, DefaultNoseOffsetMeters,
// the flap and pushback tunables. Its Type is "" (generic).
func DefaultAircraftProfile() AircraftProfile {
	p := a320Profile()
	p.Type = ""
	return p
}

func a320Profile() AircraftProfile {
	m := DefaultMotionProfile()
	return AircraftProfile{
		Type: "A320", Category: CategoryJet,
		WingspanM: m.SpanMeters, LengthM: 37.6, WheelbaseM: m.WheelbaseMeters, ICAOCode: ICAOCodeFor(m.SpanMeters),
		CGHeightM: spawnCGFt * 0.3048, TakeoffDistanceM: 2100,
		Motion: m, Takeoff: DefaultTakeoffProfile(), Approach: DefaultApproachProfile(), Rollout: DefaultRolloutProfile(),
		NoseOffsetM: DefaultNoseOffsetMeters,
		Flaps:       FlapSchedule{TakeoffPct: TakeoffFlapsPct, ApproachPct: ApproachFlapsPct, LandingPct: 100, RetractFt: FlapsRetractFt, FullFt: FlapsFullFt},
		PushbackKts: PushbackSpeedKts,
	}
}

// ProfileFor resolves an aircraft by its container title ("FSLTL A320 Air
// France SL", "Asobo PassiveAircraft B777-300ER"), ATC MODEL or ICAO type
// designator ("B38M") against the known types, the most specific match
// first. Titles that match none get DefaultAircraftProfile, the A320
// family figures every default in this package stands for; see
// GenericProfile for a fallback by size when the span is known.
func ProfileFor(model string) AircraftProfile {
	t := strings.ToUpper(model)
	for _, k := range knownTypes {
		for _, m := range k.match {
			if strings.Contains(t, m) {
				return k.profile()
			}
		}
	}
	return DefaultAircraftProfile()
}

// KnownTypes lists the ICAO type designators ProfileFor knows.
func KnownTypes() []string {
	out := make([]string, 0, len(knownTypes))
	for _, k := range knownTypes {
		out = append(out, k.Type)
	}
	return out
}

// GenericProfile is a profile for an unknown type of the given wing span
// (meters) and category: the figures of a representative type of its size
// — an ATR 72 for turboprops and pistons, an E190 for jets below 32 m, the
// A320 family below 40 m, a 787 below 62 m, a 777 below 70 m, an A380
// above — with Type "" and the airframe scaled to the span. No span gives
// DefaultAircraftProfile.
func GenericProfile(spanM float64, category AircraftCategory) AircraftProfile {
	var p AircraftProfile
	switch {
	case spanM <= 0:
		return DefaultAircraftProfile()
	case category == CategoryTurboprop || category == CategoryPiston:
		p = ProfileFor("AT76")
		p.Category = category
	case spanM < 32:
		p = ProfileFor("E190")
	case spanM < 40:
		p = DefaultAircraftProfile()
	case spanM < 62:
		p = ProfileFor("B789")
	case spanM < 70:
		p = ProfileFor("B77W")
	default:
		p = ProfileFor("A388")
	}
	p.Type = ""
	p.scaleTo(spanM)
	return p
}

// scaleTo scales the airframe to a wing span.
func (p *AircraftProfile) scaleTo(spanM float64) {
	if p.WingspanM <= 0 || spanM <= 0 {
		return
	}
	f := spanM / p.WingspanM
	p.WingspanM, p.LengthM, p.WheelbaseM = spanM, p.LengthM*f, p.WheelbaseM*f
	p.ICAOCode = ICAOCodeFor(spanM)
	p.Motion.SpanMeters, p.Motion.WheelbaseMeters, p.Motion.TailMeters = spanM, p.Motion.WheelbaseMeters*f, p.Motion.TailMeters*f
	p.NoseOffsetM *= f
}

// typeSpec is a known type's published figures; profile derives the rest.
type typeSpec struct {
	match    []string
	Type     string
	Category AircraftCategory
	// Airframe (m) and published take-off distance (m).
	span, length, wheelbase, cg, tod float64
	// vapp is the final approach speed; the touchdown speed is 5 kt less.
	vapp float64
	// Approach and flare pitch (°), flare height (ft), touchdown rate (fpm).
	pitch, flarePitch, flareFt, tdFpm float64
	takeoff                           TakeoffProfile
	// brake is the rollout deceleration (m/s²), taxi the taxi speed (kt).
	brake, taxi float64
	flaps       FlapSchedule
	heavy       bool // widebody: softer ground motion, slower pushback
}

func (s typeSpec) profile() AircraftProfile {
	if s.Type == "A320" || s.Type == "A20N" {
		p := a320Profile() // the tuned defaults, exactly
		p.Type = s.Type
		return p
	}
	m := DefaultMotionProfile()
	m.WheelbaseMeters, m.SpanMeters = s.wheelbase, s.span
	// The nose sits about 13% of the length ahead of the nose gear.
	m.TailMeters = s.length - 0.13*s.length - s.wheelbase
	m.CruiseKts = s.taxi
	push := PushbackSpeedKts
	r := DefaultRolloutProfile()
	r.BrakeDecel = s.brake
	a := DefaultApproachProfile()
	a.ApproachKts, a.TouchdownKts, a.StartKts = s.vapp, s.vapp-5, s.vapp+15
	a.ApproachPitchDeg, a.FlarePitchDeg, a.FlareFt, a.TouchdownFpm = s.pitch, s.flarePitch, s.flareFt, s.tdFpm
	if s.heavy {
		m.LateralAccel, m.Accel, m.Decel = 0.5, 0.35, 0.45
		push = 2.5
		r.HighSpeedExitKts = 30
		a.DerotateSeconds = 5
	}
	if s.Category == CategoryTurboprop {
		r.SlowKts = 60
	}
	flaps := s.flaps
	flaps.RetractFt, flaps.FullFt = FlapsRetractFt, FlapsFullFt
	return AircraftProfile{
		Type: s.Type, Category: s.Category,
		WingspanM: s.span, LengthM: s.length, WheelbaseM: s.wheelbase, ICAOCode: ICAOCodeFor(s.span),
		CGHeightM: s.cg, TakeoffDistanceM: s.tod,
		Motion: m, Takeoff: s.takeoff, Approach: a, Rollout: r,
		NoseOffsetM: math.Round((s.wheelbase-m.RefAheadMeters+0.13*s.length+0.5)*10) / 10,
		Flaps:       flaps,
		PushbackKts: push,
	}
}

// fill completes a request's figures from the aircraft profile where they
// are zero.
func (p AircraftProfile) fill(motion *MotionProfile, takeoff *TakeoffProfile, approach *ApproachProfile, rollout *RolloutProfile, nose *float64) {
	if motion != nil && *motion == (MotionProfile{}) {
		*motion = p.Motion
	}
	if takeoff != nil && *takeoff == (TakeoffProfile{}) {
		*takeoff = p.Takeoff
	}
	if approach != nil && *approach == (ApproachProfile{}) {
		*approach = p.Approach
	}
	if rollout != nil && *rollout == (RolloutProfile{}) {
		*rollout = p.Rollout
	}
	if nose != nil && *nose == 0 {
		*nose = p.NoseOffsetM
	}
}

// aircraftOf is the request's profile: aircraft, or resolved from the
// model, with anything zero from the defaults.
func aircraftOf(aircraft *AircraftProfile, model string) *AircraftProfile {
	var p AircraftProfile
	if aircraft != nil {
		p = *aircraft
	} else {
		p = ProfileFor(model)
	}
	d := DefaultAircraftProfile()
	d.fill(&p.Motion, &p.Takeoff, &p.Approach, &p.Rollout, &p.NoseOffsetM)
	if p.Flaps == (FlapSchedule{}) {
		p.Flaps = d.Flaps
	}
	if p.PushbackKts <= 0 {
		p.PushbackKts = d.PushbackKts
	}
	if p.CGHeightM <= 0 {
		p.CGHeightM = d.CGHeightM
	}
	return &p
}

// aircraftFor is a controller's resolved profile: the request's, or one
// resolved from the model when the request did not go through Start.
func aircraftFor(p *AircraftProfile, model string) *AircraftProfile {
	if p != nil {
		return p
	}
	return aircraftOf(nil, model)
}

func (c *TaxiController) aircraft() *AircraftProfile { return aircraftFor(c.req.Aircraft, c.req.Model) }

func (c *ArrivalController) aircraft() *AircraftProfile {
	return aircraftFor(c.req.Aircraft, c.req.Model)
}

// resolveAircraft sets req.Aircraft (from Model when nil) and fills the
// request's zero figures from it.
func (req *TaxiRequest) resolveAircraft() {
	req.Aircraft = aircraftOf(req.Aircraft, req.Model)
	req.Aircraft.fill(&req.Profile, &req.Takeoff, nil, nil, &req.NoseOffset)
	capTaxiSpeed(&req.Profile, req.Airport)
}

// resolveAircraft sets req.Aircraft (from Model when nil) and fills the
// request's zero figures from it.
func (req *ArrivalRequest) resolveAircraft() {
	req.Aircraft = aircraftOf(req.Aircraft, req.Model)
	req.Aircraft.fill(&req.Profile, nil, &req.Approach, &req.Rollout, &req.NoseOffset)
	capTaxiSpeed(&req.Profile, req.Airport)
}
