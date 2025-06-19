//go:build windows
// +build windows

package types

// SIMCONNECT_SIMOBJECT_TYPE defines the type of simulation object.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_SIMOBJECT_TYPE.htm
type SIMCONNECT_SIMOBJECT_TYPE uint32

const (
	SIMCONNECT_SIMOBJECT_TYPE_USER SIMCONNECT_SIMOBJECT_TYPE = iota
	SIMCONNECT_SIMOBJECT_TYPE_ALL
	SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT
	SIMCONNECT_SIMOBJECT_TYPE_HELICOPTER
	SIMCONNECT_SIMOBJECT_TYPE_BOAT
	SIMCONNECT_SIMOBJECT_TYPE_GROUND
)

// SIMCONNECT_OBJECT_ID_USER is the object ID for the user aircraft.
// OBJECT ID -> https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestDataOnSimObject.htm
const (
	SIMCONNECT_OBJECT_ID_USER uint32 = iota
)
