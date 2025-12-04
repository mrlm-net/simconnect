//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_AddToFacilityDefinition.htm
func (sc *SimConnect) AddToFacilityDefinition(definitionID uint32, fieldName string) error {
	szFieldName, err := stringToBytePtr(fieldName)
	if err != nil {
		return fmt.Errorf("failed to convert field name to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_AddToFacilityDefinition")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
		szFieldName,
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddToFacilityDefinition failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_AddFacilityDataDefinitionFilter.htm
func (sc *SimConnect) AddFacilityDataDefinitionFilter(definitionID uint32, filterPath string, filterData unsafe.Pointer, filterDataSize uint32) error {
	szFilterPath, err := stringToBytePtr(filterPath)
	if err != nil {
		return fmt.Errorf("failed to convert filter path to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_AddFacilityDataDefinitionFilter")
	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
		szFilterPath,
		uintptr(filterDataSize),
		uintptr(filterData),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddFacilityDataDefinitionFilter failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_ClearAllFacilityDataDefinitionFilters.htm
func (sc *SimConnect) ClearAllFacilityDataDefinitionFilters(definitionID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_ClearAllFacilityDataDefinitionFilters")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_ClearAllFacilityDataDefinitionFilters failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_RequestFacilitesList.htm
func (sc *SimConnect) RequestFacilitiesList(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	procedure := sc.library.LoadProcedure("SimConnect_RequestFacilitesList")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(listType),
		uintptr(definitionID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestFacilitesList failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_RequestFacilitiesList_EX1.htm
func (sc *SimConnect) RequestFacilitiesListEX1(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	procedure := sc.library.LoadProcedure("SimConnect_RequestFacilitesList_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(listType),
		uintptr(definitionID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestFacilitesList_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_RequestFacilityData.htm
func (sc *SimConnect) RequestFacilityData(definitionID uint32, icao string, region string) error {
	szICAO, err := stringToBytePtr(icao)
	if err != nil {
		return fmt.Errorf("failed to convert ICAO to byte pointer: %w", err)
	}

	szRegion, err := stringToBytePtr(region)
	if err != nil {
		return fmt.Errorf("failed to convert region to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_RequestFacilityData")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
		szICAO,
		szRegion,
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestFacilityData failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_RequestFacilityData_EX1.htm
func (sc *SimConnect) RequestFacilityDataEX1(definitionID uint32, icao string, region string, facilityType byte) error {
	szICAO, err := stringToBytePtr(icao)
	if err != nil {
		return fmt.Errorf("failed to convert ICAO to byte pointer: %w", err)
	}

	szRegion, err := stringToBytePtr(region)
	if err != nil {
		return fmt.Errorf("failed to convert region to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_RequestFacilityData_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(definitionID),
		szICAO,
		szRegion,
		uintptr(facilityType),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestFacilityData_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_RequestJetwayData.htm
func (sc *SimConnect) RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error {
	szAirportICAO, err := stringToBytePtr(airportICAO)
	if err != nil {
		return fmt.Errorf("failed to convert airport ICAO to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_RequestJetwayData")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szAirportICAO,
		uintptr(arrayCount),
		uintptr(unsafe.Pointer(indexes)),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestJetwayData failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_SubscribeToFacilities.htm
func (sc *SimConnect) SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_SubscribeToFacilities")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(listType),
		uintptr(requestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeToFacilities failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_SubscribeToFacilities_EX1.htm
func (sc *SimConnect) SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error {
	procedure := sc.library.LoadProcedure("SimConnect_SubscribeToFacilities_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(listType),
		uintptr(newElemInRangeRequestID),
		uintptr(oldElemOutRangeRequestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeToFacilities_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_UnsubscribeToFacilities.htm
func (sc *SimConnect) UnsubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	procedure := sc.library.LoadProcedure("SimConnect_UnsubscribeToFacilities")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(listType),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_UnsubscribeToFacilities failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Facilities/SimConnect_UnsubscribeToFacilities_EX1.htm
func (sc *SimConnect) UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error {
	procedure := sc.library.LoadProcedure("SimConnect_UnsubscribeToFacilities_EX1")

	var unsubNew uint8
	if unsubscribeNewInRange {
		unsubNew = 1
	} else {
		unsubNew = 0
	}

	var unsubOld uint8
	if unsubscribeOldOutRange {
		unsubOld = 1
	} else {
		unsubOld = 0
	}

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(listType),
		uintptr(unsubNew),
		uintptr(unsubOld),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_UnsubscribeToFacilities_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}
