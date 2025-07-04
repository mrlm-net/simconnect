//go:build windows
// +build windows

package client

import "syscall"

var (
	// SimConnect connection procedures
	SimConnect_Open  *syscall.LazyProc
	SimConnect_Close *syscall.LazyProc

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
)

func (e *Engine) bootstrapProcedures() {
	// SimConnect connection procedures
	SimConnect_Open = e.dll.NewProc("SimConnect_Open")
	SimConnect_Close = e.dll.NewProc("SimConnect_Close")

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
}
