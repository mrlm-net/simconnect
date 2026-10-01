//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// validatedAirports are the airports validated against their facility data
// beyond LKPR (#376); airport.TestValidateAirportLayouts checks their
// layouts and routes.
var validatedAirports = []string{"EDDM", "LOWW", "EGLL"}

// runwayEndNames are the names of every runway end of l.
func runwayEndNames(l *airport.Layout) []string {
	var out []string
	for _, r := range l.Runways {
		out = append(out, r.Primary.Name, r.Secondary.Name)
	}
	return out
}

// crossingsOf are the runways route crosses other than rwy (the departure
// or landing runway, which is lined up on or vacated, not crossed).
func crossingsOf(route *airport.Route, rwy string) []string {
	var out []string
	for _, c := range route.RunwayCrossings {
		if c != rwy {
			out = append(out, c)
		}
	}
	return out
}

// TestValidatePushPoses (#376): every stand of the validated airports an
// A320 fits (DefaultMotionProfile), for every runway end, plans its
// departure with a B737 model; one that does not face out is pushed to a
// pose (a push onto a taxiway, nose along the way out). Every stand and runway without one is reported; as in
// TestPushToPoseEverywhere nearly all must have one.
func TestValidatePushPoses(t *testing.T) {
	if testing.Short() {
		t.Skip("plans every stand for every runway end: several minutes")
	}
	for _, icao := range validatedAirports {
		g := airportGraph(t, icao)
		l := g.Layout
		ends := runwayEndNames(l)
		planned, posed, out, towed, failed := 0, 0, 0, 0, 0
		var missing []string
		var suitable []int
		for i, st := range l.Parking {
			for _, rwy := range ends {
				ec := &eventClient{}
				ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(NewInjector(ec)))
				if suitable == nil {
					suitable = l.SuitableStands(DefaultMotionProfile().SpanMeters / 2)
				}
				if !slices.Contains(suitable, i) {
					break
				}
				err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: "FSLTL_B738_RYR", Tail: "T1", RollingTakeoffChance: -1})
				if err != nil {
					failed++
					t.Errorf("%s %s for %s: %v", icao, st.Label(), rwy, err)
					continue
				}
				if ctl.facesOut() {
					out++
					continue
				}
				planned++
				if ctl.pushPose == nil {
					// The older plan (straight back, or push and turn) is
					// kept; it must still be one a tug can push.
					path, err := ctl.pushPath()
					if err != nil {
						t.Errorf("%s %s for %s: no pose and no push: %v", icao, st.Label(), rwy, err)
						continue
					}
					missing = append(missing, fmt.Sprintf("%s for %s (%.0f m push)", st.Label(), rwy, path.Length()))
					continue
				}
				posed++
				if ctl.towPts != nil {
					towed++
				}
			}
		}
		t.Logf("%s: %d pushes, %d to a pose (%d with a tow), %d stand/runway pairs face out, %d fail to start", icao, planned, posed, towed, out, failed)
		if len(missing) > 0 {
			t.Logf("%s: no pose for %v", icao, missing)
		}
		if posed < planned*85/100 {
			t.Errorf("%s: only %d of %d pushes to a pose", icao, posed, planned)
		}
	}
}

// TestValidateInjectedDepartures (#376): injected departures from a sample
// of stands of each validated airport to every runway end reach the
// climb-out without failing or jumping, lined up on the runway heading, and
// hold short of every runway they cross (a crossing zone for each).
func TestValidateInjectedDepartures(t *testing.T) {
	for _, icao := range validatedAirports {
		g := airportGraph(t, icao)
		l := g.Layout
		stands := l.SuitableStands(18)
		ok, total := 0, 0
		for k, i := range stands {
			if k%12 != 0 {
				continue
			}
			p := l.Parking[i]
			for _, end := range runwayEndNames(l) {
				name := icao + " " + p.Label() + " → " + end
				total++
				ec := &eventClient{}
				inj := NewInjector(ec)
				ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
				if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: end, Model: "A320", RollingTakeoffChance: -1}); err != nil {
					t.Errorf("%s: start: %v", name, err)
					continue
				}
				now := time.Now()
				ctl.now = func() time.Time { return now }
				ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
				inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
				errs := make(chan error, 1)
				go func() {
					var last error
					for ev := range ctl.Events() {
						if ev.Err != nil {
							last = ev.Err
						}
					}
					errs <- last
				}()
				checked := map[TaxiState]bool{}
				for f := 0; f < 60*3600 && !ctl.State().Terminal(); f++ {
					now = now.Add(time.Second / 60)
					ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, p.Position, 0, 0, true))
					switch s := ctl.State(); {
					case s == TaxiTaxiing && !checked[s]:
						checked[s] = true
						rwy, _, _ := l.RunwayEnd(end)
						if want := crossingsOf(ctl.route, rwy.Name()); len(ctl.crossZones) != len(want) {
							t.Errorf("%s via %v: %d crossing holds for crossings %v", name, ctl.route.Taxiways, len(ctl.crossZones), want)
						}
					case s == TaxiLinedUp && !checked[s]:
						checked[s] = true
						if hd := math.Abs(headingDiff(ctl.mover.Pose().Heading, ctl.end.Heading)); hd > 3 {
							t.Errorf("%s: lined up %.1f° off the runway", name, hd)
						}
					}
				}
				if ctl.State() != TaxiComplete {
					var err error
					if ctl.State().Terminal() {
						err = <-errs
					}
					t.Errorf("%s: ended %v (%v)", name, ctl.State(), err)
					continue
				}
				all := placements(ec)
				jumped := false
				for j := 1; j < len(all); j++ {
					if d := calc.HaversineMeters(all[j-1].Latitude, all[j-1].Longitude, all[j].Latitude, all[j].Longitude); d > 2 {
						t.Errorf("%s: jumped %.2f m at placement %d/%d", name, d, j, len(all))
						jumped = true
						break
					}
				}
				if !jumped {
					ok++
				}
			}
		}
		t.Logf("%s: %d of %d departures complete", icao, ok, total)
	}
}

// TestValidateArrivalExits (#376): the exit chosen for an arrival (bestExit)
// to every stand of the validated airports from every runway end leads to
// the stand without turning back across the runway just vacated, and its
// taxi-in crosses no runway without holds on both sides.
func TestValidateArrivalExits(t *testing.T) {
	for _, icao := range validatedAirports {
		g := airportGraph(t, icao)
		l := g.Layout
		planned := 0
		for _, i := range l.SuitableStands(18) {
			if i%2 != 0 {
				continue
			}
			for _, end := range runwayEndNames(l) {
				p, err := PlanArrival(g, end, i, ArrivalOptions{})
				if err != nil {
					t.Errorf("%s %s → %s: %v", icao, end, l.Parking[i].Label(), err)
					continue
				}
				planned++
				if slices.Contains(p.Route.RunwayCrossings, p.Runway.Name()) {
					t.Errorf("%s %s exit %s → %s: crosses back over the runway vacated via %v", icao, end, p.Exit.Taxiway, l.Parking[i].Label(), p.Route.Taxiways)
				}
			}
		}
		t.Logf("%s: %d arrivals planned", icao, planned)
	}
}

// TestValidateInjectedArrivals (#376): fully injected arrivals (approach,
// landing, exit, taxi-in) on every runway end to a sample of stands of each
// validated airport park on the stop mark without failing or jumping, and
// hold short of every runway they cross on the way in.
func TestValidateInjectedArrivals(t *testing.T) {
	for _, icao := range validatedAirports {
		g := airportGraph(t, icao)
		l := g.Layout
		ok, total := 0, 0
		for k, i := range l.SuitableStands(18) {
			if k%15 != 0 {
				continue
			}
			p := l.Parking[i]
			for _, end := range runwayEndNames(l) {
				name := icao + " " + end + " → " + p.Label()
				ec := &eventClient{}
				inj := NewInjector(ec)
				ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
				if err := ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: i, Model: "A320", InjectApproach: true,
					RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
					t.Errorf("%s: start: %v", name, err)
					continue
				}
				total++
				now := time.Now()
				ctl.now = func() time.Time { return now }
				ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
				inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
				errs := make(chan error, 1)
				go func() {
					var last error
					for ev := range ctl.Events() {
						if ev.Err != nil {
							last = ev.Err
						}
					}
					errs <- last
				}()
				mon := DefaultArrivalRequestBase + arrReqMonitor
				zones := -1
				for f := 0; f < 60*3600 && !ctl.State().Terminal(); f++ {
					now = now.Add(time.Second / 60)
					ctl.Handle(arrivalPositionMsg(mon, 77, p.Position, 0, 0, 0, false))
					if zones < 0 && ctl.State() == ArrivalTaxiing {
						zones = len(ctl.crossZones)
					}
				}
				if ctl.State() != ArrivalParked {
					var err error
					if ctl.State().Terminal() {
						err = <-errs
					}
					t.Errorf("%s: ended %v (%v)", name, ctl.State(), err)
					continue
				}
				plan := ctl.Plan()
				if want := crossingsOf(plan.Route, plan.Runway.Name()); zones != len(want) {
					t.Errorf("%s via %v: %d crossing holds for crossings %v", name, plan.Route.Taxiways, zones, want)
				}
				all := placements(ec)
				good := true
				for j := 1; j < len(all); j++ {
					if d := calc.HaversineMeters(all[j-1].Latitude, all[j-1].Longitude, all[j].Latitude, all[j].Longitude); d > 1.5 {
						t.Errorf("%s: jumped %.2f m at placement %d/%d", name, d, j, len(all))
						good = false
						break
					}
				}
				last := all[len(all)-1]
				if hd := math.Abs(headingDiff(last.Heading, p.Heading)); hd > 3 {
					t.Errorf("%s: parked %.1f° off the stand heading", name, hd)
					good = false
				}
				if d := calc.HaversineMeters(plan.Stop.Lat, plan.Stop.Lon, last.Latitude, last.Longitude); d > 1 {
					t.Errorf("%s: parked %.1f m from the stop mark", name, d)
					good = false
				}
				if good {
					ok++
				}
			}
		}
		t.Logf("%s: %d of %d arrivals parked", icao, ok, total)
	}
}
