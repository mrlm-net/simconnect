//go:build windows
// +build windows

package airport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrTimeout is returned when the simulator does not finish sending an
// airport's facility data in time. SimConnect sends nothing at all for an
// unknown ICAO code, so an unknown airport also ends in ErrTimeout.
var ErrTimeout = errors.New("airport: timed out waiting for facility data")

// FacilityClient is the part of engine.Client (and manager.Manager) the
// Loader uses.
type FacilityClient interface {
	AddToFacilityDefinition(definitionID uint32, fieldName string) error
	RequestFacilityData(definitionID uint32, requestID uint32, icao string, region string) error
}

// Defaults for LoaderOptions.
const (
	DefaultLoaderDefinitionBase uint32 = 7100
	DefaultLoaderRequestBase    uint32 = 7200
	DefaultLoaderTimeout               = 30 * time.Second
	// loaderSlots is how many airports can be in flight at once; each uses
	// len(loaderDefinitions) consecutive request IDs.
	loaderSlots = 16
)

// LoaderOption configures a Loader.
type LoaderOption func(*Loader)

// LoaderWithIDs sets the first facility definition ID and first request ID.
// The Loader uses 6 definition IDs from defBase and 6*16 request IDs from
// reqBase; keep both ranges clear of the application's own IDs.
func LoaderWithIDs(defBase, reqBase uint32) LoaderOption {
	return func(l *Loader) { l.defBase, l.reqBase = defBase, reqBase }
}

// LoaderWithCache stores every successfully loaded Layout in c.
func LoaderWithCache(c *Cache) LoaderOption {
	return func(l *Loader) { l.cache = c }
}

// LoaderWithTimeout sets how long a request may take before Expire reports
// ErrTimeout.
func LoaderWithTimeout(d time.Duration) LoaderOption {
	return func(l *Loader) { l.timeout = d }
}

// Result is a finished airport request. Raw holds the decoded facility
// records the Layout was built from (useful for saving test fixtures).
type Result struct {
	ICAO   string
	Layout *Layout
	Raw    RawAirport
	Err    error
}

// Loader fetches airport layouts through an application's own message loop.
// It never reads engine.Client.Stream(): the application calls Request, then
// passes every received message to Handle, which reports each airport when
// all of its facility data has arrived.
//
//	loader := airport.NewLoader(client)
//	loader.Request("LKPR")
//	for msg := range client.Stream() {
//	    if res, ok := loader.Handle(msg); ok {
//	        // res.Layout or res.Err
//	    }
//	    for _, res := range loader.Expire(time.Now()) { /* timeouts */ }
//	}
//
// A Loader is safe for concurrent use. After a reconnect, call Reset so the
// facility definitions are registered again on the new connection.
type Loader struct {
	mu         sync.Mutex
	client     FacilityClient
	defBase    uint32
	reqBase    uint32
	timeout    time.Duration
	registered bool
	slots      [loaderSlots]*loadState
	next       int
	cache      *Cache
}

type loadState struct {
	icao     string
	deadline time.Time
	pending  int
	raw      RawAirport
	// parkingByID maps a parking record's unique request ID to its index,
	// for the airline records that follow it as children.
	parkingByID map[uint32]int
}

// loaderDefinitions are the facility definitions, in request order. The
// field order of each must match the wire decoding in handleData.
var loaderDefinitions = [][]string{
	{"OPEN AIRPORT", "LATITUDE", "LONGITUDE", "ALTITUDE", "ICAO", "NAME", "NAME64", "CLOSE AIRPORT"},
	{"OPEN AIRPORT", "OPEN RUNWAY", "LATITUDE", "LONGITUDE", "ALTITUDE", "HEADING", "LENGTH", "WIDTH",
		"PRIMARY_NUMBER", "PRIMARY_DESIGNATOR", "SECONDARY_NUMBER", "SECONDARY_DESIGNATOR", "CLOSE RUNWAY", "CLOSE AIRPORT"},
	{"OPEN AIRPORT", "OPEN TAXI_PARKING", "NAME", "SUFFIX", "NUMBER", "TYPE", "HEADING", "RADIUS", "BIAS_X", "BIAS_Z",
		"OPEN AIRLINE", "NAME", "CLOSE AIRLINE", "CLOSE TAXI_PARKING", "CLOSE AIRPORT"},
	{"OPEN AIRPORT", "OPEN TAXI_POINT", "TYPE", "ORIENTATION", "BIAS_X", "BIAS_Z", "CLOSE TAXI_POINT", "CLOSE AIRPORT"},
	{"OPEN AIRPORT", "OPEN TAXI_PATH", "TYPE", "WIDTH", "RUNWAY_NUMBER", "RUNWAY_DESIGNATOR", "START", "END", "NAME_INDEX",
		"CLOSE TAXI_PATH", "CLOSE AIRPORT"},
	{"OPEN AIRPORT", "OPEN TAXI_NAME", "NAME", "CLOSE TAXI_NAME", "CLOSE AIRPORT"},
}

// Record kinds by definition position.
const (
	partAirport = iota
	partRunway
	partParking
	partTaxiPoint
	partTaxiPath
	partTaxiName
)

// NewLoader creates a Loader that sends requests through client.
func NewLoader(client FacilityClient, opts ...LoaderOption) *Loader {
	l := &Loader{
		client:  client,
		defBase: DefaultLoaderDefinitionBase,
		reqBase: DefaultLoaderRequestBase,
		timeout: DefaultLoaderTimeout,
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Reset forgets registered definitions and in-flight requests, e.g. after the
// simulator reconnects. A non-nil client replaces the current one.
func (l *Loader) Reset(client FacilityClient) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if client != nil {
		l.client = client
	}
	l.registered = false
	l.slots = [loaderSlots]*loadState{}
}

// Request starts fetching an airport's layout. The result is delivered by
// Handle or, on timeout, by Expire.
func (l *Loader) Request(icao string) error {
	icao = strings.ToUpper(strings.TrimSpace(icao))
	if icao == "" {
		return errors.New("airport: empty ICAO code")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.registered {
		for i, fields := range loaderDefinitions {
			for _, f := range fields {
				if err := l.client.AddToFacilityDefinition(l.defBase+uint32(i), f); err != nil {
					return fmt.Errorf("airport: define %s: %w", f, err)
				}
			}
		}
		l.registered = true
	}
	slot := -1
	for i := range loaderSlots {
		s := (l.next + i) % loaderSlots
		if l.slots[s] == nil {
			slot = s
			break
		}
	}
	if slot < 0 {
		return fmt.Errorf("airport: %d requests already in flight", loaderSlots)
	}
	l.next = (slot + 1) % loaderSlots
	st := &loadState{icao: icao, deadline: time.Now().Add(l.timeout), pending: len(loaderDefinitions), raw: RawAirport{ICAO: icao}}
	l.slots[slot] = st
	for i := range loaderDefinitions {
		if err := l.client.RequestFacilityData(l.defBase+uint32(i), l.requestID(slot, i), icao, ""); err != nil {
			l.slots[slot] = nil
			return fmt.Errorf("airport: request %s: %w", icao, err)
		}
	}
	return nil
}

// Pending returns the ICAO codes still being fetched.
func (l *Loader) Pending() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, s := range l.slots {
		if s != nil {
			out = append(out, s.icao)
		}
	}
	return out
}

func (l *Loader) requestID(slot, part int) uint32 {
	return l.reqBase + uint32(slot*len(loaderDefinitions)+part)
}

// lookup maps a request ID to its slot and part; ok is false for IDs the
// Loader does not own or slots that are not in flight.
func (l *Loader) lookup(id uint32) (st *loadState, slot, part int, ok bool) {
	if id < l.reqBase || id >= l.reqBase+uint32(loaderSlots*len(loaderDefinitions)) {
		return nil, 0, 0, false
	}
	off := int(id - l.reqBase)
	slot, part = off/len(loaderDefinitions), off%len(loaderDefinitions)
	st = l.slots[slot]
	return st, slot, part, st != nil
}

// Handle processes one message. It returns ok=true with the Result when msg
// completes an airport. Messages the Loader does not own are ignored.
func (l *Loader) Handle(msg engine.Message) (Result, bool) {
	if msg.SIMCONNECT_RECV == nil {
		return Result{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
		m := msg.AsFacilityData()
		if st, _, part, ok := l.lookup(uint32(m.UserRequestId)); ok {
			st.add(part, m)
		}
	case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
		m := msg.AsFacilityDataEnd()
		st, slot, _, ok := l.lookup(uint32(m.RequestId))
		if !ok {
			return Result{}, false
		}
		st.pending--
		if st.pending > 0 {
			return Result{}, false
		}
		l.slots[slot] = nil
		layout, err := BuildLayout(st.raw)
		if err != nil {
			err = fmt.Errorf("%w for %s", err, st.icao)
		}
		if err == nil && l.cache != nil {
			l.cache.Put(layout)
		}
		return Result{ICAO: st.icao, Layout: layout, Raw: st.raw, Err: err}, true
	}
	return Result{}, false
}

// Expire ends requests whose deadline has passed at now with ErrTimeout.
// Call it periodically, e.g. from a ticker in the message loop.
func (l *Loader) Expire(now time.Time) []Result {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Result
	for i, s := range l.slots {
		if s != nil && now.After(s.deadline) {
			out = append(out, Result{ICAO: s.icao, Err: fmt.Errorf("%w: %s", ErrTimeout, s.icao)})
			l.slots[i] = nil
		}
	}
	return out
}

// airportWire is the AIRPORT record of the first definition.
type airportWire struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	ICAO      [8]byte
	Name      [32]byte
	Name64    [64]byte
}

// add stores one FACILITY_DATA record. Every request first delivers the
// AIRPORT record it was opened with; only the airport definition carries
// fields, so list requests keep only items of their own type.
func (s *loadState) add(part int, m *types.SIMCONNECT_RECV_FACILITY_DATA) {
	i := int(m.ItemIndex)
	data := &m.Data
	switch part {
	case partAirport:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_AIRPORT {
			a := engine.CastDataAs[airportWire](data)
			s.raw.Latitude, s.raw.Longitude, s.raw.Altitude = a.Latitude, a.Longitude, a.Altitude
			if icao := engine.BytesToString(a.ICAO[:]); icao != "" {
				s.raw.ICAO = icao
			}
			s.raw.Name = engine.BytesToString(a.Name64[:])
			if s.raw.Name == "" {
				s.raw.Name = engine.BytesToString(a.Name[:])
			}
		}
	case partRunway:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_RUNWAY {
			s.raw.Runways = setAt(s.raw.Runways, i, decodeRunway(data))
		}
	case partParking:
		switch m.Type {
		case types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING:
			s.raw.Parking = setAt(s.raw.Parking, i, *engine.CastDataAs[RawParking](data))
			if s.parkingByID == nil {
				s.parkingByID = map[uint32]int{}
			}
			s.parkingByID[uint32(m.UniqueRequestId)] = i
		case types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING_AIRLINE:
			p, ok := s.parkingByID[uint32(m.ParentUniqueRequestId)]
			n := int(m.DwSize) - int(unsafe.Offsetof(m.Data))
			if !ok || n <= 0 {
				break
			}
			if code := engine.BytesToString(unsafe.Slice((*byte)(unsafe.Pointer(data)), min(n, 32))); code != "" {
				if s.raw.ParkingAirlines == nil {
					s.raw.ParkingAirlines = map[int][]string{}
				}
				s.raw.ParkingAirlines[p] = append(s.raw.ParkingAirlines[p], code)
			}
		}
	case partTaxiPoint:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_POINT {
			s.raw.TaxiPoints = setAt(s.raw.TaxiPoints, i, *engine.CastDataAs[RawTaxiPoint](data))
		}
	case partTaxiPath:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_PATH {
			s.raw.TaxiPaths = setAt(s.raw.TaxiPaths, i, *engine.CastDataAs[RawTaxiPath](data))
		}
	case partTaxiName:
		if m.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_NAME {
			n := engine.CastDataAs[[32]byte](data)
			s.raw.TaxiNames = setAt(s.raw.TaxiNames, i, engine.BytesToString(n[:]))
		}
	}
}

// runwayWireSize is the packed RUNWAY record: 3×f64, 3×f32, 4×i32.
const runwayWireSize = 52

// decodeRunway reads the packed RUNWAY record field by field; the 52-byte
// record would be read past its end by a cast to an 8-byte aligned struct.
func decodeRunway(data *types.DWORD) RawRunway {
	b := unsafe.Slice((*byte)(unsafe.Pointer(data)), runwayWireSize)
	f64 := func(o int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b[o:])) }
	f32 := func(o int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b[o:])) }
	i32 := func(o int) int32 { return int32(binary.LittleEndian.Uint32(b[o:])) }
	return RawRunway{
		Latitude: f64(0), Longitude: f64(8), Altitude: f64(16),
		Heading: f32(24), Length: f32(28), Width: f32(32),
		PrimaryNumber: i32(36), PrimaryDesignator: i32(40), SecondaryNumber: i32(44), SecondaryDesignator: i32(48),
	}
}

// setAt stores v at index i, growing s as needed, so list items land at their
// SimConnect index even if they arrive out of order.
func setAt[T any](s []T, i int, v T) []T {
	for len(s) <= i {
		var zero T
		s = append(s, zero)
	}
	s[i] = v
	return s
}
