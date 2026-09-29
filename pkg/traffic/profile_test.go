//go:build windows
// +build windows

package traffic

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func TestProfileForTitles(t *testing.T) {
	for model, want := range map[string]string{
		"FSLTL A320 Air France SL":         "A320",
		"FSLTL A20N Wizz Air":              "A20N",
		"Asobo A320 Neo":                   "A20N",
		"FSLTL_A21N_BAW-British Airways":   "A21N",
		"FSLTL A321 Lufthansa":             "A321",
		"FSLTL A319 easyJet":               "A319",
		"FSLTL B77W Emirates":              "B77W",
		"Asobo PassiveAircraft B777-300ER": "B77W",
		"FSLTL B772 British Airways":       "B772",
		"FSLTL B789 Qatar":                 "B789",
		"Asobo PassiveAircraft B787-09":    "B789",
		"FSLTL B788 LOT":                   "B788",
		"FSLTL A333 Delta":                 "A333",
		"FSLTL A359 Qatar":                 "A359",
		"AIB_B738_BAW-British Airways":     "B738",
		"FSLTL B38M Smartwings":            "B38M",
		"Asobo PassiveAircraft B737-Max8":  "B38M",
		"Asobo PassiveAircraft B737-900ER": "B739",
		"FSLTL E190 KLM Cityhopper":        "E190",
		"FSLTL E195 Lufthansa CityLine":    "E195",
		"FSLTL AT76 CSA":                   "AT76",
		"FSLTL DH8D Eurowings":             "DH8D",
		"FSLTL CRJ9 Lufthansa":             "CRJ9",
		"ATCCOM.AC_MODEL_A20N.0.text":      "A20N",
		"b38m":                             "B38M",
		"Something unknown":                "",
	} {
		if got := ProfileFor(model).Type; got != want {
			t.Errorf("%q: %q, want %q", model, got, want)
		}
	}
}

func TestICAOCodes(t *testing.T) {
	for span, want := range map[float64]byte{0: 0, 11: 'A', 20: 'B', 24.9: 'C', 35.8: 'C', 38: 'D', 47.6: 'D', 60.1: 'E', 64.8: 'E', 68.4: 'F', 79.8: 'F'} {
		if got := ICAOCodeFor(span); got != want {
			t.Errorf("%.1f m: %q, want %q", span, got, want)
		}
	}
	for model, want := range map[string]byte{"A320": 'C', "CRJ9": 'C', "AT76": 'C', "B77W": 'E', "A359": 'E', "B789": 'E', "A388": 'F', "B748": 'F'} {
		if got := ProfileFor(model).ICAOCode; got != want {
			t.Errorf("%s: code %q, want %q", model, got, want)
		}
	}
}

// TestProfileDefaults: the A320 and unknown types are exactly the package
// defaults, so existing callers keep their behaviour.
func TestProfileDefaults(t *testing.T) {
	for _, model := range []string{"FSLTL A320 Air France SL", "A20N", "x"} {
		p := ProfileFor(model)
		if p.Motion != DefaultMotionProfile() || p.Takeoff != DefaultTakeoffProfile() || p.Approach != DefaultApproachProfile() ||
			p.Rollout != DefaultRolloutProfile() || p.NoseOffsetM != DefaultNoseOffsetMeters || p.PushbackKts != PushbackSpeedKts ||
			p.Flaps.TakeoffPct != TakeoffFlapsPct || p.Flaps.ApproachPct != ApproachFlapsPct || p.Flaps.LandingPct != 100 {
			t.Errorf("%s: %+v differs from the defaults", model, p)
		}
	}
	for _, typ := range KnownTypes() {
		p := ProfileFor(typ)
		if p.Type != typ {
			t.Errorf("%s resolves to %s", typ, p.Type)
		}
		if p.WingspanM <= 0 || p.LengthM <= p.WheelbaseM || p.NoseOffsetM <= p.WheelbaseM/2 || p.Motion.TailMeters <= 0 {
			t.Errorf("%s: airframe %+v", typ, p)
		}
		if p.Approach.TouchdownKts >= p.Approach.ApproachKts || p.Approach.FlarePitchDeg >= p.Takeoff.TailstrikePitch-TailstrikeMarginDeg+2 {
			t.Errorf("%s: approach %+v", typ, p.Approach)
		}
		if p.Flaps.TakeoffPct <= 0 || p.Flaps.ApproachPct < p.Flaps.TakeoffPct-30 || p.Flaps.LandingPct < p.Flaps.ApproachPct {
			t.Errorf("%s: flaps %+v", typ, p.Flaps)
		}
		// The take-off the mover flies is about the published distance.
		run := RequiredTakeoffRun(p.Takeoff, TakeoffConditions{}) / TakeoffRunMargin
		if run < 0.4*p.TakeoffDistanceM || run > 1.2*p.TakeoffDistanceM {
			t.Errorf("%s: take-off run %.0f m, published %.0f m", typ, run, p.TakeoffDistanceM)
		}
	}
}

func TestGenericProfile(t *testing.T) {
	if p := GenericProfile(0, ""); p != DefaultAircraftProfile() {
		t.Errorf("no span: %+v", p)
	}
	p := GenericProfile(35.8, CategoryJet)
	if p.Type != "" || p.Motion != DefaultMotionProfile() {
		t.Errorf("35.8 m jet: %+v", p)
	}
	big := GenericProfile(64, CategoryJet)
	if big.ICAOCode != 'E' || big.WheelbaseM < 25 || big.Approach.ApproachKts < 145 {
		t.Errorf("64 m jet: %+v", big)
	}
	tp := GenericProfile(25, CategoryTurboprop)
	if tp.Category != CategoryTurboprop || tp.Motion.SpanMeters != 25 || tp.Approach.ApproachKts > 120 {
		t.Errorf("25 m turboprop: %+v", tp)
	}
}

// TestRequestFillsFromAircraft: zero figures come from the aircraft
// profile (resolved from the model when Aircraft is nil); set ones stay.
func TestRequestFillsFromAircraft(t *testing.T) {
	b77w := ProfileFor("B77W")
	req := TaxiRequest{Model: "FSLTL B77W Emirates"}
	req.resolveAircraft()
	if req.Profile != b77w.Motion || req.Takeoff != b77w.Takeoff || req.NoseOffset != b77w.NoseOffsetM || req.Aircraft.Type != "B77W" {
		t.Errorf("taxi request not filled from the model: %+v", req)
	}
	mine := DefaultMotionProfile()
	mine.CruiseKts = 9
	req = TaxiRequest{Model: "FSLTL B77W Emirates", Profile: mine, NoseOffset: 12}
	req.resolveAircraft()
	if req.Profile != mine || req.NoseOffset != 12 || req.Takeoff != b77w.Takeoff {
		t.Errorf("set figures overwritten: %+v", req)
	}
	e190 := ProfileFor("E190")
	arr := ArrivalRequest{Model: "FSLTL A320 Air France SL", Aircraft: &e190}
	arr.resolveAircraft()
	if arr.Approach != e190.Approach || arr.Rollout != e190.Rollout || arr.Profile != e190.Motion || arr.NoseOffset != e190.NoseOffsetM {
		t.Errorf("arrival request not filled from Aircraft: %+v", arr)
	}
	arr = ArrivalRequest{Model: "x"}
	arr.resolveAircraft()
	if arr.Approach != DefaultApproachProfile() || arr.Rollout != DefaultRolloutProfile() || arr.NoseOffset != DefaultNoseOffsetMeters || arr.Aircraft.Flaps.LandingPct != 100 {
		t.Errorf("unknown model: %+v", arr)
	}
	// A partial Aircraft is completed from the defaults.
	arr = ArrivalRequest{Model: "x", Aircraft: &AircraftProfile{Type: "ZZZZ", WingspanM: 30}}
	arr.resolveAircraft()
	if arr.Profile != DefaultMotionProfile() || arr.Aircraft.PushbackKts != PushbackSpeedKts || arr.Aircraft.Flaps.ApproachPct != ApproachFlapsPct {
		t.Errorf("partial profile: %+v %+v", arr, arr.Aircraft)
	}
}

func TestRefine(t *testing.T) {
	// A known type keeps its speeds at the reference weight.
	a320 := ProfileFor("A320")
	same := Refine(a320, SimVarData{MaxGrossKg: 78000, TotalWeightKg: 78000 * refLandingWeight, FlapPositions: 5})
	if same.Approach.ApproachKts != a320.Approach.ApproachKts || same.Flaps != a320.Flaps {
		t.Errorf("reference weight changed the profile: %+v", same.Approach)
	}
	light := Refine(a320, SimVarData{MaxGrossKg: 78000, TotalWeightKg: 50000})
	heavy := Refine(a320, SimVarData{MaxGrossKg: 78000, TotalWeightKg: 78000})
	if !(light.Approach.ApproachKts < a320.Approach.ApproachKts && heavy.Approach.ApproachKts > a320.Approach.ApproachKts) {
		t.Errorf("approach speed light %.0f, heavy %.0f, reference %.0f", light.Approach.ApproachKts, heavy.Approach.ApproachKts, a320.Approach.ApproachKts)
	}
	if !(light.Takeoff.RotateKts < heavy.Takeoff.RotateKts && light.Takeoff.RollAccel > heavy.Takeoff.RollAccel) {
		t.Errorf("take-off light %+v heavy %+v", light.Takeoff, heavy.Takeoff)
	}
	// Speeds scale within limits.
	if empty := Refine(a320, SimVarData{MaxGrossKg: 78000, TotalWeightKg: 10000}); empty.Approach.ApproachKts < 0.87*a320.Approach.ApproachKts {
		t.Errorf("empty aircraft approach %.0f kt", empty.Approach.ApproachKts)
	}
	// A generic profile takes the sim's size, engines and design speeds.
	g := Refine(DefaultAircraftProfile(), SimVarData{WingspanM: 27, EngineType: EngineTurboprop, VS0Kts: 85, TakeoffKts: 110, ClimbKts: 130, CGHeightM: 2.5, FlapPositions: 3})
	if g.Category != CategoryTurboprop || g.WingspanM != 27 || g.Approach.ApproachKts != 116 || g.Takeoff.RotateKts != 110 || g.Takeoff.ClimbKts != 130 || g.CGHeightM != 2.5 {
		t.Errorf("generic refined: %+v", g)
	}
	// A generic profile's flaps snap to the reported detents; a known
	// type keeps its schedule.
	gen := DefaultAircraftProfile()
	gen.Flaps = ProfileFor("B738").Flaps
	f := Refine(gen, SimVarData{FlapPositions: 5})
	if f.Flaps.TakeoffPct != 50 || f.Flaps.ApproachPct != 75 || f.Flaps.LandingPct != 100 { // 37.5, 62.5, 87.5 of 0 25 50 75 100
		t.Errorf("flaps snapped to %+v", f.Flaps)
	}
	if k := Refine(ProfileFor("A320"), SimVarData{FlapPositions: 4}); k.Flaps != a320.Flaps {
		t.Errorf("known type's flaps snapped to %+v", k.Flaps)
	}
	// Live FSLTL A320: a total weight above the maximum is ignored.
	if bogus := Refine(a320, SimVarData{MaxGrossKg: 68039, TotalWeightKg: 87271}); bogus.Approach != a320.Approach || bogus.Takeoff != a320.Takeoff {
		t.Errorf("inconsistent weight used: %+v", bogus.Approach)
	}
	// Nothing reported: unchanged.
	if Refine(ProfileFor("B77W"), SimVarData{}) != ProfileFor("B77W") {
		t.Error("empty data changed the profile")
	}
}

func profileDataMsg(req, obj uint32, w profileWire) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(w)))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	*(*profileWire)(unsafe.Pointer(&buf[off])) = w
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

func TestProfileReader(t *testing.T) {
	fc := &fakeClient{}
	r := NewProfileReader(fc, 8400, 8401)
	if err := r.Request(77); err != nil {
		t.Fatal(err)
	}
	if err := r.Request(78); err != nil {
		t.Fatal(err)
	}
	if n := len(fc.defs[8400]); n != len(profileVars) {
		t.Fatalf("defined %d vars, want %d (once)", n, len(profileVars))
	}
	w := profileWire{SpanFt: 117.4, VS0: 107, VS1: 120, TakeoffKts: 145, ClimbKts: 250, CruiseKts: 290, Engines: 2, EngineType: 1,
		MaxGrossLb: 171961, TotalLb: 130000, FlapPositions: 5, CGFt: 12.3}
	copy(w.ATCModel[:], "A320\x00")
	copy(w.ATCType[:], "AIRBUS")
	copy(w.Category[:], "Airplane")
	if _, ok := r.Handle(profileDataMsg(9999, 77, w)); ok {
		t.Error("handled another request's data")
	}
	v, ok := r.Handle(profileDataMsg(8401, 77, w))
	if !ok || v.ObjectID != 77 || v.ATCModel != "A320" || v.ATCType != "AIRBUS" || v.Category != "Airplane" ||
		v.Engines != 2 || v.EngineType != EngineJet || v.FlapPositions != 5 || v.WingspanM < 35.7 || v.WingspanM > 35.8 || v.MaxGrossKg < 77999 || v.MaxGrossKg > 78001 {
		t.Fatalf("decoded %+v", v)
	}
	if got, ok := r.Data(77); !ok || got != v {
		t.Errorf("Data(77) = %+v, %v", got, ok)
	}
	if _, ok := r.Data(78); ok {
		t.Error("data for an object not answered yet")
	}
	if p := Refine(ProfileFor(v.ATCModel), v); p.Type != "A320" {
		t.Errorf("refined %+v", p)
	}
	if err := NewProfileReader(nil, 1, 2).Request(0); !errors.Is(err, ErrNotConnected) {
		t.Errorf("nil client: %v", err)
	}
}

// injectedTouchdown flies a fully injected arrival at LKPR 24 until it has
// touched down and returns the touchdown event.
func injectedTouchdown(t *testing.T, model string) ArrivalEvent {
	t.Helper()
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: model, InjectApproach: true, RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	td := make(chan ArrivalEvent, 1)
	go func() {
		var got bool
		for ev := range ctl.Events() {
			if !got && ev.Touchdown != 0 {
				got = true
				td <- ev
			}
		}
		if !got {
			close(td)
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := ctl.Plan()
	for i := 0; i < 60*600 && ctl.State() < ArrivalRollout; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
	}
	if ctl.State() != ArrivalRollout {
		t.Fatalf("%s: no touchdown, %v", model, ctl.State())
	}
	ctl.Cancel()
	ev, ok := <-td
	if !ok {
		t.Fatalf("%s: no touchdown event", model)
	}
	return ev
}

// TestArrivalByType: a 777-300ER flies a faster approach with a higher
// flare than an A320 and touches down farther down the runway.
func TestArrivalByType(t *testing.T) {
	a := injectedTouchdown(t, "FSLTL A320 Air France SL")
	b := injectedTouchdown(t, "FSLTL B77W Emirates")
	t.Logf("A320 touchdown %.0f m at %.0f kt, %.0f fpm; B77W %.0f m at %.0f kt, %.0f fpm",
		a.Touchdown, a.GroundSpeed, a.TouchdownFpm, b.Touchdown, b.GroundSpeed, b.TouchdownFpm)
	if b.Touchdown <= a.Touchdown || b.GroundSpeed <= a.GroundSpeed+10 {
		t.Errorf("B77W touched down at %.0f m, %.0f kt; A320 at %.0f m, %.0f kt", b.Touchdown, b.GroundSpeed, a.Touchdown, a.GroundSpeed)
	}
	if b.TouchdownFpm > -80 || b.TouchdownFpm < -250 {
		t.Errorf("B77W touchdown %.0f fpm", b.TouchdownFpm)
	}
}

func TestRecorder(t *testing.T) {
	var buf bytes.Buffer
	rec := NewRecorder(&buf)
	rec.now = func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) }
	thr := airport.LatLon{Lat: 50.1, Lon: 14.26}
	stop := offsetHeading(thr, 90, 2000)
	info := MovementInfo{Model: "FSLTL B77W Emirates", Stop: &stop}
	for _, ev := range []ArrivalEvent{
		{State: ArrivalSpawning}, // no object yet: ignored
		{State: ArrivalApproaching, ObjectID: 5, GroundSpeed: 160},
		{State: ArrivalRollout, ObjectID: 5, GroundSpeed: 150, Touchdown: 612.34, TouchdownFpm: -131.2, OnGround: true},
		{State: ArrivalVacating, ObjectID: 5, GroundSpeed: 28.44},
		{State: ArrivalVacating, ObjectID: 5, GroundSpeed: 12},
		{State: ArrivalTaxiing, ObjectID: 5, GroundSpeed: 14},
		{State: ArrivalTaxiing, ObjectID: 5, GroundSpeed: 0.5}, // stopped: not a taxi speed
		{State: ArrivalTaxiing, ObjectID: 5, GroundSpeed: 10},
		{State: ArrivalParked, ObjectID: 5, Position: offsetHeading(stop, 0, 0.8)},
	} {
		rec.Arrival(info, ev)
	}
	start := airport.LatLon{Lat: 50.1, Lon: 14.2}
	for _, ev := range []TaxiEvent{
		{State: TaxiTaxiing, ObjectID: 6, GroundSpeed: 15},
		{State: TaxiTaxiing, ObjectID: 6, GroundSpeed: 9},
		{State: TaxiDeparting, ObjectID: 6, Position: start, OnGround: true},
		{State: TaxiDeparting, ObjectID: 6, Position: offsetHeading(start, 90, 1500), OnGround: true},
		{State: TaxiDeparting, ObjectID: 6, Position: offsetHeading(start, 90, 1700), HeightFt: 1},
		{State: TaxiDeparting, ObjectID: 6, Position: offsetHeading(start, 90, 3000), HeightFt: 300},
		{State: TaxiComplete, ObjectID: 6},
	} {
		rec.Departure(MovementInfo{Model: "FSLTL A320 Air France SL"}, ev)
	}
	rec.Departure(MovementInfo{Model: "FSLTL A320 Air France SL"}, TaxiEvent{State: TaxiFailed, ObjectID: 7, Err: errors.New("boom")})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines:\n%s", len(lines), buf.String())
	}
	var arr, dep Movement
	if err := json.Unmarshal([]byte(lines[0]), &arr); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &dep); err != nil {
		t.Fatal(err)
	}
	if arr.Kind != "arrival" || arr.Type != "B77W" || arr.Phase != "parked" || arr.TouchdownM != 612.3 || arr.TouchdownFpm != -131.2 ||
		arr.ExitKts != 28.4 || arr.StandStopErrorM != 0.8 || arr.TaxiMeanKts != 12 || arr.TaxiMaxKts != 14 {
		t.Errorf("arrival %+v", arr)
	}
	if dep.Kind != "departure" || dep.Type != "A320" || dep.Phase != "complete" || dep.LiftoffM != 1700 || dep.TaxiMeanKts != 12 || dep.TaxiMaxKts != 15 {
		t.Errorf("departure %+v", dep)
	}
	sum := rec.Summary()
	if len(sum) != 2 || sum[0].Type != "A320" || sum[0].Departures != 2 || sum[0].Failed != 1 || sum[0].LiftoffM != 1700 ||
		sum[1].Type != "B77W" || sum[1].Arrivals != 1 || sum[1].TouchdownM != 612.3 {
		t.Errorf("summary %+v", sum)
	}
	if !strings.Contains(lines[2], `"error":"boom"`) {
		t.Errorf("failed movement: %s", lines[2])
	}
}

// TestPartialFlapScheduleFilled: detents without heights get the default
// heights, so the flaps neither retract at lift-off nor stay short of full.
func TestPartialFlapScheduleFilled(t *testing.T) {
	p := aircraftOf(&AircraftProfile{Flaps: FlapSchedule{TakeoffPct: 25, ApproachPct: 75, LandingPct: 100}}, "A320")
	d := DefaultAircraftProfile().Flaps
	if p.Flaps.TakeoffPct != 25 || p.Flaps.RetractFt != d.RetractFt || p.Flaps.FullFt != d.FullFt {
		t.Errorf("flaps %+v, want the given detents with heights %v/%v", p.Flaps, d.RetractFt, d.FullFt)
	}
}
