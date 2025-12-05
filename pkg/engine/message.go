//go:build windows
// +build windows

package engine

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type Message struct {
	*types.SIMCONNECT_RECV
	Size uint32
	Err  error
}

func CastAs[T any](m *Message) T {
	switch types.SIMCONNECT_RECV_ID(m.DwID) {
	case types.SIMCONNECT_RECV_ID_EVENT:
		return any((*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(&m.SIMCONNECT_RECV))).(T)
	}
	var zero T
	return zero
}

func (m *Message) AsEventType() *types.SIMCONNECT_RECV_EVENT {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(&m.SIMCONNECT_RECV))
}
