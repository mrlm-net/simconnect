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

// SystemStateFloat64 extracts the float64 value from a SYSTEM_STATE receive struct.
// FFloatBytes is stored as [8]byte at wire offset 20 to avoid Go alignment padding
// (float64 after 12+4+4 bytes would be padded to offset 24 by Go).
func SystemStateFloat64(recv *types.SIMCONNECT_RECV_SYSTEM_STATE) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(recv.FFloatBytes[:]))
}
