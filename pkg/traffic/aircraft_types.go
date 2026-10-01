//go:build windows
// +build windows

package traffic

import (
	"github.com/mrlm-net/simconnect/pkg/airport"
	"strings"
)

// Take-off figures by family. Tail-strike pitches are with the main gear on
// the runway; rotation and climb figures are typical, not for a given
// weight.
var (
	toB77W  = TakeoffProfile{RollAccel: 1.9, RotateKts: 160, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 175, ClimbFpm: 2000, ClimbRampSeconds: 3, TailstrikePitch: 8.5}
	toB772  = TakeoffProfile{RollAccel: 1.9, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 8.5, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2000, ClimbRampSeconds: 3, TailstrikePitch: 10.5}
	toB78X  = TakeoffProfile{RollAccel: 2.0, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2100, ClimbRampSeconds: 3, TailstrikePitch: 8.5}
	toB787  = TakeoffProfile{RollAccel: 2.1, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 8.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2300, ClimbRampSeconds: 3, TailstrikePitch: 9.5}
	toB747  = TakeoffProfile{RollAccel: 1.8, RotateKts: 160, RotateRate: 2.5, LiftoffPitch: 9, ClimbPitch: 14, ClimbKts: 175, ClimbFpm: 1800, ClimbRampSeconds: 3, TailstrikePitch: 11}
	toA388  = TakeoffProfile{RollAccel: 1.7, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 9, ClimbPitch: 13, ClimbKts: 170, ClimbFpm: 1700, ClimbRampSeconds: 3, TailstrikePitch: 12.5}
	toA35K  = TakeoffProfile{RollAccel: 2.0, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2100, ClimbRampSeconds: 3, TailstrikePitch: 9.5}
	toA359  = TakeoffProfile{RollAccel: 2.1, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 8.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2300, ClimbRampSeconds: 3, TailstrikePitch: 10.5}
	toA330  = TakeoffProfile{RollAccel: 2.0, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 165, ClimbFpm: 2000, ClimbRampSeconds: 3, TailstrikePitch: 10}
	toA321  = TakeoffProfile{RollAccel: 2.3, RotateKts: 145, RotateRate: 3, LiftoffPitch: 7.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2100, ClimbRampSeconds: 2.5, TailstrikePitch: 9.5}
	toA319  = TakeoffProfile{RollAccel: 2.5, RotateKts: 132, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15, ClimbKts: 155, ClimbFpm: 2400, ClimbRampSeconds: 2.5, TailstrikePitch: 13.5}
	toB739  = TakeoffProfile{RollAccel: 2.3, RotateKts: 150, RotateRate: 3, LiftoffPitch: 7.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2100, ClimbRampSeconds: 2.5, TailstrikePitch: 9}
	toB737  = TakeoffProfile{RollAccel: 2.4, RotateKts: 145, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15, ClimbKts: 160, ClimbFpm: 2200, ClimbRampSeconds: 2.5, TailstrikePitch: 11}
	toEJet  = TakeoffProfile{RollAccel: 2.6, RotateKts: 130, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15, ClimbKts: 150, ClimbFpm: 2300, ClimbRampSeconds: 2.5, TailstrikePitch: 12}
	toE195  = TakeoffProfile{RollAccel: 2.5, RotateKts: 135, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15, ClimbKts: 155, ClimbFpm: 2200, ClimbRampSeconds: 2.5, TailstrikePitch: 11}
	toCRJ9  = TakeoffProfile{RollAccel: 2.5, RotateKts: 140, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15, ClimbKts: 160, ClimbFpm: 2300, ClimbRampSeconds: 2.5, TailstrikePitch: 11}
	toTprop = TakeoffProfile{RollAccel: 2.4, RotateKts: 115, RotateRate: 3, LiftoffPitch: 7, ClimbPitch: 12, ClimbKts: 135, ClimbFpm: 1500, ClimbRampSeconds: 2.5, TailstrikePitch: 10}
)

// Flap schedules as percent of the handle travel: the detent's index over
// the last detent's (an A320's 1+F is 1 of 4, a 737's flaps 5 is 3 of 8).
var (
	flapsAirbus = FlapSchedule{TakeoffPct: 25, ApproachPct: 75, LandingPct: 100}      // 0 1 2 3 FULL: 1+F, 3, FULL
	flaps737    = FlapSchedule{TakeoffPct: 37.5, ApproachPct: 62.5, LandingPct: 87.5} // 0 1 2 5 10 15 25 30 40: 5, 15, 30
	flaps777    = FlapSchedule{TakeoffPct: 50, ApproachPct: 66.7, LandingPct: 100}    // 0 1 5 15 20 25 30: 15, 20, 30
	flaps787    = FlapSchedule{TakeoffPct: 22.2, ApproachPct: 77.8, LandingPct: 100}  // 0 1 5 10 15 17 18 20 25 30: 5, 20, 30
	flaps747    = FlapSchedule{TakeoffPct: 60, ApproachPct: 80, LandingPct: 100}      // 0 1 5 10 20 25 30: 10/20, 25, 30
	flapsEJet   = FlapSchedule{TakeoffPct: 33.3, ApproachPct: 50, LandingPct: 100}    // 0 1 2 3 4 5 FULL: 2, 3, FULL
	flapsCRJ    = FlapSchedule{TakeoffPct: 40, ApproachPct: 60, LandingPct: 100}      // 0 1 8 20 30 45: 8, 20, 45
	flapsATR    = FlapSchedule{TakeoffPct: 50, ApproachPct: 50, LandingPct: 100}      // 0 15 30: 15, 15, 30
	flapsQ400   = FlapSchedule{TakeoffPct: 25, ApproachPct: 75, LandingPct: 100}      // 0 5 10 15 35: 5, 15, 35
)

// knownTypes are the types ProfileFor resolves, matched against the
// upper-cased model title, ATC MODEL or type designator, the most specific
// first. Published figures, rounded: span, length and wheelbase (MSFS 2024
// does not report gear contact points to SimConnect, CONTACT POINT
// POSITION reads a constant 1 m), take-off distance at MTOW, final
// approach speed and attitude. Values the sim does not expose (pitch,
// flare, taxi speeds) are typical and tuned from live runs (#326).
var knownTypes = []typeSpec{
	// Widebodies.
	{match: []string{"B77W", "B773", "777-300", "777300", "777-3"}, Type: "B77W", Category: CategoryJet, span: 64.8, length: 73.9, wheelbase: 31.2, cg: 5.8, tod: 3050,
		vapp: 155, pitch: 2, flarePitch: 4.5, flareFt: 35, tdFpm: -150, takeoff: toB77W, brake: 2.0, taxi: 14, flaps: flaps777, heavy: true},
	{match: []string{"B77L", "777-200LR", "777F", "B77F"}, Type: "B77L", Category: CategoryJet, span: 64.8, length: 63.7, wheelbase: 25.9, cg: 5.6, tod: 3000,
		vapp: 150, pitch: 2.5, flarePitch: 5, flareFt: 35, tdFpm: -150, takeoff: toB772, brake: 2.0, taxi: 14, flaps: flaps777, heavy: true},
	{match: []string{"B772", "777-200", "777"}, Type: "B772", Category: CategoryJet, span: 60.9, length: 63.7, wheelbase: 25.9, cg: 5.6, tod: 3100,
		vapp: 145, pitch: 2.5, flarePitch: 5, flareFt: 35, tdFpm: -150, takeoff: toB772, brake: 2.0, taxi: 14, flaps: flaps777, heavy: true},
	{match: []string{"B78X", "787-10"}, Type: "B78X", Category: CategoryJet, span: 60.1, length: 68.3, wheelbase: 29.0, cg: 5.5, tod: 2900,
		vapp: 150, pitch: 2, flarePitch: 4.5, flareFt: 35, tdFpm: -150, takeoff: toB78X, brake: 2.0, taxi: 14, flaps: flaps787, heavy: true},
	{match: []string{"B789", "787-9", "787-09"}, Type: "B789", Category: CategoryJet, span: 60.1, length: 62.8, wheelbase: 25.9, cg: 5.5, tod: 2800,
		vapp: 150, pitch: 2, flarePitch: 5, flareFt: 35, tdFpm: -150, takeoff: toB787, brake: 2.0, taxi: 14, flaps: flaps787, heavy: true},
	{match: []string{"B788", "787"}, Type: "B788", Category: CategoryJet, span: 60.1, length: 56.7, wheelbase: 22.8, cg: 5.4, tod: 2600,
		vapp: 145, pitch: 2.5, flarePitch: 5, flareFt: 35, tdFpm: -150, takeoff: toB787, brake: 2.0, taxi: 14, flaps: flaps787, heavy: true},
	{match: []string{"B748", "747-8"}, Type: "B748", Category: CategoryJet, span: 68.4, length: 76.3, wheelbase: 29.7, cg: 6.5, tod: 3100,
		vapp: 155, pitch: 2.5, flarePitch: 5, flareFt: 40, tdFpm: -150, takeoff: toB747, brake: 1.8, taxi: 14, flaps: flaps747, heavy: true},
	{match: []string{"B744", "B74F", "747"}, Type: "B744", Category: CategoryJet, span: 64.4, length: 70.7, wheelbase: 25.6, cg: 6.5, tod: 3000,
		vapp: 150, pitch: 2.5, flarePitch: 5, flareFt: 40, tdFpm: -150, takeoff: toB747, brake: 1.8, taxi: 14, flaps: flaps747, heavy: true},
	{match: []string{"A388", "A380"}, Type: "A388", Category: CategoryJet, span: 79.8, length: 72.7, wheelbase: 33.6, cg: 7.0, tod: 3000,
		vapp: 145, pitch: 2.5, flarePitch: 5.5, flareFt: 40, tdFpm: -150, takeoff: toA388, brake: 1.8, taxi: 14, flaps: flapsAirbus, heavy: true},
	{match: []string{"A35K", "A350-1000"}, Type: "A35K", Category: CategoryJet, span: 64.8, length: 73.8, wheelbase: 32.5, cg: 5.6, tod: 2750,
		vapp: 150, pitch: 2.5, flarePitch: 5, flareFt: 35, tdFpm: -150, takeoff: toA35K, brake: 2.0, taxi: 14, flaps: flapsAirbus, heavy: true},
	{match: []string{"A359", "A350"}, Type: "A359", Category: CategoryJet, span: 64.8, length: 66.8, wheelbase: 28.7, cg: 5.5, tod: 2600,
		vapp: 145, pitch: 2.5, flarePitch: 5.5, flareFt: 35, tdFpm: -150, takeoff: toA359, brake: 2.0, taxi: 14, flaps: flapsAirbus, heavy: true},
	{match: []string{"A332", "A330-200"}, Type: "A332", Category: CategoryJet, span: 60.3, length: 58.8, wheelbase: 22.2, cg: 5.4, tod: 2600,
		vapp: 140, pitch: 2.5, flarePitch: 5, flareFt: 35, tdFpm: -150, takeoff: toA330, brake: 2.0, taxi: 14, flaps: flapsAirbus, heavy: true},
	{match: []string{"A333", "A338", "A339", "A330"}, Type: "A333", Category: CategoryJet, span: 60.3, length: 63.7, wheelbase: 25.4, cg: 5.4, tod: 2770,
		vapp: 140, pitch: 2.5, flarePitch: 5, flareFt: 35, tdFpm: -150, takeoff: toA330, brake: 2.0, taxi: 14, flaps: flapsAirbus, heavy: true},
	// Narrowbodies.
	{match: []string{"A21N", "A321NEO", "A321 NEO", "A321-NEO"}, Type: "A21N", Category: CategoryJet, span: 35.8, length: 44.5, wheelbase: 16.9, cg: 3.7, tod: 2200,
		vapp: 138, pitch: 2.5, flarePitch: 5, flareFt: 30, tdFpm: -120, takeoff: toA321, brake: 2.5, taxi: 15, flaps: flapsAirbus},
	{match: []string{"A321"}, Type: "A321", Category: CategoryJet, span: 35.8, length: 44.5, wheelbase: 16.9, cg: 3.7, tod: 2400,
		vapp: 142, pitch: 2.5, flarePitch: 5, flareFt: 30, tdFpm: -120, takeoff: toA321, brake: 2.5, taxi: 15, flaps: flapsAirbus},
	{match: []string{"A19N", "A319"}, Type: "A319", Category: CategoryJet, span: 35.8, length: 33.8, wheelbase: 11.0, cg: 3.7, tod: 1850,
		vapp: 128, pitch: 3, flarePitch: 6, flareFt: 30, tdFpm: -120, takeoff: toA319, brake: 2.5, taxi: 15, flaps: flapsAirbus},
	{match: []string{"A20N", "A320NEO", "A320 NEO", "A320-NEO"}, Type: "A20N"},
	{match: []string{"A320"}, Type: "A320"},
	{match: []string{"B39M", "737 MAX 9", "737-MAX9", "737MAX9", "MAX 9"}, Type: "B39M", Category: CategoryJet, span: 35.9, length: 42.2, wheelbase: 17.2, cg: 3.5, tod: 2700,
		vapp: 150, pitch: 1.5, flarePitch: 4.5, flareFt: 30, tdFpm: -130, takeoff: toB739, brake: 2.5, taxi: 15, flaps: flaps737},
	{match: []string{"B739", "737-900"}, Type: "B739", Category: CategoryJet, span: 35.8, length: 42.1, wheelbase: 17.2, cg: 3.5, tod: 2700,
		vapp: 150, pitch: 1.5, flarePitch: 4.5, flareFt: 30, tdFpm: -130, takeoff: toB739, brake: 2.5, taxi: 15, flaps: flaps737},
	{match: []string{"B38M", "737 MAX 8", "737-MAX8", "737MAX8", "MAX 8", "737 MAX"}, Type: "B38M", Category: CategoryJet, span: 35.9, length: 39.5, wheelbase: 15.6, cg: 3.5, tod: 2500,
		vapp: 145, pitch: 2, flarePitch: 5, flareFt: 30, tdFpm: -130, takeoff: toB737, brake: 2.5, taxi: 15, flaps: flaps737},
	{match: []string{"B37M", "B737", "737-7"}, Type: "B737", Category: CategoryJet, span: 35.8, length: 33.6, wheelbase: 12.6, cg: 3.5, tod: 2100,
		vapp: 135, pitch: 2, flarePitch: 5.5, flareFt: 30, tdFpm: -130, takeoff: toB737, brake: 2.5, taxi: 15, flaps: flaps737},
	{match: []string{"B738", "B736", "737"}, Type: "B738", Category: CategoryJet, span: 35.8, length: 39.5, wheelbase: 15.6, cg: 3.5, tod: 2300,
		vapp: 145, pitch: 2, flarePitch: 5, flareFt: 30, tdFpm: -130, takeoff: toB737, brake: 2.5, taxi: 15, flaps: flaps737},
	// Regional jets and turboprops.
	{match: []string{"BCS3", "A220-300", "A223", "A220"}, Type: "BCS3", Category: CategoryJet, span: 35.1, length: 38.7, wheelbase: 16.4, cg: 3.3, tod: 1900,
		vapp: 133, pitch: 3, flarePitch: 5.5, flareFt: 25, tdFpm: -120, takeoff: toE195, brake: 2.5, taxi: 15, flaps: flapsEJet},
	{match: []string{"E195", "E-195", "ERJ-195", "ERJ195", "E295"}, Type: "E195", Category: CategoryJet, span: 28.7, length: 38.7, wheelbase: 16.3, cg: 3.2, tod: 2180,
		vapp: 135, pitch: 3, flarePitch: 5.5, flareFt: 25, tdFpm: -120, takeoff: toE195, brake: 2.5, taxi: 15, flaps: flapsEJet},
	{match: []string{"E190", "E-190", "ERJ-190", "ERJ190", "E290", "E19"}, Type: "E190", Category: CategoryJet, span: 28.7, length: 36.2, wheelbase: 14.7, cg: 3.2, tod: 2050,
		vapp: 130, pitch: 3, flarePitch: 5.5, flareFt: 25, tdFpm: -120, takeoff: toEJet, brake: 2.5, taxi: 15, flaps: flapsEJet},
	{match: []string{"E170", "E175", "E-175", "E75"}, Type: "E175", Category: CategoryJet, span: 26.0, length: 31.7, wheelbase: 12.1, cg: 3.0, tod: 2000,
		vapp: 128, pitch: 3, flarePitch: 5.5, flareFt: 25, tdFpm: -120, takeoff: toEJet, brake: 2.5, taxi: 15, flaps: flapsEJet},
	{match: []string{"CRJ9", "CRJ-900", "CRJ900", "CRJ"}, Type: "CRJ9", Category: CategoryJet, span: 24.9, length: 36.2, wheelbase: 15.9, cg: 2.8, tod: 1950,
		vapp: 140, pitch: 3.5, flarePitch: 6, flareFt: 25, tdFpm: -130, takeoff: toCRJ9, brake: 2.5, taxi: 15, flaps: flapsCRJ},
	{match: []string{"AT76", "AT75", "AT72", "ATR 72", "ATR72", "ATR"}, Type: "AT76", Category: CategoryTurboprop, span: 27.1, length: 27.2, wheelbase: 10.8, cg: 2.9, tod: 1370,
		vapp: 113, pitch: 0.5, flarePitch: 3, flareFt: 20, tdFpm: -100, takeoff: toTprop, brake: 2.2, taxi: 15, flaps: flapsATR},
	{match: []string{"DH8D", "Q400", "DASH 8", "DHC-8", "DH8"}, Type: "DH8D", Category: CategoryTurboprop, span: 28.4, length: 32.8, wheelbase: 13.9, cg: 3.0, tod: 1400,
		vapp: 125, pitch: 0, flarePitch: 3, flareFt: 20, tdFpm: -100, takeoff: toTprop, brake: 2.2, taxi: 15, flaps: flapsQ400},
}

// TakeoffProfileFor picks take-off figures for an aircraft model title,
// e.g. "FSLTL B77W Emirates" or "Boeing 777-300", by its family (see
// ProfileFor); titles that match none get DefaultTakeoffProfile (A320
// family).
func TakeoffProfileFor(model string) TakeoffProfile {
	return ProfileFor(model).Takeoff
}

// EngineCount is how many engines the type has: four for the 747, A340
// and A380 families, two otherwise (twins and the turboprops listed).
func (p AircraftProfile) EngineCount() int {
	switch {
	case strings.HasPrefix(p.Type, "B74"), strings.HasPrefix(p.Type, "A34"), strings.HasPrefix(p.Type, "A38"):
		return 4
	}
	return 2
}

// MotionProfileFor is the ground motion of an aircraft model title (e.g.
// "FSLTL B77W Emirates") by its family (see ProfileFor): wheelbase, span,
// the tail behind the main gear and taxi figures; titles that match none
// get DefaultMotionProfile (A320).
func MotionProfileFor(model string) MotionProfile {
	return ProfileFor(model).Motion
}

// withSpan routes for the aircraft's size: RouteOptions.HalfSpan from the
// profile's span (DefaultMotionProfile's when the profile is zero), unless
// the caller set one.
func withSpan(o airport.RouteOptions, p MotionProfile) airport.RouteOptions {
	if o.HalfSpan != 0 {
		return o
	}
	if p == (MotionProfile{}) {
		p = DefaultMotionProfile()
	}
	o.HalfSpan = p.SpanMeters / 2
	return o
}
