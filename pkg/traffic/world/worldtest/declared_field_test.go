package worldtest

import (
	"encoding/json"
	"errors"
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
// parallel taxiway A and the apron north of it. Mirrored, the same field
// the other way round: 27 takes off, 09 lands only, B and D at the 27 end.
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
	// declaredJet: an airliner XDEC's take-off run available fits
	// (traffic.RunwayEndLongEnough).
	declaredJet = "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery"
)

// declaredFieldDir writes XDEC to a new data folder, mirrored east to
// west when mirror is set.
func declaredFieldDir(t *testing.T, mirror bool) string {
	t.Helper()
	sx := float32(1)
	rwyNum, hdg, primary, secondary := int32(9), float32(90), int32(9), int32(27)
	if mirror {
		sx, rwyNum, hdg, primary, secondary = -1, 27, 270, 27, 9
	}
	var pts []airport.RawTaxiPoint
	var paths []airport.RawTaxiPath
	pt := func(x, z float32, typ int32) int32 {
		pts = append(pts, airport.RawTaxiPoint{Type: typ, BiasX: sx * x, BiasZ: z})
		return int32(len(pts) - 1)
	}
	link := func(a, b, typ int32, name uint32, width float32, rwy int32) {
		paths = append(paths, airport.RawTaxiPath{Type: typ, Width: width, RunwayNumber: rwy, Start: a, End: b, NameIndex: name})
	}
	west := float32(-declaredLength / 2)
	dx := west + declaredDM
	w, d, e := pt(west, 0, 1), pt(dx, 0, 1), pt(-west, 0, 1)
	link(w, d, 2, 0, 30, rwyNum)
	link(d, e, 2, 0, 30, rwyNum)
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
	for _, px := range []float32{-60, 0, 60} {
		at := pt(px, 280, 1)
		link(last, at, 1, 1, 16, 0)
		parking = append(parking, airport.RawParking{Name: 1, Number: uint32(len(parking) + 1), Type: 3, Heading: 0, Radius: 13, BiasX: sx * px, BiasZ: 315})
		link(at, int32(len(parking)-1), 3, 0, 16, 0)
		last = at
	}
	link(last, pt(120, 280, 1), 1, 1, 16, 0)

	raw := airport.RawAirport{ICAO: declaredICAO, Name: "Declared", Latitude: 45, Longitude: 15, Altitude: 10,
		Runways: []airport.RawRunway{{Latitude: 45, Longitude: 15, Altitude: 10, Heading: hdg, Length: declaredLength, Width: 30,
			PrimaryNumber: primary, SecondaryNumber: secondary,
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

// declaredAirborneLimit: an arrival asked for with the departure airborne
// appears once the departure is clear of its way in, minutes on (a
// later release), and lands within this.
const declaredAirborneLimit = time.Hour

// declaredAskEvery: an arrival asked for and not let in yet (an arrival
// appearing would meet the departure) is asked for again this often.
const declaredAskEvery = 30 * time.Second

// declaredCase is a run of TestDeclaredField: the arrival appears spawnNm
// out (0: the default, inside OppositeArrivalNM: it lands first, the
// departure goes first otherwise), asked for once the departure is
// airborne when airborne is set; on XDEC mirrored when mirror is set; dep
// and arr the departure's and the arrival's models ("" declaredTitle).
type declaredCase struct {
	spawnNm  float64
	airborne bool
	mirror   bool
	dep, arr string
}

// TestDeclaredField (#1660, #1661): at XDEC with the wind down 27, a
// departure and an arrival planned by the World. The departure takes off
// from 09, never 27 (landing only), lined up and rolling no nearer the 09
// end than its take-off start; the arrival lands on 27 and is down to its
// exit speed by the end of 27's landing distance, its exit beyond it.
// Opposite directions on one runway: the departure is airborne and past
// the far end before the arrival is inside traffic.OppositeArrivalNM,
// never on the runway with it there or cleared to land, and in the air
// they keep traffic.OppositeSeparationNM apart horizontally, whatever
// their levels (the user, 2026-11-27). The orders: the arrival on a short
// final first, then the departure; the arrival far out (declaredFarNM),
// the departure first; the arrival asked for with the departure airborne
// already. Neither jumps.
func TestDeclaredField(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	t.Run("arrivalFirst", func(t *testing.T) { declaredRun(t, declaredCase{}) })
	t.Run("departureFirst", func(t *testing.T) { declaredRun(t, declaredCase{spawnNm: declaredFarNM}) })
	t.Run("departureAirborne", func(t *testing.T) { declaredRun(t, declaredCase{spawnNm: declaredFarNM, airborne: true}) })
}

// TestDeclaredFieldMirrored is TestDeclaredField the other way round: off
// 27, landing on 09, the wind down 09.
func TestDeclaredFieldMirrored(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	t.Run("arrivalFirst", func(t *testing.T) { declaredRun(t, declaredCase{mirror: true}) })
	t.Run("departureFirst", func(t *testing.T) { declaredRun(t, declaredCase{spawnNm: declaredFarNM, mirror: true}) })
	t.Run("departureAirborne", func(t *testing.T) { declaredRun(t, declaredCase{spawnNm: declaredFarNM, airborne: true, mirror: true}) })
}

// TestDeclaredFieldTypes is TestDeclaredField's opposite directions with
// an airliner (declaredJet) departing, both ways round, the arrival a
// DHC-6: faster out, kept as far apart; and an airliner landing first,
// the departure a DHC-6, down to its exit speed by the landing distance
// (#1718: an A320 rolled past XDEC's 1500 m) and off by D, not B where the
// departure holds short (#1719, TestDeclaredFieldExit).
func TestDeclaredFieldTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for _, mirror := range []bool{false, true} {
		way := "09"
		if mirror {
			way = "27"
		}
		t.Run(way+"/jetOut", func(t *testing.T) {
			declaredRun(t, declaredCase{spawnNm: declaredFarNM, mirror: mirror, dep: declaredJet})
		})
		t.Run(way+"/jetOutAirborne", func(t *testing.T) {
			declaredRun(t, declaredCase{spawnNm: declaredFarNM, airborne: true, mirror: mirror, dep: declaredJet})
		})
		t.Run(way+"/jetIn", func(t *testing.T) {
			declaredRun(t, declaredCase{mirror: mirror, arr: declaredJet})
		})
	}
}

// declaredRun is a case of TestDeclaredField.
func declaredRun(t *testing.T, k declaredCase) {
	depFirst := k.spawnNm > 0
	depModel, arrModel := k.dep, k.arr
	if depModel == "" {
		depModel = declaredTitle
	}
	if arrModel == "" {
		arrModel = declaredTitle
	}
	depName, arrName, windFrom := "09", "27", 270.0
	if k.mirror {
		depName, arrName, windFrom = "27", "09", 90
	}
	log := &lockedBuf{}
	o := Options{DataDir: declaredFieldDir(t, k.mirror), ICAO: declaredICAO, Manual: true, FrameRate: 10,
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
	if eArr.ForTakeoff() || eDep.TakeoffStartM != declaredStartM || eArr.LDAM != declaredLDAM {
		t.Fatalf("declared values not read: %s %s, %s start %.0f m, %s LDA %.0f m", arrName, eArr.Use, depName, eDep.TakeoffStartM, arrName, eArr.LDAM)
	}
	wx := nav.StaticWeather(windFrom, 3, 9999, 15, 5, 1013)
	h.World.SetWeather(&wx)
	h.Advance(2 * time.Second)
	jumps := len(h.Sim.Jumps())

	const dep, arr = "XDD1", "XDA1"
	arrival := world.SpawnRequest{Kind: "arrival", ICAO: declaredICAO, Tail: arr, Stand: 2, Model: arrModel, InjectApproach: true, SpawnNm: k.spawnNm}
	requests := []world.SpawnRequest{{Kind: "departure", ICAO: declaredICAO, Tail: dep, Stand: 0, Model: depModel}}
	if !k.airborne {
		requests = append(requests, arrival)
	}
	for _, r := range requests {
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
		rollFromArr    bool
		depOn, cleared bool
		arrIn          string
		overlap        string
		airNM, airFt   = math.Inf(1), 0.0
		airAt          string
		depAt          Object
		rolledAt       time.Time // the departure's roll began
		pastEndAt      time.Time // the departure airborne past the far end
		landedAt       time.Time // the arrival down on its end
		asked          = !k.airborne
		askedAt        time.Time // the arrival last asked for (airborne)
		held           []string  // why it was not let in yet
	)
	note := func(tail, rw string) {
		if rw != "" && !slices.Contains(runways[tail], rw) {
			runways[tail] = append(runways[tail], rw)
		}
	}
	limit := 40 * time.Minute
	if k.airborne {
		limit = declaredAirborneLimit
	}
	finished := h.Until(limit, func() bool {
		now := h.Clock.Now()
		depOn, arrIn = false, ""
		if v, ob, ok := object(dep); ok {
			note(dep, v.Runway)
			pos := airport.LatLon{Lat: ob.Lat, Lon: ob.Lon}
			along, across := alongAcross(eDep.Threshold, pos, eDep.Heading)
			onRunway := ob.OnGround && math.Abs(across) < rwy.Width/2 && along > -10 && along < declaredLength+10
			if onRunway && ob.GroundKts > 35 {
				if !rolled {
					rolled, rollAlong, rolledAt = true, along, now
				}
				if math.Abs(math.Remainder(ob.Heading-eDep.Heading, 360)) > 90 {
					rollFromArr = true
				}
			}
			if onRunway && !rolled && ob.GroundKts < 1 && math.Abs(math.Remainder(ob.Heading-eDep.Heading, 360)) < 10 && math.Abs(across) < 3 {
				stoppedAlong = math.Min(stoppedAlong, along)
			}
			away = away || rolled && !ob.OnGround
			if rolled && !ob.OnGround && along >= declaredLength && pastEndAt.IsZero() {
				pastEndAt = now
			}
			// On the runway, or airborne short of its far end.
			depOn, depAt = onRunway || rolled && !ob.OnGround && along < declaredLength, ob
			// Airborne and handed on: the arrival asked for now, and again
			// while it is not let in.
			if !asked && v.State == traffic.TaxiComplete.String() && (askedAt.IsZero() || now.Sub(askedAt) >= declaredAskEvery) {
				askedAt = now
				if _, err := h.World.Do(http.MethodPost, "/api/control", arrival); err == nil {
					asked = true
				} else {
					held = append(held, fmt.Sprintf("%d s: %v (blocked %v)", int(now.Sub(start).Seconds()), err, errors.Is(err, traffic.ErrSpawnBlocked)))
				}
			}
		}
		if v, ob, ok := object(arr); ok {
			note(arr, v.Runway)
			pos := airport.LatLon{Lat: ob.Lat, Lon: ob.Lon}
			along, across := alongAcross(eArr.LandingThreshold(), pos, eArr.Heading)
			if ob.OnGround && math.Abs(across) < rwy.Width/2 && along > 0 && math.Abs(math.Remainder(ob.Heading-eArr.Heading, 360)) < 45 {
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
			thr := eArr.LandingThreshold()
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
					da, dc := alongAcross(eDep.Threshold, airport.LatLon{Lat: depAt.Lat, Lon: depAt.Lon}, eDep.Heading)
					airAt = fmt.Sprintf("at %d s, the departure %.1f NM out and %.1f NM to the side at %.0f ft, the arrival %.1f NM from its threshold at %.0f ft",
						int(now.Sub(start).Seconds()), da/1852, dc/1852, depAt.AltFt, nm, ob.AltFt)
				}
			}
		}
		if depOn && arrIn != "" && overlap == "" {
			overlap = fmt.Sprintf("the departure on %s or short of its far end with the arrival %s, at %d s", depName, arrIn, int(now.Sub(start).Seconds()))
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
	for _, tx := range h.Heard() {
		if tx.Callsign == dep && !tx.Pilot && slices.Contains([]traffic.Intent{traffic.IntentTakeoff, "after_departure", "after_departure_turn"}, tx.Intent) {
			t.Logf("said to %s: %q", dep, tx.Text)
		}
	}
	if len(held) > 0 {
		t.Logf("the arrival not let in %d times: first %s; last %s", len(held), held[0], held[len(held)-1])
	}
	if !finished {
		t.Errorf("not finished in %v simulated: departure away %v, arrival asked for %v, parked %v", limit, away, asked, parked)
	}
	if slices.Contains(runways[dep], arrName) || rollFromArr {
		t.Errorf("the departure was given or rolled on %s, landing only: %v", arrName, runways[dep])
	}
	if !rolled {
		t.Errorf("the departure never rolled on %s", depName)
	} else if tol := 10.0; rollAlong < declaredStartM-tol || stoppedAlong < declaredStartM-tol {
		t.Errorf("the departure lined up %.0f m and rolled from %.0f m in from the %s end, before the take-off start %.0f m in", stoppedAlong, rollAlong, depName, declaredStartM)
	}
	if !landed || !slices.Contains(runways[arr], arrName) {
		t.Errorf("the arrival did not land on %s: %v", arrName, runways[arr])
	}
	// The order each case is for.
	switch {
	case depFirst && (pastEndAt.IsZero() || !landedAt.IsZero() && !pastEndAt.Before(landedAt)):
		t.Errorf("the arrival spawned %.0f NM out: want the departure past the far end (%s) before the arrival lands (%s)", k.spawnNm, at(pastEndAt), at(landedAt))
	case !depFirst && (landedAt.IsZero() || !rolledAt.IsZero() && !landedAt.Before(rolledAt)):
		t.Errorf("the arrival spawned on a short final: want it down (%s) before the departure rolls (%s)", at(landedAt), at(rolledAt))
	}
	if overlap != "" {
		t.Errorf("opposite-direction traffic at once (the departure must be past the far end before the arrival is inside %.0f NM): %s", traffic.OppositeArrivalNM, overlap)
	}
	t.Logf("both airborne: closest %.1f NM, %.0f ft apart %s", airNM, airFt, airAt)
	if airNM < 3 {
		t.Errorf("both airborne %.1f NM apart (%.0f ft): want at least 3 NM horizontally, an arrival to the other end of the departure's runway, %s", airNM, airFt, airAt)
	}
	if pastLDA > 0 {
		t.Errorf("the arrival rolled at %.0f kt past %s's landing distance (%.0f m), over %.0f kt: %s", pastLDA, arrName, declaredLDAM, slowKts, pastLDAAt)
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
