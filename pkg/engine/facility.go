//go:build windows
// +build windows

package engine

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func (e *Engine) AddToFacilityDefinition(definitionID uint32, fieldName string) error {
	return e.api.AddToFacilityDefinition(definitionID, fieldName)
}

func (e *Engine) AddFacilityDataDefinitionFilter(definitionID uint32, filterPath string, filterData unsafe.Pointer, filterDataSize uint32) error {
	return e.api.AddFacilityDataDefinitionFilter(definitionID, filterPath, filterData, filterDataSize)
}

func (e *Engine) ClearAllFacilityDataDefinitionFilters(definitionID uint32) error {
	return e.api.ClearAllFacilityDataDefinitionFilters(definitionID)
}

func (e *Engine) RequestFacilitiesList(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	return e.api.RequestFacilitiesList(definitionID, listType)
}

func (e *Engine) RequestFacilitiesListEX1(definitionID uint32, listType types.SIMCONNECT_FACILITY_LIST_TYPE) error {
	return e.api.RequestFacilitiesListEX1(definitionID, listType)
}

func (e *Engine) RequestFacilityData(definitionID uint32, icao string, region string) error {
	return e.api.RequestFacilityData(definitionID, icao, region)
}

func (e *Engine) RequestFacilityDataEX1(definitionID uint32, icao string, region string, facilityType byte) error {
	return e.api.RequestFacilityDataEX1(definitionID, icao, region, facilityType)
}

func (e *Engine) RequestJetwayData(airportICAO string, arrayCount uint32, indexes *int32) error {
	return e.api.RequestJetwayData(airportICAO, arrayCount, indexes)
}

func (e *Engine) SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error {
	return e.api.SubscribeToFacilities(listType, requestID)
}

func (e *Engine) SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error {
	return e.api.SubscribeToFacilitiesEX1(listType, newElemInRangeRequestID, oldElemOutRangeRequestID)
}

func (e *Engine) UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error {
	return e.api.UnsubscribeToFacilitiesEX1(listType, unsubscribeNewInRange, unsubscribeOldOutRange)
}
