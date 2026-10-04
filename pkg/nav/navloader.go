package nav

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// FacilityClient is the part of engine.Client (and manager.Manager) the
// NavLoader uses. Waypoints, VORs and NDBs share identifiers, so requests
// go through RequestFacilityDataEX1 with the fix kind as its type.
type FacilityClient interface {
	AddToFacilityDefinition(definitionID uint32, fieldName string) error
	RequestFacilityDataEX1(definitionID uint32, requestID uint32, icao string, region string, facilityType byte) error
}

// sendIDClient is implemented by clients that report the send ID of the
// last packet (engine.Client, manager.Manager). The loader then matches the
// exception an unknown fix raises to its request; without it the request
// ends by Expire.
type sendIDClient interface {
	GetLastSentPacketID() (uint32, error)
}

// Default SimConnect IDs of a NavLoader: three definition IDs (waypoint
// with airways, VOR, NDB) and two request IDs per slot.
const (
	DefaultNavDefinitionBase uint32 = 8700
	DefaultNavRequestBase    uint32 = 8800
	// DefaultNavSlots is how many fixes a NavLoader loads at a time.
	DefaultNavSlots = 16
	// DefaultNavTimeout ends a request the simulator never answered.
	DefaultNavTimeout = 10 * time.Second
	navParts          = 2 // waypoint record, navaid record
)

// navDefinitions are the facility definitions, field for field as the
// SDK's SimConnect_AddToFacilityDefinition lists them for WAYPOINT/ROUTE,
// VOR and NDB (VOR and NDB records have no ICAO/REGION fields).
func navDefinitions() [][]string {
	return [][]string{
		{"OPEN WAYPOINT", "LATITUDE", "LONGITUDE", "TYPE", "ICAO", "REGION", "N_ROUTES", "IS_TERMINAL_WPT",
			"OPEN ROUTE", "NAME", "TYPE",
			"NEXT_ICAO", "NEXT_REGION", "NEXT_TYPE", "NEXT_LATITUDE", "NEXT_LONGITUDE", "NEXT_ALTITUDE",
			"PREV_ICAO", "PREV_REGION", "PREV_TYPE", "PREV_LATITUDE", "PREV_LONGITUDE", "PREV_ALTITUDE",
			"CLOSE ROUTE", "CLOSE WAYPOINT"},
		{"OPEN VOR", "VOR_LATITUDE", "VOR_LONGITUDE", "FREQUENCY", "NAME", "CLOSE VOR"},
		{"OPEN NDB", "LATITUDE", "LONGITUDE", "FREQUENCY", "NAME", "CLOSE NDB"},
	}
}

// NavLoader loads enroute fixes (waypoints, VORs, NDBs) with their airway
// links through the facility API (#328). Call Request, feed every message
// to Handle; it returns each fix once its records have arrived. Call
// Expire now and then to end requests the simulator never answers.
type NavLoader struct {
	client           FacilityClient
	defBase, reqBase uint32
	timeout          time.Duration
	registered       bool

	mu     sync.Mutex
	slots  []*navState
	sendID map[uint32]uint32 // packet send ID -> request ID
}

type navState struct {
	key      FixKey
	started  time.Time
	pending  int
	sendIDs  []uint32
	wpt, aid bool // records received
	fix      Fix
	routes   []RouteLink
}

// NewNavLoader creates a loader sending its requests through client, with
// DefaultNavSlots slots.
func NewNavLoader(client FacilityClient) *NavLoader {
	return NewNavLoaderWithIDs(client, DefaultNavDefinitionBase, DefaultNavRequestBase, DefaultNavSlots)
}

// NewNavLoaderWithIDs creates a loader using definition IDs defBase..+2
// and request IDs reqBase..+2*slots-1.
func NewNavLoaderWithIDs(client FacilityClient, defBase, reqBase uint32, slots int) *NavLoader {
	if slots < 1 {
		slots = 1
	}
	return &NavLoader{client: client, defBase: defBase, reqBase: reqBase, timeout: DefaultNavTimeout,
		slots: make([]*navState, slots), sendID: map[uint32]uint32{}}
}

// SetTimeout sets how long a request may stay unanswered before Expire
// ends it.
func (l *NavLoader) SetTimeout(d time.Duration) { l.timeout = d }

// Free returns how many more fixes can be requested now.
func (l *NavLoader) Free() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, s := range l.slots {
		if s == nil {
			n++
		}
	}
	return n
}

// Pending returns how many fixes are loading.
func (l *NavLoader) Pending() int { return len(l.slots) - l.Free() }

// Request starts loading a fix. A VOR or NDB is requested twice: as a
// waypoint (for its airways) and as a navaid (for frequency and name).
func (l *NavLoader) Request(key FixKey) error {
	key = Key(key.Ident, key.Region, key.Kind)
	if key.Kind != KindWaypoint && key.Kind != KindVOR && key.Kind != KindNDB {
		return fmt.Errorf("nav: fix %s has no kind", key)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.registered {
		for i, def := range navDefinitions() {
			for _, f := range def {
				if err := l.client.AddToFacilityDefinition(l.defBase+uint32(i), f); err != nil {
					return err
				}
			}
		}
		l.registered = true
	}
	slot := -1
	for i, s := range l.slots {
		if s == nil {
			slot = i
			break
		}
	}
	if slot < 0 {
		return fmt.Errorf("nav: %d fix loads already pending", len(l.slots))
	}
	st := &navState{key: key, started: time.Now(), fix: Fix{Ident: key.Ident, Region: key.Region, Kind: key.Kind}}
	l.slots[slot] = st
	send := func(def uint32, part int) error {
		req := l.reqBase + uint32(slot*navParts+part)
		if err := l.client.RequestFacilityDataEX1(def, req, key.Ident, key.Region, byte(key.Kind)); err != nil {
			return err
		}
		st.pending++
		if c, ok := l.client.(sendIDClient); ok {
			if id, err := c.GetLastSentPacketID(); err == nil {
				l.sendID[id] = req
				st.sendIDs = append(st.sendIDs, id)
			}
		}
		return nil
	}
	err := send(l.defBase, 0)
	if err == nil && key.Kind == KindVOR {
		err = send(l.defBase+1, 1)
	} else if err == nil && key.Kind == KindNDB {
		err = send(l.defBase+2, 1)
	}
	if err != nil && st.pending == 0 {
		l.slots[slot] = nil
		return err
	}
	return nil
}

func (l *NavLoader) lookup(req uint32) (*navState, int, bool) {
	if req < l.reqBase || req >= l.reqBase+uint32(len(l.slots)*navParts) {
		return nil, 0, false
	}
	slot := int(req-l.reqBase) / navParts
	s := l.slots[slot]
	return s, slot, s != nil
}

// Handle consumes the loader's facility messages (and the exception an
// unknown fix raises); when a fix is complete it returns it and true.
func (l *NavLoader) Handle(msg engine.Message) (NavResult, bool) {
	if msg.SIMCONNECT_RECV == nil {
		return NavResult{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var req uint32
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
		m := msg.AsFacilityData()
		if st, _, ok := l.lookup(uint32(m.UserRequestId)); ok {
			n := int(m.DwSize) - int(unsafe.Offsetof(m.Data))
			if n > 0 {
				st.add(m.Type, unsafe.Slice((*byte)(unsafe.Pointer(&m.Data)), n))
			}
		}
		return NavResult{}, false
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
		req = uint32(msg.AsFacilityDataEnd().RequestId)
	case types.SIMCONNECT_RECV_ID_EXCEPTION:
		r, ok := l.sendID[uint32(msg.AsException().DwSendID)]
		if !ok {
			return NavResult{}, false
		}
		req = r
	default:
		return NavResult{}, false
	}
	st, slot, ok := l.lookup(req)
	if !ok {
		return NavResult{}, false
	}
	if st.pending--; st.pending > 0 {
		return NavResult{}, false
	}
	return l.finish(slot), true
}

// Expire ends the requests older than the loader's timeout and returns
// them (Found set if a record did arrive).
func (l *NavLoader) Expire(now time.Time) []NavResult {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []NavResult
	for i, s := range l.slots {
		if s != nil && now.Sub(s.started) > l.timeout {
			out = append(out, l.finish(i))
		}
	}
	return out
}

func (l *NavLoader) finish(slot int) NavResult {
	st := l.slots[slot]
	l.slots[slot] = nil
	for _, id := range st.sendIDs {
		delete(l.sendID, id)
	}
	if st.routes == nil {
		st.routes = []RouteLink{}
	}
	return NavResult{Key: st.key, Fix: st.fix, Routes: st.routes, Found: st.wpt || st.aid}
}

// add decodes one record into the fix being loaded.
func (s *navState) add(t types.SIMCONNECT_FACILITY_DATA_TYPE, b []byte) {
	r := recordReader{b: b}
	switch t {
	case types.SIMCONNECT_FACILITY_DATA_WAYPOINT:
		pos := airport.LatLon{Lat: r.f64(), Lon: r.f64()}
		typ := WaypointType(r.i32())
		ident, region := r.str(8), r.str(8)
		_ = r.i32() // N_ROUTES: the ROUTE records follow
		s.fix.Position, s.fix.Type = pos, typ
		s.fix.Terminal = r.i32() != 0
		if ident != "" {
			s.fix.Ident, s.fix.Region = ident, region
		}
		s.wpt = true
	case types.SIMCONNECT_FACILITY_DATA_ROUTE:
		link := RouteLink{Airway: r.str(32), Type: AirwayType(r.i32())}
		link.Next = r.fixRef()
		link.Prev = r.fixRef()
		s.routes = append(s.routes, link)
	case types.SIMCONNECT_FACILITY_DATA_VOR, types.SIMCONNECT_FACILITY_DATA_NDB:
		pos := airport.LatLon{Lat: r.f64(), Lon: r.f64()}
		hz := float64(r.u32())
		if !s.wpt {
			s.fix.Position = pos
		}
		if t == types.SIMCONNECT_FACILITY_DATA_VOR {
			s.fix.Freq = math.Round(hz/1e3) / 1e3 // MHz, to the kHz
		} else {
			s.fix.Freq = math.Round(hz/10) / 100 // kHz
		}
		s.fix.Name = r.str(64)
		s.aid = true
	}
}

// recordReader reads a packed facility record, like pkg/airport's.
type recordReader struct {
	b   []byte
	off int
}

func (r *recordReader) take(n int) []byte {
	if r.off+n > len(r.b) {
		r.off = len(r.b)
		return make([]byte, n)
	}
	s := r.b[r.off : r.off+n]
	r.off += n
	return s
}
func (r *recordReader) i32() int32  { return int32(binary.LittleEndian.Uint32(r.take(4))) }
func (r *recordReader) u32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *recordReader) f32() float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(r.take(4))))
}
func (r *recordReader) f64() float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(r.take(8)))
}
func (r *recordReader) str(n int) string {
	s := string(r.take(n))
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// fixRef reads a ROUTE record's NEXT_* or PREV_* block (ICAO, REGION,
// TYPE, LATITUDE, LONGITUDE, ALTITUDE); nil when the ICAO is empty.
func (r *recordReader) fixRef() *FixRef {
	ident, region, kind := r.str(8), r.str(8), r.i32()
	pos := airport.LatLon{Lat: r.f64(), Lon: r.f64()}
	alt := math.Round(r.f32()*10) / 10 // FLOAT32 meters: 1219.19995 -> 1219.2
	if ident == "" {
		return nil
	}
	return &FixRef{Key: FixKey{Ident: ident, Region: region, Kind: FixKind(kind)}, Position: pos, MinAltM: alt}
}
