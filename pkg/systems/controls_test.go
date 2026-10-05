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
	if strings.Join(fx.Doors, ",") != "L1,L2,R1,R2" {
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
	if _, ok := fx.Values[Door(4)]; ok {
		t.Error("a fifth Fenix door")
	}
	// The real L2 open (exit 4): L2 reads open, nothing else.
	s := resolveState(fx, map[varUnit]float64{{"EXIT OPEN:3", "percent"}: 100, {"EXIT OPEN:1", "percent"}: 100})
	if len(s.DoorsOpen) != 4 || !s.DoorsOpen[1] || s.DoorsOpen[0] || s.DoorsOpen[2] || s.DoorNames[1] != "L2" {
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
