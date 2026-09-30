//go:build windows
// +build windows

package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

func TestDetailInterval(t *testing.T) {
	d := NewDetail()
	v := airport.LatLon{Lat: 50.1, Lon: 14.26}
	at := func(m float64) airport.LatLon { return offsetHeading(v, 90, m) }
	if n := d.Interval(at(20000), true, false); n != 0 {
		t.Errorf("no viewer: %d", n)
	}
	d.SetViewer(v)
	for _, c := range []struct {
		m            float64
		moving, full bool
		want         uint32
	}{
		{1000, true, false, 0}, {5000, true, false, DefaultDetailMidInterval}, {20000, true, false, DefaultDetailFarInterval},
		{1000, false, false, DefaultDetailStillInterval}, {20000, true, true, 0}, {20000, false, true, 0},
	} {
		if n := d.Interval(at(c.m), c.moving, c.full); n != c.want {
			t.Errorf("%.0f m moving %v full %v: %d, want %d", c.m, c.moving, c.full, n, c.want)
		}
	}
}

func TestDetailHysteresis(t *testing.T) {
	d := NewDetail()
	v := airport.LatLon{Lat: 50.1, Lon: 14.26}
	d.SetViewer(v)
	var s detailState
	now := time.Now()
	if n, ch := s.want(d, now, v, false, false); ch || n != 0 {
		t.Fatalf("first frame: %d %v (the monitor starts at every frame)", n, ch)
	}
	if _, ch := s.want(d, now.Add(time.Second), v, false, false); ch {
		t.Fatal("slower before SlowerAfter")
	}
	if n, ch := s.want(d, now.Add(2*time.Second), v, false, false); !ch || n != DefaultDetailStillInterval {
		t.Fatalf("still after 2 s: %d %v", n, ch)
	}
	if n, ch := s.want(d, now.Add(2*time.Second+time.Millisecond), v, true, false); !ch || n != 0 {
		t.Fatalf("moving again: %d %v, want every frame at once", n, ch)
	}
}

// TestDepartureDetail: a departure far from the viewer standing on its
// stand is driven twice a second; the push brings it back to its moving
// rate at once, lining up to every frame.
func TestDepartureDetail(t *testing.T) {
	d := NewDetail()
	ctl, ec, run, _ := injectedDeparture(t, TaxiRequest{HoldForClearances: true}, TaxiWithDetail(d))
	stand := ctl.req.Graph.Layout.Parking[ctl.req.Parking].Position
	d.SetViewer(offsetHeading(stand, 0, 20000)) // 20 km away
	if !run(TaxiAwaitingPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	run(TaxiPushback, 60*3)
	last := func() uint32 { return ec.intervals[len(ec.intervals)-1] }
	if last() != DefaultDetailStillInterval {
		t.Fatalf("on the stand: interval %d, want %d", last(), DefaultDetailStillInterval)
	}
	if n, perSec, full := d.Load(); n != 1 || perSec > 3 || full != 0 {
		t.Errorf("load %d aircraft, %.1f updates/s, %d every frame", n, perSec, full)
	}
	ctl.ClearPushback()
	run(TaxiAwaitingTaxi, 60*10)
	if ctl.State() != TaxiPushback || last() != DefaultDetailFarInterval {
		t.Fatalf("pushing back 20 km away: %v, interval %d, want %d", ctl.State(), last(), DefaultDetailFarInterval)
	}
	ctl.Cancel()
	if n, _, _ := d.Load(); n != 0 {
		t.Errorf("%d aircraft after the cancel", n)
	}
}

// BenchmarkDepartureTaxiFrame is the cost of one sim frame of one injected
// departure taxiing: the monitor answer, the mover step, the traffic look
// ahead and the placement (#370).
func BenchmarkDepartureTaxiFrame(b *testing.B) {
	t := &testing.T{}
	ctl, ec, run, now := injectedDeparture(t, TaxiRequest{})
	if !run(TaxiTaxiing, 60*600) {
		b.Fatalf("state %v", ctl.State())
	}
	mon := DefaultTaxiRequestBase + reqOffMonitor
	stand := ctl.req.Graph.Layout.Parking[ctl.req.Parking].Position
	msg := positionMsg(mon, 77, stand, 0, 0, true)
	sets := len(ec.waypoints)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N && ctl.State() == TaxiTaxiing; i++ {
		*now = now.Add(time.Second / 60)
		ctl.Handle(msg)
	}
	b.StopTimer()
	b.ReportMetric(float64(len(ec.waypoints)-sets)/float64(b.N), "sets/frame")
}

// A departure's tug drives off after the push while the aircraft stands
// still: every frame, even far from the viewer (at a still aircraft's rate
// the tug stuttered); once the tug is gone the aircraft slows down again.
func TestDepartureDetailTug(t *testing.T) {
	d := NewDetail()
	tug := &fakeTug{doneAfter: 60 * 20}
	ctl, ec, run, _ := injectedDeparture(t, TaxiRequest{HoldForClearances: true, Tug: tug}, TaxiWithDetail(d))
	stand := ctl.req.Graph.Layout.Parking[ctl.req.Parking].Position
	d.SetViewer(offsetHeading(stand, 0, 20000)) // 20 km away
	if !run(TaxiAwaitingPushback, 60*60) {
		t.Fatal(ctl.State())
	}
	ctl.ClearPushback()
	if !run(TaxiAwaitingTaxi, 60*300) {
		t.Fatal(ctl.State())
	}
	last := func() uint32 { return ec.intervals[len(ec.intervals)-1] }
	run(TaxiTaxiing, 60*5) // still waiting for the taxi clearance: the tug drives off
	if tug.Done() || last() != 0 {
		t.Fatalf("tug driving off: done %v, interval %d, want every frame", tug.Done(), last())
	}
	run(TaxiTaxiing, 60*30) // the tug gone: standing still again
	if !tug.Done() || last() != DefaultDetailStillInterval {
		t.Errorf("tug gone: done %v, interval %d, want %d", tug.Done(), last(), DefaultDetailStillInterval)
	}
}
