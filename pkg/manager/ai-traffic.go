//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/types"

// AICreateParkedATCAircraft creates a parked ATC aircraft at an airport.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AICreateParkedATCAircraft(szContainerTitle, szTailNumber, szAirportID, RequestID)
}

// AISetAircraftFlightPlan assigns a flight plan to an AI aircraft.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AISetAircraftFlightPlan(objectID uint32, szFlightPlanPath string, requestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AISetAircraftFlightPlan(objectID, szFlightPlanPath, requestID)
}

// AICreateEnrouteATCAircraft creates an enroute ATC aircraft along a flight plan.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AICreateEnrouteATCAircraft(szContainerTitle, szTailNumber, iFlightNumber, szFlightPlanPath, dFlightPlanPosition, bTouchAndGo, RequestID)
}

// AICreateNonATCAircraft creates a non-ATC aircraft at a specific position.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AICreateNonATCAircraft(szContainerTitle, szTailNumber, initPos, RequestID)
}

// AICreateSimulatedObject creates a simulated object at a specific position.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AICreateSimulatedObject(szContainerTitle, initPos, RequestID)
}

// AIReleaseControl releases control of an AI object back to the simulator.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AIReleaseControl(objectID uint32, requestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AIReleaseControl(objectID, requestID)
}

// AIRemoveObject removes an AI object from the simulation.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AIRemoveObject(objectID uint32, requestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AIRemoveObject(objectID, requestID)
}

// EnumerateSimObjectsAndLiveries enumerates available sim objects and their liveries.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.EnumerateSimObjectsAndLiveries(requestID, objectType)
}

// AICreateEnrouteATCAircraftEX1 creates an enroute ATC aircraft with livery selection.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AICreateEnrouteATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AICreateEnrouteATCAircraftEX1(szContainerTitle, szLivery, szTailNumber, iFlightNumber, szFlightPlanPath, dFlightPlanPosition, bTouchAndGo, RequestID)
}

// AICreateNonATCAircraftEX1 creates a non-ATC aircraft with livery selection.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AICreateNonATCAircraftEX1(szContainerTitle, szLivery, szTailNumber, initPos, RequestID)
}

// AICreateParkedATCAircraftEX1 creates a parked ATC aircraft with livery selection.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AICreateParkedATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, szAirportID string, RequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AICreateParkedATCAircraftEX1(szContainerTitle, szLivery, szTailNumber, szAirportID, RequestID)
}
