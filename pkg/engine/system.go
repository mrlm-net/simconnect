//go:build windows
// +build windows

package engine

import (
	"encoding/binary"
	"math"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func (e *Engine) RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error {
	return e.api.RequestSystemState(requestID, state)
}

func (e *Engine) SubscribeToSystemEvent(eventID uint32, eventName string) error {
	return e.api.SubscribeToSystemEvent(eventID, eventName)
}

func (e *Engine) UnsubscribeFromSystemEvent(eventID uint32) error {
	return e.api.UnsubscribeFromSystemEvent(eventID)
}

func (e *Engine) SetSystemEventState(eventID uint32, state types.SIMCONNECT_STATE) error {
	return e.api.SetSystemEventState(eventID, state)
}

// SystemStateFloat64 extracts the float value from a SYSTEM_STATE receive struct.
// On the wire fFloat is a 4-byte float at offset 20; it is widened to float64.
func SystemStateFloat64(recv *types.SIMCONNECT_RECV_SYSTEM_STATE) float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(recv.FFloatBytes[:])))
}

// GetLastSentPacketID returns the send ID of the last request this client
// sent to SimConnect. SimConnect reports errors asynchronously as
// SIMCONNECT_RECV_EXCEPTION messages whose DwSendID is this value, so record
// it right after a call to attribute a later exception to that call.
//
// Call it on the same goroutine immediately after the request: a request sent
// from another goroutine in between would be reported instead.
func (e *Engine) GetLastSentPacketID() (uint32, error) {
	return e.api.GetLastSentPacketID()
}

// CallFor is the call that had send ID id, with its arguments, when the
// engine traces calls (WithCallTrace) and still remembers it: what an
// exception's DwSendID was raised for.
func (e *Engine) CallFor(id uint32) (string, bool) {
	if t, ok := e.api.(interface{ CallFor(uint32) (string, bool) }); ok {
		return t.CallFor(id)
	}
	return "", false
}
