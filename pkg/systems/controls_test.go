//go:build windows
// +build windows

package systems

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeControlClient struct {
	mapped map[uint32]string
	sent   []string  // "EVENT data"
	set    []float64 // values set, in order
	defs   map[uint32]string
	setVar []string
}

func (f *fakeControlClient) MapClientEventToSimEvent(id uint32, name string) error {
	f.mapped[id] = name
	return nil
}

func (f *fakeControlClient) TransmitClientEvent(_ uint32, id uint32, data uint32, _ uint32, _ types.SIMCONNECT_EVENT_FLAG) error {
	f.sent = append(f.sent, f.mapped[id]+" "+string(rune('0'+data)))
	return nil
}

func (f *fakeControlClient) AddToDataDefinition(def uint32, name, _ string, _ types.SIMCONNECT_DATATYPE, _ float32, _ uint32) error {
	f.defs[def] = name
	return nil
}

func (f *fakeControlClient) SetDataOnSimObject(def uint32, _ uint32, _ types.SIMCONNECT_DATA_SET_FLAG, _ uint32, _ uint32, data unsafe.Pointer) error {
	f.setVar = append(f.setVar, f.defs[def])
	f.set = append(f.set, *(*float64)(data))
	return nil
}

// TestControlsDefault: the standard key events, toggled only when the
// state differs; no chocks or GPU in the default (#667).
func TestControlsDefault(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c := NewControls(f, 0)
	c.Use(For(Aircraft{Title: "Asobo A320neo", ATCType: "A320"}))
	if c.Can(Chocks) || c.Can(GPU) || !c.Can(Door(0)) || !c.Can(ParkingBrake) {
		t.Fatalf("default: chocks %v gpu %v door %v brake %v", c.Can(Chocks), c.Can(GPU), c.Can(Door(0)), c.Can(ParkingBrake))
	}
	closed := State{Values: map[string]float64{Door(0): 0, ParkingBrake: 1}}
	if err := c.Set(Door(0), true, closed); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ParkingBrake, true, closed); err != nil { // set already: nothing sent
		t.Fatal(err)
	}
	if err := c.Set(ParkingBrake, false, closed); err != nil {
		t.Fatal(err)
	}
	if len(f.sent) != 2 || f.sent[0] != "TOGGLE_AIRCRAFT_EXIT 1" || f.sent[1] != "PARKING_BRAKES 0" {
		t.Errorf("sent %q, want the main exit toggled (1) and the brake toggled once", f.sent)
	}
	if err := c.Set(Chocks, true, closed); err == nil {
		t.Error("chocks on an aircraft without them: no error")
	}
}

// TestControlsFenix: the Fenix's own variables set, and its EFB (#667).
func TestControlsFenix(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	p := For(Aircraft{Package: "fnx-aircraft-320", Title: "FenixA319 CFM WF HD"})
	c := NewControls(f, 0)
	c.Use(p)
	// Chocks and GPU through its EFB API (measured: no L:var write sticks).
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Query     string
			Variables map[string]bool
		}
		json.NewDecoder(r.Body).Decode(&q)
		got = append(got, fmt.Sprintf("%s %s %v", r.URL.Path, q.Query[strings.Index(q.Query, "name: "):strings.Index(q.Query, ", value")], q.Variables["v"]))
		fmt.Fprint(w, `{"data":{"dataRef":{"writeBool":true}}}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	p.EFB = &EFB{Port: port}
	c.Use(p)
	c.SetEFBHost(u.Hostname())
	now := State{Values: map[string]float64{Chocks: 1, GPU: 1, ParkingBrake: 0}}
	for _, step := range []struct {
		name string
		on   bool
	}{{Chocks, false}, {GPU, false}, {ParkingBrake, true}} {
		if err := c.Set(step.name, step.on, now); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{`/graphql name: "fenix.efb.chocks" false`, `/graphql name: "groundservice.groundpower" false`}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("EFB writes %q, want %q", got, want)
	}
	// The parking brake: the default key event (measured on the Fenix).
	if len(f.sent) != 1 || f.sent[0] != "PARKING_BRAKES 1" {
		t.Errorf("sent %q, want the parking brake toggled", f.sent)
	}
	if p := For(Aircraft{Package: "fnx-aircraft-320"}); p.EFB == nil || p.EFB.Port != 8083 {
		t.Errorf("EFB %+v, want port 8083", p.EFB)
	}
	if Default().EFB != nil {
		t.Error("the default has an EFB")
	}
	s := resolveState(p, map[varUnit]float64{{"L:B_CONFIG_CHOCKS", "number"}: 1})
	if !s.HasChocks || !s.Chocks || !s.HasGPU || s.GPU {
		t.Errorf("state chocks %v/%v gpu %v/%v", s.HasChocks, s.Chocks, s.HasGPU, s.GPU)
	}
}

// TestGroundServices: the sim's ground services requested by name, each its
// standard key event once; the pushback state read (#666).
func TestGroundServices(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c := NewControls(f, 0)
	c.Use(For(Aircraft{Title: "Asobo A320neo"}))
	want := map[string]string{Jetway: "TOGGLE_JETWAY", Stairs: "TOGGLE_RAMPTRUCK", Baggage: "REQUEST_LUGGAGE",
		Catering: "REQUEST_CATERING", PowerSupply: "REQUEST_POWER_SUPPLY", FuelTruck: "REQUEST_FUEL_KEY", Pushback: "TOGGLE_PUSHBACK"}
	for _, name := range []string{Jetway, Stairs, Baggage, Catering, PowerSupply, FuelTruck, Pushback} {
		if !c.Can(name) {
			t.Fatalf("%s: not available", name)
		}
		f.sent = nil
		if err := c.Request(name); err != nil {
			t.Fatal(err)
		}
		if len(f.sent) != 1 || !strings.HasPrefix(f.sent[0], want[name]+" ") {
			t.Errorf("%s: sent %q, want %s once", name, f.sent, want[name])
		}
	}
	s := resolveState(Default(), map[varUnit]float64{{"PUSHBACK AVAILABLE", "bool"}: 1})
	if !s.PushbackAvailable || s.PushbackAttached || s.PushbackWait {
		t.Errorf("pushback state %v %v %v", s.PushbackAvailable, s.PushbackAttached, s.PushbackWait)
	}
}

// TestDoors: a profile's doors by name and exit; the Fenix's passenger
// doors on the exits measured (L1 1, L2 4, R1 5, R2 8), the default's 4
// by position (#700).
func TestDoors(t *testing.T) {
	fx := For(Aircraft{Package: "fnx-aircraft-319"})
	if strings.Join(fx.Doors, ",") != "L1,L2,R1,R2,FWD cargo,AFT cargo" {
		t.Fatalf("Fenix doors %v", fx.Doors)
	}
	for n, exit := range []uint32{1, 4, 5, 8} {
		if v := fx.Values[Door(n)]; len(v.Vars) != 1 || v.Vars[0] != fmt.Sprintf("EXIT OPEN:%d", exit-1) {
			t.Errorf("%s value %+v", fx.Doors[n], v)
		}
		if a := fx.Actions[Door(n)]; a.Event != "TOGGLE_AIRCRAFT_EXIT" || a.Data == nil || *a.Data != exit {
			t.Errorf("%s action %+v", fx.Doors[n], a)
		}
	}
	// The cargo doors: read on exits 9 and 10, moved through the EFB.
	if v := fx.Values[Door(4)]; len(v.Vars) != 1 || v.Vars[0] != "EXIT OPEN:8" || fx.Actions[Door(4)].EFB != "doors.cargo.forward" {
		t.Errorf("FWD cargo %+v %+v", v, fx.Actions[Door(4)])
	}
	if v := fx.Values[Door(5)]; len(v.Vars) != 1 || v.Vars[0] != "EXIT OPEN:9" || fx.Actions[Door(5)].EFB != "doors.cargo.aft" {
		t.Errorf("AFT cargo %+v %+v", v, fx.Actions[Door(5)])
	}
	if _, ok := fx.Values[Door(6)]; ok {
		t.Error("a seventh Fenix door")
	}
	// The real L2 open (exit 4): L2 reads open, nothing else.
	s := resolveState(fx, map[varUnit]float64{{"EXIT OPEN:3", "percent"}: 100, {"EXIT OPEN:1", "percent"}: 100})
	if len(s.DoorsOpen) != 6 || !s.DoorsOpen[1] || s.DoorsOpen[0] || s.DoorsOpen[2] || s.DoorNames[1] != "L2" {
		t.Errorf("state doors %v %v", s.DoorsOpen, s.DoorNames)
	}
	// Names alone still work: by position.
	var p Profile
	if err := json.Unmarshal([]byte(`{"name":"x","doors":["A","B"]}`), &p); err != nil || len(p.Doors) != 2 || len(p.Exits) != 0 {
		t.Errorf("names: %+v %v", p, err)
	}
	def := For(Aircraft{Title: "Asobo A320neo"})
	c := NewControls(&fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}, 0)
	c.Use(def)
	if len(def.Doors) != 4 || c.Can(Door(4)) || !c.Can(Door(3)) {
		t.Errorf("default doors %v, door 5 %v", def.Doors, c.Can(Door(4)))
	}
}

// TestTypeBase: the A320 family base on the standard variables for a
// stock A320; the Fenix on top of it where it differs; an aircraft of
// another type keeps the default (#759).
func TestTypeBase(t *testing.T) {
	stock := For(Aircraft{Title: "Airbus A320 Neo Asobo", ATCType: "A320"})
	if v := stock.Values[Seatbelts]; len(v.Vars) != 1 || v.Vars[0] != "CABIN SEATBELTS ALERT SWITCH" || stock.Actions[ExtPower].Event != "TOGGLE_EXTERNAL_POWER" {
		t.Errorf("stock A320 %+v %+v", v, stock.Actions[ExtPower])
	}
	fx := For(Aircraft{Package: "fnx-aircraft-319"})
	if fx.Name != "Fenix A320 family" || fx.Values[Seatbelts].Vars[0] != "CABIN SEATBELTS ALERT SWITCH" || fx.Actions[Seatbelts].Set != "L:S_OH_SIGNS" ||
		fx.Values[NoSmoking].Vars[0] != "L:S_OH_SIGNS_SMOKING" || fx.Actions[CabinCall].Counter != "L:S_OH_CALLS_ALL" || fx.Values[CabinCall+"Counter"].Vars[0] != "L:S_OH_CALLS_ALL" {
		t.Errorf("Fenix on the base: %+v", fx)
	}
	other := For(Aircraft{Title: "Cessna 172", ATCType: "C172"})
	if _, ok := other.Values[Seatbelts]; ok {
		t.Error("a Cessna got the A320 base")
	}
}

// TestCountedButtons: a counted button pressed from its count (+1, +2);
// one stopped half way goes on from the next even count; a no smoking
// sign set to 2 (#759).
func TestCountedButtons(t *testing.T) {
	fx := For(Aircraft{Package: "fnx-aircraft-319"})
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c := NewControls(f, 0)
	c.Use(fx)
	if err := c.Press(CabinCall, State{Values: map[string]float64{CabinCall + "Counter": 4}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Press(CabinCall, State{Values: map[string]float64{CabinCall + "Counter": 7}}); err != nil {
		t.Fatal(err)
	}
	// External power: pressed only when it is not as wanted.
	if err := c.Set(ExtPower, true, State{Values: map[string]float64{ExtPower: 1, ExtPower + "Counter": 0}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(ExtPower, false, State{Values: map[string]float64{ExtPower: 1, ExtPower + "Counter": 2}}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetValue(NoSmoking, 2, State{}); err != nil {
		t.Fatal(err)
	}
	want := "L:S_OH_CALLS_ALL=5 L:S_OH_CALLS_ALL=6 L:S_OH_CALLS_ALL=9 L:S_OH_CALLS_ALL=10 L:S_OH_ELEC_EXT_PWR=3 L:S_OH_ELEC_EXT_PWR=4 L:S_OH_SIGNS_SMOKING=2"
	var got []string
	for i, v := range f.set {
		got = append(got, f.setVar[i]+"="+strconv.Itoa(int(v)))
	}
	if strings.Join(got, " ") != want {
		t.Errorf("writes\n%s\nwant\n%s", strings.Join(got, " "), want)
	}
}

// Copilot callout values from the default SimVars: spoilers armed and the
// higher side deployed; per engine the reverser and N1.
func TestCalloutValues(t *testing.T) {
	s := resolveState(Default(), map[varUnit]float64{
		{"SPOILERS ARMED", "bool"}: 1, {"SPOILERS LEFT POSITION", "percent"}: 80, {"SPOILERS RIGHT POSITION", "percent"}: 95,
		{"GENERAL ENG REVERSE THRUST ENGAGED:2", "bool"}: 1, {"TURB ENG REVERSE NOZZLE PERCENT:2", "percent"}: 100,
		{"TURB ENG N1:1", "percent"}: 84.5, {"TURB ENG N1:2", "percent"}: 84.7,
	})
	if !s.SpoilersArmed || s.SpoilersPct != 95 {
		t.Errorf("spoilers armed %v, %.0f %%", s.SpoilersArmed, s.SpoilersPct)
	}
	if s.Reverser[0] || !s.Reverser[1] || s.ReverserPct[1] != 100 {
		t.Errorf("reversers %v, %v", s.Reverser, s.ReverserPct)
	}
	if s.N1[0] != 84.5 || s.N1[1] != 84.7 || s.N1[2] != 0 {
		t.Errorf("N1 %v", s.N1)
	}
}

// TestLandingLights: the default sends LANDING_LIGHTS_ON and _OFF; the
// Fenix sets its switch to 2 (on) and 1 (off).
func TestLandingLights(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c := NewControls(f, 0)
	c.Use(For(Aircraft{Title: "Asobo A320neo", ATCType: "A320"}))
	if err := c.Set(LightLanding, true, State{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(LightLanding, false, State{}); err != nil {
		t.Fatal(err)
	}
	if len(f.sent) != 2 || !strings.HasPrefix(f.sent[0], "LANDING_LIGHTS_ON") || !strings.HasPrefix(f.sent[1], "LANDING_LIGHTS_OFF") {
		t.Errorf("default sent %q", f.sent)
	}
	f = &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c = NewControls(f, 0)
	c.Use(For(Aircraft{Package: "fnx-aircraft-320", Title: "FenixA319 CFM WF HD"}))
	c.Set(LightLanding, true, State{})
	c.Set(LightLanding, false, State{})
	if len(f.set) != 2 || f.set[0] != 2 || f.set[1] != 1 || !strings.Contains(f.setVar[0], "S_OH_EXT_LT_LANDING_BOTH") {
		t.Errorf("Fenix set %v on %v", f.set, f.setVar)
	}
}

// TestFlapsSaid: the flap lever's detent as said, by FLAPS HANDLE INDEX:
// an A320 at 3 "three", a 737 at 3 "5"; an aircraft without detents "".
func TestFlapsSaid(t *testing.T) {
	for _, c := range []struct {
		a    Aircraft
		idx  float64
		want string
	}{
		{Aircraft{Title: "Asobo A320neo", ATCType: "A320"}, 3, "three"},
		{Aircraft{Title: "FenixA319 CFM", Package: "fnx-aircraft-320"}, 4, "full"},
		{Aircraft{Title: "PMDG 737-800", ATCType: "B738"}, 3, "5"},
		{Aircraft{Title: "Cessna 172", ATCType: "C172"}, 1, ""},
	} {
		s := resolveState(For(c.a), map[varUnit]float64{{"FLAPS HANDLE INDEX", "number"}: c.idx})
		if s.FlapsSaid != c.want {
			t.Errorf("%s at %v: %q, want %q", c.a.Title, c.idx, s.FlapsSaid, c.want)
		}
	}
}

// TestPressValueAndEncoder: a knob pushed (+1) for on and pulled (−1) for
// off on one variable, each released to 0; a relative knob turned by the
// clicks from the value shown, from its count; a dashed display woken by
// one click first.
func TestPressValueAndEncoder(t *testing.T) {
	f := &fakeControlClient{mapped: map[uint32]string{}, defs: map[uint32]string{}}
	c := NewControls(f, 0)
	push, pull := 1.0, -1.0
	c.Use(Profile{Actions: map[string]Action{
		APSpeedManaged: {Press: "L:S_FCU_SPEED", On: &push, Off: &pull},
		APSpeedSel:     {Encoder: "L:E_FCU_SPEED", Display: "fcuSpeed", Step: 1},
		APHeadingSel:   {Encoder: "L:E_FCU_HEADING", Display: "fcuHeading", Step: 1, Wake: true},
	}})
	if err := c.Set(APSpeedManaged, true, State{Values: map[string]float64{APSpeedManaged: 0}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(APSpeedManaged, false, State{Values: map[string]float64{APSpeedManaged: 1}}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(f.set) != "[1 0 -1 0]" {
		t.Errorf("push and pull wrote %v, want [1 0 -1 0]", f.set)
	}
	f.set = nil
	st := State{Values: map[string]float64{"fcuSpeed": 250, APSpeedSel + "Encoder": 37}}
	if err := c.SetValue(APSpeedSel, 230, st); err != nil {
		t.Fatal(err)
	}
	if len(f.set) != 1 || f.set[0] != 17 || f.setVar[len(f.setVar)-1] != "L:E_FCU_SPEED" {
		t.Errorf("speed 250 → 230 from count 37: wrote %v to %v", f.set, f.setVar)
	}
	f.set = nil
	dashed := State{Values: map[string]float64{"fcuHeading": 0, APHeadingSel + "Encoder": 5}}
	if err := c.SetValue(APHeadingSel, 240, dashed); err != ErrEncoderWoken || len(f.set) != 1 || f.set[0] != 6 {
		t.Errorf("dashed heading: %v, wrote %v", err, f.set)
	}
	f.set = nil
	shown := State{Values: map[string]float64{"fcuHeading": 168, APHeadingSel + "Encoder": 6}}
	if err := c.SetValue(APHeadingSel, 240, shown); err != nil || len(f.set) != 1 || f.set[0] != 78 {
		t.Errorf("heading 168 → 240 from 6: %v, wrote %v", err, f.set)
	}
}
