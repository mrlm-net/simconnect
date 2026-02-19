//go:build windows
// +build windows

package manager

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// RegisterFacilityDataset registers a complete facility dataset definition with SimConnect.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RegisterFacilityDataset(definitionID uint32, dataset *datasets.FacilityDataSet) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RegisterFacilityDataset(definitionID, dataset)
}

// AddToFacilityDefinition adds a field to a facility data definition.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AddToFacilityDefinition(definitionID uint32, fieldName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AddToFacilityDefinition(definitionID, fieldName)
}

// AddFacilityDataDefinitionFilter adds a filter to a facility data definition.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AddFacilityDataDefinitionFilter(definitionID uint32, filterPath string, filterData unsafe.Pointer, filterDataSize uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AddFacilityDataDefinitionFilter(definitionID, filterPath, filterData, filterDataSize)
}

// ClearAllFacilityDataDefinitionFilters clears all filters from a facility data definition.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) ClearAllFacilityDataDefinitionFilters(definitionID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.ClearAllFacilityDataDefinitionFilters(definitionID)
}

// RequestFacilitiesList requests a list of facilities of the specified type.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestFacilitiesList(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestFacilitiesList(definitionID, listType)
}

// RequestFacilitiesListEX1 requests a list of facilities of the specified type (extended version).
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestFacilitiesListEX1(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestFacilitiesListEX1(definitionID, listType)
}

// RequestFacilityData requests facility data for a specific ICAO code and region.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestFacilityData(definitionID uint32, requestID uint32, icao string, region string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestFacilityData(definitionID, requestID, icao, region)
}

// RequestFacilityDataEX1 requests facility data with a facility type filter (extended version).
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestFacilityDataEX1(definitionID uint32, requestID uint32, icao string, region string, facilityType byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestFacilityDataEX1(definitionID, requestID, icao, region, facilityType)
}

// RequestJetwayData requests jetway data for an airport.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestJetwayData(airportICAO, arrayCount, indexes)
}

// SubscribeToFacilities subscribes to facility list updates of the specified type.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SubscribeToFacilities(listType, requestID)
}

// SubscribeToFacilitiesEX1 subscribes to facility list updates with separate in-range and out-of-range request IDs.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SubscribeToFacilitiesEX1(listType, newElemInRangeRequestID, oldElemOutRangeRequestID)
}

// UnsubscribeToFacilitiesEX1 unsubscribes from facility list updates.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.UnsubscribeToFacilitiesEX1(listType, unsubscribeNewInRange, unsubscribeOldOutRange)
}

// RequestAllFacilities requests all facilities of the specified type.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestAllFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestAllFacilities(listType, requestID)
}
