//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Fleet returns the manager's internal aircraft fleet.
//
// The fleet is valid for the lifetime of the manager but is reset (cleared) on
// each reconnect — ObjectIDs are invalidated when the simulator disconnects.
//
// Usage pattern:
//
//	mgr.TrafficNonATC(opts, reqID)
//	// ... receive SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID ...
//	aircraft, ok := mgr.Fleet().Acknowledge(reqID, objectID)
//	mgr.Fleet().ReleaseControl(aircraft.ObjectID, releaseReqID)
//	mgr.Fleet().SetWaypoints(aircraft.ObjectID, defID, wps)
func (m *Instance) Fleet() *traffic.Fleet {
	return m.fleet
}

// TrafficParked queues a parked ATC aircraft creation at an airport gate.
// Returns ErrNotConnected if not connected to the simulator.
//
// An SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID message will arrive shortly after;
// call m.Fleet().Acknowledge(reqID, objectID) from your message handler to
// register the aircraft in the fleet.
func (m *Instance) TrafficParked(opts traffic.ParkedOpts, reqID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.fleet.RequestParked(opts, reqID)
}

// TrafficEnroute queues an enroute ATC aircraft creation along a flight plan.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) TrafficEnroute(opts traffic.EnrouteOpts, reqID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.fleet.RequestEnroute(opts, reqID)
}

// TrafficNonATC queues a non-ATC aircraft creation at an explicit position.
// Returns ErrNotConnected if not connected to the simulator.
//
// After acknowledging the ObjectID, call TrafficReleaseControl followed by
// TrafficSetWaypoints to begin waypoint-guided movement (pushback, taxi, takeoff).
func (m *Instance) TrafficNonATC(opts traffic.NonATCOpts, reqID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.fleet.RequestNonATC(opts, reqID)
}

// TrafficRemove removes an AI aircraft from the simulation and from the fleet.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) TrafficRemove(objectID uint32, reqID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.fleet.Remove(objectID, reqID)
}

// TrafficReleaseControl releases simulator control over a NonATC aircraft.
// Must be called before TrafficSetWaypoints — the simulator ignores waypoints
// for aircraft it still holds under its own AI control.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) TrafficReleaseControl(objectID uint32, reqID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.fleet.ReleaseControl(objectID, reqID)
}

// TrafficSetWaypoints assigns a waypoint chain to a non-ATC aircraft.
// defID must be a SimConnect data definition registered for "AI Waypoint List".
// Build wps with traffic.PushbackWaypoint, traffic.TaxiWaypoint,
// traffic.LineupWaypoint, traffic.ClimbWaypoint, and traffic.TakeoffClimb.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) TrafficSetWaypoints(objectID uint32, defID uint32, wps []types.SIMCONNECT_DATA_WAYPOINT) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.fleet.SetWaypoints(objectID, defID, wps)
}

// TrafficSetFlightPlan assigns a flight plan to an ATC aircraft.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) TrafficSetFlightPlan(objectID uint32, planPath string, reqID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.fleet.SetFlightPlan(objectID, planPath, reqID)
}
