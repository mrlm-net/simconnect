package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

var (
	pictureLKPR = AirportRef{ICAO: "LKPR", Position: airport.LatLon{Lat: 50.1008, Lon: 14.26}}
	pictureLOWW = AirportRef{ICAO: "LOWW", Position: airport.LatLon{Lat: 48.1103, Lon: 16.5697}} // ~151 NM
	pictureEDDF = AirportRef{ICAO: "EDDF", Position: airport.LatLon{Lat: 50.0333, Lon: 8.5706}}  // ~220 NM
)

func drainPicture(p *TrafficPicture) []PictureEvent {
	var out []PictureEvent
	for {
		select {
		case e := <-p.Events():
			out = append(out, e)
		default:
			return out
		}
	}
}

func hasEvent(evs []PictureEvent, kind PictureEventKind, icao string, id uint32) bool {
	for _, e := range evs {
		if e.Kind == kind && e.ICAO == icao && e.ObjectID == id {
			return true
		}
	}
	return false
}

// TestPictureFixedCentre: a centre at an airport, the airports within the
// radius and the events as the radius changes (#366).
func TestPictureFixedCentre(t *testing.T) {
	p := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 160})
	p.SetAirports([]AirportRef{pictureLKPR, pictureLOWW, pictureEDDF})
	if c, ok := p.Centre(); !ok || c != pictureLKPR.Position {
		t.Fatalf("centre %v %v, want LKPR", c, ok)
	}
	got := p.Airports()
	if len(got) != 2 || got[0].ICAO != "LKPR" || got[1].ICAO != "LOWW" {
		t.Fatalf("airports %+v, want LKPR then LOWW", got)
	}
	evs := drainPicture(p)
	if !hasEvent(evs, AirportEntered, "LOWW", 0) || hasEvent(evs, AirportEntered, "EDDF", 0) {
		t.Errorf("events %+v", evs)
	}
	p.SetRadius(300)
	if !hasEvent(drainPicture(p), AirportEntered, "EDDF", 0) || len(p.Airports()) != 3 {
		t.Error("EDDF did not enter at 300 NM")
	}
	p.SetRadius(100)
	if evs := drainPicture(p); !hasEvent(evs, AirportLeft, "LOWW", 0) || !hasEvent(evs, AirportLeft, "EDDF", 0) {
		t.Errorf("events %+v, want LOWW and EDDF leaving", evs)
	}
}

// TestPictureFollowsUser: the centre follows the user's aircraft, moving
// only once it is RecentreNM away.
func TestPictureFollowsUser(t *testing.T) {
	p := NewTrafficPicture(PictureOptions{Centre: Centre{FollowUser: true}, RadiusNM: 250})
	p.SetAirports([]AirportRef{pictureLKPR, pictureLOWW})
	now := time.Now()
	user := Observation{ObjectID: 1, User: true, Position: pictureLKPR.Position, OnGround: true}
	p.Observe(now, []Observation{user})
	if c, _ := p.Centre(); c != pictureLKPR.Position {
		t.Fatalf("centre %v, want the user at LKPR", c)
	}
	drainPicture(p)
	user.Position = offsetHeading(pictureLKPR.Position, 90, 10*1852)
	p.Observe(now.Add(time.Second), []Observation{user})
	if c, _ := p.Centre(); c != pictureLKPR.Position || hasEvent(drainPicture(p), Recentred, "", 0) {
		t.Fatal("recentred after 10 NM")
	}
	user.Position = offsetHeading(pictureLKPR.Position, 90, 30*1852)
	p.Observe(now.Add(2*time.Second), []Observation{user})
	if c, _ := p.Centre(); c != user.Position {
		t.Fatalf("centre %v, want the user 30 NM away", c)
	}
}

// TestPicturePhases: parked, taxiing, rolling, departing, arriving and
// enroute aircraft, with their airports; out of the radius or not seen any
// more, they leave.
func TestPicturePhases(t *testing.T) {
	p := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 150})
	p.SetAirports([]AirportRef{pictureLKPR, pictureLOWW})
	now := time.Now()
	near := func(d float64) airport.LatLon { return offsetHeading(pictureLKPR.Position, 45, d) }
	scan := []Observation{
		{ObjectID: 10, Tail: "PARK", Position: near(500), OnGround: true},
		{ObjectID: 11, Tail: "TAXI", Position: near(800), OnGround: true, GroundKts: 15},
		{ObjectID: 12, Tail: "ROLL", Position: near(1200), OnGround: true, GroundKts: 120},
		{ObjectID: 13, Tail: "DEP", Position: near(8000), AltFt: 4000, AGLFt: 3000, VSFpm: 2000},
		{ObjectID: 14, Tail: "ARR", Position: near(20000), AltFt: 5000, AGLFt: 4000, VSFpm: -800},
		{ObjectID: 15, Tail: "CRZ", Position: offsetHeading(pictureLKPR.Position, 0, 80*1852), AltFt: 36000, AGLFt: 35000},
		{ObjectID: 16, Tail: "FAR", Position: offsetHeading(pictureLKPR.Position, 0, 200*1852), AltFt: 36000, AGLFt: 35000},
	}
	// The vertical rate comes from the altitude: a scan 6 s earlier.
	before := append([]Observation(nil), scan...)
	before[3].AltFt -= 200 // 2000 fpm up
	before[4].AltFt += 80  // 800 fpm down
	p.Observe(now.Add(-6*time.Second), before)
	p.Observe(now, scan)
	want := map[string][2]string{"PARK": {"parked", "LKPR"}, "TAXI": {"taxiing", "LKPR"}, "ROLL": {"runway", "LKPR"},
		"DEP": {"departing", "LKPR"}, "ARR": {"arriving", "LKPR"}, "CRZ": {"enroute", ""}}
	got := p.Aircraft()
	if len(got) != len(want) {
		t.Fatalf("%d aircraft, want %d (FAR is outside the radius)", len(got), len(want))
	}
	for _, a := range got {
		if w := want[a.Tail]; string(a.Phase) != w[0] || a.Airport != w[1] {
			t.Errorf("%s: %s at %q, want %s at %q", a.Tail, a.Phase, a.Airport, w[0], w[1])
		}
	}
	if !hasEvent(drainPicture(p), AircraftEntered, "", 10) {
		t.Error("no entered event")
	}
	// Only some seen again; the rest go stale.
	p.Observe(now.Add(PictureStaleAfter+time.Second), scan[:2])
	if n := len(p.Aircraft()); n != 2 {
		t.Errorf("%d aircraft after the others went stale, want 2", n)
	}
	if !hasEvent(drainPicture(p), AircraftLeft, "", 15) {
		t.Error("no left event for the stale aircraft")
	}
}

// TestPictureOwnAndFeeds: our aircraft take the controller's phase and are
// not reported to the ground picture (they report themselves); others on
// the ground are, and the stand allocator sees an aircraft on its stand.
func TestPictureOwnAndFeeds(t *testing.T) {
	g := lkprGraph(t)
	p := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 150})
	p.SetAirports([]AirportRef{pictureLKPR})
	gp := p.Ground("LKPR")
	alloc := NewStandAllocator(nil, g)
	p.Allocate("LKPR", alloc)
	c22, _ := g.Layout.ParkingIndex("C22")
	stand := g.Layout.Parking[c22]
	p.SetOwn(20, PhaseTaxiing, "LKPR")
	p.Observe(time.Now(), []Observation{
		{ObjectID: 20, Tail: "OURS", Position: offsetHeading(stand.Position, 90, 300), OnGround: true},
		{ObjectID: 21, Tail: "AI", Position: stand.Position, OnGround: true, SpanM: 35.8},
	})
	for _, a := range p.Aircraft() {
		if a.Tail == "OURS" && (!a.Ours || a.Phase != PhaseTaxiing) {
			t.Errorf("ours: %+v", a)
		}
	}
	gp.mu.Lock()
	_, ours := gp.aircraft[20]
	_, ai := gp.aircraft[21]
	gp.mu.Unlock()
	if ours || !ai {
		t.Errorf("ground picture: ours reported %v, AI reported %v", ours, ai)
	}
	if o, ok := alloc.Occupant(c22); !ok || !o.Detected || o.ObjectID != 21 {
		t.Errorf("C22 occupant %+v %v, want the AI aircraft detected", o, ok)
	}
}

// The vertical speed of ours is measured from the altitude between scans:
// an injected aircraft's is meaningless (live, +560 fpm descending on the
// glide path).
func TestPictureMeasuresOurVerticalSpeed(t *testing.T) {
	p := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 160})
	p.SetAirports([]AirportRef{pictureLKPR})
	p.SetOwn(7, PhaseArriving, "LKPR")
	now := time.Now()
	for i := 0; i < 10; i++ {
		p.Observe(now.Add(time.Duration(i)*time.Second), []Observation{{ObjectID: 7, Tail: "ENT464", Position: pictureLKPR.Position,
			AltFt: 4000 - float64(i)*1000.0/60, VSFpm: 560}})
	}
	for _, a := range p.Aircraft() {
		if a.ObjectID == 7 && math.Abs(a.VSFpm+1000) > 30 {
			t.Errorf("vertical speed %.0f fpm, want -1000", a.VSFpm)
		}
	}
}
