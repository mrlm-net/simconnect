//go:build windows
// +build windows

package simconnect

import (
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/internal/dll"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func New(name string, config *Config) *SimConnect {
	return &SimConnect{
		0, dll.New(config.DLLPath), name, sync.RWMutex{},
	}
}

type SimConnect struct {
	// Add fields as necessary
	connection uintptr
	library    *dll.DLL
	name       string
	sync       sync.RWMutex
}

type API interface {
	Connect() error
	Disconnect() error

	GetNextDispatch() (*types.SIMCONNECT_RECV, uint32, error)
	RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error
	SubscribeToSystemEvent(eventID uint32, eventName string) error

	FlightLoad(flightFile string) error
	FlightPlanLoad(flightPlanFile string) error
	FlightSave(flightFile string, title string, description string) error

	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	RequestDataOnSimObjectType(requestID uint32, definitionID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	ClearDataDefinition(definitionID uint32) error
	SetDataOnSimObject(uint32, uint32, types.SIMCONNECT_DATA_SET_FLAG, unsafe.Pointer, uint32) error

	AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error
	AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error
	AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AIReleaseControl(objectID uint32, requestID uint32) error
}

func (sc *SimConnect) getConnection() uintptr {
	sc.sync.RLock()
	defer sc.sync.RUnlock()
	return uintptr(sc.connection)
}

func (sc *SimConnect) getConnectionPtr() uintptr {
	sc.sync.RLock()
	defer sc.sync.RUnlock()
	return uintptr(unsafe.Pointer(&sc.connection))
}
