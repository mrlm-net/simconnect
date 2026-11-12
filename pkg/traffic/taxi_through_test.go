package traffic

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestTaxiThroughStand (#1508): LKTB 2A, joined to the taxiways only behind
// it, faces taxiway A 31 m ahead of the nose across open apron; the
// departure starts up on the stand and taxis straight out onto A, both
// runways, without a pushback, never behind the stand.
func TestTaxiThroughStand(t *testing.T) {
	g := airportGraph(t, "LKTB")
	i, err := g.Layout.ParkingIndex("2A")
	if err != nil {
		t.Fatal(err)
	}
	stand := g.Layout.Parking[i]
	for _, rwy := range []string{"27", "09"} {
		ec := &eventClient{}
		inj := NewInjector(ec)
		ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: i, Runway: rwy, Model: "FSLTL_B738_KLM", Tail: "T1", RollingTakeoffChance: -1}); err != nil {
			t.Fatal(err)
		}
		if !ctl.facesOut() {
			t.Fatalf("%s: 2A does not face out", rwy)
		}
		now := time.Now()
		ctl.now = func() time.Time { return now }
		ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
		var states []TaxiState
		done := make(chan struct{})
		go func() {
			for ev := range ctl.Events() {
				if len(states) == 0 || states[len(states)-1] != ev.State {
					states = append(states, ev.State)
				}
			}
			close(done)
		}()
		for k := 0; k < 60*3600 && !ctl.State().Terminal(); k++ {
			now = now.Add(time.Second / 60)
			ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, stand.Position, 0, 0, true))
		}
		<-done
		if ctl.State() != TaxiComplete || slices.Contains(states, TaxiPushback) {
			t.Fatalf("%s: states %v, want complete without a pushback", rwy, states)
		}
		// Straight ahead, nose along the stand, until past the nose's 20 m;
		// never behind the stand.
		for _, p := range placements(ec) {
			at := airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}
			ahead := alongHeading(stand.Position, stand.Heading, at)
			if ahead < -1 {
				t.Fatalf("%s: %.1f m behind the stand", rwy, -ahead)
			}
			if ahead < 20 && math.Abs(headingDiff(p.Heading, stand.Heading)) > 10 {
				t.Fatalf("%s: heading %.0f %.1f m ahead of the stand (stand %.0f)", rwy, p.Heading, ahead, stand.Heading)
			}
			if ahead > 60 {
				break
			}
		}
	}
}
