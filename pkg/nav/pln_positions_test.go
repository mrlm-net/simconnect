//go:build windows

package nav

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// fakeFixLoader answers each request on the next Handle, two at a time.
type fakeFixLoader struct {
	known     map[FixKey]airport.LatLon
	asked     []FixKey
	inFlight  []FixKey
	requested int
}

func (f *fakeFixLoader) Free() int { return 2 - len(f.inFlight) }
func (f *fakeFixLoader) Request(k FixKey) error {
	f.asked, f.inFlight = append(f.asked, k), append(f.inFlight, k)
	return nil
}
func (f *fakeFixLoader) Handle(engine.Message) (NavResult, bool) {
	if len(f.inFlight) == 0 {
		return NavResult{}, false
	}
	k := f.inFlight[0]
	f.inFlight = f.inFlight[1:]
	p, ok := f.known[k]
	return NavResult{Key: k, Fix: Fix{Position: p}, Found: ok}, true
}
func (f *fakeFixLoader) Expire(time.Time) []NavResult { return nil }

// TestPLNResolver: the 2024 layout's positions filled in — fixes by kind
// through the facility API (a fix twice in the plan asked once), airports
// by the lookup; what is unknown listed (#679).
func TestPLNResolver(t *testing.T) {
	plan := &PLNPlan{Waypoints: []PLNWaypoint{
		{Type: "Airport", Ident: "LKPR"},
		{Type: "Intersection", Ident: "GOLOP", Region: "LK"},
		{Type: "VOR", Ident: "VOZ", Region: "LK"},
		{Type: "Intersection", Ident: "GOLOP", Region: "LK"},
		{Type: "NDB", Ident: "XXX", Region: "LK"},
		{Type: "User", ID: "MYPOINT"},
		{Type: "Airport", Ident: "LKPD", Position: airport.LatLon{Lat: 50.01, Lon: 15.74}},
	}}
	golop, voz := airport.LatLon{Lat: 50.3, Lon: 13.9}, airport.LatLon{Lat: 49.9, Lon: 14.2}
	f := &fakeFixLoader{known: map[FixKey]airport.LatLon{Key("GOLOP", "LK", KindWaypoint): golop, Key("VOZ", "LK", KindVOR): voz}}
	r := newPLNResolver(f, plan, func(icao string) (airport.LatLon, bool) {
		return airport.LatLon{Lat: 50.1, Lon: 14.26}, icao == "LKPR"
	})
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10 && !r.Done(); i++ {
		r.Handle(engine.Message{})
	}
	if !r.Done() {
		t.Fatal("not done")
	}
	if len(f.asked) != 3 {
		t.Errorf("asked %v, want GOLOP once, VOZ, XXX", f.asked)
	}
	w := plan.Waypoints
	if w[0].Position.Lat != 50.1 || w[1].Position != golop || w[3].Position != golop || w[2].Position != voz || w[6].Position.Lat != 50.01 {
		t.Errorf("positions %+v", w)
	}
	if m := r.Missing(); len(m) != 1 || m[0] != "XXX.LK.N" {
		t.Errorf("missing %v, want the unknown NDB (the user point has no ident)", m)
	}
}
