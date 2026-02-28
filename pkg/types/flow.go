//go:build windows
// +build windows

package types

import "fmt"

// SIMCONNECT_FLOW_EVENT identifies the category of simulator flow event delivered
// in a SIMCONNECT_RECV_FLOW_EVENT message.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_FLOW_EVENT.htm
type SIMCONNECT_FLOW_EVENT DWORD

const (
	SIMCONNECT_FLOW_EVENT_NONE              SIMCONNECT_FLOW_EVENT = iota // Should never be received.
	SIMCONNECT_FLOW_EVENT_FLT_LOAD                                       // A .flt file is about to be loaded.
	SIMCONNECT_FLOW_EVENT_FLT_LOADED                                     // A .flt file has been loaded; aircraft is in its new state.
	SIMCONNECT_FLOW_EVENT_TELEPORT_START                                 // A teleport action is about to execute.
	SIMCONNECT_FLOW_EVENT_TELEPORT_DONE                                  // A teleport action has completed.
	SIMCONNECT_FLOW_EVENT_BACK_ON_TRACK_START                            // A back-on-track operation is about to execute.
	SIMCONNECT_FLOW_EVENT_BACK_ON_TRACK_DONE                             // A back-on-track operation has completed.
	SIMCONNECT_FLOW_EVENT_SKIP_START                                     // An activity section is about to be skipped.
	SIMCONNECT_FLOW_EVENT_SKIP_DONE                                      // An activity section has been skipped.
	SIMCONNECT_FLOW_EVENT_BACK_TO_MAIN_MENU                              // The simulator is returning to the main menu.
	SIMCONNECT_FLOW_EVENT_RTC_START                                      // A real-time challenge has started.
	SIMCONNECT_FLOW_EVENT_RTC_END                                        // A real-time challenge has ended.
	SIMCONNECT_FLOW_EVENT_REPLAY_START                                   // A replay has started.
	SIMCONNECT_FLOW_EVENT_REPLAY_END                                     // A replay has ended.
	SIMCONNECT_FLOW_EVENT_FLIGHT_START                                   // A flight session has started.
	SIMCONNECT_FLOW_EVENT_FLIGHT_END                                     // A flight session has ended.
	SIMCONNECT_FLOW_EVENT_PLANE_CRASH                                    // An aircraft has crashed.
)

// String returns the human-readable name of the flow event.
func (e SIMCONNECT_FLOW_EVENT) String() string {
	switch e {
	case SIMCONNECT_FLOW_EVENT_NONE:
		return "NONE"
	case SIMCONNECT_FLOW_EVENT_FLT_LOAD:
		return "FLT_LOAD"
	case SIMCONNECT_FLOW_EVENT_FLT_LOADED:
		return "FLT_LOADED"
	case SIMCONNECT_FLOW_EVENT_TELEPORT_START:
		return "TELEPORT_START"
	case SIMCONNECT_FLOW_EVENT_TELEPORT_DONE:
		return "TELEPORT_DONE"
	case SIMCONNECT_FLOW_EVENT_BACK_ON_TRACK_START:
		return "BACK_ON_TRACK_START"
	case SIMCONNECT_FLOW_EVENT_BACK_ON_TRACK_DONE:
		return "BACK_ON_TRACK_DONE"
	case SIMCONNECT_FLOW_EVENT_SKIP_START:
		return "SKIP_START"
	case SIMCONNECT_FLOW_EVENT_SKIP_DONE:
		return "SKIP_DONE"
	case SIMCONNECT_FLOW_EVENT_BACK_TO_MAIN_MENU:
		return "BACK_TO_MAIN_MENU"
	case SIMCONNECT_FLOW_EVENT_RTC_START:
		return "RTC_START"
	case SIMCONNECT_FLOW_EVENT_RTC_END:
		return "RTC_END"
	case SIMCONNECT_FLOW_EVENT_REPLAY_START:
		return "REPLAY_START"
	case SIMCONNECT_FLOW_EVENT_REPLAY_END:
		return "REPLAY_END"
	case SIMCONNECT_FLOW_EVENT_FLIGHT_START:
		return "FLIGHT_START"
	case SIMCONNECT_FLOW_EVENT_FLIGHT_END:
		return "FLIGHT_END"
	case SIMCONNECT_FLOW_EVENT_PLANE_CRASH:
		return "PLANE_CRASH"
	default:
		return fmt.Sprintf("SIMCONNECT_FLOW_EVENT(%d)", uint32(e))
	}
}

// SIMCONNECT_RECV_FLOW_EVENT is delivered when a simulator flow event fires and the
// client has called SimConnect_SubscribeToFlowEvent. It reports which event occurred
// and the virtual file system path of the .flt file currently loaded (if any).
//
// Note: MSFS 2024 only — SimConnect_SubscribeToFlowEvent is not available in MSFS 2020.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_RECV_FLOW_EVENT.htm
type SIMCONNECT_RECV_FLOW_EVENT struct {
	SIMCONNECT_RECV                  // Base header: DwSize, DwVersion, DwID.
	FlowEvent      SIMCONNECT_FLOW_EVENT // Category of the flow event.
	FltPath        [256]byte             // VFS path of the currently loaded .flt file (null-terminated). Empty when not applicable.
}
