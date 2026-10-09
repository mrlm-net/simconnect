package traffic

import (
	"math"
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

// RequestAll asks for every airport the simulator knows, worldwide, not
// only the reality bubble (RequestAllFacilities; live from LKPR in MSFS
// 2024: 85,723 airports in about a second). Handle collects it
// the same way. The list carries no names or runways: look those up per
// ICAO for the airports used.
func (l *AirportLister) RequestAll() error {
	l.mu.Lock()
	l.pending = nil
	l.mu.Unlock()
	return l.client.RequestAllFacilities(types.SIMCONNECT_FACILITY_LIST_AIRPORT, l.reqID)
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
	n := int(list.DwArraySize)
	header := int(unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITIES_LIST{}))
	total := int(msg.DwSize)
	if msg.Size != 0 && int(msg.Size) < total {
		total = int(msg.Size) // never past the buffer
	}
	if n == 0 || total <= header {
		return nil
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(list)), total)[header:]
	return decodeAirportEntries(buf, n)
}

// airportEntryLayouts are the entry sizes seen, with their ident length
// (the latitude follows ident and region[3]).
var airportEntryLayouts = []struct{ size, identLen int }{{36, 9}, {40, 9}, {41, 9}, {33, 6}}

// decodeAirportEntries decodes n entries packed in buf. The entry size is
// the one of airportEntryLayouts that fits buf (n entries and under 8
// bytes of padding) and gives every entry a clean ident and a position on
// the globe: a size worked out by dividing alone came out 41 for 40-byte
// entries with padding in a short last part, and every entry after the
// first was read a byte further off (live, MSFS 2024 at LROP: "?0?", "P",
// ",?R@@" thousands of miles away). Entries that are still not clean are
// left out.
func decodeAirportEntries(buf []byte, n int) []AirportRef {
	var best []AirportRef
	for _, lay := range airportEntryLayouts {
		if rest := len(buf) - lay.size*n; rest < 0 || rest >= 8+lay.size {
			continue
		}
		out, clean := airportEntriesAs(buf, n, lay.size, lay.identLen)
		if clean {
			return out
		}
		if len(out) > len(best) {
			best = out
		}
	}
	return best
}

// airportEntriesAs decodes n entries of size bytes; clean when every one
// is sane, only the sane ones returned.
func airportEntriesAs(buf []byte, n, size, identLen int) ([]AirportRef, bool) {
	out := make([]AirportRef, 0, n)
	clean := true
	f64 := func(b []byte) float64 { return *(*float64)(unsafe.Pointer(&b[0])) }
	for i := range n {
		e := buf[i*size : (i+1)*size]
		ident := cString(e[:identLen])
		region := cString(e[identLen : identLen+3])
		lat, lon, alt := f64(e[identLen+3:]), f64(e[identLen+11:]), f64(e[identLen+19:])
		if !saneAirport(ident, lat, lon, alt) {
			clean = false
			continue
		}
		out = append(out, AirportRef{ICAO: ident, Region: region, Position: airport.LatLon{Lat: lat, Lon: lon}, AltM: alt})
	}
	return out, clean
}

func cString(b []byte) string {
	if j := strings.IndexByte(string(b), 0); j >= 0 {
		b = b[:j]
	}
	return strings.TrimRight(string(b), " ")
}

// saneAirport: an ident of 2 to 9 letters and digits, a position on the
// globe and not 0/0 (no coordinate so near 0 it is a stray float's
// bytes), an elevation between −500 and 6000 m.
func saneAirport(ident string, lat, lon, altM float64) bool {
	tiny := func(x float64) bool { return x != 0 && math.Abs(x) < 1e-6 }
	if tiny(lat) || tiny(lon) || math.IsNaN(altM) || altM < -500 || altM > 6000 {
		return false
	}
	if len(ident) < 2 || len(ident) > 9 {
		return false
	}
	for _, c := range ident {
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return !math.IsNaN(lat) && !math.IsNaN(lon) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180 && (lat != 0 || lon != 0)
}
