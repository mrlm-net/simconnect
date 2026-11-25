package worldtest

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// XDEC, a made-up field with declared distances and a one-way end (#1660,
// #1661): runway 09/27 of 2400 m. 09 takes off from declaredStartM in;
// 27 is for landings only, with a landing distance of declaredLDAM. Its
// ways off are B at the 09 end and D declaredDM in from it, both to a
// parallel taxiway A and the apron north of it.
//
//	          [0] [1] [2]
//	   A ----+----+---+---+----
//	     B   |    D
//	09 ==+=======+=========================== 27
//	     |   ^ take-off start (400 m)
//	         D at 500 m; 27's LDA ends 900 m from the 09 end
const (
	declaredICAO   = "XDEC"
	declaredLength = 2400.0
	declaredStartM = 400.0
	declaredLDAM   = 1500.0
	declaredDM     = 500.0
	declaredTitle  = "FSLTL_DHC6_WIA-Winair"
)

// declaredFieldDir writes XDEC to a new data folder.
func declaredFieldDir(t *testing.T) string {
	t.Helper()
	var pts []airport.RawTaxiPoint
	var paths []airport.RawTaxiPath
	pt := func(x, z float32, typ int32) int32 {
		pts = append(pts, airport.RawTaxiPoint{Type: typ, BiasX: x, BiasZ: z})
		return int32(len(pts) - 1)
	}
	link := func(a, b, typ int32, name uint32, width float32, rwy int32) {
		paths = append(paths, airport.RawTaxiPath{Type: typ, Width: width, RunwayNumber: rwy, Start: a, End: b, NameIndex: name})
	}
	west := float32(-declaredLength / 2)
	dx := west + declaredDM
	w, d, e := pt(west, 0, 1), pt(dx, 0, 1), pt(-west, 0, 1)
	link(w, d, 2, 0, 30, 9)
	link(d, e, 2, 0, 30, 9)
	hB, hD := pt(west, 60, 2), pt(dx, 60, 2)
	a1, a2, a3, a4 := pt(west, 180, 1), pt(dx, 180, 1), pt(0, 180, 1), pt(200, 180, 1)
	link(w, hB, 1, 2, 18, 0)
	link(hB, a1, 1, 2, 18, 0)
	link(d, hD, 1, 3, 18, 0)
	link(hD, a2, 1, 3, 18, 0)
	link(a1, a2, 1, 1, 18, 0)
	link(a2, a3, 1, 1, 18, 0)
	link(a3, a4, 1, 1, 18, 0)

	var parking []airport.RawParking
	last := a3
	for _, sx := range []float32{-60, 0, 60} {
		at := pt(sx, 280, 1)
		link(last, at, 1, 1, 16, 0)
		parking = append(parking, airport.RawParking{Name: 1, Number: uint32(len(parking) + 1), Type: 3, Heading: 0, Radius: 13, BiasX: sx, BiasZ: 315})
		link(at, int32(len(parking)-1), 3, 0, 16, 0)
		last = at
	}
	link(last, pt(120, 280, 1), 1, 1, 16, 0)

	raw := airport.RawAirport{ICAO: declaredICAO, Name: "Declared", Latitude: 45, Longitude: 15, Altitude: 10,
		Runways: []airport.RawRunway{{Latitude: 45, Longitude: 15, Altitude: 10, Heading: 90, Length: declaredLength, Width: 30,
			PrimaryNumber: 9, SecondaryNumber: 27,
			PrimaryTakeoffStartM: declaredStartM, SecondaryLDAM: declaredLDAM, SecondaryUse: int32(airport.RunwayUseLanding)}},
		Parking: parking, TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "B", "D"},
		Frequencies: []airport.RawFrequency{{Type: 5, Hz: 121900000, Name: "DECLARED"}, {Type: 6, Hz: 118100000, Name: "DECLARED"}},
		Tower:       &airport.RawTower{Latitude: 45.003, Longitude: 15, Altitude: 10}}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, raw.ICAO), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, raw.ICAO, "layout.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// alongAcross are how far p is ahead of from along hdg, and to the side.
func alongAcross(from, p airport.LatLon, hdg float64) (float64, float64) {
	d := calc.HaversineMeters(from.Lat, from.Lon, p.Lat, p.Lon)
	off := (calc.BearingDegrees(from.Lat, from.Lon, p.Lat, p.Lon) - hdg) * math.Pi / 180
	return d * math.Cos(off), d * math.Sin(off)
}

// declaredFarNM: the arrival of TestDeclaredField's departureFirst appears
// this far out, so the departure is planned to go before it.
const declaredFarNM = 40.0

// TestDeclaredField (#1660, #1661): at XDEC with the wind down 27, a
// departure and an arrival planned by the World. The departure takes off
// from 09, never 27 (landing only), lined up and rolling no nearer the 09
// end than its take-off start; the arrival lands on 27 and is down to its
// exit speed by the end of 27's landing distance, its exit beyond it.
// Opposite directions on one runway: the departure is airborne and past
// the far end before the arrival is inside traffic.OppositeArrivalNM,
// never on the runway with it there or cleared to land. Both orders: the
// arrival on a short final first, then the departure; the arrival far out
// (declaredFarNM), the departure first. Neither jumps.
func TestDeclaredField(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	t.Run("arrivalFirst", func(t *testing.T) { declaredRun(t, 0) })
	t.Run("departureFirst", func(t *testing.T) { declaredRun(t, declaredFarNM) })
}

// declaredRun is a case of TestDeclaredField: the arrival appears spawnNm
// out (0: the default, inside OppositeArrivalNM: it lands first), the
// departure goes first otherwise.
func declaredRun(t *testing.T, spawnNm float64) {
	depFirst := spawnNm > 0
	log := &lockedBuf{}
	o := Options{DataDir: declaredFieldDir(t), ICAO: declaredICAO, Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), declaredTitle)}
	o.World.Output = log
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, e09, ok09 := h.Layout.RunwayEnd("09")
	rwy, e27, ok27 := h.Layout.RunwayEnd("27")
	if !ok09 || !ok27 {
		t.Fatal("no runway 09/27")
	}
	if e27.ForTakeoff() || e09.TakeoffStartM != declaredStartM || e27.LDAM != declaredLDAM {
		t.Fatalf("declared values not read: 27 %s, 09 start %.0f m, 27 LDA %.0f m", e27.Use, e09.TakeoffStartM, e27.LDAM)
	}
	wx := nav.StaticWeather(270, 3, 9999, 15, 5, 1013)
	h.World.SetWeather(&wx)
	h.Advance(2 * time.Second)
	jumps := len(h.Sim.Jumps())

	const dep, arr = "XDD1", "XDA1"
	for _, r := range []world.SpawnRequest{
		{Kind: "departure", ICAO: declaredICAO, Tail: dep, Stand: 0, Model: declaredTitle},
		{Kind: "arrival", ICAO: declaredICAO, Tail: arr, Stand: 2, Model: declaredTitle, InjectApproach: true, SpawnNm: spawnNm},
	} {
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatalf("%s: %v", r.Kind, err)
		}
	}
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
	slowKts := traffic.ExitSpeedKts + 3
	var (
		runways        = map[string][]string{}
		rolled, away   bool
		rollAlong      = math.NaN()
		stoppedAlong   = math.Inf(1)
		landed, parked bool
		pastLDA        float64
		pastLDAAt      string
		start          = h.Clock.Now()
		rollFrom27     bool
		depOn, cleared bool
		arrIn          string
		overlap        string
		airNM, airFt   = math.Inf(1), 0.0
		depAt          Object
		rolledAt       time.Time // the departure's roll began
		pastEndAt      time.Time // the departure airborne past the far end
		landedAt       time.Time // the arrival down on 27
	)
	note := func(tail, rw string) {
		if rw != "" && !slices.Contains(runways[tail], rw) {
			runways[tail] = append(runways[tail], rw)
		}
	}
	finished := h.Until(40*time.Minute, func() bool {
		now := h.Clock.Now()
		depOn, arrIn = false, ""
		if v, ob, ok := object(dep); ok {
			note(dep, v.Runway)
			pos := airport.LatLon{Lat: ob.Lat, Lon: ob.Lon}
			along, across := alongAcross(e09.Threshold, pos, e09.Heading)
			onRunway := ob.OnGround && math.Abs(across) < rwy.Width/2 && along > -10 && along < declaredLength+10
			if onRunway && ob.GroundKts > 35 {
				if !rolled {
					rolled, rollAlong, rolledAt = true, along, now
				}
				if math.Abs(math.Remainder(ob.Heading-e09.Heading, 360)) > 90 {
					rollFrom27 = true
				}
			}
			if onRunway && !rolled && ob.GroundKts < 1 && math.Abs(math.Remainder(ob.Heading-e09.Heading, 360)) < 10 && math.Abs(across) < 3 {
				stoppedAlong = math.Min(stoppedAlong, along)
			}
			away = away || rolled && !ob.OnGround
			if rolled && !ob.OnGround && along >= declaredLength && pastEndAt.IsZero() {
				pastEndAt = now
			}
			// On the runway, or airborne short of its far end.
			depOn, depAt = onRunway || rolled && !ob.OnGround && along < declaredLength, ob
		}
		if v, ob, ok := object(arr); ok {
			note(arr, v.Runway)
			pos := airport.LatLon{Lat: ob.Lat, Lon: ob.Lon}
			along, across := alongAcross(e27.LandingThreshold(), pos, e27.Heading)
			if ob.OnGround && math.Abs(across) < rwy.Width/2 && along > 0 && math.Abs(math.Remainder(ob.Heading-e27.Heading, 360)) < 45 {
				if !landed {
					landed, landedAt = true, now
				}
				if along > declaredLDAM && ob.GroundKts > slowKts && ob.GroundKts > pastLDA {
					pastLDA = ob.GroundKts
					pastLDAAt = fmt.Sprintf("%.0f m past the threshold, %s, at %d s", along, v.State, int(now.Sub(start).Seconds()))
				}
			}
			parked = parked || v.State == "parked"
			if !cleared {
				cleared = slices.ContainsFunc(h.Heard(), func(tx traffic.Transmission) bool {
					return tx.Callsign == arr && !tx.Pilot && tx.Intent == traffic.IntentLanding
				})
			}
			thr := e27.LandingThreshold()
			nm := calc.HaversineNM(ob.Lat, ob.Lon, thr.Lat, thr.Lon)
			switch {
			case !ob.OnGround && cleared:
				arrIn = fmt.Sprintf("cleared to land, %.1f NM out", nm)
			case !ob.OnGround && nm <= traffic.OppositeArrivalNM:
				arrIn = fmt.Sprintf("%.1f NM out", nm)
			case ob.OnGround && math.Abs(across) < rwy.Width/2+15 && along > -100 && along < declaredLength+100:
				arrIn = "on the runway, " + v.State
			}
			if !ob.OnGround && depAt.ID != 0 && !depAt.OnGround {
				if d := calc.HaversineNM(ob.Lat, ob.Lon, depAt.Lat, depAt.Lon); d < airNM {
					airNM, airFt = d, math.Abs(ob.AltFt-depAt.AltFt)
				}
			}
		}
		if depOn && arrIn != "" && overlap == "" {
			overlap = fmt.Sprintf("the departure on 09 or short of its far end with the arrival %s, at %d s", arrIn, int(now.Sub(start).Seconds()))
		}
		return away && parked
	})
	at := func(tm time.Time) string {
		if tm.IsZero() {
			return "never"
		}
		return fmt.Sprintf("%d s", int(tm.Sub(start).Seconds()))
	}
	t.Logf("runways %v; roll from %.0f m at %s, past the far end at %s, lined up stopped at %.0f m; the arrival down at %s",
		runways, rollAlong, at(rolledAt), at(pastEndAt), stoppedAlong, at(landedAt))
	if !finished {
		t.Errorf("not finished in 40 simulated minutes: departure away %v, arrival parked %v", away, parked)
	}
	if slices.Contains(runways[dep], "27") || rollFrom27 {
		t.Errorf("the departure was given or rolled on 27, landing only: %v", runways[dep])
	}
	if !rolled {
		t.Error("the departure never rolled on 09")
	} else if tol := 10.0; rollAlong < declaredStartM-tol || stoppedAlong < declaredStartM-tol {
		t.Errorf("the departure lined up %.0f m and rolled from %.0f m in from the 09 end, before the take-off start %.0f m in", stoppedAlong, rollAlong, declaredStartM)
	}
	if !landed || !slices.Contains(runways[arr], "27") {
		t.Errorf("the arrival did not land on 27: %v", runways[arr])
	}
	// The order each case is for.
	switch {
	case depFirst && (pastEndAt.IsZero() || !landedAt.IsZero() && !pastEndAt.Before(landedAt)):
		t.Errorf("the arrival spawned %.0f NM out: want the departure past the far end (%s) before the arrival lands (%s)", spawnNm, at(pastEndAt), at(landedAt))
	case !depFirst && (landedAt.IsZero() || !rolledAt.IsZero() && !landedAt.Before(rolledAt)):
		t.Errorf("the arrival spawned on a short final: want it down (%s) before the departure rolls (%s)", at(landedAt), at(rolledAt))
	}
	if overlap != "" {
		t.Errorf("opposite-direction traffic at once (the departure must be past the far end before the arrival is inside %.0f NM): %s", traffic.OppositeArrivalNM, overlap)
	}
	t.Logf("both airborne: closest %.1f NM, %.0f ft apart", airNM, airFt)
	if pastLDA > 0 {
		t.Errorf("the arrival rolled at %.0f kt past 27's landing distance (%.0f m), over %.0f kt: %s", pastLDA, declaredLDAM, slowKts, pastLDAAt)
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
