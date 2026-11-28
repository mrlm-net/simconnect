package worldtest

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// declaredExitLimit: TestDeclaredFieldExit's arrival is parked and its
// departure away within this.
const declaredExitLimit = 40 * time.Minute

// TestDeclaredFieldExit (#1719): at XDEC an airliner (declaredJet)
// taxiing out to 09, which it takes off from at B for the whole run, and
// one landing on 27 asked for as the departure starts to taxi, on a short
// final: the departure holds short of 09 for it. The arrival, down to its
// exit speed by 27's landing distance on its short-field landing, leaves
// by D, not B where the departure holds (it stood on the runway at B's
// mouth and the departure short of it, each waiting for the other). Both
// ways round. The arrival lands before the departure rolls, the departure
// is not on the runway with the arrival on it or inside
// traffic.OppositeArrivalNM, the arrival parks and the departure goes, and
// neither jumps.
func TestDeclaredFieldExit(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for _, mirror := range []bool{false, true} {
		way := "09"
		if mirror {
			way = "27"
		}
		t.Run(way, func(t *testing.T) { declaredExitRun(t, mirror) })
	}
}

// declaredExitRun is a case of TestDeclaredFieldExit.
func declaredExitRun(t *testing.T, mirror bool) {
	depName, arrName, windFrom := "09", "27", 270.0
	if mirror {
		depName, arrName, windFrom = "27", "09", 90
	}
	log := &lockedBuf{}
	o := Options{DataDir: declaredFieldDir(t, mirror), ICAO: declaredICAO, Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), declaredTitle)}
	o.World.Output = log
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, eDep, okDep := h.Layout.RunwayEnd(depName)
	rwy, eArr, okArr := h.Layout.RunwayEnd(arrName)
	if !okDep || !okArr {
		t.Fatalf("no runway %s/%s", depName, arrName)
	}
	wx := nav.StaticWeather(windFrom, 3, 9999, 15, 5, 1013)
	h.World.SetWeather(&wx)
	h.Advance(2 * time.Second)
	jumps := len(h.Sim.Jumps())

	const dep, arr = "XDD1", "XDA1"
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "departure", ICAO: declaredICAO, Tail: dep, Stand: 0, Model: declaredJet}); err != nil {
		t.Fatalf("departure: %v", err)
	}
	arrival := world.SpawnRequest{Kind: "arrival", ICAO: declaredICAO, Tail: arr, Stand: 2, Model: declaredJet, InjectApproach: true}
	object := func(tail string) (world.ControlView, Object, bool) {
		v, ok := h.Aircraft(tail)
		if !ok || v.ObjectID == 0 {
			return v, Object{}, false
		}
		for _, ob := range h.Sim.Objects() {
			if ob.ID == v.ObjectID {
				return v, ob, true
			}
		}
		return v, Object{}, false
	}
	var (
		asked, landed, parked, rolled, away bool
		landedAt, rolledAt                  time.Time
		start                               = h.Clock.Now()
		overlap                             string
		heldShort                           bool
	)
	finished := h.Until(declaredExitLimit, func() bool {
		now := h.Clock.Now()
		depOn := false
		if v, ob, ok := object(dep); ok {
			if !asked && v.State == traffic.TaxiTaxiing.String() {
				if _, err := h.World.Do(http.MethodPost, "/api/control", arrival); err != nil {
					t.Fatalf("arrival: %v", err)
				}
				asked = true
			}
			heldShort = heldShort || asked && v.State == traffic.TaxiHoldingShort.String()
			along, across := alongAcross(eDep.Threshold, airport.LatLon{Lat: ob.Lat, Lon: ob.Lon}, eDep.Heading)
			onRunway := ob.OnGround && math.Abs(across) < rwy.Width/2 && along > -10 && along < declaredLength+10
			if onRunway && ob.GroundKts > 35 && !rolled {
				rolled, rolledAt = true, now
			}
			away = away || rolled && !ob.OnGround
			depOn = onRunway || rolled && !ob.OnGround && along < declaredLength
		}
		arrIn := ""
		if v, ob, ok := object(arr); ok {
			along, across := alongAcross(eArr.LandingThreshold(), airport.LatLon{Lat: ob.Lat, Lon: ob.Lon}, eArr.Heading)
			if ob.OnGround && math.Abs(across) < rwy.Width/2 && along > 0 && !landed {
				landed, landedAt = true, now
			}
			parked = parked || v.State == "parked"
			thr := eArr.LandingThreshold()
			switch {
			case !ob.OnGround && !landed:
				if nm := calc.HaversineNM(ob.Lat, ob.Lon, thr.Lat, thr.Lon); nm <= traffic.OppositeArrivalNM {
					arrIn = fmt.Sprintf("%.1f NM out", nm)
				}
			case ob.OnGround && math.Abs(across) < rwy.Width/2+15 && along > -100 && along < declaredLength+100:
				arrIn = "on the runway, " + v.State
			}
		}
		if depOn && arrIn != "" && overlap == "" {
			overlap = arrIn + " at " + now.Sub(start).Round(time.Second).String()
		}
		return away && parked
	})
	if !finished {
		t.Errorf("not finished in %v simulated: arrival asked for %v, down %v, parked %v; departure held short for it %v, rolled %v, away %v",
			declaredExitLimit, asked, landed, parked, heldShort, rolled, away)
	}
	if !heldShort {
		t.Errorf("the departure never held short with the arrival coming in: nothing checked")
	}
	at := func(tm time.Time) string {
		if tm.IsZero() {
			return "never"
		}
		return tm.Sub(start).Round(time.Second).String()
	}
	if !landed || !rolled || !landedAt.Before(rolledAt) {
		t.Errorf("want the arrival down (%s) before the departure rolls (%s)", at(landedAt), at(rolledAt))
	}
	if overlap != "" {
		t.Errorf("the departure on %s or short of its far end with the arrival %s", depName, overlap)
	}
	for _, j := range h.Sim.Jumps()[jumps:] {
		if j.Tail == dep || j.Tail == arr {
			t.Errorf("%s jumped: %+v", j.Tail, j)
		}
	}
	if t.Failed() {
		t.Log(log.String())
	}
}
