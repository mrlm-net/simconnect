package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Pushed back and then taxiing on, the same aircraft's wheels spin again:
// backwards through the push (VELOCITY BODY Z, wheels.go), set back to 0
// once it rolls forward, INITPOSITION Airspeed its ground speed from the
// first half metre of the taxi on, through a tight turn
// right after the push (user, live LKPR 2026-11-07, TVS469: "wheels are
// not spinning"). The push and the taxi on one injector and one object, as
// a departure has them.
func TestWheelsSpinAfterThePush(t *testing.T) {
	c := &wheelsClient{}
	inj := NewInjector(c)
	inj.Takeover(42)
	inj.SetModel(42, "FSLTL_FAIB_B738_TVS-Smartwings_NC")
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 42, 1200, 8.48))

	prof := ProfileFor("B738").Motion
	push := prof
	push.CruiseKts, push.MinTurnKts, push.Accel, push.Decel = 3, 1, 0.15, 0.25
	// Backwards 40 m south, then swung round 90° to the east.
	path, err := NewArcPath([]airport.LatLon{lkpr, offset(lkpr, 0, -40), offset(lkpr, 40, -40)}, push, 20)
	if err != nil {
		t.Fatal(err)
	}
	pushed := drive(NewPushbackMover(path, push, 0), 600)
	for _, p := range pushed {
		if err := inj.Place(42, p); err != nil {
			t.Fatal(err)
		}
	}
	checkPushWheels(t, pushed, c.speeds, c.vz)
	end := pushed[len(pushed)-1]

	// Taxiing on from where the push ended, the path from its nose gear as
	// the departure starts it (departure_inject.go): a tight turn left at
	// once, then straight on.
	fwd := math.Mod(end.Heading, 360)
	nose := NoseGear(end.Position, end.Heading, prof)
	a := offsetHeading(nose, fwd, 8)
	b := offsetHeading(a, fwd-90, 200)
	taxi, err := NewGroundPath([]airport.LatLon{nose, a, b}, prof)
	if err != nil {
		t.Fatal(err)
	}
	n := len(c.speeds)
	poses := drive(NewGroundMoverFrom(taxi, prof, end.Heading, 0), 120)
	rolling := 0
	for k, p := range poses {
		if err := inj.Place(42, p); err != nil {
			t.Fatal(err)
		}
		if localDist(end.Position, p.Position) < 0.5 {
			continue
		}
		got, want := c.speeds[n+k], uint32(math.Round(p.GroundSpeedKts))
		if v := c.vz[n+k]; !math.IsNaN(v) && v != 0 {
			t.Fatalf("taxi frame %d: VELOCITY BODY Z %.3f written rolling forward", k, v)
		}
		if got != want {
			t.Fatalf("taxi frame %d, %.1f m on (%.1f kt, heading %.0f): Airspeed %d, want %d",
				k, localDist(end.Position, p.Position), p.GroundSpeedKts, p.Heading, got, want)
		}
		if want >= 1 {
			rolling++
		}
	}
	if rolling < 60 {
		t.Fatalf("the taxi barely rolled: %d frames at 1 kt or more", rolling)
	}
}
