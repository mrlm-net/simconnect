//go:build windows
// +build windows

package engine

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type Client interface {
	Connect() error
	Disconnect() error

	Stream() <-chan Message
	RegisterDataset(definitionID uint32, dataset *datasets.DataSet) error
	RegisterFacilityDataset(definitionID uint32, dataset *datasets.FacilityDataSet) error

	RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error
	SubscribeToSystemEvent(eventID uint32, eventName string) error
	UnsubscribeFromSystemEvent(eventID uint32) error

	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
	RequestDataOnSimObjectType(requestID uint32, definitionID uint32, dwRadiusMeters uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error
	ClearDataDefinition(definitionID uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error

	AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error
	AISetAircraftFlightPlan(objectID uint32, szFlightPlanPath string, requestID uint32) error
	AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error
	AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AIReleaseControl(objectID uint32, requestID uint32) error
	AIRemoveObject(objectID uint32, requestID uint32) error
	EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error
	AICreateEnrouteATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error
	AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error
	AICreateParkedATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, szAirportID string, RequestID uint32) error

	FlightLoad(flightFile string) error
	FlightPlanLoad(flightPlanFile string) error
	FlightSave(flightFile string, title string, description string) error

	AddToFacilityDefinition(definitionID uint32, fieldName string) error
	AddFacilityDataDefinitionFilter(definitionID uint32, filterPath string, filterData unsafe.Pointer, filterDataSize uint32) error
	ClearAllFacilityDataDefinitionFilters(definitionID uint32) error
	RequestFacilitiesList(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
	RequestFacilitiesListEX1(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error
	RequestFacilityData(definitionID uint32, requestID uint32, icao string, region string) error
	RequestFacilityDataEX1(definitionID uint32, requestID uint32, icao string, region string, facilityType byte) error
	RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error
	SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error
	SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error
	UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error
	RequestAllFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error

	MapClientEventToSimEvent(eventID uint32, eventName string) error
	RemoveClientEvent(groupID uint32, eventID uint32) error
	TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error
	TransmitClientEventEx1(objectID uint32, eventID uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG, data [5]uint32) error
	MapClientDataNameToID(clientDataName string, clientDataID uint32) error

	AddClientEventToNotificationGroup(groupID uint32, eventID uint32, mask bool) error
	ClearNotificationGroup(groupID uint32) error
	RequestNotificationGroup(groupID uint32, dwReserved uint32, flags uint32) error
	SetNotificationGroupPriority(groupID uint32, priority uint32) error
}
