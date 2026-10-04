package types

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_INPUT_EVENT_TYPE.htm#h
type SIMCONNECT_INPUT_EVENT_TYPE DWORD

const (
	SIMCONNECT_INPUT_EVENT_TYPE_NONE SIMCONNECT_INPUT_EVENT_TYPE = iota
	SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE
	SIMCONNECT_INPUT_EVENT_TYPE_STRING
)

// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_INPUT_EVENT_DESCRIPTOR.htm
type SIMCONNECT_INPUT_EVENT_DESCRIPTOR struct {
	Name      [64]byte            // SIMCONNECT_STRING(Name, 64);
	Hash      DWORD               // DWORD Hash;
	Type      SIMCONNECT_DATATYPE // SIMCONNECT_DATATYPE Type;
	NodeNames [1024]byte          // SIMCONNECT_STRING(NodeNames, 1024);
}

type SIMCONNECT_EVENT_FLAG uint32

// Event flags are bit flags (SimConnect.h), not a sequence.
const (
	SIMCONNECT_EVENT_FLAG_DEFAULT             SIMCONNECT_EVENT_FLAG = 0x00000000
	SIMCONNECT_EVENT_FLAG_FAST_REPEAT_TIMER   SIMCONNECT_EVENT_FLAG = 0x00000001 // set event to repeat at a fast rate (4 per second)
	SIMCONNECT_EVENT_FLAG_SLOW_REPEAT_TIMER   SIMCONNECT_EVENT_FLAG = 0x00000002 // set event to repeat at a slow rate (1 per second)
	SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY SIMCONNECT_EVENT_FLAG = 0x00000010 // interpret GroupID parameter as priority value
)

// ConnectionOpenData contains information passed when the connection opens
type ConnectionOpenData struct {
	ApplicationName         string
	ApplicationVersionMajor uint32
	ApplicationVersionMinor uint32
	ApplicationBuildMajor   uint32
	ApplicationBuildMinor   uint32
	SimConnectVersionMajor  uint32
	SimConnectVersionMinor  uint32
	SimConnectBuildMajor    uint32
	SimConnectBuildMinor    uint32
}

// ConnectionQuitData contains information passed when the connection closes
type ConnectionQuitData struct {
}
