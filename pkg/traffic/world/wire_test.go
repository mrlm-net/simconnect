package world

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// fakeDep answers a few of departureCtl's calls (the rest are not called).
type fakeDep struct {
	departureCtl
	entry string
}

func (f *fakeDep) State() traffic.TaxiState { return traffic.TaxiHoldingShort }
func (f *fakeDep) ClimbPlan(pos airport.LatLon) []traffic.RoutePoint {
	return []traffic.RoutePoint{{Position: pos, AltFt: 3000, Kts: 220}}
}
func (f *fakeDep) ChangeEntry(entry string) error {
	if entry == "Z" {
		return errors.New("no entry Z")
	}
	f.entry = entry
	return nil
}

type fakeArr struct{ arrivalCtl }

func (f *fakeArr) Holding() (traffic.Hold, float64, bool) {
	return traffic.Hold{Ident: "BAVOK"}, 7000, true
}

// serve answers the calls coming in on l with srv (the actuator's loop).
func serve(l link, srv *wireServer) {
	for {
		m, err := l.Recv()
		if err != nil {
			return
		}
		if m.Kind == wireCall {
			_ = l.Send(srv.dispatch(m))
		}
	}
}

// TestWireCalls: the director's stand-ins call the actuator's controllers:
// arguments, several results and errors come through.
func TestWireCalls(t *testing.T) {
	dir, act := pipe()
	defer dir.Close()
	srv := newWireServer()
	dep := &fakeDep{}
	srv.add("dep/1", dep)
	srv.add("arr/2", &fakeArr{})
	go serve(act, srv)
	c := newWireClient(dir, nil)
	d, a := &remoteDep{c: c, t: "dep/1"}, &remoteArr{c: c, t: "arr/2"}

	if s := d.State(); s != traffic.TaxiHoldingShort {
		t.Errorf("state %v", s)
	}
	p := airport.LatLon{Lat: 50.1, Lon: 14.26}
	if plan := d.ClimbPlan(p); len(plan) != 1 || plan[0].Position != p || plan[0].AltFt != 3000 {
		t.Errorf("climb plan %+v", plan)
	}
	if err := d.ChangeEntry("B"); err != nil || dep.entry != "B" {
		t.Errorf("change entry: %v, entry %q", err, dep.entry)
	}
	if err := d.ChangeEntry("Z"); err == nil || err.Error() != "no entry Z" {
		t.Errorf("change entry Z: %v", err)
	}
	if h, alt, ok := a.Holding(); !ok || h.Ident != "BAVOK" || alt != 7000 {
		t.Errorf("holding %+v %v %v", h, alt, ok)
	}
	if d.Handle(nilMessage()) {
		t.Error("the director handled a simulator message")
	}
	if err := (&remoteDep{c: c, t: "dep/9"}).ChangeEntry("B"); err == nil {
		t.Error("a target that is not there answered")
	}
}

// TestWireEvents: a controller's events reach the director in order, an
// event's error as an error; the end of its events ends the director's.
func TestWireEvents(t *testing.T) {
	dir, act := pipe()
	defer dir.Close()
	c := newWireClient(dir, nil)
	evs := c.subscribe("dep/1")
	a := &actuatorSim{srv: newWireServer(), send: act.Send}
	in := make(chan traffic.TaxiEvent, 3)
	in <- traffic.TaxiEvent{State: traffic.TaxiPushback}
	in <- traffic.TaxiEvent{State: traffic.TaxiTaxiing, Err: errors.New("held")}
	close(in)
	go a.pump("dep/1", func(yield func(any, error) bool) {
		for ev := range in {
			if !yield(ev, ev.Err) {
				return
			}
		}
	})
	var got []wireMsg
	for len(got) < 2 {
		select {
		case m := <-evs:
			got = append(got, m)
		case <-time.After(2 * time.Second):
			t.Fatalf("got %d events", len(got))
		}
	}
	if got[0].Err != "" || got[1].Err != "held" {
		t.Errorf("errors %q %q", got[0].Err, got[1].Err)
	}
}

// recFeed records what the feed tells.
type recFeed struct {
	traffic []Traffic
	layout  string
	paused  bool
	airways int // segments of the airways fed
}

func (r *recFeed) Airports([]traffic.AirportRef)                  {}
func (r *recFeed) Weather(nav.Weather)                            {}
func (r *recFeed) ILS(nav.NavResult)                              {}
func (r *recFeed) Procedures(airport.Procedures)                  {}
func (r *recFeed) Airways(_ string, g *nav.AirwayGraph)           { r.airways = g.SegmentCount() }
func (r *recFeed) Layout(icao string, _ *airport.Layout, _ error) { r.layout = icao }
func (r *recFeed) UserAircraft(Aircraft, float64, string)         {}
func (r *recFeed) Paused(p bool)                                  { r.paused = p }
func (r *recFeed) Traffic(scan []Traffic)                         { r.traffic = scan }

// TestWireFeed: what the actuator's simulator tells reaches the director's
// feed, a layout its airport cache.
func TestWireFeed(t *testing.T) {
	dir, act := pipe()
	defer dir.Close()
	rec, cache := &recFeed{}, airport.NewCache()
	done := make(chan struct{}, 8)
	newWireClient(dir, func(m wireMsg) {
		if err := feedIn(m, rec, cache); err != nil {
			t.Error(err)
		}
		done <- struct{}{}
	})
	out := &wireFeedOut{send: act.Send}
	out.Traffic([]Traffic{{ObjectID: 7, Tail: "CSA1", Latitude: 50.1, Longitude: 14.26}})
	out.Paused(true)
	out.Layout("LKXX", &airport.Layout{ICAO: "LKXX", Name: "Test"}, nil)
	g, err := nav.LoadAirwayGraph("../../nav/testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	out.Airways("LKPR", g)
	for range 4 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("feed not delivered")
		}
	}
	if len(rec.traffic) != 1 || rec.traffic[0].Tail != "CSA1" || !rec.paused || rec.layout != "LKXX" {
		t.Errorf("fed %+v", rec)
	}
	if l, ok := cache.Layout("LKXX"); !ok || l.Name != "Test" {
		t.Error("layout not in the cache")
	}
	if rec.airways != g.SegmentCount() {
		t.Errorf("airways fed with %d segments, want %d", rec.airways, g.SegmentCount())
	}
}

func nilMessage() engine.Message { return engine.Message{} }

// TestWireWeatherNaN: weather with an unknown dewpoint crosses the wire.
func TestWireWeatherNaN(t *testing.T) {
	dir, act := pipe()
	defer dir.Close()
	got := make(chan nav.Weather, 1)
	f := &weatherFeed{got: got}
	newWireClient(dir, func(m wireMsg) { _ = feedIn(m, f, nil) })
	w := nav.StaticWeather(240, 8, 9999, 15, math.NaN(), 1013)
	(&wireFeedOut{send: act.Send}).Weather(w)
	select {
	case g := <-got:
		if g.QNHhPa != 1013 || !math.IsNaN(g.DewpointC) {
			t.Errorf("%+v", g)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("weather lost")
	}
}

type weatherFeed struct {
	recFeed
	got chan nav.Weather
}

func (f *weatherFeed) Weather(w nav.Weather) { f.got <- w }
