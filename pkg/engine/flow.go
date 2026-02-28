//go:build windows
// +build windows

package engine

// SubscribeToFlowEvent subscribes to all simulator flow events.
// Once subscribed, SIMCONNECT_RECV_FLOW_EVENT messages are delivered via Stream().
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
func (e *Engine) SubscribeToFlowEvent() error {
	return e.api.SubscribeToFlowEvent()
}

// UnsubscribeFromFlowEvent cancels the active flow event subscription.
//
// Note: MSFS 2024 only — returns an error on MSFS 2020.
func (e *Engine) UnsubscribeFromFlowEvent() error {
	return e.api.UnsubscribeFromFlowEvent()
}
