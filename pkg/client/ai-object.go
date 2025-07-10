//go:build windows
// +build windows

package client

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ...existing code...

// EnumerateSimObjectsAndLiveries requests the list of spawnable SimObjects and their liveries.
// requestID: Client-defined request ID for tracking the response.
// simObjectType: The type of SimObjects to enumerate (see types.SIMCONNECT_SIMOBJECT_TYPE).
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_EnumerateSimObjectsAndLiveries.htm
func (e *Engine) EnumerateSimObjectsAndLiveries(requestID uint32, simObjectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	hresult, _, _ := SimConnect_EnumerateSimObjectsAndLiveries.Call(
		e.handle,               // hSimConnect
		uintptr(requestID),     // RequestID
		uintptr(simObjectType), // Type
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_EnumerateSimObjectsAndLiveries failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AICreateSimulatedObject_EX1 creates an AI controlled simulated object (modular or legacy).
// containerTitle: The title of the object (from aircraft.cfg or SimObject).
// livery: The livery name or folder (can be empty for default livery).
// initPos: The initial position of the object.
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateSimulatedObject_EX1.htm
func (e *Engine) AICreateSimulatedObject_EX1(containerTitle, livery string, initPos types.SIMCONNECT_DATA_INITPOSITION, requestID uint32) error {
	titlePtr, err := helpers.StringToBytePtr(containerTitle)
	if err != nil {
		return fmt.Errorf("invalid container title: %w", err)
	}
	liveryPtr, err := helpers.StringToBytePtr(livery)
	if err != nil {
		return fmt.Errorf("invalid livery: %w", err)
	}
	hresult, _, _ := SimConnect_AICreateSimulatedObject_EX1.Call(
		e.handle,                          // hSimConnect
		titlePtr,                          // szContainerTitle
		liveryPtr,                         // szLivery
		uintptr(unsafe.Pointer(&initPos)), // InitPos
		uintptr(requestID),                // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateSimulatedObject_EX1 failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AIReleaseControl releases AI control of a SimObject so it can be controlled by the SimConnect client.
// objectID: The server-defined object ID to release control of.
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AIReleaseControl.htm
func (e *Engine) AIReleaseControl(objectID uint32, requestID uint32) error {
	hresult, _, _ := SimConnect_AIReleaseControl.Call(
		e.handle,           // hSimConnect
		uintptr(objectID),  // ObjectID
		uintptr(requestID), // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AIReleaseControl failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AIRemoveObject removes an AI controlled object created by the client.
// objectID: The server-defined object ID to remove.
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AIRemoveObject.htm
func (e *Engine) AIRemoveObject(objectID uint32, requestID uint32) error {
	hresult, _, _ := SimConnect_AIRemoveObject.Call(
		e.handle,           // hSimConnect
		uintptr(objectID),  // ObjectID
		uintptr(requestID), // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AIRemoveObject failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AISetAircraftFlightPlan sets or changes the flight plan of an AI controlled aircraft.
// objectID: The server-defined object ID of the AI aircraft.
// flightPlanPath: Path to the flight plan file (no extension needed).
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AISetAircraftFlightPlan.htm
func (e *Engine) AISetAircraftFlightPlan(objectID uint32, flightPlanPath string, requestID uint32) error {
	pathPtr, err := helpers.StringToBytePtr(flightPlanPath)
	if err != nil {
		return fmt.Errorf("invalid flight plan path: %w", err)
	}
	hresult, _, _ := SimConnect_AISetAircraftFlightPlan.Call(
		e.handle,           // hSimConnect
		uintptr(objectID),  // ObjectID
		pathPtr,            // szFlightPlanPath
		uintptr(requestID), // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AISetAircraftFlightPlan failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AICreateEnrouteATCAircraft_EX1 creates an AI controlled enroute ATC aircraft (modular or legacy).
// containerTitle: The title of the aircraft (from aircraft.cfg or SimObject).
// livery: The livery name or folder (can be empty for default livery).
// tailNumber: The tail number (max 12 chars).
// flightNumber: The flight number (negative for none).
// flightPlanPath: Path to the flight plan file (no extension needed).
// flightPlanPosition: Waypoint index and fractional position (e.g., 2.5 for halfway between 3rd and 4th waypoint).
// touchAndGo: True for touch-and-go landings, false for full stop.
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateEnrouteATCAircraft_EX1.htm
func (e *Engine) AICreateEnrouteATCAircraft_EX1(
	containerTitle, livery, tailNumber, flightPlanPath string,
	flightNumber int32,
	flightPlanPosition float64,
	touchAndGo bool,
	requestID uint32,
) error {
	titlePtr, err := helpers.StringToBytePtr(containerTitle)
	if err != nil {
		return fmt.Errorf("invalid container title: %w", err)
	}
	liveryPtr, err := helpers.StringToBytePtr(livery)
	if err != nil {
		return fmt.Errorf("invalid livery: %w", err)
	}
	tailPtr, err := helpers.StringToBytePtr(tailNumber)
	if err != nil {
		return fmt.Errorf("invalid tail number: %w", err)
	}
	planPtr, err := helpers.StringToBytePtr(flightPlanPath)
	if err != nil {
		return fmt.Errorf("invalid flight plan path: %w", err)
	}
	var bTouchAndGo uintptr
	if touchAndGo {
		bTouchAndGo = 1
	} else {
		bTouchAndGo = 0
	}
	hresult, _, _ := SimConnect_AICreateEnrouteATCAircraft_EX1.Call(
		e.handle,              // hSimConnect
		titlePtr,              // szContainerTitle
		liveryPtr,             // szLivery
		tailPtr,               // szTailNumber
		uintptr(flightNumber), // iFlightNumber
		planPtr,               // szFlightPlanPath
		helpers.Float64ToUintptr(flightPlanPosition), // dFlightPlanPosition
		bTouchAndGo,        // bTouchAndGo
		uintptr(requestID), // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateEnrouteATCAircraft_EX1 failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AICreateNonATCAircraft_EX1 creates an AI controlled non-ATC aircraft (modular or legacy).
// containerTitle: The title of the aircraft (from aircraft.cfg or SimObject).
// livery: The livery name or folder (can be empty for default livery).
// tailNumber: The tail number (max 12 chars).
// initPos: The initial position of the aircraft.
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateNonATCAircraft_EX1.htm
func (e *Engine) AICreateNonATCAircraft_EX1(
	containerTitle, livery, tailNumber string,
	initPos types.SIMCONNECT_DATA_INITPOSITION,
	requestID uint32,
) error {
	titlePtr, err := helpers.StringToBytePtr(containerTitle)
	if err != nil {
		return fmt.Errorf("invalid container title: %w", err)
	}
	liveryPtr, err := helpers.StringToBytePtr(livery)
	if err != nil {
		return fmt.Errorf("invalid livery: %w", err)
	}
	tailPtr, err := helpers.StringToBytePtr(tailNumber)
	if err != nil {
		return fmt.Errorf("invalid tail number: %w", err)
	}
	hresult, _, _ := SimConnect_AICreateNonATCAircraft_EX1.Call(
		e.handle,                          // hSimConnect
		titlePtr,                          // szContainerTitle
		liveryPtr,                         // szLivery
		tailPtr,                           // szTailNumber
		uintptr(unsafe.Pointer(&initPos)), // InitPos
		uintptr(requestID),                // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateNonATCAircraft_EX1 failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// AICreateParkedATCAircraft_EX1 creates an AI controlled parked ATC aircraft (modular or legacy).
// containerTitle: The title of the aircraft (from aircraft.cfg or SimObject).
// livery: The livery name or folder (can be empty for default livery).
// tailNumber: The tail number (max 12 chars).
// airportID: The ICAO code of the airport (e.g., KSEA).
// requestID: Client-defined request ID for tracking the response.
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/AI_Object/SimConnect_AICreateParkedATCAircraft_EX1.htm
func (e *Engine) AICreateParkedATCAircraft_EX1(
	containerTitle, livery, tailNumber, airportID string,
	requestID uint32,
) error {
	titlePtr, err := helpers.StringToBytePtr(containerTitle)
	if err != nil {
		return fmt.Errorf("invalid container title: %w", err)
	}
	liveryPtr, err := helpers.StringToBytePtr(livery)
	if err != nil {
		return fmt.Errorf("invalid livery: %w", err)
	}
	tailPtr, err := helpers.StringToBytePtr(tailNumber)
	if err != nil {
		return fmt.Errorf("invalid tail number: %w", err)
	}
	airportPtr, err := helpers.StringToBytePtr(airportID)
	if err != nil {
		return fmt.Errorf("invalid airport ID: %w", err)
	}
	hresult, _, _ := SimConnect_AICreateParkedATCAircraft_EX1.Call(
		e.handle,           // hSimConnect
		titlePtr,           // szContainerTitle
		liveryPtr,          // szLivery
		tailPtr,            // szTailNumber
		airportPtr,         // szAirportID
		uintptr(requestID), // RequestID
	)
	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AICreateParkedATCAircraft_EX1 failed: 0x%08X", uint32(hresult))
	}
	return nil
}
