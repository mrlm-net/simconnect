package types

import (
	"encoding/binary"
	"math"
	"unsafe"
)

// List messages (SIMCONNECT_RECV_LIST_TEMPLATE, SIMCONNECT_RECV_FACILITIES_LIST)
// carry DwArraySize entries right after their 28-byte header. SimConnect packs
// the entries (#pragma pack(1)), so a Go struct with doubles is laid out
// differently: those entries are decoded (DecodeFacility*), the others are
// read in place. Entries reads them from the message the header points into,
// within its DwSize, so a short message gives fewer entries, never a read
// past it.

// Packed sizes of list entries on the wire (SimConnect.h, MSFS 2024).
const (
	FacilityAirportSize  = 36 // char Ident[9], Region[3], 3 doubles
	FacilityWaypointSize = 40 // airport + float fMagVar
	FacilityNDBSize      = 44 // waypoint + DWORD fFrequency
	FacilityVORSize      = 80 // NDB + Flags, fLocalizer, GlideLat/Lon/Alt, fGlideSlopeAngle
	FacilityMinimalSize  = 42 // SIMCONNECT_ICAO (18) + SIMCONNECT_DATA_LATLONALT (24)
	ControllerItemSize   = 276
	SimObjectLiverySize  = 512
)

// listHeaderSize is the size of SIMCONNECT_RECV_LIST_TEMPLATE and
// SIMCONNECT_RECV_FACILITIES_LIST: where the entries start.
const listHeaderSize = 28

// listEntries is the bytes of the n entries of size bytes after the header of
// the list message at recv (dwSize bytes long), capped at what the message
// holds.
func listEntries(recv unsafe.Pointer, dwSize, n DWORD, size int) []byte {
	if recv == nil || int(dwSize) <= listHeaderSize || n == 0 {
		return nil
	}
	fit := (int(dwSize) - listHeaderSize) / size
	if int(n) < fit {
		fit = int(n)
	}
	return unsafe.Slice((*byte)(unsafe.Add(recv, listHeaderSize)), fit*size)
}

// listOf is the entries of a list whose Go type T matches the wire (no
// padding, alignment of at most 4), in place.
func listOf[T any](recv unsafe.Pointer, dwSize, n DWORD) []T {
	size := int(unsafe.Sizeof(*new(T)))
	b := listEntries(recv, dwSize, n, size)
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*T)(unsafe.Pointer(&b[0])), len(b)/size)
}

// decodeList decodes the entries of a packed list.
func decodeList[T any](b []byte, size int, decode func([]byte) (T, bool)) []T {
	out := make([]T, 0, len(b)/size)
	for at := 0; at+size <= len(b); at += size {
		if v, ok := decode(b[at : at+size]); ok {
			out = append(out, v)
		}
	}
	return out
}

func f64At(b []byte, at int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b[at:])) }
func f32At(b []byte, at int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b[at:])) }

// DecodeFacilityAirport reads one packed SIMCONNECT_DATA_FACILITY_AIRPORT
// (at least FacilityAirportSize bytes); false when b is too short.
func DecodeFacilityAirport(b []byte) (SIMCONNECT_DATA_FACILITY_AIRPORT, bool) {
	var a SIMCONNECT_DATA_FACILITY_AIRPORT
	if len(b) < FacilityAirportSize {
		return a, false
	}
	copy(a.Ident[:], b[0:9])
	copy(a.Region[:], b[9:12])
	a.Latitude, a.Longitude, a.Altitude = f64At(b, 12), f64At(b, 20), f64At(b, 28)
	return a, true
}

// DecodeFacilityWaypoint reads one packed SIMCONNECT_DATA_FACILITY_WAYPOINT
// (at least FacilityWaypointSize bytes); false when b is too short.
func DecodeFacilityWaypoint(b []byte) (SIMCONNECT_DATA_FACILITY_WAYPOINT, bool) {
	if len(b) < FacilityWaypointSize {
		return SIMCONNECT_DATA_FACILITY_WAYPOINT{}, false
	}
	a, _ := DecodeFacilityAirport(b)
	return SIMCONNECT_DATA_FACILITY_WAYPOINT{SIMCONNECT_DATA_FACILITY_AIRPORT: a, FMagVar: f32At(b, 36)}, true
}

// DecodeFacilityNDB reads one packed SIMCONNECT_DATA_FACILITY_NDB (at least
// FacilityNDBSize bytes); false when b is too short.
func DecodeFacilityNDB(b []byte) (SIMCONNECT_DATA_FACILITY_NDB, bool) {
	if len(b) < FacilityNDBSize {
		return SIMCONNECT_DATA_FACILITY_NDB{}, false
	}
	w, _ := DecodeFacilityWaypoint(b)
	return SIMCONNECT_DATA_FACILITY_NDB{SIMCONNECT_DATA_FACILITY_WAYPOINT: w, FFrequency: DWORD(binary.LittleEndian.Uint32(b[40:]))}, true
}

// DecodeFacilityVOR reads one packed SIMCONNECT_DATA_FACILITY_VOR (at least
// FacilityVORSize bytes); false when b is too short.
func DecodeFacilityVOR(b []byte) (SIMCONNECT_DATA_FACILITY_VOR, bool) {
	if len(b) < FacilityVORSize {
		return SIMCONNECT_DATA_FACILITY_VOR{}, false
	}
	n, _ := DecodeFacilityNDB(b)
	return SIMCONNECT_DATA_FACILITY_VOR{
		SIMCONNECT_DATA_FACILITY_NDB: n,
		Flags:                        DWORD(binary.LittleEndian.Uint32(b[44:])),
		FLocalizer:                   f32At(b, 48),
		GlideLat:                     f64At(b, 52),
		GlideLon:                     f64At(b, 60),
		GlideAlt:                     f64At(b, 68),
		FGlideSlopeAngle:             f32At(b, 76),
	}, true
}

// DecodeFacilityMinimal reads one packed SIMCONNECT_FACILITY_MINIMAL (at least
// FacilityMinimalSize bytes); false when b is too short.
func DecodeFacilityMinimal(b []byte) (SIMCONNECT_FACILITY_MINIMAL, bool) {
	var m SIMCONNECT_FACILITY_MINIMAL
	if len(b) < FacilityMinimalSize {
		return m, false
	}
	m.ICAO.Type = b[0]
	copy(m.ICAO.Ident[:], b[1:10])
	copy(m.ICAO.Region[:], b[10:13])
	copy(m.ICAO.Airport[:], b[13:18])
	m.LLA = SIMCONNECT_DATA_LATLONALT{Latitude: f64At(b, 18), Longitude: f64At(b, 26), Altitude: f64At(b, 34)}
	return m, true
}

// Entries decodes the airports of the message (it must be the whole
// message, DwSize bytes, as engine.Message holds it).
func (l *SIMCONNECT_RECV_AIRPORT_LIST) Entries() []SIMCONNECT_DATA_FACILITY_AIRPORT {
	return decodeList(listEntries(unsafe.Pointer(l), l.DwSize, l.DwArraySize, FacilityAirportSize), FacilityAirportSize, DecodeFacilityAirport)
}

// Entries decodes the waypoints of the message.
func (l *SIMCONNECT_RECV_WAYPOINT_LIST) Entries() []SIMCONNECT_DATA_FACILITY_WAYPOINT {
	return decodeList(listEntries(unsafe.Pointer(l), l.DwSize, l.DwArraySize, FacilityWaypointSize), FacilityWaypointSize, DecodeFacilityWaypoint)
}

// Entries decodes the NDBs of the message.
func (l *SIMCONNECT_RECV_NDB_LIST) Entries() []SIMCONNECT_DATA_FACILITY_NDB {
	return decodeList(listEntries(unsafe.Pointer(l), l.DwSize, l.DwArraySize, FacilityNDBSize), FacilityNDBSize, DecodeFacilityNDB)
}

// Entries decodes the VORs of the message.
func (l *SIMCONNECT_RECV_VOR_LIST) Entries() []SIMCONNECT_DATA_FACILITY_VOR {
	return decodeList(listEntries(unsafe.Pointer(l), l.DwSize, l.DwArraySize, FacilityVORSize), FacilityVORSize, DecodeFacilityVOR)
}

// Entries decodes the facilities of the message.
func (l *SIMCONNECT_RECV_FACILITY_MINIMAL_LIST) Entries() []SIMCONNECT_FACILITY_MINIMAL {
	return decodeList(listEntries(unsafe.Pointer(l), l.DwSize, l.ArraySize, FacilityMinimalSize), FacilityMinimalSize, DecodeFacilityMinimal)
}

// Entries decodes the jetways of the message.
func (l *SIMCONNECT_RECV_JETWAY_DATA) Entries() []SIMCONNECT_JETWAY_DATA {
	return decodeList(listEntries(unsafe.Pointer(l), l.DwSize, l.DwArraySize, JetwayDataSize), JetwayDataSize, DecodeJetwayData)
}

// Entries is the input event descriptors of the message, in place: valid
// while the message is.
func (l *SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS) Entries() []SIMCONNECT_INPUT_EVENT_DESCRIPTOR {
	return listOf[SIMCONNECT_INPUT_EVENT_DESCRIPTOR](unsafe.Pointer(l), l.DwSize, l.DwArraySize)
}

// Entries is the controllers of the message, in place: valid while the
// message is.
func (l *SIMCONNECT_RECV_CONTROLLERS_LIST) Entries() []SIMCONNECT_CONTROLLER_ITEM {
	return listOf[SIMCONNECT_CONTROLLER_ITEM](unsafe.Pointer(l), l.DwSize, l.DwArraySize)
}

// Entries is the titles and liveries of the message, in place: valid while
// the message is.
func (l *SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST) Entries() []SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY {
	return listOf[SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY](unsafe.Pointer(l), l.DwSize, l.DwArraySize)
}
