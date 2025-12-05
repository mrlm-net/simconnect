//go:build windows
// +build windows

package engine

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func (e *Engine) AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error {
	return e.api.AddToDataDefinition(definitionID, datumName, unitsName, datumType, epsilon, datumID)
}

func (e *Engine) RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	return e.api.RequestDataOnSimObject(requestID, definitionID, objectID, period, flags, origin, interval, limit)
}

func (e *Engine) RequestDataOnSimObjectType(requestID uint32, definitionID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	return e.api.RequestDataOnSimObjectType(requestID, definitionID, objectType, period, flags, origin, interval, limit)
}

func (e *Engine) ClearDataDefinition(definitionID uint32) error {
	return e.api.ClearDataDefinition(definitionID)
}

func (e *Engine) SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, data unsafe.Pointer, dataSize uint32) error {
	return e.api.SetDataOnSimObject(definitionID, objectID, flags, data, dataSize)
}
