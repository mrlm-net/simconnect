//go:build windows
// +build windows

package client

import "syscall"

var (
	// SimConnect connection procedures
	SimConnect_Open  *syscall.LazyProc
	SimConnect_Close *syscall.LazyProc

	// SimConnect Execution procedures
	SimConnect_ExecuteAction *syscall.LazyProc

	// SimConnect message handling procedures
	SimConnect_CallDispatch    *syscall.LazyProc
	SimConnect_GetNextDispatch *syscall.LazyProc

	// SimConnect System state procedure
	SimConnect_RequestSystemState         *syscall.LazyProc
	SimConnect_SubscribeToSystemEvent     *syscall.LazyProc
	SimConnect_UnsubscribeFromSystemEvent *syscall.LazyProc

	// SimConnect procedures for setting up and getting values of simvars
	SimConnect_AddToDataDefinition        *syscall.LazyProc
	SimConnect_ClearDataDefinition        *syscall.LazyProc
	SimConnect_RequestDataOnSimObject     *syscall.LazyProc
	SimConnect_RequestDataOnSimObjectType *syscall.LazyProc
	SimConnect_SetDataOnSimObject         *syscall.LazyProc

	// SimConnect procedures for sending and receiving events
	SimConnect_MapClientEventToSimEvent          *syscall.LazyProc
	SimConnect_AddClientEventToNotificationGroup *syscall.LazyProc
	SimConnect_SetNotificationGroupPriority      *syscall.LazyProc
	SimConnect_TransmitClientEvent               *syscall.LazyProc
	SimConnect_TransmitClientEvent_EX1           *syscall.LazyProc
	SimConnect_RemoveClientEvent                 *syscall.LazyProc
	SimConnect_ClearNotificationGroup            *syscall.LazyProc
	SimConnect_RequestNotificationGroup          *syscall.LazyProc

	// SimConnect procedures for client data management
	SimConnect_MapClientDataNameToID     *syscall.LazyProc
	SimConnect_CreateClientData          *syscall.LazyProc
	SimConnect_AddToClientDataDefinition *syscall.LazyProc
	SimConnect_ClearClientDataDefinition *syscall.LazyProc
	SimConnect_RequestClientData         *syscall.LazyProc
	SimConnect_SetClientData             *syscall.LazyProc

	// SimConnect procedures for input event management
	SimConnect_ClearInputGroup       *syscall.LazyProc
	SimConnect_RequestReservedKey    *syscall.LazyProc
	SimConnect_SetInputGroupPriority *syscall.LazyProc
	SimConnect_SetInputGroupState    *syscall.LazyProc
	SimConnect_RemoveInputEvent      *syscall.LazyProc

	// SimConnect procedures for managing facilities
	SimConnect_AddToFacilityDefinition         *syscall.LazyProc
	SimConnect_RequestFacilitesList            *syscall.LazyProc
	SimConnect_RequestFacilityData             *syscall.LazyProc
	SimConnect_AddFacilityDataDefinitionFilter *syscall.LazyProc
	SimConnect_SubscribeToFacilities           *syscall.LazyProc
	SimConnect_UnsubscribeToFacilities         *syscall.LazyProc

	// SimConnect procedures for flight operations
	SimConnect_FlightLoad     *syscall.LazyProc
	SimConnect_FlightSave     *syscall.LazyProc
	SimConnect_FlightPlanLoad *syscall.LazyProc

	// SimConnect procedures for debugging and diagnostics
	SimConnect_GetLastSentPacketID  *syscall.LazyProc
	SimConnect_RequestResponseTimes *syscall.LazyProc

	// SimConnect AI objects procedures
	SimConnect_EnumerateSimObjectsAndLiveries *syscall.LazyProc
	SimConnect_AICreateSimulatedObject_EX1    *syscall.LazyProc
	SimConnect_AIReleaseControl               *syscall.LazyProc
	SimConnect_AIRemoveObject                 *syscall.LazyProc
	SimConnect_AISetAircraftFlightPlan        *syscall.LazyProc
	SimConnect_AICreateEnrouteATCAircraft_EX1 *syscall.LazyProc
	SimConnect_AICreateNonATCAircraft_EX1     *syscall.LazyProc
	SimConnect_AICreateParkedATCAircraft_EX1  *syscall.LazyProc
)

func (e *Engine) bootstrapProcedures() {
	// SimConnect connection procedures
	SimConnect_Open = e.dll.NewProc("SimConnect_Open")
	SimConnect_Close = e.dll.NewProc("SimConnect_Close")

	// SimConnect Execution procedures
	SimConnect_ExecuteAction = e.dll.NewProc("SimConnect_ExecuteAction")

	// SimConnect message handling procedures
	SimConnect_CallDispatch = e.dll.NewProc("SimConnect_CallDispatch")
	SimConnect_GetNextDispatch = e.dll.NewProc("SimConnect_GetNextDispatch")

	// SimConnect System state and events procedures
	SimConnect_RequestSystemState = e.dll.NewProc("SimConnect_RequestSystemState")
	SimConnect_SubscribeToSystemEvent = e.dll.NewProc("SimConnect_SubscribeToSystemEvent")
	SimConnect_UnsubscribeFromSystemEvent = e.dll.NewProc("SimConnect_UnsubscribeFromSystemEvent")

	// SimConnect procedures for setting up and getting values of simvars
	SimConnect_AddToDataDefinition = e.dll.NewProc("SimConnect_AddToDataDefinition")
	SimConnect_ClearDataDefinition = e.dll.NewProc("SimConnect_ClearDataDefinition")
	SimConnect_RequestDataOnSimObject = e.dll.NewProc("SimConnect_RequestDataOnSimObject")
	SimConnect_RequestDataOnSimObjectType = e.dll.NewProc("SimConnect_RequestDataOnSimObjectType")
	SimConnect_SetDataOnSimObject = e.dll.NewProc("SimConnect_SetDataOnSimObject")

	// SimConnect procedures for sending and receiving events
	SimConnect_TransmitClientEvent = e.dll.NewProc("SimConnect_TransmitClientEvent")
	SimConnect_MapClientEventToSimEvent = e.dll.NewProc("SimConnect_MapClientEventToSimEvent")
	SimConnect_AddClientEventToNotificationGroup = e.dll.NewProc("SimConnect_AddClientEventToNotificationGroup")
	SimConnect_SetNotificationGroupPriority = e.dll.NewProc("SimConnect_SetNotificationGroupPriority")
	SimConnect_TransmitClientEvent_EX1 = e.dll.NewProc("SimConnect_TransmitClientEvent_EX1")
	SimConnect_RemoveClientEvent = e.dll.NewProc("SimConnect_RemoveClientEvent")
	SimConnect_ClearNotificationGroup = e.dll.NewProc("SimConnect_ClearNotificationGroup")
	SimConnect_RequestNotificationGroup = e.dll.NewProc("SimConnect_RequestNotificationGroup")

	// SimConnect procedures for client data management
	SimConnect_MapClientDataNameToID = e.dll.NewProc("SimConnect_MapClientDataNameToID")
	SimConnect_CreateClientData = e.dll.NewProc("SimConnect_CreateClientData")
	SimConnect_AddToClientDataDefinition = e.dll.NewProc("SimConnect_AddToClientDataDefinition")
	SimConnect_ClearClientDataDefinition = e.dll.NewProc("SimConnect_ClearClientDataDefinition")
	SimConnect_RequestClientData = e.dll.NewProc("SimConnect_RequestClientData")
	SimConnect_SetClientData = e.dll.NewProc("SimConnect_SetClientData")

	// SimConnect procedures for input event management
	SimConnect_ClearInputGroup = e.dll.NewProc("SimConnect_ClearInputGroup")
	SimConnect_RequestReservedKey = e.dll.NewProc("SimConnect_RequestReservedKey")
	SimConnect_SetInputGroupPriority = e.dll.NewProc("SimConnect_SetInputGroupPriority")
	SimConnect_SetInputGroupState = e.dll.NewProc("SimConnect_SetInputGroupState")
	SimConnect_RemoveInputEvent = e.dll.NewProc("SimConnect_RemoveInputEvent")

	// SimConnect procedures for managing facilities
	SimConnect_AddToFacilityDefinition = e.dll.NewProc("SimConnect_AddToFacilityDefinition")
	SimConnect_RequestFacilitesList = e.dll.NewProc("SimConnect_RequestFacilitiesList")
	SimConnect_RequestFacilityData = e.dll.NewProc("SimConnect_RequestFacilityData")
	SimConnect_AddFacilityDataDefinitionFilter = e.dll.NewProc("SimConnect_AddFacilityDataDefinitionFilter")
	SimConnect_SubscribeToFacilities = e.dll.NewProc("SimConnect_SubscribeToFacilities")
	SimConnect_UnsubscribeToFacilities = e.dll.NewProc("SimConnect_UnsubscribeToFacilities")

	// SimConnect procedures for flight operations
	SimConnect_FlightLoad = e.dll.NewProc("SimConnect_FlightLoad")
	SimConnect_FlightSave = e.dll.NewProc("SimConnect_FlightSave")
	SimConnect_FlightPlanLoad = e.dll.NewProc("SimConnect_FlightPlanLoad")

	// SimConnect procedures for debigging and diagnostics
	SimConnect_GetLastSentPacketID = e.dll.NewProc("SimConnect_GetLastSentPacketID")
	SimConnect_RequestResponseTimes = e.dll.NewProc("SimConnect_RequestResponseTimes")

	// SimConnect AI objects procedures
	SimConnect_EnumerateSimObjectsAndLiveries = e.dll.NewProc("SimConnect_EnumerateSimObjectsAndLiveries")
	SimConnect_AICreateSimulatedObject_EX1 = e.dll.NewProc("SimConnect_AICreateSimulatedObject_EX1")
	SimConnect_AIReleaseControl = e.dll.NewProc("SimConnect_AIReleaseControl")
	SimConnect_AIRemoveObject = e.dll.NewProc("SimConnect_AIRemoveObject")
	SimConnect_AISetAircraftFlightPlan = e.dll.NewProc("SimConnect_AI")
	SimConnect_AICreateEnrouteATCAircraft_EX1 = e.dll.NewProc("SimConnect_AICreateEnrouteATCAircraft_EX1")
	SimConnect_AICreateNonATCAircraft_EX1 = e.dll.NewProc("SimConnect_AICreateNonATCAircraft_EX1")
	SimConnect_AICreateParkedATCAircraft_EX1 = e.dll.NewProc("SimConnect_AICreateParkedATCAircraft_EX1")

}
