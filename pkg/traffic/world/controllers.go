package world

import (
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// departureCtl is what the World asks of a departure's controller
// (traffic.TaxiController): the line between the decisions and the
// simulator side, where a remote actuator's proxy can stand in (#710).
type departureCtl interface {
	SetPushbackAt(at time.Time) bool
	ClearPushback()
	FacesOut() bool
	ClearToTaxi()
	ClearUpTo(node airport.NodeID) error
	Cancel() error
	HoldPosition() error
	AbortTakeoff() error
	ChangeEntry(entry string) error
	ChangeRunway(runway, entry string, departure []airport.NavPoint) error
	ClearForTakeoff() error
	ClearPushbackFacing(dir string) error
	ClearStartUp()
	ClearToCross()
	ClearToLineUp()
	ClimbPlan(pos airport.LatLon) []traffic.RoutePoint
	ClimbRoute(pos airport.LatLon) []airport.LatLon
	DirectTo(pos airport.LatLon, altFt, kts float64, fix airport.LatLon) error
	Expedite(on bool)
	Handle(msg engine.Message) bool
	HoldPushback(on bool)
	PushFacingSaid() string
	Reroute(route []traffic.RoutePoint) error
	Route() *airport.Route
	State() traffic.TaxiState
}

// arrivalCtl is what the World asks of an arrival's controller
// (traffic.ArrivalController), as departureCtl.
type arrivalCtl interface {
	AbsorbDelay(delay time.Duration) (traffic.Absorption, error)
	AnotherCircuit() (time.Duration, error)
	Cancel() error
	ChangeRunway(runway string, procedure, missed []airport.NavPoint) error
	ChangeStand(parking int) error
	CircuitFixes() []airport.NavPoint
	ClearToCross()
	ClearToTaxi()
	ClearUpTo(node airport.NodeID) error
	DirectToJoin() error
	DirectTo(p airport.LatLon) (string, traffic.Vector, error)
	JoinFinal(p airport.LatLon) (float64, traffic.Vector, error)
	AssignSpeed(kts float64) (float64, error)
	EnterHold(h traffic.Hold, altFt float64) (traffic.HoldEntry, error)
	Expedite(on bool)
	GoAround() error
	Handle(msg engine.Message) bool
	HoldAltitude(altFt float64) error
	HoldFix(minFromThresholdNM float64) (traffic.Hold, bool)
	Holding() (traffic.Hold, float64, bool)
	HoldPosition() error
	InterceptHeading() (float64, bool)
	LeaveHold() error
	Orbit() (time.Duration, error)
	Plan() *traffic.ArrivalPlan
	ProcedurePlan() []traffic.RoutePoint
	ProcedureRoute() []airport.LatLon
	ProcedureCorners() []airport.LatLon
	ReduceToFinalSpeed() (time.Duration, error)
	Shortcut(maxSaveNM float64, keep []string) (string, float64, error)
	State() traffic.ArrivalState
	StopDescent(altFt, forNM float64) error
	TouchAndGosLeft() int
	TurningFinal() bool
	VectorDue() (traffic.Vector, bool)
}

var (
	_ departureCtl = (*traffic.TaxiController)(nil)
	_ arrivalCtl   = (*traffic.ArrivalController)(nil)
	_              = time.Second
	_ engine.Message
)
