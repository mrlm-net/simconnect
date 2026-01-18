//go:build windows
// +build windows

package manager

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ensureConnected checks if the manager is connected and returns the client.
// Returns ErrNotConnected if not connected.
func (m *Instance) ensureConnected() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return nil
}

// RegisterDataset registers a complete dataset definition with SimConnect.
// This is a convenience method that iterates over all definitions in the dataset
// and calls AddToDataDefinition for each one.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RegisterDataset(definitionID uint32, dataset *datasets.DataSet) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RegisterDataset(definitionID, dataset)
}

// AddToDataDefinition adds a single data definition to a definition group.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.AddToDataDefinition(definitionID, datumName, unitsName, datumType, epsilon, datumID)
}

// RequestDataOnSimObject requests data for a specific simulation object.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestDataOnSimObject(requestID, definitionID, objectID, period, flags, origin, interval, limit)
}

// RequestDataOnSimObjectType requests data for all objects of a specific type within a radius.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) RequestDataOnSimObjectType(requestID uint32, definitionID uint32, dwRadiusMeters uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.RequestDataOnSimObjectType(requestID, definitionID, dwRadiusMeters, objectType)
}

// ClearDataDefinition clears all data definitions for a definition group.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) ClearDataDefinition(definitionID uint32) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.ClearDataDefinition(definitionID)
}

// SetDataOnSimObject sets data on a simulation object.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.SetDataOnSimObject(definitionID, objectID, flags, arrayCount, cbUnitSize, data)
}
