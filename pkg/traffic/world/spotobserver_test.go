package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
)

// observerBed is a spotter with the observer on an airfield of two apron
// spots, near (by the drone) and far (1500 m east), and two aircraft
// taxiing: one 100 m from the near spot, one 20 m from the far one (out of
// the near spot's reach).
func observerBed(t *testing.T, req SpottingRequest) (*spotter, []Traffic) {
	t.Helper()
	const lat, lon = 50.0, 14.0
	east := func(m float64) float64 {
		_, l := calc.DisplaceByHeading(lat, lon, 90, m)
		return l
	}
	s := &spotter{req: req, g: &airport.Graph{Layout: &airport.Layout{ICAO: "TEST"}},
		spots: []CameraSpot{
			{Name: "apron:near", Kind: "apron", Lat: lat, Lon: lon, AltM: 300},
			{Name: "apron:far", Kind: "apron", Lat: lat, Lon: east(1500), AltM: 300},
		}}
	s.observer, s.obs = observerFor(req)
	s.pose, s.havePose = camera.Pose{Eye: camera.At(lat, lon, 300), Target: camera.At(lat, east(100), 290)}, true
	list := []Traffic{
		{ObjectID: 1, Tail: "FAR1", Latitude: lat, Longitude: east(1520), Heading: 0, GroundKts: 12, OnGround: true},
		{ObjectID: 2, Tail: "NEAR2", Latitude: lat, Longitude: east(100), Heading: 0, GroundKts: 12, OnGround: true},
	}
	return s, list
}

// TestObserverPrefersNear (#1475): the director as it was picks the
// aircraft nearest its spot, across the airport; the observer adds the way
// from the drone and takes the one by it, with a cut (#1473).
func TestObserverPrefersNear(t *testing.T) {
	now := time.Unix(1000, 0)
	old, list := observerBed(t, SpottingRequest{ICAO: "TEST"})
	if old.obs {
		t.Fatal("observer on without travelMaxM")
	}
	old.see(now, list)
	if p, _, _ := old.pick(list, now, false); p == nil || p.Tail != "FAR1" {
		t.Fatalf("without the observer picked %v, want FAR1 (nearest its spot)", p)
	}

	s, list := observerBed(t, SpottingRequest{ICAO: "TEST", TravelMaxM: 2000})
	if s.cutM != DefaultSpotCutM || s.speed != camera.DefaultTravelSpeed {
		t.Errorf("defaults: cutM %.0f, speed %.0f", s.cutM, s.speed)
	}
	cut := s.look(now, list)
	if cut == nil || cut.t.Tail != "NEAR2" || cut.spot != "apron:near" || cut.travel {
		t.Fatalf("observer: %+v, want a cut to NEAR2 from apron:near", cut)
	}
}

// TestObserverTravelOrRefuse (#1473): a spot within cutM is cut to, one
// within travelMaxM flown to, one farther not gone to.
func TestObserverTravelOrRefuse(t *testing.T) {
	now := time.Unix(1000, 0)
	s, list := observerBed(t, SpottingRequest{ICAO: "TEST", TravelMaxM: 2000})
	list = list[:1] // only the one by the far spot
	cut := s.look(now, list)
	if cut == nil || cut.spot != "apron:far" || !cut.travel {
		t.Fatalf("1500 m away: %+v, want a travel", cut)
	}

	s, list = observerBed(t, SpottingRequest{ICAO: "TEST", TravelMaxM: 1000, CutM: 1200})
	if s.cutM != 1000 {
		t.Errorf("cutM %.0f past travelMaxM 1000", s.cutM)
	}
	if cut := s.look(now, list[:1]); cut != nil && cut.look == nil {
		t.Fatalf("1500 m away with travelMaxM 1000: %+v, want nothing", cut)
	}

	// The first shot of a spotting is a cut, wherever the camera was.
	s, list = observerBed(t, SpottingRequest{ICAO: "TEST", TravelMaxM: 1000})
	s.havePose = false
	if cut := s.look(now, list[:1]); cut == nil || cut.travel {
		t.Fatalf("first shot: %+v, want a cut", cut)
	}
}

// TestObserverSpeedKept: travelSpeed is kept to 10–20 m/s.
func TestObserverSpeedKept(t *testing.T) {
	for in, want := range map[float64]float64{0: camera.DefaultTravelSpeed, 4: 10, 12: 12, 40: 20} {
		o, _ := observerFor(SpottingRequest{TravelMaxM: 1000, TravelSpeed: in})
		if o.speed != want {
			t.Errorf("travelSpeed %.0f: %.0f, want %.0f", in, o.speed, want)
		}
	}
}
