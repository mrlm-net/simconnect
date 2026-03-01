//go:build windows
// +build windows

package traffic

import "github.com/mrlm-net/simconnect/pkg/types"

// AircraftKind identifies how an AI aircraft was created.
type AircraftKind uint8

const (
	// KindParked — created via AICreateParkedATCAircraft / EX1.
	KindParked AircraftKind = iota
	// KindEnroute — created via AICreateEnrouteATCAircraft / EX1 with a flight plan.
	KindEnroute
	// KindNonATC — created via AICreateNonATCAircraftEX1 at an explicit position.
	// These aircraft follow a waypoint chain rather than ATC instructions.
	KindNonATC
)

// ParkedOpts configures a parked ATC aircraft at an airport gate.
type ParkedOpts struct {
	Model   string // container title (e.g. "FSLTL A320 Air France SL")
	Livery  string // livery folder name; "" selects the default livery
	Tail    string // ATC tail number (e.g. "AFR123")
	Airport string // ICAO airport code (e.g. "LKPR")
}

// EnrouteOpts configures an enroute ATC aircraft along a flight plan.
type EnrouteOpts struct {
	Model        string  // container title
	Livery       string  // livery folder name; "" selects the default livery
	Tail         string  // ATC tail number
	FlightNumber uint32  // ATC flight number used for ATC comms
	FlightPlan   string  // path to a .PLN or MSFS flight plan file
	Phase        float64 // plan start offset — 0.0 = beginning, 1.0 = end
	TouchAndGo   bool    // enable touch-and-go mode on arrival
}

// NonATCOpts configures a non-ATC aircraft placed at an explicit initial position.
// Use this for aircraft that will follow a hand-built waypoint chain (e.g.
// pushback → taxi → takeoff) rather than ATC routing.
type NonATCOpts struct {
	Model    string                             // container title
	Livery   string                             // livery folder name; "" selects the default livery
	Tail     string                             // identifier visible in telemetry
	Position types.SIMCONNECT_DATA_INITPOSITION // initial position on spawn
}

// Pending records the metadata for an aircraft creation request that is still
// awaiting an ObjectID from the simulator. Created internally by Fleet.Request*
// and resolved to an Aircraft by Fleet.Acknowledge.
type Pending struct {
	ReqID  uint32
	Kind   AircraftKind
	Model  string
	Livery string
	Tail   string
}
