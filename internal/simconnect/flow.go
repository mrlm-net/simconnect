//go:build windows
// +build windows

package simconnect

import "fmt"

// SubscribeToFlowEvent subscribes to all simulator flow events. Whenever a flow
// event fires, the dispatcher delivers a SIMCONNECT_RECV_FLOW_EVENT message.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_SubscribeToFlowEvent.htm
func (sc *SimConnect) SubscribeToFlowEvent() error {
	procedure := sc.library.LoadProcedure("SimConnect_SubscribeToFlowEvent")

	hresult, _, _ := procedure.Call(sc.getConnection())

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SubscribeToFlowEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}

// UnsubscribeFromFlowEvent cancels the active flow event subscription.
// After this call the dispatcher will no longer deliver SIMCONNECT_RECV_FLOW_EVENT messages.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Events_And_Data/SimConnect_UnsubscribeToFlowEvent.htm
func (sc *SimConnect) UnsubscribeFromFlowEvent() error {
	procedure := sc.library.LoadProcedure("SimConnect_UnsubscribeToFlowEvent")

	hresult, _, _ := procedure.Call(sc.getConnection())

	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_UnsubscribeToFlowEvent failed with HRESULT: 0x%08X", uint32(hresult))
	}
	return nil
}
