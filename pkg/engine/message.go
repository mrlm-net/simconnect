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

	data []byte // Internal field to keep the copied data alive
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

func (m *Message) AsEventFrame() *types.SIMCONNECT_RECV_EVENT_FRAME {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_EVENT_FRAME {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_FRAME)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsEventFilename() *types.SIMCONNECT_RECV_EVENT_FILENAME {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_EVENT_FILENAME {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_FILENAME)(unsafe.Pointer(m.SIMCONNECT_RECV))
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

func (m *Message) AsSimObjectAndLiveryEnumeration() *types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
		return nil
	}
	return (*types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsFacilityData() *types.SIMCONNECT_RECV_FACILITY_DATA {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_FACILITY_DATA {
		return nil
	}
	return (*types.SIMCONNECT_RECV_FACILITY_DATA)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsFacilityList() *types.SIMCONNECT_RECV_FACILITIES_LIST {
	if types.SIMCONNECT_RECV_ID(m.DwID) == types.SIMCONNECT_RECV_ID_AIRPORT_LIST ||
		types.SIMCONNECT_RECV_ID(m.DwID) == types.SIMCONNECT_RECV_ID_VOR_LIST ||
		types.SIMCONNECT_RECV_ID(m.DwID) == types.SIMCONNECT_RECV_ID_NDB_LIST ||
		types.SIMCONNECT_RECV_ID(m.DwID) == types.SIMCONNECT_RECV_ID_WAYPOINT_LIST {
		return (*types.SIMCONNECT_RECV_FACILITIES_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))

	}
	return nil
}

func (m *Message) AsAirportList() *types.SIMCONNECT_RECV_AIRPORT_LIST {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_AIRPORT_LIST {
		return nil
	}
	return (*types.SIMCONNECT_RECV_AIRPORT_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsNDBList() *types.SIMCONNECT_RECV_NDB_LIST {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_NDB_LIST {
		return nil
	}
	return (*types.SIMCONNECT_RECV_NDB_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsVORList() *types.SIMCONNECT_RECV_VOR_LIST {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_VOR_LIST {
		return nil
	}
	return (*types.SIMCONNECT_RECV_VOR_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsWaypointList() *types.SIMCONNECT_RECV_WAYPOINT_LIST {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_WAYPOINT_LIST {
		return nil
	}
	return (*types.SIMCONNECT_RECV_WAYPOINT_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsException() *types.SIMCONNECT_RECV_EXCEPTION {
	if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_EXCEPTION {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EXCEPTION)(unsafe.Pointer(m.SIMCONNECT_RECV))
}
