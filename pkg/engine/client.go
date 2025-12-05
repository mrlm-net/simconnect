//go:build windows
// +build windows

package engine

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type Client interface {
	Connect() error
	Disconnect() error

	Stream() <-chan Message

	RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error
	SubscribeToSystemEvent(eventID uint32, eventName string) error
	UnsubscribeFromSystemEvent(eventID uint32) error

	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	RequestDataOnSimObjectType(requestID uint32, definitionID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	ClearDataDefinition(definitionID uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, data unsafe.Pointer, dataSize uint32) error
}
