package engine

import (
	"encoding/binary"
	"math"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// waypointWireSize is the packed size of SIMCONNECT_DATA_WAYPOINT in the
// SimConnect wire format (C #pragma pack(1), no alignment padding).
const waypointWireSize = 44

// PackWaypoints serialises a slice of SIMCONNECT_DATA_WAYPOINT into the
// 44-byte-per-element packed wire format that SimConnect expects.
//
// The Go struct is 48 bytes due to 4 bytes of implicit padding inserted after
// the uint32 Flags field to align the following float64.  SimConnect's C header
// uses #pragma pack(1) so that padding does not exist on the wire.
//
// Use the returned slice with SetDataOnSimObject:
//
//	packed := engine.PackWaypoints(wps)
//	client.SetDataOnSimObject(defID, objID, flag,
//	    uint32(len(wps)), engine.WaypointWireSize, unsafe.Pointer(&packed[0]))
func PackWaypoints(wps []types.SIMCONNECT_DATA_WAYPOINT) []byte {
	buf := make([]byte, len(wps)*waypointWireSize)
	for i, wp := range wps {
		b := buf[i*waypointWireSize:]
		binary.LittleEndian.PutUint64(b[0:], math.Float64bits(wp.Latitude))
		binary.LittleEndian.PutUint64(b[8:], math.Float64bits(wp.Longitude))
		binary.LittleEndian.PutUint64(b[16:], math.Float64bits(wp.Altitude))
		binary.LittleEndian.PutUint32(b[24:], wp.Flags)
		binary.LittleEndian.PutUint64(b[28:], math.Float64bits(wp.KtsSpeed))
		binary.LittleEndian.PutUint64(b[36:], math.Float64bits(wp.PercentThrottle))
	}
	return buf
}

// WaypointWireSize is the packed wire size per SIMCONNECT_DATA_WAYPOINT element.
const WaypointWireSize = waypointWireSize

type Message struct {
	*types.SIMCONNECT_RECV
	Size uint32
	Err  error

	data    []byte    // Internal field to keep the copied data alive
	release *releaser // Internal: returns the buffer to the pool, shared by copies
}

// releaser returns a message's buffer to its pool once, however many copies
// of the Message call Release (Message is passed by value).
type releaser struct {
	once sync.Once
	fn   func()
}

// newMessage creates a new Message with pooled buffer.
// IMPORTANT: Callers should call Release() when done with the message to return
// the buffer to the pool. If Release() is not called, the buffer will be garbage
// collected but pool efficiency will be reduced under high load.
func newMessage(recv *types.SIMCONNECT_RECV, size uint32, err error, data []byte, release func()) Message {
	m := Message{
		SIMCONNECT_RECV: recv,
		Size:            size,
		Err:             err,
		data:            data,
	}
	if release != nil {
		m.release = &releaser{fn: release}
	}
	return m
}

// Release returns the message's buffer to the appropriate pool.
// Call this when the message is no longer needed for best performance.
// Safe to call multiple times, also on copies of the message: the buffer
// is returned once.
func (m *Message) Release() {
	if m.release != nil {
		m.release.once.Do(m.release.fn)
		m.release = nil
	}
}

// Detach is a copy of m that owns its buffer, with no release: for keeping
// past m.Release, e.g. on another goroutine. Messages are copied from a
// pool the dispatcher reuses once released; a subscriber reading a message
// later would see the next one written over it (#404).
func (m Message) Detach() Message {
	m.release = nil
	if m.SIMCONNECT_RECV == nil || len(m.data) == 0 {
		return m
	}
	buf := make([]byte, len(m.data))
	copy(buf, m.data)
	m.data = buf
	m.SIMCONNECT_RECV = (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))
	return m
}

func CastAs[T any](m *Message) T {
	if m.is(types.SIMCONNECT_RECV_ID_EVENT) {
		return any((*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(m.SIMCONNECT_RECV))).(T)
	}
	var zero T
	return zero
}

// is reports whether m carries a message of the ID: false for an error
// Message, which has no SIMCONNECT_RECV.
func (m *Message) is(id types.SIMCONNECT_RECV_ID) bool {
	return m.SIMCONNECT_RECV != nil && types.SIMCONNECT_RECV_ID(m.DwID) == id
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
	if !m.is(types.SIMCONNECT_RECV_ID_EVENT) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsEventFrame() *types.SIMCONNECT_RECV_EVENT_FRAME {
	if !m.is(types.SIMCONNECT_RECV_ID_EVENT_FRAME) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_FRAME)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsEventFilename() *types.SIMCONNECT_RECV_EVENT_FILENAME {
	if !m.is(types.SIMCONNECT_RECV_ID_EVENT_FILENAME) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_FILENAME)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsEventObjectAddRemove() *types.SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE {
	if !m.is(types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsOpen() *types.SIMCONNECT_RECV_OPEN {
	if !m.is(types.SIMCONNECT_RECV_ID_OPEN) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_OPEN)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsSimObjectData() *types.SIMCONNECT_RECV_SIMOBJECT_DATA {
	if !m.is(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsSimObjectDataBType() *types.SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE {
	if !m.is(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SIMOBJECT_DATA_BTYPE)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsSimObjectAndLiveryEnumeration() *types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
	if !m.is(types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsFacilityData() *types.SIMCONNECT_RECV_FACILITY_DATA {
	if !m.is(types.SIMCONNECT_RECV_ID_FACILITY_DATA) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_FACILITY_DATA)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsFacilityList() *types.SIMCONNECT_RECV_FACILITIES_LIST {
	if m.is(types.SIMCONNECT_RECV_ID_AIRPORT_LIST) ||
		m.is(types.SIMCONNECT_RECV_ID_VOR_LIST) ||
		m.is(types.SIMCONNECT_RECV_ID_NDB_LIST) ||
		m.is(types.SIMCONNECT_RECV_ID_WAYPOINT_LIST) {
		return (*types.SIMCONNECT_RECV_FACILITIES_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))

	}
	return nil
}

func (m *Message) AsAirportList() *types.SIMCONNECT_RECV_AIRPORT_LIST {
	if !m.is(types.SIMCONNECT_RECV_ID_AIRPORT_LIST) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_AIRPORT_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsNDBList() *types.SIMCONNECT_RECV_NDB_LIST {
	if !m.is(types.SIMCONNECT_RECV_ID_NDB_LIST) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_NDB_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsVORList() *types.SIMCONNECT_RECV_VOR_LIST {
	if !m.is(types.SIMCONNECT_RECV_ID_VOR_LIST) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_VOR_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsWaypointList() *types.SIMCONNECT_RECV_WAYPOINT_LIST {
	if !m.is(types.SIMCONNECT_RECV_ID_WAYPOINT_LIST) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_WAYPOINT_LIST)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsAssignedObjectID() *types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID {
	if !m.is(types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsFacilityDataEnd() *types.SIMCONNECT_RECV_FACILITY_DATA_END {
	if !m.is(types.SIMCONNECT_RECV_ID_FACILITY_DATA_END) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_FACILITY_DATA_END)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

func (m *Message) AsException() *types.SIMCONNECT_RECV_EXCEPTION {
	if !m.is(types.SIMCONNECT_RECV_ID_EXCEPTION) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_EXCEPTION)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

// AsFlowEvent casts the message to SIMCONNECT_RECV_FLOW_EVENT.
// Returns nil if the message is not a flow event.
// Note: MSFS 2024 only.
func (m *Message) AsFlowEvent() *types.SIMCONNECT_RECV_FLOW_EVENT {
	if !m.is(types.SIMCONNECT_RECV_ID_FLOW_EVENT) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_FLOW_EVENT)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

// AsCameraStatus casts the message to SIMCONNECT_RECV_CAMERA_STATUS; nil
// if it is not one. MSFS 2024 only.
func (m *Message) AsCameraStatus() *types.SIMCONNECT_RECV_CAMERA_STATUS {
	if !m.is(types.SIMCONNECT_RECV_ID_CAMERA_STATUS) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_CAMERA_STATUS)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

// AsCameraData reads a SIMCONNECT_RECV_CAMERA_DATA (the camera is packed,
// so it is decoded rather than cast); false if the message is not one.
// MSFS 2024 only.
func (m *Message) AsCameraData() (types.SIMCONNECT_DATA_CAMERA, bool) {
	if !m.is(types.SIMCONNECT_RECV_ID_CAMERA_DATA) || m.DwSize < 12+types.SimConnectCameraDataSize {
		return types.SIMCONNECT_DATA_CAMERA{}, false
	}
	b := unsafe.Slice((*byte)(unsafe.Pointer(m.SIMCONNECT_RECV)), m.DwSize)
	return types.CameraDataFrom(b[12:])
}

// AsEnumerateInputEvents casts the message to SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS.
// Returns nil if the message is not an enumerate input events response.
// Note: MSFS 2024 only.
func (m *Message) AsEnumerateInputEvents() *types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS {
	if !m.is(types.SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

// AsGetInputEvent casts the message to SIMCONNECT_RECV_GET_INPUT_EVENT.
// Returns nil if the message is not a get input event response.
// Note: MSFS 2024 only.
func (m *Message) AsGetInputEvent() *types.SIMCONNECT_RECV_GET_INPUT_EVENT {
	if !m.is(types.SIMCONNECT_RECV_ID_GET_INPUT_EVENT) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_GET_INPUT_EVENT)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

// AsSubscribeInputEvent casts the message to SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT.
// Returns nil if the message is not a subscribe input event notification.
// Note: MSFS 2024 only.
func (m *Message) AsSubscribeInputEvent() *types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT {
	if !m.is(types.SIMCONNECT_RECV_ID_SUBSCRIBE_INPUT_EVENT) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

// AsClientData casts the message to SIMCONNECT_RECV_CLIENT_DATA.
// Returns nil if the message is not a client data notification.
func (m *Message) AsClientData() *types.SIMCONNECT_RECV_CLIENT_DATA {
	if !m.is(types.SIMCONNECT_RECV_ID_CLIENT_DATA) {
		return nil
	}
	return (*types.SIMCONNECT_RECV_CLIENT_DATA)(unsafe.Pointer(m.SIMCONNECT_RECV))
}
