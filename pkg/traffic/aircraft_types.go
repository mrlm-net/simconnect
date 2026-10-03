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
	toB77W  = TakeoffProfile{RollAccel: 1.9, RotateKts: 160, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 14, ClimbKts: 175, ClimbFpm: 2000, ClimbRampSeconds: 3, TailstrikePitch: 8.5}
	toB772  = TakeoffProfile{RollAccel: 1.9, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2000, ClimbRampSeconds: 3, TailstrikePitch: 10.5}
	toB78X  = TakeoffProfile{RollAccel: 2.0, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2100, ClimbRampSeconds: 3, TailstrikePitch: 8.5}
	toB787  = TakeoffProfile{RollAccel: 2.1, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2300, ClimbRampSeconds: 3, TailstrikePitch: 9.5}
	toB747  = TakeoffProfile{RollAccel: 1.8, RotateKts: 160, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 14, ClimbKts: 175, ClimbFpm: 1800, ClimbRampSeconds: 3, TailstrikePitch: 11}
	toA388  = TakeoffProfile{RollAccel: 1.7, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 13, ClimbKts: 170, ClimbFpm: 1700, ClimbRampSeconds: 3, TailstrikePitch: 12.5}
	toA35K  = TakeoffProfile{RollAccel: 2.0, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2100, ClimbRampSeconds: 3, TailstrikePitch: 9.5}
	toA359  = TakeoffProfile{RollAccel: 2.1, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2300, ClimbRampSeconds: 3, TailstrikePitch: 10.5}
	toA330  = TakeoffProfile{RollAccel: 2.0, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 7, ClimbPitch: 14, ClimbKts: 165, ClimbFpm: 2000, ClimbRampSeconds: 3, TailstrikePitch: 10}
	toA321  = TakeoffProfile{RollAccel: 2.3, RotateKts: 145, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2100, ClimbRampSeconds: 2.5, TailstrikePitch: 9.5}
	toA319  = TakeoffProfile{RollAccel: 2.5, RotateKts: 132, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: 155, ClimbFpm: 2400, ClimbRampSeconds: 2.5, TailstrikePitch: 13.5}
	toB739  = TakeoffProfile{RollAccel: 2.3, RotateKts: 150, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2100, ClimbRampSeconds: 2.5, TailstrikePitch: 9}
	toB737  = TakeoffProfile{RollAccel: 2.4, RotateKts: 145, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: 160, ClimbFpm: 2200, ClimbRampSeconds: 2.5, TailstrikePitch: 11}
	toEJet  = TakeoffProfile{RollAccel: 2.6, RotateKts: 130, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: 150, ClimbFpm: 2300, ClimbRampSeconds: 2.5, TailstrikePitch: 12}
	toE195  = TakeoffProfile{RollAccel: 2.5, RotateKts: 135, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: 155, ClimbFpm: 2200, ClimbRampSeconds: 2.5, TailstrikePitch: 11}
	toCRJ9  = TakeoffProfile{RollAccel: 2.5, RotateKts: 140, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: 160, ClimbFpm: 2300, ClimbRampSeconds: 2.5, TailstrikePitch: 11}
	toTprop = TakeoffProfile{RollAccel: 2.4, RotateKts: 115, RotateRate: 3, LiftoffPitch: 6, ClimbPitch: 12, ClimbKts: 135, ClimbFpm: 1500, ClimbRampSeconds: 2.5, TailstrikePitch: 10}
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
	// Business aviation and mid-size GA (#619). Span, length, wheelbase
	// (Textron and Daher only) and take-off distance as published: Textron
	// (cessna/beechcraft.txtav.com), Pilatus, Cirrus, Diamond, Daher (TBM
	// 930, archived) and Wikipedia's specifications (Embraer). No Vref is
	// published: vapp is 1.3 times the published stall speed where there is
	// one, else an estimate; an unpublished wheelbase is an estimate (0.42
	// of the length for jets, as Textron's are), CG heights are estimates.
	// The more specific titles first ("CITATION XLS" before "CITATION X").
	{match: []string{"C25B", "CITATION CJ3"}, Type: "C25B", Category: CategoryJet, span: 16.26, length: 15.6, wheelbase: 6.10, cg: 1.5, tod: 969,
		vapp: 115, pitch: 3, flarePitch: 5, flareFt: 20, tdFpm: -120, takeoff: toBizJet(110, 140, 3100), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"C25C", "CITATION CJ4"}, Type: "C25C", Category: CategoryJet, span: 15.77, length: 16.3, wheelbase: 6.45, cg: 1.5, tod: 1039,
		vapp: 115, pitch: 3, flarePitch: 5, flareFt: 20, tdFpm: -120, takeoff: toBizJet(112, 145, 2700), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"C56X", "CITATION XLS", "CITATION EXCEL"}, Type: "C56X", Category: CategoryJet, span: 17.17, length: 16.0, wheelbase: 6.68, cg: 1.6, tod: 1085,
		vapp: 112, pitch: 3, flarePitch: 5, flareFt: 20, tdFpm: -120, takeoff: toBizJet(110, 140, 2500), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"C68A", "CITATION LATITUDE"}, Type: "C68A", Category: CategoryJet, span: 22.05, length: 19.0, wheelbase: 8.23, cg: 1.8, tod: 1091,
		vapp: 110, pitch: 3, flarePitch: 5, flareFt: 25, tdFpm: -120, takeoff: toBizJet(110, 140, 2800), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"C700", "CITATION LONGITUDE"}, Type: "C700", Category: CategoryJet, span: 21.01, length: 22.3, wheelbase: 9.63, cg: 1.9, tod: 1466,
		vapp: 115, pitch: 3, flarePitch: 5, flareFt: 25, tdFpm: -120, takeoff: toBizJet(118, 150, 3400), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"C680", "CITATION SOVEREIGN"}, Type: "C680", Category: CategoryJet, span: 22.05, length: 19.4, wheelbase: 8.49, cg: 1.8, tod: 1076,
		vapp: 108, pitch: 3, flarePitch: 5, flareFt: 25, tdFpm: -120, takeoff: toBizJet(108, 140, 2900), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"C750", "CITATION X"}, Type: "C750", Category: CategoryJet, span: 21.09, length: 22.43, wheelbase: 9.11, cg: 1.9, tod: 1600,
		vapp: 128, pitch: 3, flarePitch: 5, flareFt: 25, tdFpm: -130, takeoff: toBizJet(125, 160, 2900), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"E50P", "PHENOM 100"}, Type: "E50P", Category: CategoryJet, span: 12.3, length: 12.82, wheelbase: 5.4, cg: 1.3, tod: 975,
		vapp: 100, pitch: 3, flarePitch: 5, flareFt: 15, tdFpm: -120, takeoff: toBizJet(100, 125, 2000), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"E55P", "PHENOM 300"}, Type: "E55P", Category: CategoryJet, span: 15.91, length: 15.64, wheelbase: 6.6, cg: 1.5, tod: 978,
		vapp: 112, pitch: 3, flarePitch: 5, flareFt: 20, tdFpm: -120, takeoff: toBizJet(110, 140, 2800), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"E545", "PRAETOR 500", "LEGACY 450"}, Type: "E545", Category: CategoryJet, span: 21.5, length: 19.69, wheelbase: 8.3, cg: 1.8, tod: 1287,
		vapp: 110, pitch: 3, flarePitch: 5, flareFt: 25, tdFpm: -120, takeoff: toBizJet(112, 145, 3000), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"E550", "PRAETOR 600", "LEGACY 500"}, Type: "E550", Category: CategoryJet, span: 21.5, length: 20.74, wheelbase: 8.7, cg: 1.8, tod: 1352,
		vapp: 112, pitch: 3, flarePitch: 5, flareFt: 25, tdFpm: -120, takeoff: toBizJet(115, 145, 3000), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"PC24", "PC-24"}, Type: "PC24", Category: CategoryJet, span: 17.0, length: 16.8, wheelbase: 7.1, cg: 1.6, tod: 940,
		vapp: 108, pitch: 3, flarePitch: 5, flareFt: 20, tdFpm: -120, takeoff: toBizJet(105, 135, 2800), brake: 2.2, taxi: 14, flaps: flapsBiz},
	{match: []string{"SF50", "VISION"}, Type: "SF50", Category: CategoryJet, span: 11.79, length: 9.42, wheelbase: 3.4, cg: 1.2, tod: 858,
		vapp: 87, pitch: 3, flarePitch: 5, flareFt: 12, tdFpm: -110, takeoff: toBizJet(85, 110, 1300), brake: 2.0, taxi: 12, flaps: flapsBiz},
	{match: []string{"PC12", "PC-12"}, Type: "PC12", Category: CategoryTurboprop, span: 16.28, length: 14.4, wheelbase: 5.0, cg: 1.6, tod: 758,
		vapp: 87, pitch: 1, flarePitch: 4, flareFt: 15, tdFpm: -100, takeoff: toLightTprop(85, 120, 1600), brake: 2.0, taxi: 12, flaps: flapsBiz},
	{match: []string{"TBM9", "TBM930", "TBM 930", "TBM"}, Type: "TBM9", Category: CategoryTurboprop, span: 12.83, length: 10.74, wheelbase: 2.914, cg: 1.3, tod: 726,
		vapp: 85, pitch: 1, flarePitch: 4, flareFt: 12, tdFpm: -100, takeoff: toLightTprop(85, 120, 1500), brake: 2.0, taxi: 12, flaps: flapsBiz},
	{match: []string{"B350", "KING AIR 350", "KING AIR 360"}, Type: "B350", Category: CategoryTurboprop, span: 17.65, length: 14.2, wheelbase: 4.95, cg: 1.6, tod: 1006,
		vapp: 105, pitch: 1, flarePitch: 4, flareFt: 15, tdFpm: -100, takeoff: toLightTprop(100, 125, 2000), brake: 2.0, taxi: 12, flaps: flapsBiz},
	{match: []string{"BE20", "KING AIR 200", "KING AIR 250", "KING AIR 260", "KING AIR"}, Type: "BE20", Category: CategoryTurboprop, span: 17.65, length: 13.4, wheelbase: 4.55, cg: 1.6, tod: 643,
		vapp: 104, pitch: 1, flarePitch: 4, flareFt: 15, tdFpm: -100, takeoff: toLightTprop(85, 115, 1900), brake: 2.0, taxi: 12, flaps: flapsBiz},
	{match: []string{"DA62", "DA 62"}, Type: "DA62", Category: CategoryPiston, span: 14.55, length: 9.19, wheelbase: 2.3, cg: 1.1, tod: 883,
		vapp: 88, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -100, takeoff: toPiston(80, 100, 1028), brake: 2.0, taxi: 10, flaps: flapsBiz},
	{match: []string{"DA42", "DA 42", "TWIN STAR"}, Type: "DA42", Category: CategoryPiston, span: 13.55, length: 8.56, wheelbase: 2.1, cg: 1.0, tod: 649,
		vapp: 81, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -100, takeoff: toPiston(70, 90, 1280), brake: 2.0, taxi: 10, flaps: flapsBiz},
	{match: []string{"BE58", "BARON 58", "BARON G58"}, Type: "BE58", Category: CategoryPiston, span: 11.53, length: 9.1, wheelbase: 2.3, cg: 1.1, tod: 715,
		vapp: 95, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -100, takeoff: toPiston(75, 100, 1700), brake: 2.0, taxi: 10, flaps: flapsBiz},
	// Light aircraft (VFR traffic, #565). Span and length as published
	// (Wikipedia specifications: C172R, PA-28-140, DA40 XL, SR22-G5, C152);
	// vapp is 1.3 times the published flaps-down stall speed. Wheelbase,
	// CG height and take-off distance are estimates (no published figure
	// read); the simulator's SimVars refine span and the rest (Refine).
	{match: []string{"C172", "CESSNA 172", "SKYHAWK", "C-172"}, Type: "C172", Category: CategoryPiston, span: 11.0, length: 8.28, wheelbase: 1.7, cg: 1.0, tod: 500,
		vapp: 61, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -100, takeoff: toPiston(55, 75, 721), brake: 2.0, taxi: 10, flaps: flapsCessna},
	{match: []string{"C152", "CESSNA 152", "C-152"}, Type: "C152", Category: CategoryPiston, span: 10.16, length: 7.34, wheelbase: 1.5, cg: 0.9, tod: 450,
		vapp: 56, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -100, takeoff: toPiston(50, 70, 715), brake: 2.0, taxi: 10, flaps: flapsCessna},
	{match: []string{"P28A", "PA28", "PA-28", "CHEROKEE", "ARCHER", "WARRIOR"}, Type: "P28A", Category: CategoryPiston, span: 9.14, length: 7.10, wheelbase: 1.9, cg: 0.9, tod: 500,
		vapp: 61, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -100, takeoff: toPiston(55, 75, 660), brake: 2.0, taxi: 10, flaps: flapsPiper},
	{match: []string{"DA40", "DA 40", "DIAMOND STAR"}, Type: "DA40", Category: CategoryPiston, span: 11.9, length: 8.1, wheelbase: 1.7, cg: 1.0, tod: 580,
		vapp: 64, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -100, takeoff: toPiston(59, 75, 1120), brake: 2.0, taxi: 10, flaps: flapsDiamond},
	{match: []string{"SR22", "SR-22", "CIRRUS"}, Type: "SR22", Category: CategoryPiston, span: 11.68, length: 7.92, wheelbase: 1.8, cg: 1.0, tod: 760,
		vapp: 78, pitch: 2, flarePitch: 6, flareFt: 10, tdFpm: -110, takeoff: toPiston(70, 100, 1270), brake: 2.0, taxi: 10, flaps: flapsCirrus},
}

// toPiston is a light single's take-off: rotation and climb speeds (kt,
// estimates) and the published rate of climb (fpm).
func toPiston(rotateKts, climbKts, climbFpm float64) TakeoffProfile {
	return TakeoffProfile{RollAccel: 1.5, RotateKts: rotateKts, RotateRate: 3, LiftoffPitch: 6, ClimbPitch: 9, ClimbKts: climbKts,
		ClimbFpm: climbFpm, ClimbRampSeconds: 2, TailstrikePitch: 12}
}

// Light aircraft flap settings, as percent of travel: Cessna 0 10 20 30
// (take-off 10, approach 20, landing 30), Piper 0 10 25 40 (0, 25, 40),
// Diamond UP T/O LDG (T/O, T/O, LDG), Cirrus 0 50 100 (50, 50, 100).
var (
	flapsCessna  = FlapSchedule{TakeoffPct: 33.3, ApproachPct: 66.7, LandingPct: 100}
	flapsPiper   = FlapSchedule{TakeoffPct: 0, ApproachPct: 62.5, LandingPct: 100}
	flapsDiamond = FlapSchedule{TakeoffPct: 50, ApproachPct: 50, LandingPct: 100}
	flapsCirrus  = FlapSchedule{TakeoffPct: 50, ApproachPct: 50, LandingPct: 100}
)

// TakeoffProfileFor picks take-off figures for an aircraft model title,
// e.g. "FSLTL B77W Emirates" or "Boeing 777-300", by its family (see
// ProfileFor); titles that match none get DefaultTakeoffProfile (A320
// family).
func TakeoffProfileFor(model string) TakeoffProfile {
	return ProfileFor(model).Takeoff
}

// EngineCount is how many engines the type has: four for the 747, A340
// and A380 families, one for the light singles (pistons), two otherwise
// (twins and the turboprops listed).
func (p AircraftProfile) EngineCount() int {
	switch {
	case p.Category == CategoryPiston:
		return 1
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

// toBizJet is a business jet's take-off: rotation and initial climb speeds
// (kt, estimates) and the initial climb rate (fpm, below the published
// maximum).
func toBizJet(rotateKts, climbKts, climbFpm float64) TakeoffProfile {
	return TakeoffProfile{RollAccel: 2.6, RotateKts: rotateKts, RotateRate: 3, LiftoffPitch: 6.5, ClimbPitch: 15, ClimbKts: climbKts,
		ClimbFpm: climbFpm, ClimbRampSeconds: 2.5, TailstrikePitch: 13}
}

// toLightTprop is a single or light twin turboprop's take-off (PC-12, TBM,
// King Air): rotation and climb speeds (kt, estimates) and climb rate (fpm).
func toLightTprop(rotateKts, climbKts, climbFpm float64) TakeoffProfile {
	return TakeoffProfile{RollAccel: 2.2, RotateKts: rotateKts, RotateRate: 3, LiftoffPitch: 6, ClimbPitch: 12, ClimbKts: climbKts,
		ClimbFpm: climbFpm, ClimbRampSeconds: 2.5, TailstrikePitch: 12}
}

// flapsBiz is a business type's flap settings, as percent of travel:
// take-off a third, approach two thirds, landing full (typical; the types'
// detents differ).
var flapsBiz = FlapSchedule{TakeoffPct: 33.3, ApproachPct: 66.7, LandingPct: 100}
