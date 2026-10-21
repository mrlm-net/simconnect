package worldtest

import (
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestPlaceUser: the user aircraft, parked away from a stand with its
// engines off, is put on the stand's point facing its heading; airborne,
// moving or with an engine running it is refused and stays where it is.
func TestPlaceUser(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	stand := -1
	for i, p := range h.Layout.Parking {
		if p.Size() != airport.StandNone && p.Label() != "" {
			if _, err := h.Layout.ParkingIndex(p.Label()); err == nil {
				stand = i
				break
			}
		}
	}
	if stand < 0 {
		t.Fatal("no stand")
	}
	p := h.Layout.Parking[stand]
	field := h.Layout.Altitude / 0.3048
	_, end, _ := h.Layout.RunwayEnd("24")
	park := func(u *Object) { // on the runway threshold, stopped, engines off
		u.Lat, u.Lon, u.Heading, u.AltFt = end.Threshold.Lat, end.Threshold.Lon, end.Heading, field
		u.OnGround, u.GroundKts, u.VSFpm, u.EnginesRunning = true, 0, 0, false
	}
	h.SetUser(park)
	h.Advance(3 * time.Second)

	for _, c := range []struct {
		name string
		set  func(u *Object)
		want error
	}{
		{"airborne", func(u *Object) { u.OnGround, u.AltFt = false, field+1500 }, world.ErrUserAirborne},
		{"moving", func(u *Object) { u.GroundKts = 12 }, world.ErrUserMoving},
		{"engines", func(u *Object) { u.EnginesRunning = true }, world.ErrUserEngines},
	} {
		h.SetUser(func(u *Object) { park(u); c.set(u) })
		h.Advance(2 * time.Second)
		if err := h.World.PlaceUser("LKPR", p.Label()); !errors.Is(err, c.want) {
			t.Errorf("%s: PlaceUser = %v, want %v", c.name, err, c.want)
		}
	}
	h.SetUser(park)
	h.Advance(2 * time.Second)
	if err := h.World.PlaceUser("LKPR", "no such stand"); !errors.Is(err, airport.ErrUnknownParking) {
		t.Errorf("unknown stand: PlaceUser = %v", err)
	}

	h.SetUser(park)
	h.Advance(2 * time.Second)
	if err := h.World.PlaceUser("LKPR", p.Label()); err != nil {
		t.Fatalf("PlaceUser %s: %v", p.Label(), err)
	}
	h.Advance(time.Second)
	want := traffic.StandPoint(p, 0)
	var u Object
	h.Sim.User(func(o *Object) { u = *o })
	if d := calc.HaversineMeters(u.Lat, u.Lon, want.Lat, want.Lon); d > 0.5 {
		t.Errorf("user %.1f m from %s's point", d, p.Label())
	}
	if dh := math.Abs(math.Remainder(u.Heading-p.Heading, 360)); dh > 0.5 {
		t.Errorf("user heading %.1f, stand %.1f", u.Heading, p.Heading)
	}
	if math.Abs(u.AltFt-field) > 1 {
		t.Errorf("user at %.0f ft, field %.0f ft", u.AltFt, field)
	}
}
