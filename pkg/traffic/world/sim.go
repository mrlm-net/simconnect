package world

import (
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// simPort is what the World does to the simulator beside its controllers
// (#710, option 3): said as intents, not SimConnect calls, so a remote
// actuator can stand in for localSim. All calls are made in the
// connection's goroutine (controlCenter.do).
type simPort interface {
	// SpawnEnroute creates an airborne aircraft flown by MSFS AI (NonATC);
	// its object comes back as an assigned object ID for reqID.
	SpawnEnroute(o traffic.NonATCOpts, reqID uint32) error
	// FlyEnroute takes the aircraft created for reqID as ours and sends
	// it along wps, released to MSFS AI.
	FlyEnroute(reqID, objectID uint32, wps []types.SIMCONNECT_DATA_WAYPOINT) error
	// SetRoute sends an enroute aircraft along wps from now on.
	SetRoute(objectID uint32, wps []types.SIMCONNECT_DATA_WAYPOINT) error
	// RemoveObject removes an aircraft from the simulator.
	RemoveObject(objectID, reqID uint32) error
	// ListModels and ListGroundVehicles ask for the aircraft and ground
	// vehicle titles the simulator offers; they come back as messages.
	ListModels() error
	ListGroundVehicles() error
	// StartDeparture and StartArrival start an aircraft's controller on
	// the IDs from defBase and reqBase: its commands and its events.
	StartDeparture(defBase, reqBase uint32, req traffic.TaxiRequest) (departureCtl, <-chan traffic.TaxiEvent, error)
	StartArrival(defBase, reqBase uint32, req traffic.ArrivalRequest) (arrivalCtl, <-chan traffic.ArrivalEvent, error)
}

// localSim is the simPort on the World's own connection.
type localSim struct {
	client  engine.Client
	fleet   *traffic.Fleet
	defOnce sync.Once
	defErr  error
	// The controllers' simulator side: injection, level of detail, the
	// ground picture they give way by, traffic time.
	inj    *traffic.Injector
	detail *traffic.Detail
	world  *traffic.TrafficPicture
	clock  *traffic.SimClock
}

func (l *localSim) StartDeparture(defBase, reqBase uint32, req traffic.TaxiRequest) (departureCtl, <-chan traffic.TaxiEvent, error) {
	ctl := traffic.NewTaxiController(l.fleet, traffic.TaxiWithIDs(defBase, reqBase), traffic.TaxiWithInjector(l.inj), traffic.TaxiWithDetail(l.detail),
		traffic.TaxiWithGroundPicture(l.world.Ground(req.Graph.Layout.ICAO)), traffic.TaxiWithClock(l.clock.Now))
	if err := ctl.Start(req); err != nil {
		return nil, nil, err
	}
	return ctl, ctl.Events(), nil
}

func (l *localSim) StartArrival(defBase, reqBase uint32, req traffic.ArrivalRequest) (arrivalCtl, <-chan traffic.ArrivalEvent, error) {
	ctl := traffic.NewArrivalController(l.fleet, traffic.ArrivalWithIDs(defBase, reqBase), traffic.ArrivalWithInjector(l.inj), traffic.ArrivalWithDetail(l.detail),
		traffic.ArrivalWithGroundPicture(l.world.Ground(req.Graph.Layout.ICAO)), traffic.ArrivalWithClock(l.clock.Now))
	if err := ctl.Start(req); err != nil {
		return nil, nil, err
	}
	return ctl, ctl.Events(), nil
}

func (l *localSim) SpawnEnroute(o traffic.NonATCOpts, reqID uint32) error {
	return l.fleet.RequestNonATC(o, reqID)
}

func (l *localSim) FlyEnroute(reqID, objectID uint32, wps []types.SIMCONNECT_DATA_WAYPOINT) error {
	l.fleet.Acknowledge(reqID, objectID)
	if err := l.waypointDef(); err != nil {
		return err
	}
	if err := l.fleet.ReleaseControl(objectID, reqReleaseEnroute); err != nil {
		return err
	}
	return l.fleet.SetWaypoints(objectID, enrouteDefWaypoints, wps)
}

func (l *localSim) SetRoute(objectID uint32, wps []types.SIMCONNECT_DATA_WAYPOINT) error {
	return l.fleet.SetWaypoints(objectID, enrouteDefWaypoints, wps)
}

func (l *localSim) RemoveObject(objectID, reqID uint32) error {
	return l.client.AIRemoveObject(objectID, reqID)
}

func (l *localSim) ListModels() error {
	return l.client.EnumerateSimObjectsAndLiveries(reqModels, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
}

func (l *localSim) ListGroundVehicles() error {
	return l.client.EnumerateSimObjectsAndLiveries(reqGroundVehicles, types.SIMCONNECT_SIMOBJECT_TYPE_GROUND)
}

// waypointDef defines the waypoint list once.
func (l *localSim) waypointDef() error {
	l.defOnce.Do(func() {
		l.defErr = l.client.AddToDataDefinition(enrouteDefWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0)
	})
	return l.defErr
}
