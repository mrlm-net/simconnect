//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/pkg/types"

func (e *Engine) AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error {
	return e.api.AICreateParkedATCAircraft(szContainerTitle, szTailNumber, szAirportID, RequestID)
}

func (e *Engine) AISetAircraftFlightPlan(objectID uint32, szFlightPlanPath string, requestID uint32) error {
	return e.api.AISetAircraftFlightPlan(objectID, szFlightPlanPath, requestID)
}

func (e *Engine) AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	return e.api.AICreateEnrouteATCAircraft(szContainerTitle, szTailNumber, iFlightNumber, szFlightPlanPath, dFlightPlanPosition, bTouchAndGo, RequestID)
}

func (e *Engine) AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	return e.api.AICreateNonATCAircraft(szContainerTitle, szTailNumber, initPos, RequestID)
}

func (e *Engine) AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	return e.api.AICreateSimulatedObject(szContainerTitle, initPos, RequestID)
}

func (e *Engine) AIReleaseControl(objectID uint32, requestID uint32) error {
	return e.api.AIReleaseControl(objectID, requestID)
}

func (e *Engine) EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	return e.api.EnumerateSimObjectsAndLiveries(requestID, objectType)
}

func (e *Engine) AIRemoveObject(objectID uint32, requestID uint32) error {
	return e.api.AIRemoveObject(objectID, requestID)
}

func (e *Engine) AICreateEnrouteATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	return e.api.AICreateEnrouteATCAircraftEX1(szContainerTitle, szLivery, szTailNumber, iFlightNumber, szFlightPlanPath, dFlightPlanPosition, bTouchAndGo, RequestID)
}

func (e *Engine) AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	return e.api.AICreateNonATCAircraftEX1(szContainerTitle, szLivery, szTailNumber, initPos, RequestID)
}

func (e *Engine) AICreateParkedATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, szAirportID string, RequestID uint32) error {
	return e.api.AICreateParkedATCAircraftEX1(szContainerTitle, szLivery, szTailNumber, szAirportID, RequestID)
}
