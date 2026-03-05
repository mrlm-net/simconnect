//go:build windows
// +build windows

package engine

import (
	"encoding/binary"
	"math"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func (e *Engine) EnumerateInputEvents(requestID uint32) error {
	return e.api.EnumerateInputEvents(requestID)
}

func (e *Engine) GetInputEvent(requestID uint32, hash uint64) error {
	return e.api.GetInputEvent(requestID, hash)
}

// SetInputEventDouble sets a DOUBLE-typed input event value.
// The float64 is stack-allocated; its address is valid for the duration of the synchronous DLL call.
func (e *Engine) SetInputEventDouble(hash uint64, value float64) error {
	return e.api.SetInputEvent(hash, unsafe.Pointer(&value))
}

// SetInputEventString sets a STRING-typed input event value.
// value is copied into a 260-byte null-terminated buffer. Strings longer than 259
// bytes are silently truncated to 259 bytes to preserve the null terminator at buf[259].
func (e *Engine) SetInputEventString(hash uint64, value string) error {
	var buf [260]byte
	copy(buf[:259], value) // reserve buf[259] as null terminator
	return e.api.SetInputEvent(hash, unsafe.Pointer(&buf[0]))
}

func (e *Engine) SubscribeInputEvent(hash uint64) error {
	return e.api.SubscribeInputEvent(hash)
}

func (e *Engine) UnsubscribeInputEvent(hash uint64) error {
	return e.api.UnsubscribeInputEvent(hash)
}

// bytesAsFloat64 interprets the first 8 bytes of b as a little-endian IEEE 754 float64.
func bytesAsFloat64(b []byte) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(b[:8]))
}

// InputEventValueAsFloat64 extracts the float64 value from a GET_INPUT_EVENT receive struct.
// Returns (0, false) if Type is not SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE.
func InputEventValueAsFloat64(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (float64, bool) {
	if recv.Type != types.SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE {
		return 0, false
	}
	return bytesAsFloat64(recv.Value[:]), true
}

// InputEventValueAsString extracts the string value from a GET_INPUT_EVENT receive struct.
// Returns ("", false) if Type is not SIMCONNECT_INPUT_EVENT_TYPE_STRING.
func InputEventValueAsString(recv *types.SIMCONNECT_RECV_GET_INPUT_EVENT) (string, bool) {
	if recv.Type != types.SIMCONNECT_INPUT_EVENT_TYPE_STRING {
		return "", false
	}
	return BytesToString(recv.Value[:]), true
}

// SubscribeInputEventHash extracts the event hash from a SUBSCRIBE_INPUT_EVENT receive struct.
// HashBytes is stored as [8]byte at wire offset 12 to avoid Go alignment padding.
func SubscribeInputEventHash(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) uint64 {
	return binary.LittleEndian.Uint64(recv.HashBytes[:])
}

// SubscribeInputEventValueAsFloat64 extracts the float64 value from a SUBSCRIBE_INPUT_EVENT receive struct.
// Returns (0, false) if EType is not SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE.
func SubscribeInputEventValueAsFloat64(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (float64, bool) {
	if recv.EType != types.SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE {
		return 0, false
	}
	return bytesAsFloat64(recv.Value[:]), true
}

// SubscribeInputEventValueAsString extracts the string value from a SUBSCRIBE_INPUT_EVENT receive struct.
// Returns ("", false) if EType is not SIMCONNECT_INPUT_EVENT_TYPE_STRING.
func SubscribeInputEventValueAsString(recv *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT) (string, bool) {
	if recv.EType != types.SIMCONNECT_INPUT_EVENT_TYPE_STRING {
		return "", false
	}
	return BytesToString(recv.Value[:]), true
}
