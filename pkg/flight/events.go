package flight

import (
	"fmt"
	"math"
)

// Event is a milestone of a recorded flight, for a replay's timeline: T
// seconds after the first sample, its Kind and a line saying it.
type Event struct {
	T    float64 `json:"t"`
	Kind string  `json:"kind"`
	Text string  `json:"text"`
}

// The kinds of Event, in the order a flight has them.
const (
	EventEnginesStart = "engines-start" // the first engine's N1 over EventN1Running
	EventPushback     = "pushback"      // moving backwards on the ground
	EventTaxi         = "taxi"          // over EventTaxiKts forward on the ground, engines running
	EventTakeoffRoll  = "takeoff-roll"  // on the ground at the take-off roll's speed
	EventRotate       = "rotate"        // nose up past EventRotatePitch on the roll
	EventLiftoff      = "liftoff"
	EventGearUp       = "gear-up"
	EventTouchdown    = "touchdown"
	EventVacate       = "vacate" // under EventVacateKts after the touchdown
	EventPark         = "park"   // parking brake set, engines off
)

// Thresholds of Events.
const (
	EventN1Running   = 15.0 // percent: an engine runs
	EventTaxiKts     = 3.0
	EventRotatePitch = 3.0  // degrees nose up, on the ground at speed
	EventVacateKts   = 30.0 // after the touchdown: off the runway's speed
	eventRollKts     = assessRunwayRollKts
	eventBackwardDeg = 120.0 // moving this far off the heading: backwards
)

// Events are t's milestones, in time order: engines start, pushback, taxi,
// take-off roll, rotate, lift-off, gear up, touchdown, vacate, park. Each
// is found once (its first time; the touchdown its last), from the samples
// alone, and left out when the recording does not show it (a flight
// recorded from the air has no pushback).
func Events(t *Track) []Event {
	ss := t.Samples
	if len(ss) == 0 {
		return nil
	}
	t0 := ss[0].T
	var out []Event
	add := func(kind string, s Sample, format string, args ...any) {
		out = append(out, Event{T: math.Round((s.T-t0)*10) / 10, Kind: kind, Text: fmt.Sprintf(format, args...)})
	}
	running := func(s Sample) bool {
		for i := 0; i < len(s.N1) && (s.EngineCount == 0 || i < s.EngineCount); i++ {
			if s.N1[i] > EventN1Running {
				return true
			}
		}
		return false
	}
	// The lift-off and the last touchdown frame the flight.
	lift, touch := -1, -1
	for i := 1; i < len(ss); i++ {
		if lift < 0 && ss[i-1].OnGround && !ss[i].OnGround && ss[i].IAS > eventRollKts {
			lift = i
		}
		if lift >= 0 && !ss[i-1].OnGround && ss[i].OnGround {
			touch = i
		}
	}
	before := func(i int) bool { return lift < 0 || i < lift }

	// On the ground before the lift-off: start, push, taxi, roll, rotate.
	engines, push, taxi, roll, rotate := false, false, false, -1, false
	for i := 1; i < len(ss) && before(i); i++ {
		s, p := ss[i], ss[i-1]
		if !engines && running(s) && !running(p) {
			engines = true
			add(EventEnginesStart, s, "Engines start")
		}
		if !s.OnGround {
			continue
		}
		moving := s.GS > 0.5
		backwards := moving && math.Abs(headingDiffDeg(movedBearing(p, s), s.Heading)) > eventBackwardDeg && movedM(p, s) > 0.05
		if !push && roll < 0 && backwards {
			push = true
			add(EventPushback, s, "Pushback")
		}
		if !taxi && roll < 0 && s.GS > EventTaxiKts && !backwards && running(s) {
			taxi = true
			add(EventTaxi, s, "Taxi")
		}
		if roll < 0 && s.IAS >= eventRollKts {
			roll = i
			add(EventTakeoffRoll, s, "Take-off roll")
		}
		if roll >= 0 && !rotate && s.Pitch > EventRotatePitch && s.IAS > eventRollKts {
			rotate = true
			add(EventRotate, s, "Rotate at %.0f kt", s.IAS)
		}
	}
	if lift >= 0 {
		add(EventLiftoff, ss[lift], "Lift-off at %.0f kt", ss[lift].IAS)
		for i := lift + 1; i < len(ss) && (touch < 0 || i < touch); i++ {
			if ss[i-1].GearHandle && !ss[i].GearHandle {
				add(EventGearUp, ss[i], "Gear up")
				break
			}
		}
	}
	after := 0
	if touch >= 0 {
		add(EventTouchdown, ss[touch], "Touchdown at %.0f kt, %.0f fpm", ss[touch].IAS, math.Abs(ss[touch-1].VS))
		after = touch
		for i := touch; i < len(ss); i++ {
			if ss[i].OnGround && ss[i].GS < EventVacateKts {
				add(EventVacate, ss[i], "Vacate")
				after = i
				break
			}
		}
	}
	// Parked: brake set, engines off, after the flight (or at all, a flight
	// recorded on the ground only).
	if lift < 0 || touch >= 0 {
		for i := max(after, 1); i < len(ss); i++ {
			if ss[i].OnGround && ss[i].ParkingBrake && !running(ss[i]) && (running(ss[i-1]) || !ss[i-1].ParkingBrake) {
				add(EventPark, ss[i], "Parked")
				break
			}
		}
	}
	return out
}

// movedBearing is the bearing from a to b, degrees true.
func movedBearing(a, b Sample) float64 {
	n := (b.Lat - a.Lat) * 111_320
	e := (b.Lon - a.Lon) * 111_320 * math.Cos(a.Lat*math.Pi/180)
	return math.Mod(math.Atan2(e, n)*180/math.Pi+360, 360)
}

// movedM is how far b is from a, metres.
func movedM(a, b Sample) float64 {
	n := (b.Lat - a.Lat) * 111_320
	e := (b.Lon - a.Lon) * 111_320 * math.Cos(a.Lat*math.Pi/180)
	return math.Hypot(n, e)
}

// headingDiffDeg is b - a in -180…180.
func headingDiffDeg(a, b float64) float64 {
	return math.Mod(b-a+540, 360) - 180
}
