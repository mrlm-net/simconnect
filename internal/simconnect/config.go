//go:build windows
// +build windows

package simconnect

import "context"

type Config struct {
	BufferSize int
	Context    context.Context
	DLLPath    string
	AutoDetect bool
	// TraceCalls keeps the last calls with their send IDs, so an exception
	// names the call it was for (SimConnect.CallFor). Costs a
	// GetLastSentPacketID per call.
	TraceCalls bool
	// DispatchEvent opens the connection with an event SimConnect sets
	// when a message waits (WaitDispatch), instead of polling (#855 E6).
	DispatchEvent bool
}
