package world

import (
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
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
	// services are the airports' tug and fuel truck fleets (#830), made on
	// first use; logf is told when a departure waits for one.
	servicesMu sync.Mutex
	services   map[string]*traffic.VehicleFleet
	logf       func(string, ...any)
}

// servicesAt is the fleet of l's airport: its limits' sizes, else by its
// stands.
func (l *localSim) servicesAt(layout *airport.Layout) *traffic.VehicleFleet {
	l.servicesMu.Lock()
	defer l.servicesMu.Unlock()
	if f, ok := l.services[layout.ICAO]; ok {
		return f
	}
	stands := 0
	for _, p := range layout.Parking {
		if p.Size() != airport.StandNone {
			stands++
		}
	}
	size := traffic.DefaultFleetSize(stands)
	lim := airport.LimitsFor(layout, nil)
	if lim.Tugs > 0 {
		size[traffic.VehicleTug] = lim.Tugs
	}
	if lim.FuelTrucks > 0 {
		size[traffic.VehicleFuel] = lim.FuelTrucks
	}
	if lim.Stairs > 0 {
		size[traffic.VehicleStairs] = lim.Stairs
	}
	if lim.GPUs > 0 {
		size[traffic.VehicleGPU] = lim.GPUs
	}
	if lim.Buses > 0 {
		size[traffic.VehicleBus] = lim.Buses
	}
	f := traffic.NewVehicleFleet(size)
	icao := layout.ICAO
	f.OnWait = func(kind traffic.VehicleKind, owner string, busy int) {
		if l.logf != nil {
			l.logf("%-6s waits for a %s at %s: all %d busy", owner, map[traffic.VehicleKind]string{traffic.VehicleTug: "tug", traffic.VehicleFuel: "fuel truck", traffic.VehicleStairs: "set of stairs", traffic.VehicleGPU: "GPU", traffic.VehicleBus: "bus"}[kind], icao, busy)
		}
	}
	if l.services == nil {
		l.services = map[string]*traffic.VehicleFleet{}
	}
	l.services[layout.ICAO] = f
	if l.logf != nil {
		l.logf("ground services at %s: %d tugs, %d fuel trucks, %d stairs, %d GPUs, %d buses (%d stands)", icao,
			size[traffic.VehicleTug], size[traffic.VehicleFuel], size[traffic.VehicleStairs], size[traffic.VehicleGPU], size[traffic.VehicleBus], stands)
	}
	return f
}

func (l *localSim) StartDeparture(defBase, reqBase uint32, req traffic.TaxiRequest) (departureCtl, <-chan traffic.TaxiEvent, error) {
	ctl := traffic.NewTaxiController(l.fleet, traffic.TaxiWithIDs(defBase, reqBase), traffic.TaxiWithInjector(l.inj), traffic.TaxiWithDetail(l.detail),
		traffic.TaxiWithGroundPicture(l.world.Ground(req.Graph.Layout.ICAO)), traffic.TaxiWithClock(l.clock.Now),
		traffic.TaxiWithServices(l.servicesAt(req.Graph.Layout)))
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
