//go:build windows
// +build windows

// Package gsx reads GSX Pro's state from its L:vars on the user aircraft,
// as GSX documents them (GSX_manual_MSFS.pdf, "DEVELOPERS - Interfacing
// with GSX", pp. 106–109): each service's state, passenger and cargo
// progress, the doors GSX waits for, the fuel hose, the pushback freeze,
// the gate selected, pilots and crew on board. Writing GSX's settable
// L:vars (passenger, pilot and crew counts, the doors message, remote
// control) goes through pkg/lvars.
package gsx

import (
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/systems"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Service is a GSX service's state (L:FSDT_GSX_*_STATE).
type Service int

// The states GSX documents; 0 before GSX runs.
const (
	Unknown    Service = 0
	Callable   Service = 1 // the service can be called
	NotHere    Service = 2 // not available
	Bypassed   Service = 3
	Requested  Service = 4
	Performing Service = 5
	Completed  Service = 6
)

var serviceNames = [...]string{"unknown", "callable", "not available", "bypassed", "requested", "performing", "completed"}

func (s Service) String() string {
	if s < 0 || int(s) >= len(serviceNames) {
		return "unknown"
	}
	return serviceNames[s]
}

// State is GSX's state.
type State struct {
	// Running: GSX is running (its services have a state).
	Running bool
	// The services.
	Boarding, Deboarding, Catering, Refueling, Departure, Deice Service
	// Passengers: the number GSX boards (FSDT_GSX_NUMPASSENGERS);
	// boarded and deboarded so far on this bus and in all, the
	// SimBrief maximum.
	Passengers                                      int
	PassengersBoarding, PassengersBoardingTotal     int
	PassengersDeboarding, PassengersDeboardingTotal int
	MaxPassengers                                   int
	// Cargo: loading or unloading now, and how far (0–100 %).
	LoadingCargo, UnloadingCargo     bool
	CargoLoadedPct, CargoUnloadedPct float64
	// WaitingFor: the doors GSX waits for you to open or close ("exit 1",
	// "service 2", "cargo 1", "main cargo").
	WaitingFor []string
	// Refuelling: the hydrant hose connected, the fuel counter and its
	// maximum for this truck.
	FuelHose                    bool
	FuelCounter, FuelCounterMax float64
	// Frozen: the pushback has frozen the aircraft (FSDT_VAR_Frozen);
	// BypassPin: the bypass pin is in.
	Frozen, BypassPin bool
	// PilotsOnBoard, CrewOnBoard as GSX considers them.
	PilotsOnBoard, CrewOnBoard bool
	// Gate is the parking selected in GSX ("C19"); "" before one is.
	Gate string
	// DeiceFluid is the de-icing fluid type asked for (1–4; 0 none).
	DeiceFluid int
}

// doors are the door variables GSX sets while waiting for them, by name.
var doors = []struct{ v, name string }{
	{"L:FSDT_GSX_AIRCRAFT_EXIT_1_TOGGLE", "exit 1"}, {"L:FSDT_GSX_AIRCRAFT_EXIT_2_TOGGLE", "exit 2"},
	{"L:FSDT_GSX_AIRCRAFT_EXIT_3_TOGGLE", "exit 3"}, {"L:FSDT_GSX_AIRCRAFT_EXIT_4_TOGGLE", "exit 4"},
	{"L:FSDT_GSX_AIRCRAFT_SERVICE_1_TOGGLE", "service 1"}, {"L:FSDT_GSX_AIRCRAFT_SERVICE_2_TOGGLE", "service 2"},
	{"L:FSDT_GSX_AIRCRAFT_CARGO_1_TOGGLE", "cargo 1"}, {"L:FSDT_GSX_AIRCRAFT_CARGO_2_TOGGLE", "cargo 2"},
	{"L:FSDT_GSX_AIRCRAFT_MAINCARGO_TOGGLE", "main cargo"},
}

// vars are every L:var read.
var vars = []string{
	"L:FSDT_GSX_BOARDING_STATE", "L:FSDT_GSX_DEBOARDING_STATE", "L:FSDT_GSX_CATERING_STATE",
	"L:FSDT_GSX_REFUELING_STATE", "L:FSDT_GSX_DEPARTURE_STATE", "L:FSDT_GSX_DEICE_STATE",
	"L:FSDT_GSX_NUMPASSENGERS", "L:FSDT_GSX_NUMPASSENGERS_BOARDING", "L:FSDT_GSX_NUMPASSENGERS_BOARDING_TOTAL",
	"L:FSDT_GSX_NUMPASSENGERS_DEBOARDING", "L:FSDT_GSX_NUMPASSENGERS_DEBOARDING_TOTAL", "L:FSDT_GSX_MAX_NUMPASSENGERS",
	"L:FSDT_GSX_BOARDING_CARGO", "L:FSDT_GSX_DEBOARDING_CARGO",
	"L:FSDT_GSX_BOARDING_CARGO_PERCENT", "L:FSDT_GSX_DEBOARDING_CARGO_PERCENT",
	"L:FSDT_GSX_FUELHOSE_CONNECTED", "L:FSDT_GSX_FUEL_COUNTER", "L:FSDT_GSX_FUEL_COUNTER_MAX",
	"L:FSDT_VAR_Frozen", "L:FSDT_GSX_BYPASS_PIN",
	"L:FSDT_GSX_PILOTS_ON_BOARD", "L:FSDT_GSX_CREW_ON_BOARD",
	"L:FSDT_GSX_SetGate_Name", "L:FSDT_GSX_SetGate_Number", "L:FSDT_GSX_SetGate_Suffix",
	"L:FSDT_GSX_DEICING_TYPE",
}

// Profile is the GSX variables as a systems profile (each value named by
// its variable), for a systems.Reader.
func Profile() systems.Profile {
	p := systems.Profile{Name: "GSX", Values: map[string]systems.Value{}}
	for _, v := range vars {
		p.Values[v] = systems.Value{Vars: []string{v}}
	}
	for _, d := range doors {
		p.Values[d.v] = systems.Value{Vars: []string{d.v}}
	}
	return p
}

// Reader reads GSX's state: Request (once or every period), Handle each
// message.
type Reader struct{ r *systems.Reader }

// NewReader is a Reader on client with its definition and request IDs.
func NewReader(client systems.Client, defID, reqID uint32) *Reader {
	r := systems.NewReader(client, defID, reqID)
	r.Use(Profile())
	return &Reader{r: r}
}

// Request asks for GSX's state every period, or once.
func (g *Reader) Request(period types.SIMCONNECT_PERIOD) error { return g.r.Request(period) }

// Reset forgets the registration (a new connection), and takes client when
// not nil.
func (g *Reader) Reset(client systems.Client) { g.r.Reset(client) }

// Handle takes GSX's state from msg when it is the Reader's.
func (g *Reader) Handle(msg engine.Message) (State, bool) {
	s, ok := g.r.Handle(msg)
	if !ok {
		return State{}, false
	}
	return Read(s.Values), true
}

// Read is GSX's state from its variables' values, by variable name.
func Read(v map[string]float64) State {
	i := func(k string) int { return int(v[k]) }
	on := func(k string) bool { return v[k] != 0 }
	svc := func(k string) Service { return Service(i(k)) }
	s := State{
		Boarding: svc("L:FSDT_GSX_BOARDING_STATE"), Deboarding: svc("L:FSDT_GSX_DEBOARDING_STATE"),
		Catering: svc("L:FSDT_GSX_CATERING_STATE"), Refueling: svc("L:FSDT_GSX_REFUELING_STATE"),
		Departure: svc("L:FSDT_GSX_DEPARTURE_STATE"), Deice: svc("L:FSDT_GSX_DEICE_STATE"),
		Passengers:                i("L:FSDT_GSX_NUMPASSENGERS"),
		PassengersBoarding:        i("L:FSDT_GSX_NUMPASSENGERS_BOARDING"),
		PassengersBoardingTotal:   i("L:FSDT_GSX_NUMPASSENGERS_BOARDING_TOTAL"),
		PassengersDeboarding:      i("L:FSDT_GSX_NUMPASSENGERS_DEBOARDING"),
		PassengersDeboardingTotal: i("L:FSDT_GSX_NUMPASSENGERS_DEBOARDING_TOTAL"),
		MaxPassengers:             i("L:FSDT_GSX_MAX_NUMPASSENGERS"),
		LoadingCargo:              on("L:FSDT_GSX_BOARDING_CARGO"),
		UnloadingCargo:            on("L:FSDT_GSX_DEBOARDING_CARGO"),
		CargoLoadedPct:            v["L:FSDT_GSX_BOARDING_CARGO_PERCENT"],
		CargoUnloadedPct:          v["L:FSDT_GSX_DEBOARDING_CARGO_PERCENT"],
		FuelHose:                  on("L:FSDT_GSX_FUELHOSE_CONNECTED"),
		FuelCounter:               v["L:FSDT_GSX_FUEL_COUNTER"],
		FuelCounterMax:            v["L:FSDT_GSX_FUEL_COUNTER_MAX"],
		Frozen:                    on("L:FSDT_VAR_Frozen"),
		BypassPin:                 on("L:FSDT_GSX_BYPASS_PIN"),
		PilotsOnBoard:             on("L:FSDT_GSX_PILOTS_ON_BOARD"),
		CrewOnBoard:               on("L:FSDT_GSX_CREW_ON_BOARD"),
		DeiceFluid:                i("L:FSDT_GSX_DEICING_TYPE"),
	}
	s.Running = s.Boarding != Unknown || s.Departure != Unknown || s.Refueling != Unknown
	for _, d := range doors {
		if on(d.v) {
			s.WaitingFor = append(s.WaitingFor, d.name)
		}
	}
	// The gate: the SDK's parking name enum, its number and suffix; -1
	// before one is selected (GSX's manual).
	if name := v["L:FSDT_GSX_SetGate_Name"]; name > 0 && v["L:FSDT_GSX_SetGate_Number"] >= 0 {
		suffix := v["L:FSDT_GSX_SetGate_Suffix"]
		if suffix < 0 {
			suffix = 0
		}
		s.Gate = airport.Parking{Name: types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME(name),
			Number: uint32(v["L:FSDT_GSX_SetGate_Number"]), Suffix: types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME(suffix)}.Label()
	}
	return s
}

// Settable L:vars GSX documents for add-ons to write (pkg/lvars): the
// passengers, pilots and crew to board (before boarding; 0 lets GSX
// decide), keeping pilots or crew on board, no "waiting for your action"
// messages, the toolbar under remote control.
const (
	SetPassengers       = "FSDT_GSX_NUMPASSENGERS"
	SetPilots           = "FSDT_GSX_NUMPILOTS"
	SetCrew             = "FSDT_GSX_NUMCREW"
	PilotsNotBoarding   = "FSDT_GSX_PILOTS_NOT_BOARDING"
	CrewNotBoarding     = "FSDT_GSX_CREW_NOT_BOARDING"
	PilotsNotDeboarding = "FSDT_GSX_PILOTS_NOT_DEBOARDING"
	CrewNotDeboarding   = "FSDT_GSX_CREW_NOT_DEBOARDING"
	DisableDoorsMessage = "FSDT_GSX_DISABLE_DOORS_MSG"
	RemoteControl       = "FSDT_GSX_SET_REMOTECONTROL"
)
