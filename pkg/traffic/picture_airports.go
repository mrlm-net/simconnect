//go:build windows
// +build windows

package traffic

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// AirportLister lists the airports the simulator has loaded around the
// user (SimConnect's facilities list, the "reality bubble") for a
// TrafficPicture: Request, feed every message to Handle; it returns the
// airports once the list is complete.
type AirportLister struct {
	client engine.Client
	reqID  uint32

	mu      sync.Mutex
	pending []AirportRef
}

// DefaultAirportListRequestID is the request ID of an AirportLister.
const DefaultAirportListRequestID uint32 = 8900

// NewAirportLister creates a lister sending its request as reqID (0:
// DefaultAirportListRequestID).
func NewAirportLister(client engine.Client, reqID uint32) *AirportLister {
	if reqID == 0 {
		reqID = DefaultAirportListRequestID
	}
	return &AirportLister{client: client, reqID: reqID}
}

// Request asks for the list.
func (l *AirportLister) Request() error {
	l.mu.Lock()
	l.pending = nil
	l.mu.Unlock()
	return l.client.RequestFacilitiesListEX1(l.reqID, types.SIMCONNECT_FACILITY_LIST_AIRPORT)
}

// Handle consumes the lister's messages; with the last part of the list it
// returns the airports and true.
func (l *AirportLister) Handle(msg engine.Message) ([]AirportRef, bool) {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_AIRPORT_LIST {
		return nil, false
	}
	list := msg.AsAirportList()
	if list == nil || uint32(list.DwRequestID) != l.reqID {
		return nil, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending = append(l.pending, decodeAirportList(msg, list)...)
	if uint32(list.DwOutOf) == 0 || uint32(list.DwEntryNumber)+1 >= uint32(list.DwOutOf) {
		out := l.pending
		l.pending = nil
		return out, true
	}
	return nil, false
}

// decodeAirportList reads the packed entries of one airport list message:
// ident, region, latitude, longitude, altitude — 33 bytes in MSFS 2020
// (ident[6], region[3]), 36 or more in MSFS 2024 (ident[9], region[3]).
func decodeAirportList(msg engine.Message, list *types.SIMCONNECT_RECV_AIRPORT_LIST) []AirportRef {
	n := uintptr(list.DwArraySize)
	if n == 0 {
		return nil
	}
	header := unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITIES_LIST{})
	size := (uintptr(msg.DwSize) - header) / n
	var identLen, latOff uintptr
	switch size {
	case 33:
		identLen, latOff = 6, 9
	case 36, 40, 41:
		identLen, latOff = 9, 12
	default:
		return nil
	}
	base := uintptr(unsafe.Pointer(list)) + header
	out := make([]AirportRef, 0, n)
	for i := uintptr(0); i < n; i++ {
		e := base + i*size
		ident := strings.TrimRight(string(unsafe.Slice((*byte)(unsafe.Pointer(e)), identLen)), "\x00 ")
		if j := strings.IndexByte(ident, 0); j >= 0 {
			ident = ident[:j]
		}
		lat := *(*float64)(unsafe.Pointer(e + latOff))
		lon := *(*float64)(unsafe.Pointer(e + latOff + 8))
		if ident != "" {
			out = append(out, AirportRef{ICAO: ident, Position: airport.LatLon{Lat: lat, Lon: lon}})
		}
	}
	return out
}
