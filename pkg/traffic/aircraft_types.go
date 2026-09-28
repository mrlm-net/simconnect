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
