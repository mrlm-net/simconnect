//go:build windows
// +build windows

package traffic

import "strings"

// takeoffTypes are take-off figures by aircraft family, matched against the
// model title (type designators such as B77W or names such as 777-300), the
// most specific first. Tail-strike pitches are with the main gear on the
// runway; rotation and climb figures are typical, not for a given weight.
// A first step towards per-type aircraft profiles (#324).
var takeoffTypes = []struct {
	match []string
	p     TakeoffProfile
}{
	{[]string{"B77W", "B773", "777-300", "777300", "777-3"}, TakeoffProfile{RollAccel: 1.9, RotateKts: 160, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 175, ClimbFpm: 2000, ClimbRampSeconds: 6, TailstrikePitch: 8.5}},
	{[]string{"B77L", "B772", "B77F", "777"}, TakeoffProfile{RollAccel: 1.9, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 8.5, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2000, ClimbRampSeconds: 6, TailstrikePitch: 10.5}},
	{[]string{"B78X", "787-10"}, TakeoffProfile{RollAccel: 2.0, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2100, ClimbRampSeconds: 6, TailstrikePitch: 8.5}},
	{[]string{"B788", "B789", "787"}, TakeoffProfile{RollAccel: 2.1, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 8.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2300, ClimbRampSeconds: 6, TailstrikePitch: 9.5}},
	{[]string{"B744", "B748", "B74F", "747"}, TakeoffProfile{RollAccel: 1.8, RotateKts: 160, RotateRate: 2.5, LiftoffPitch: 9, ClimbPitch: 14, ClimbKts: 175, ClimbFpm: 1800, ClimbRampSeconds: 6, TailstrikePitch: 11}},
	{[]string{"A388", "A380"}, TakeoffProfile{RollAccel: 1.7, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 9, ClimbPitch: 13, ClimbKts: 170, ClimbFpm: 1700, ClimbRampSeconds: 6, TailstrikePitch: 12.5}},
	{[]string{"A35K", "A350-1000"}, TakeoffProfile{RollAccel: 2.0, RotateKts: 155, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 170, ClimbFpm: 2100, ClimbRampSeconds: 6, TailstrikePitch: 9.5}},
	{[]string{"A359", "A350"}, TakeoffProfile{RollAccel: 2.1, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 8.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2300, ClimbRampSeconds: 6, TailstrikePitch: 10.5}},
	{[]string{"A332", "A333", "A338", "A339", "A330"}, TakeoffProfile{RollAccel: 2.0, RotateKts: 150, RotateRate: 2.5, LiftoffPitch: 8, ClimbPitch: 14, ClimbKts: 165, ClimbFpm: 2000, ClimbRampSeconds: 6, TailstrikePitch: 10}},
	{[]string{"A21N", "A321"}, TakeoffProfile{RollAccel: 2.3, RotateKts: 145, RotateRate: 3, LiftoffPitch: 7.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2100, ClimbRampSeconds: 5, TailstrikePitch: 9.5}},
	{[]string{"B739", "B39M", "737-9"}, TakeoffProfile{RollAccel: 2.3, RotateKts: 150, RotateRate: 3, LiftoffPitch: 7.5, ClimbPitch: 15, ClimbKts: 165, ClimbFpm: 2100, ClimbRampSeconds: 5, TailstrikePitch: 9}},
	{[]string{"B736", "B737", "B738", "B38M", "B37M", "737"}, TakeoffProfile{RollAccel: 2.4, RotateKts: 145, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15, ClimbKts: 160, ClimbFpm: 2200, ClimbRampSeconds: 5, TailstrikePitch: 11}},
	{[]string{"E170", "E175", "E190", "E195", "E75", "E19", "CRJ"}, TakeoffProfile{RollAccel: 2.6, RotateKts: 130, RotateRate: 3, LiftoffPitch: 8, ClimbPitch: 15, ClimbKts: 150, ClimbFpm: 2300, ClimbRampSeconds: 5, TailstrikePitch: 12}},
	{[]string{"ATR", "AT72", "AT76", "DH8", "Q400"}, TakeoffProfile{RollAccel: 2.4, RotateKts: 115, RotateRate: 3, LiftoffPitch: 7, ClimbPitch: 12, ClimbKts: 135, ClimbFpm: 1500, ClimbRampSeconds: 5, TailstrikePitch: 10}},
}

// TakeoffProfileFor picks take-off figures for an aircraft model title,
// e.g. "FSLTL B77W Emirates" or "Boeing 777-300ER", by its family; titles
// that match none get DefaultTakeoffProfile (A320 family).
func TakeoffProfileFor(model string) TakeoffProfile {
	t := strings.ToUpper(model)
	for _, e := range takeoffTypes {
		for _, m := range e.match {
			if strings.Contains(t, m) {
				return e.p
			}
		}
	}
	return DefaultTakeoffProfile()
}

// geometryTypes are airframe dimensions by family, matched like
// takeoffTypes: wheelbase (nose gear to main gear), wing span and overall
// length, in meters. MSFS 2024 does not report gear contact points to
// SimConnect (CONTACT POINT POSITION reads a constant 1 m), so these
// published figures place the nose gear — where a tug connects — and
// outline the airframe for clearance checks. A first step towards
// per-type aircraft profiles (#324).
var geometryTypes = []struct {
	match                 []string
	wheelbase, span, long float64
}{
	{[]string{"B77W", "B773", "777-300", "777300", "777-3"}, 31.2, 64.8, 73.9},
	{[]string{"B77L", "B772", "B77F", "777"}, 25.9, 64.8, 63.7},
	{[]string{"B78X", "787-10"}, 29.0, 60.1, 68.3},
	{[]string{"B789", "787-9", "787-09"}, 25.9, 60.1, 62.8},
	{[]string{"B788", "787"}, 22.8, 60.1, 56.7},
	{[]string{"B748", "747-8"}, 29.7, 68.4, 76.3},
	{[]string{"B744", "B74F", "747"}, 25.6, 64.4, 70.7},
	{[]string{"A388", "A380"}, 33.6, 79.8, 72.7},
	{[]string{"A35K", "A350-1000"}, 32.5, 64.8, 73.8},
	{[]string{"A359", "A350"}, 28.7, 64.8, 66.8},
	{[]string{"A332", "A330-200"}, 22.2, 60.3, 58.8},
	{[]string{"A333", "A338", "A339", "A330"}, 25.4, 60.3, 63.7},
	{[]string{"A21N", "A321"}, 16.9, 35.8, 44.5},
	{[]string{"A19N", "A319"}, 11.0, 35.8, 33.8},
	{[]string{"B739", "B39M", "737-9", "737-MAX9"}, 17.2, 35.9, 42.1},
	{[]string{"B738", "B38M", "737-8", "737 MAX 8", "737-MAX8"}, 15.6, 35.9, 39.5},
	{[]string{"B737", "B37M", "737-7"}, 12.6, 35.9, 33.6},
	{[]string{"B736", "737"}, 15.6, 35.9, 39.5},
	{[]string{"E195", "E190", "E19"}, 14.7, 28.7, 36.2},
	{[]string{"E170", "E175", "E75"}, 12.1, 26.0, 31.7},
	{[]string{"CRJ"}, 15.9, 24.9, 36.2},
	{[]string{"ATR", "AT72", "AT76"}, 10.8, 27.1, 27.2},
	{[]string{"DH8", "Q400"}, 13.9, 28.4, 32.8},
}

// MotionProfileFor is DefaultMotionProfile with the airframe of an aircraft
// model title (e.g. "FSLTL B77W Emirates"): wheelbase, span and the tail
// behind the main gear, by family; titles that match none keep the A320.
// The nose sits about 13% of the length ahead of the nose gear.
func MotionProfileFor(model string) MotionProfile {
	p := DefaultMotionProfile()
	t := strings.ToUpper(model)
	for _, g := range geometryTypes {
		for _, m := range g.match {
			if strings.Contains(t, m) {
				p.WheelbaseMeters, p.SpanMeters = g.wheelbase, g.span
				p.TailMeters = g.long - 0.13*g.long - g.wheelbase
				return p
			}
		}
	}
	return p
}
