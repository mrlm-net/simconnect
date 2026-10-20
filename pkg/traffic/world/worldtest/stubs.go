package worldtest

// Code generated from engine.Client: the calls the loopback answers
// with nothing (no error, no reply). DO NOT EDIT by hand; see loopback.go
// for the ones it answers.

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var (
	_ = unsafe.Pointer(nil)
	_ *datasets.DataSet
	_ types.DWORD
)

func (l *Loopback) RegisterDataset(definitionID uint32, dataset *datasets.DataSet) error { return nil }
func (l *Loopback) RegisterFacilityDataset(definitionID uint32, dataset *datasets.FacilityDataSet) error {
	return nil
}
func (l *Loopback) RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error {
	return nil
}
func (l *Loopback) UnsubscribeFromSystemEvent(eventID uint32) error                { return nil }
func (l *Loopback) SubscribeToFlowEvent() error                                    { return nil }
func (l *Loopback) UnsubscribeFromFlowEvent() error                                { return nil }
func (l *Loopback) SubscribeToCommBusEvent(eventID uint32, eventName string) error { return nil }
func (l *Loopback) UnsubscribeToCommBusEvent(eventID uint32) error                 { return nil }
func (l *Loopback) CallCommBusEvent(eventName string, broadcastTo types.SIMCONNECT_COMM_BUS_BROADCAST_TO, data string) error {
	return nil
}
func (l *Loopback) EnumerateInputEvents(requestID uint32) error          { return nil }
func (l *Loopback) GetInputEvent(requestID uint32, hash uint64) error    { return nil }
func (l *Loopback) SetInputEventDouble(hash uint64, value float64) error { return nil }
func (l *Loopback) SetInputEventString(hash uint64, value string) error  { return nil }
func (l *Loopback) SubscribeInputEvent(hash uint64) error                { return nil }
func (l *Loopback) UnsubscribeInputEvent(hash uint64) error              { return nil }
func (l *Loopback) AISetAircraftFlightPlan(objectID uint32, szFlightPlanPath string, requestID uint32) error {
	return nil
}
func (l *Loopback) AIReleaseControl(objectID uint32, requestID uint32) error             { return nil }
func (l *Loopback) FlightLoad(flightFile string) error                                   { return nil }
func (l *Loopback) FlightPlanLoad(flightPlanFile string) error                           { return nil }
func (l *Loopback) FlightSave(flightFile string, title string, description string) error { return nil }
func (l *Loopback) AddToFacilityDefinition(definitionID uint32, fieldName string) error  { return nil }
func (l *Loopback) AddFacilityDataDefinitionFilter(definitionID uint32, filterPath string, filterData unsafe.Pointer, filterDataSize uint32) error {
	return nil
}
func (l *Loopback) ClearAllFacilityDataDefinitionFilters(definitionID uint32) error { return nil }
func (l *Loopback) RequestFacilitiesList(requestID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	return nil
}
func (l *Loopback) RequestFacilitiesListEX1(requestID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	return nil
}
func (l *Loopback) RequestFacilityData(definitionID uint32, requestID uint32, icao string, region string) error {
	return nil
}
func (l *Loopback) RequestFacilityDataEX1(definitionID uint32, requestID uint32, icao string, region string, facilityType byte) error {
	return nil
}
func (l *Loopback) RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error {
	return nil
}
func (l *Loopback) SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error {
	return nil
}
func (l *Loopback) UnsubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	return nil
}
func (l *Loopback) SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error {
	return nil
}
func (l *Loopback) UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error {
	return nil
}
func (l *Loopback) RequestAllFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error {
	return nil
}
func (l *Loopback) MapClientEventToSimEvent(eventID uint32, eventName string) error { return nil }
func (l *Loopback) RemoveClientEvent(groupID uint32, eventID uint32) error          { return nil }
func (l *Loopback) TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	return nil
}
func (l *Loopback) TransmitClientEventEx1(objectID uint32, eventID uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG, data [5]uint32) error {
	return nil
}
func (l *Loopback) MapClientDataNameToID(clientDataName string, clientDataID uint32) error {
	return nil
}
func (l *Loopback) CreateClientData(clientDataID uint32, dwSize uint32, flags types.SIMCONNECT_CREATE_CLIENT_DATA_FLAG) error {
	return nil
}
func (l *Loopback) AddToClientDataDefinition(defineID uint32, dwOffset uint32, dwSizeOrType uint32, epsilon float32, datumID uint32) error {
	return nil
}
func (l *Loopback) ClearClientDataDefinition(defineID uint32) error { return nil }
func (l *Loopback) RequestClientData(clientDataID uint32, requestID uint32, defineID uint32, period types.SIMCONNECT_CLIENT_DATA_PERIOD, flags types.SIMCONNECT_CLIENT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	return nil
}
func (l *Loopback) SetClientData(clientDataID uint32, defineID uint32, flags uint32, dwReserved uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	return nil
}
func (l *Loopback) AddClientEventToNotificationGroup(groupID uint32, eventID uint32, mask bool) error {
	return nil
}
func (l *Loopback) ClearNotificationGroup(groupID uint32) error { return nil }
func (l *Loopback) RequestNotificationGroup(groupID uint32, dwReserved uint32, flags uint32) error {
	return nil
}
func (l *Loopback) SetNotificationGroupPriority(groupID uint32, priority uint32) error { return nil }
func (l *Loopback) MapInputEventToClientEvent(groupID uint32, definition string, downEventID, downValue, upEventID, upValue uint32, maskable bool) error {
	return nil
}
func (l *Loopback) SetInputGroupPriority(groupID uint32, priority uint32) error { return nil }
func (l *Loopback) SetInputGroupState(groupID uint32, state uint32) error       { return nil }
func (l *Loopback) RemoveInputEvent(groupID uint32, definition string) error    { return nil }
func (l *Loopback) ClearInputGroup(groupID uint32) error                        { return nil }
