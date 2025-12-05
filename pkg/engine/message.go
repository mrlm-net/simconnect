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
		return any((*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(m.SIMCONNECT_RECV))).(T)
	}
	var zero T
	return zero
}

// CastData casts the DwData field from a SimObject data response to the specified struct type.
// The type T must match the data definition structure registered with SimConnect.
func CastDataAs[T any](dwData *types.DWORD) *T {
	return (*T)(unsafe.Pointer(dwData))
}

func BytesToString(data []byte) string {
	for i, b := range data {
		if b == 0 {
			return string(data[:i])
		}
	}
	return string(data)
}

func (m *Message) AsEvent() *types.SIMCONNECT_RECV_EVENT {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_EVENT {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsOpen() *types.SIMCONNECT_RECV_OPEN {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_OPEN {
		return nil
	}
	return (*types.SIMCONNECT_RECV_OPEN)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsSimObjectData() *types.SIMCONNECT_RECV_SIMOBJECT_DATA {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsSimObjectDataBType() *types.SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE)(unsafe.Pointer(m.SIMCONNECT_RECV))
}
