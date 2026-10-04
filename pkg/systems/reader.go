//go:build windows
// +build windows

package systems

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// State is the user aircraft's systems as a profile resolves them.
type State struct {
	Battery, Powered, Avionics bool
	Volts                      float64
	ExtAvailable, ExtOn        bool
	COM1, COM2                 bool // working
	// COM frequencies in MHz (0 when the profile gives none).
	COM1Active, COM1Standby, COM2Active, COM2Standby float64
	Engines                                          int
	Running, Starter                                 [4]bool // engines 1–4
	ParkingBrake                                     bool
	Beacon, Nav, Strobe                              bool
	Landing, Taxi                                    bool
	Doors                                            [4]bool // EXIT OPEN 0–3, open
	XPDRState                                        int     // 0 off, 1 standby, 2 test, 3 on, 4 alt
	Squawk                                           string  // e.g. "4521"
	FlapsPct                                         float64
	GearDown                                         bool
	// Chocks in place and the aircraft's own GPU connected (#667);
	// HasChocks, HasGPU: the profile gives them (the model has them).
	Chocks, GPU, HasChocks, HasGPU bool
	// The sim's pushback (#666): a tug attached, possible here, waiting.
	PushbackAttached, PushbackAvailable, PushbackWait bool
	// Values are all resolved values by name (the constants above), for
	// values a profile adds.
	Values map[string]float64
}

// Client is what a Reader needs of a connection.
type Client interface {
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	ClearDataDefinition(definitionID uint32) error
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
}

// Reader reads a profile's state on the user aircraft: Use the profile,
// Request (once, or every period), Handle each message.
type Reader struct {
	client       Client
	defID, reqID uint32

	mu      sync.Mutex
	profile Profile
	vars    []varUnit
	ready   bool
	defined bool // the definition was registered once: cleared before again
	last    State
}

// NewReader returns a Reader on client with its definition and request IDs.
func NewReader(client Client, defID, reqID uint32) *Reader {
	return &Reader{client: client, defID: defID, reqID: reqID}
}

// Use sets the profile (For the aircraft loaded); its variables are
// registered on the next Request.
func (r *Reader) Use(p Profile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profile, r.vars, r.ready = p, p.vars(), false
}

// Reset forgets the registration (a new connection).
func (r *Reader) Reset(client Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if client != nil {
		r.client = client
	}
	r.ready, r.defined = false, false
}

// Request asks for the state of the user aircraft, every period (e.g.
// SIMCONNECT_PERIOD_SECOND) or once.
func (r *Reader) Request(period types.SIMCONNECT_PERIOD) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.vars) == 0 {
		return fmt.Errorf("systems: no profile in use")
	}
	if !r.ready {
		if r.defined {
			_ = r.client.ClearDataDefinition(r.defID)
		}
		for i, v := range r.vars {
			if err := r.client.AddToDataDefinition(r.defID, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
				return fmt.Errorf("systems: define %s: %w", v.name, err)
			}
		}
		r.ready, r.defined = true, true
	}
	return r.client.RequestDataOnSimObject(r.reqID, r.defID, types.SIMCONNECT_OBJECT_ID_USER, period, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
}

// Handle takes the state from msg when it is the Reader's.
func (r *Reader) Handle(msg engine.Message) (State, bool) {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return State{}, false
	}
	d := msg.AsSimObjectData()
	if uint32(d.DwRequestID) != r.reqID {
		return State{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.vars)
	// The data after the header: n FLOAT64s, as defined.
	header := uint32(unsafe.Offsetof(d.DwData))
	if n == 0 || msg.Size < header+uint32(8*n) {
		return State{}, false
	}
	raw := unsafe.Slice((*float64)(unsafe.Pointer(&d.DwData)), n)
	read := make(map[varUnit]float64, n)
	for i, v := range r.vars {
		read[v] = raw[i]
	}
	r.last = resolveState(r.profile, read)
	return r.last, true
}

// State is the last state read.
func (r *Reader) State() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

// resolveState resolves every value of p from the variables read.
func resolveState(p Profile, read map[varUnit]float64) State {
	s := State{Values: map[string]float64{}}
	for k, v := range p.Values {
		s.Values[k] = v.resolve(read)
	}
	on := func(k string) bool { return s.Values[k] != 0 }
	s.Battery, s.Powered, s.Avionics, s.Volts = on(Battery), on(Powered), on(Avionics), s.Values[Volts]
	s.ExtAvailable, s.ExtOn, s.COM1, s.COM2 = on(ExtAvailable), on(ExtOn), on(COM1Power), on(COM2Power)
	s.Engines = int(s.Values[EngineCount])
	for i := range 4 {
		s.Running[i], s.Starter[i], s.Doors[i] = on(EngineRunning(i+1)), on(Starter(i+1)), on(Door(i))
	}
	s.ParkingBrake = on(ParkingBrake)
	s.Beacon, s.Nav, s.Strobe, s.Landing, s.Taxi = on(LightBeacon), on(LightNav), on(LightStrobe), on(LightLanding), on(LightTaxi)
	s.XPDRState = int(s.Values[XPDRState])
	if _, ok := p.Values[XPDRCode]; ok {
		s.Squawk = squawkOf(uint32(s.Values[XPDRCode]))
	}
	s.FlapsPct, s.GearDown = s.Values[FlapsPct], on(GearDown)
	_, s.HasChocks = p.Values[Chocks]
	_, s.HasGPU = p.Values[GPU]
	s.Chocks, s.GPU = on(Chocks), on(GPU)
	s.PushbackAttached, s.PushbackAvailable, s.PushbackWait = on(PushbackAttached), on(PushbackAvailable), on(PushbackWait)
	s.COM1Active, s.COM1Standby, s.COM2Active, s.COM2Standby = s.Values[COM1Active], s.Values[COM1Standby], s.Values[COM2Active], s.Values[COM2Standby]
	return s
}

// squawkOf is a BCD16 code as four digits: 0x4521 → "4521".
func squawkOf(bcd uint32) string {
	return fmt.Sprintf("%d%d%d%d", bcd>>12&0xF, bcd>>8&0xF, bcd>>4&0xF, bcd&0xF)
}
