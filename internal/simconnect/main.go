//go:build windows
// +build windows

package simconnect

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/mrlm-net/simconnect/internal/dll"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func New(name string, config *Config) *SimConnect {
	sc := &SimConnect{connection: 0, library: dll.New(config.DLLPath), name: name}
	if config.TraceCalls {
		sc.trace = &callTrace{}
	}
	return sc
}

type SimConnect struct {
	// Add fields as necessary
	connection uintptr
	library    *dll.DLL
	name       string
	sync       sync.RWMutex
	trace      *callTrace // Config.TraceCalls; nil off
}

type API interface {
	Connect() error
	Disconnect() error

	GetNextDispatch() (*types.SIMCONNECT_RECV, uint32, error)
	GetLastSentPacketID() (uint32, error)
	RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error
	SubscribeToSystemEvent(eventID uint32, eventName string) error
	UnsubscribeFromSystemEvent(eventID uint32) error
	SetSystemEventState(eventID uint32, state types.SIMCONNECT_STATE) error

	SubscribeToFlowEvent() error
	UnsubscribeFromFlowEvent() error
	SubscribeToCommBusEvent(eventID uint32, eventName string) error
	UnsubscribeToCommBusEvent(eventID uint32) error
	CallCommBusEvent(eventName string, broadcastTo types.SIMCONNECT_COMM_BUS_BROADCAST_TO, data string) error
	// The add-on camera (MSFS 2024 only).
	CameraAcquire(clientID string) error
	CameraRelease(cameraDef string) error
	CameraGetStatus() error
	CameraSet(data types.SIMCONNECT_DATA_CAMERA, mask types.SIMCONNECT_CAMERA_DATA_MASK) error
	CameraGet(referential types.SIMCONNECT_POSITION_REFERENTIAL) error
	CameraEnableFlag(flag types.SIMCONNECT_CAMERA_FLAG) error
	CameraDisableFlag(flag types.SIMCONNECT_CAMERA_FLAG) error
	SubscribeToCameraStatusUpdate() error
	UnsubscribeToCameraStatusUpdate() error
	EnumerateCameraDefinitions() error
	CameraSetUsingCameraDefinition(cameraDef string) error
	RequestCameraWorldLocker(position types.SIMCONNECT_DATA_XYZ, referential types.SIMCONNECT_POSITION_REFERENTIAL, objectID uint32) error
	DeleteCameraWorldLocker() error
	SubscribeToCameraWorldLockerStatusUpdate() error

	FlightLoad(flightFile string) error
	FlightPlanLoad(flightPlanFile string) error
	FlightSave(flightFile string, title string, description string) error

	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	RequestDataOnSimObjectType(requestID uint32, definitionID uint32, dwRadiusMeters uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	ClearDataDefinition(definitionID uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error

	// AI Object Methods
	AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error
	AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error
	AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AICreateSimulatedObjectEX1(szContainerTitle string, szLivery string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AIReleaseControl(objectID uint32, requestID uint32) error
	AIRemoveObject(objectID uint32, requestID uint32) error
	AISetAircraftFlightPlan(objectID uint32, szFlightPlanPath string, requestID uint32) error
	EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error
	AICreateEnrouteATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error
	AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AICreateParkedATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, szAirportID string, RequestID uint32) error

	AddToFacilityDefinition(definitionID uint32, fieldName string) error
	AddFacilityDataDefinitionFilter(definitionID uint32, filterPath string, filterData unsafe.Pointer, filterDataSize uint32) error
	ClearAllFacilityDataDefinitionFilters(definitionID uint32) error
	RequestFacilitiesList(requestID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
	RequestFacilitiesListEX1(requestID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
	RequestFacilityData(definitionID uint32, requestID uint32, icao string, region string) error
	RequestFacilityDataEX1(definitionID uint32, requestID uint32, icao string, region string, facilityType byte) error
	RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error
	SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error
	UnsubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
	SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error
	UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error
	RequestAllFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error

	MapClientEventToSimEvent(eventID uint32, eventName string) error
	RemoveClientEvent(groupID uint32, eventID uint32) error
	TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error
	TransmitClientEventEx1(objectID uint32, eventID uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG, data [5]uint32) error
	MapClientDataNameToID(clientDataName string, clientDataID uint32) error

	// Client Data Area API
	CreateClientData(clientDataID uint32, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error
	AddToClientDataDefinition(defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32) error
	ClearClientDataDefinition(defineID uint32) error
	RequestClientData(clientDataID uint32, requestID uint32, defineID uint32, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	SetClientData(clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer) error

	AddClientEventToNotificationGroup(groupID uint32, eventID uint32, mask bool) error
	ClearNotificationGroup(groupID uint32) error
	RequestNotificationGroup(groupID uint32, dwReserved uint32, flags uint32) error
	SetNotificationGroupPriority(groupID uint32, priority uint32) error

	// Input groups: keys and joystick buttons mapped to client events.
	MapInputEventToClientEvent(groupID uint32, definition string, downEventID, downValue, upEventID, upValue uint32, maskable bool) error
	SetInputGroupPriority(groupID uint32, priority uint32) error
	SetInputGroupState(groupID uint32, state uint32) error
	RemoveInputEvent(groupID uint32, definition string) error
	ClearInputGroup(groupID uint32) error

	// Input Event API (MSFS 2024 only)
	EnumerateInputEvents(requestID uint32) error
	GetInputEvent(requestID uint32, hash uint64) error
	SetInputEvent(hash uint64, cbUnitSize uint32, value unsafe.Pointer) error
	SubscribeInputEvent(hash uint64) error
	UnsubscribeInputEvent(hash uint64) error
}

func (sc *SimConnect) getConnection() uintptr {
	sc.sync.RLock()
	defer sc.sync.RUnlock()
	return uintptr(sc.connection)
}

// boundProc is a SimConnect procedure called on the open connection.
type boundProc struct {
	sc   *SimConnect
	proc *syscall.LazyProc
}

// proc loads a procedure whose first argument is the connection handle.
func (sc *SimConnect) proc(name string) boundProc {
	return boundProc{sc: sc, proc: sc.library.LoadProcedure(name)}
}

// Call calls the procedure with the connection held: Disconnect waits for
// calls in flight before SimConnect_Close, so a handle is never closed under
// a call. args[0] is the handle; it is read again under the lock (the
// caller's copy may predate a Disconnect or a reconnect).
//
//go:uintptrescapes
func (p boundProc) Call(args ...uintptr) (uintptr, uintptr, error) {
	p.sc.sync.RLock()
	defer p.sc.sync.RUnlock()
	if len(args) > 0 {
		args[0] = p.sc.connection
	}
	if t := p.sc.trace; t != nil && p.proc.Name != "SimConnect_GetNextDispatch" {
		// Traced: the call and its send ID read as one, no other call in
		// between to take or lose the label (an exception at start-up came
		// without one while several goroutines called at once).
		t.callMu.Lock()
		defer t.callMu.Unlock()
		r1, r2, err := p.proc.Call(args...)
		if isHRESULTSuccess(r1) {
			p.sc.traced(p.proc.Name, args)
		}
		return r1, r2, err
	}
	return p.proc.Call(args...)
}
