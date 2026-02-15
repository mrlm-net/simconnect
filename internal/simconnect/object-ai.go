//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/AI_Object/SimConnect_AICreateEnrouteATCAircraft.htm
func (sc *SimConnect) AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}

	szTailNumberPtr, err := stringToBytePtr(szTailNumber)
	if err != nil {
		return fmt.Errorf("failed to convert tail number to byte pointer: %w", err)
	}

	szFlightPlanPathPtr, err := stringToBytePtr(szFlightPlanPath)
	if err != nil {
		return fmt.Errorf("failed to convert flight plan path to byte pointer: %w", err)
	}

	var bTouchAndGoUintptr uintptr
	if bTouchAndGo {
		bTouchAndGoUintptr = 1
	} else {
		bTouchAndGoUintptr = 0
	}

	procedure := sc.library.LoadProcedure("SimConnect_AICreateEnrouteATCAircraft")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		szTailNumberPtr,
		uintptr(iFlightNumber),
		szFlightPlanPathPtr,
		uintptr(unsafe.Pointer(&dFlightPlanPosition)),
		bTouchAndGoUintptr,
		uintptr(RequestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateEnrouteATCAircraft failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/AI_Object/SimConnect_AICreateNonATCAircraft.htm
func (sc *SimConnect) AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}

	szTailNumberPtr, err := stringToBytePtr(szTailNumber)
	if err != nil {
		return fmt.Errorf("failed to convert tail number to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_AICreateNonATCAircraft")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		szTailNumberPtr,
		uintptr(unsafe.Pointer(&initPos)),
		uintptr(RequestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateNonATCAircraft failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/AI_Object/SimConnect_AICreateParkedATCAircraft.htm
func (sc *SimConnect) AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}

	szTailNumberPtr, err := stringToBytePtr(szTailNumber)
	if err != nil {
		return fmt.Errorf("failed to convert tail number to byte pointer: %w", err)
	}

	szAirportIDPtr, err := stringToBytePtr(szAirportID)
	if err != nil {
		return fmt.Errorf("failed to convert airport ID to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_AICreateParkedATCAircraft")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		szTailNumberPtr,
		szAirportIDPtr,
		uintptr(RequestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateParkedATCAircraft failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/AI_Object/SimConnect_AISetAircraftFlightPlan.htm
func (sc *SimConnect) AISetAircraftFlightPlan(objectID uint32, szFlightPlanPath string, requestID uint32) error {
	szFlightPlanPathPtr, err := stringToBytePtr(szFlightPlanPath)
	if err != nil {
		return fmt.Errorf("failed to convert flight plan path to byte pointer: %w", err)
	}

	procedure := sc.library.LoadProcedure("SimConnect_AISetAircraftFlightPlan")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(objectID),
		szFlightPlanPathPtr,
		uintptr(requestID),
	)

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AISetAircraftFlightPlan failed with HRESULT: 0x%08X", uint32(hresult))
	}

	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateEnrouteATCAircraft_EX1.htm
func (sc *SimConnect) AICreateEnrouteATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}
	szLiveryPtr, err := stringToBytePtr(szLivery)
	if err != nil {
		return fmt.Errorf("failed to convert livery to byte pointer: %w", err)
	}
	szTailNumberPtr, err := stringToBytePtr(szTailNumber)
	if err != nil {
		return fmt.Errorf("failed to convert tail number to byte pointer: %w", err)
	}
	szFlightPlanPathPtr, err := stringToBytePtr(szFlightPlanPath)
	if err != nil {
		return fmt.Errorf("failed to convert flight plan path to byte pointer: %w", err)
	}
	var bTouchAndGoUintptr uintptr
	if bTouchAndGo {
		bTouchAndGoUintptr = 1
	} else {
		bTouchAndGoUintptr = 0
	}
	procedure := sc.library.LoadProcedure("SimConnect_AICreateEnrouteATCAircraft_EX1")
	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		szLiveryPtr,
		szTailNumberPtr,
		uintptr(iFlightNumber),
		szFlightPlanPathPtr,
		uintptr(unsafe.Pointer(&dFlightPlanPosition)),
		bTouchAndGoUintptr,
		uintptr(RequestID),
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateEnrouteATCAircraft_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateNonATCAircraft_EX1.htm
func (sc *SimConnect) AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}
	szLiveryPtr, err := stringToBytePtr(szLivery)
	if err != nil {
		return fmt.Errorf("failed to convert livery to byte pointer: %w", err)
	}
	szTailNumberPtr, err := stringToBytePtr(szTailNumber)
	if err != nil {
		return fmt.Errorf("failed to convert tail number to byte pointer: %w", err)
	}
	procedure := sc.library.LoadProcedure("SimConnect_AICreateNonATCAircraft_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		szLiveryPtr,
		szTailNumberPtr,
		uintptr(unsafe.Pointer(&initPos)),
		uintptr(RequestID),
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateNonATCAircraft_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateParkedATCAircraft_EX1.htm
func (sc *SimConnect) AICreateParkedATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, szAirportID string, RequestID uint32) error {
	szContainerTitlePtr, err := stringToBytePtr(szContainerTitle)
	if err != nil {
		return fmt.Errorf("failed to convert container title to byte pointer: %w", err)
	}
	szLiveryPtr, err := stringToBytePtr(szLivery)
	if err != nil {
		return fmt.Errorf("failed to convert livery to byte pointer: %w", err)
	}
	szTailNumberPtr, err := stringToBytePtr(szTailNumber)
	if err != nil {
		return fmt.Errorf("failed to convert tail number to byte pointer: %w", err)
	}
	szAirportIDPtr, err := stringToBytePtr(szAirportID)
	if err != nil {
		return fmt.Errorf("failed to convert airport ID to byte pointer: %w", err)
	}
	procedure := sc.library.LoadProcedure("SimConnect_AICreateParkedATCAircraft_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		szContainerTitlePtr,
		szLiveryPtr,
		szTailNumberPtr,
		szAirportIDPtr,
		uintptr(RequestID),
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateParkedATCAircraft_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}
