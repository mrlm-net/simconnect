//go:build windows
// +build windows

package types

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/SimConnect_API_Reference.htm#simconnect-priorities
type SIMCONNECT_GROUP_PRIORITY uint32

const (
	SIMCONNECT_GROUP_PRIORITY_HIGHEST  = 1
	SIMCONNECT_GROUP_PRIORITY_STANDARD = 1900000000
	SIMCONNECT_GROUP_PRIORITY_DEFAULT  = 2000000000
	SIMCONNECT_GROUP_PRIORITY_LOWEST   = 4000000000
)
