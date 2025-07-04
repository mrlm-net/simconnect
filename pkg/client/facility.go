//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// SimConnect_AddToFacilityDefinition adds a field to a facility definition in the SimConnect client.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_AddToFacilityDefinition.htm
func (e *Engine) AddToFacilityDefinition(defineID int, fieldName string) error {
	// Convert strings to C-style for SimConnect
	fieldNamePtr, err := helpers.StringToBytePtr(fieldName)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	hresult, _, _ := SimConnect_AddToFacilityDefinition.Call(
		e.handle,          // hSimConnect (use handle directly, not getHandle())
		uintptr(defineID), // DefineID
		fieldNamePtr,      // FiledName
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddToFacilityDefinition failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) AddFacilityDataDefinitionFilter(defineID int, filterPath string, filterDataSize int, filterData uintptr) error {
	// Convert strings to C-style for SimConnect
	filterPathPtr, err := helpers.StringToBytePtr(filterPath)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	hresult, _, _ := SimConnect_AddFacilityDataDefinitionFilter.Call(
		e.handle,                // hSimConnect (use handle directly, not getHandle())
		uintptr(defineID),       // DefineID
		filterPathPtr,           // FilterPath
		uintptr(filterDataSize), // FilterDataSize
		filterData,              // FilterData

	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddFacilityDataDefinitionFilter failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) RequestFacilitesList(facilityType types.SIMCONNECT_FACILITY_LIST_TYPE, reguestID int) error {
	hresult, _, _ := SimConnect_AddToFacilityDefinition.Call(
		e.handle,              // hSimConnect (use handle directly, not getHandle())
		uintptr(facilityType), // FacilityType
		uintptr(reguestID),    // RequestID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestFacilitesList failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) RequestFacilityData(defineID int, requestID int, icao string, region string) error {
	// Convert strings to C-style for SimConnect
	icaoPtr, err := helpers.StringToBytePtr(icao)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	regionPtr, err := helpers.StringToBytePtr(region)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	hresult, _, _ := SimConnect_AddToFacilityDefinition.Call(
		e.handle,           // hSimConnect (use handle directly, not getHandle())
		uintptr(defineID),  // DefineID
		uintptr(requestID), // RequestID
		uintptr(icaoPtr),   // FacilityType
		uintptr(regionPtr), // Region
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestFacilityData failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) SubscribeToFacilities(facilityType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID int) error {
	hresult, _, _ := SimConnect_SubscribeToFacilities.Call(
		e.handle,              // hSimConnect (use handle directly, not getHandle())
		uintptr(facilityType), // FacilityType
		uintptr(requestID),    // RequestID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeToFacilities failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) UnsubscribeToFacilities(facilityType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	hresult, _, _ := SimConnect_UnsubscribeToFacilities.Call(
		e.handle,              // hSimConnect (use handle directly, not getHandle())
		uintptr(facilityType), // DefineID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_UnsubscribeToFacilities failed: 0x%08X", uint32(hresult))
	}

	return nil
}
