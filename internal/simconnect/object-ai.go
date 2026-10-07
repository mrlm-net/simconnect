//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"math"
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

	procedure := sc.proc("SimConnect_AICreateEnrouteATCAircraft")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szContainerTitlePtr)),
		uintptr(unsafe.Pointer(szTailNumberPtr)),
		uintptr(iFlightNumber),
		uintptr(unsafe.Pointer(szFlightPlanPathPtr)),
		float64Arg(dFlightPlanPosition), // double, by value
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

	procedure := sc.proc("SimConnect_AICreateNonATCAircraft")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szContainerTitlePtr)),
		uintptr(unsafe.Pointer(szTailNumberPtr)),
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

	procedure := sc.proc("SimConnect_AICreateParkedATCAircraft")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szContainerTitlePtr)),
		uintptr(unsafe.Pointer(szTailNumberPtr)),
		uintptr(unsafe.Pointer(szAirportIDPtr)),
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

	procedure := sc.proc("SimConnect_AISetAircraftFlightPlan")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(objectID),
		uintptr(unsafe.Pointer(szFlightPlanPathPtr)),
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
	procedure := sc.proc("SimConnect_AICreateEnrouteATCAircraft_EX1")
	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szContainerTitlePtr)),
		uintptr(unsafe.Pointer(szLiveryPtr)),
		uintptr(unsafe.Pointer(szTailNumberPtr)),
		uintptr(iFlightNumber),
		uintptr(unsafe.Pointer(szFlightPlanPathPtr)),
		float64Arg(dFlightPlanPosition), // double, by value
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
	procedure := sc.proc("SimConnect_AICreateNonATCAircraft_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szContainerTitlePtr)),
		uintptr(unsafe.Pointer(szLiveryPtr)),
		uintptr(unsafe.Pointer(szTailNumberPtr)),
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
	procedure := sc.proc("SimConnect_AICreateParkedATCAircraft_EX1")

	hresult, _, _ := procedure.Call(
		sc.getConnection(), // phSimConnect - pointer to handle
		uintptr(unsafe.Pointer(szContainerTitlePtr)),
		uintptr(unsafe.Pointer(szLiveryPtr)),
		uintptr(unsafe.Pointer(szTailNumberPtr)),
		uintptr(unsafe.Pointer(szAirportIDPtr)),
		uintptr(RequestID),
	)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateParkedATCAircraft_EX1 failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// float64Arg passes a double by value, as the SDK declares
// dFlightPlanPosition. It was passed as a pointer before: the DLL read the
// pointer's bits as a tiny double (~0), so every enroute aircraft started at
// the start of its plan.
//
// In the Windows x64 calling convention a double among the first four
// arguments goes in XMM0-XMM3, any later one as its 8 raw bytes in a stack
// slot. Go's Windows syscall path (asmstdcall) copies the first four
// arguments into both RCX/RDX/R8/R9 and XMM0-XMM3 and puts the rest on the
// stack as 8-byte slots, so the IEEE-754 bits in a uintptr arrive right in
// any position. dFlightPlanPosition is argument 6 (7 in _EX1): a stack slot.
func float64Arg(v float64) uintptr {
	return uintptr(math.Float64bits(v))
}
