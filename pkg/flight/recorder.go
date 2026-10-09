//go:build windows

package flight

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Client is what a Recorder needs of a connection.
type Client interface {
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
}

// recVar is one SimVar the recorder reads, and where it goes in a Sample.
type recVar struct {
	name, unit string
	put        func(s *Sample, v float64)
}

// recVars are read every frame, in this order. Pitch and bank are kept
// nose up and right wing down positive, as the Injector takes them
// (SimConnect's own are the other way round), so a recorded Track replays
// as it was flown.
var recVars = func() []recVar {
	vs := []recVar{
		{"SIMULATION TIME", "seconds", func(s *Sample, v float64) { s.T = v }},
		{"PLANE LATITUDE", "degrees", func(s *Sample, v float64) { s.Lat = v }},
		{"PLANE LONGITUDE", "degrees", func(s *Sample, v float64) { s.Lon = v }},
		{"PLANE ALTITUDE", "feet", func(s *Sample, v float64) { s.AltFt = v }},
		{"GROUND ALTITUDE", "feet", func(s *Sample, v float64) { s.GroundFt = v }},
		{"STATIC CG TO GROUND", "feet", func(s *Sample, v float64) { s.CGFt = v }},
		{"PLANE PITCH DEGREES", "degrees", func(s *Sample, v float64) { s.Pitch = -v }},
		{"PLANE BANK DEGREES", "degrees", func(s *Sample, v float64) { s.Bank = -v }},
		{"PLANE HEADING DEGREES TRUE", "degrees", func(s *Sample, v float64) { s.Heading = v }},
		{"AIRSPEED INDICATED", "knots", func(s *Sample, v float64) { s.IAS = v }},
		{"GROUND VELOCITY", "knots", func(s *Sample, v float64) { s.GS = v }},
		{"VERTICAL SPEED", "feet per minute", func(s *Sample, v float64) { s.VS = v }},
		{"SIM ON GROUND", "bool", func(s *Sample, v float64) { s.OnGround = v != 0 }},
		{"GEAR HANDLE POSITION", "percent over 100", func(s *Sample, v float64) { s.GearHandle = v > 0.5 }},
		{"GEAR CENTER POSITION", "percent over 100", func(s *Sample, v float64) { s.GearPct = v * 100 }},
		{"FLAPS HANDLE INDEX", "number", func(s *Sample, v float64) { s.FlapsIndex = int(v) }},
		{"TRAILING EDGE FLAPS LEFT PERCENT", "percent", func(s *Sample, v float64) { s.FlapsPct = v }},
		{"FLAPS HANDLE PERCENT", "percent over 100", func(s *Sample, v float64) { s.FlapsHandle = v * 100 }},
		{"SPOILERS HANDLE POSITION", "percent", func(s *Sample, v float64) { s.Spoilers = v }},
		{"SPOILERS ARMED", "bool", func(s *Sample, v float64) { s.SpoilersArmed = v != 0 }},
		{"ELEVATOR POSITION", "percent over 100", func(s *Sample, v float64) { s.Elevator = v * 100 }},
		{"AILERON POSITION", "percent over 100", func(s *Sample, v float64) { s.Aileron = v * 100 }},
		{"RUDDER POSITION", "percent over 100", func(s *Sample, v float64) { s.Rudder = v * 100 }},
		{"BRAKE LEFT POSITION", "position 32k", func(s *Sample, v float64) { s.Brakes = max(s.Brakes, v/327.67) }},
		{"BRAKE RIGHT POSITION", "position 32k", func(s *Sample, v float64) { s.Brakes = max(s.Brakes, v/327.67) }},
		{"BRAKE PARKING POSITION", "bool", func(s *Sample, v float64) { s.ParkingBrake = v != 0 }},
		{"NUMBER OF ENGINES", "number", func(s *Sample, v float64) { s.EngineCount = int(v) }},
	}
	// The lights one by one (no LIGHT ON STATES in MSFS's SimVars).
	for _, l := range []struct {
		name string
		bit  int
	}{{"LIGHT NAV", LightNav}, {"LIGHT BEACON", LightBeacon}, {"LIGHT LANDING", LightLanding}, {"LIGHT TAXI", LightTaxi},
		{"LIGHT STROBE", LightStrobe}, {"LIGHT LOGO", LightLogo}, {"LIGHT WING", LightWing}} {
		vs = append(vs, recVar{l.name, "bool", func(s *Sample, v float64) {
			if v != 0 {
				s.Lights |= l.bit
			}
		}})
	}
	for i := range Engines {
		n := fmt.Sprint(i + 1)
		vs = append(vs,
			recVar{"GENERAL ENG THROTTLE LEVER POSITION:" + n, "percent", func(s *Sample, v float64) { s.Throttle[i] = v }},
			recVar{"TURB ENG N1:" + n, "percent", func(s *Sample, v float64) { s.N1[i] = v }},
			recVar{"TURB ENG REVERSE NOZZLE PERCENT:" + n, "percent", func(s *Sample, v float64) { s.Reverser[i] = v }})
	}
	return append(vs,
		recVar{"AUTOPILOT MASTER", "bool", func(s *Sample, v float64) { s.AP.Master = v != 0 }},
		recVar{"AUTOPILOT FLIGHT DIRECTOR ACTIVE", "bool", func(s *Sample, v float64) { s.AP.FD = v != 0 }},
		recVar{"AUTOPILOT THROTTLE ARM", "bool", func(s *Sample, v float64) { s.AP.Autothrottle = v != 0 }},
		recVar{"AUTOPILOT HEADING LOCK", "bool", func(s *Sample, v float64) { s.AP.Heading = v != 0 }},
		recVar{"AUTOPILOT ALTITUDE LOCK", "bool", func(s *Sample, v float64) { s.AP.Altitude = v != 0 }},
		recVar{"AUTOPILOT VERTICAL HOLD", "bool", func(s *Sample, v float64) { s.AP.VS = v != 0 }},
		recVar{"AUTOPILOT AIRSPEED HOLD", "bool", func(s *Sample, v float64) { s.AP.Speed = v != 0 }},
		recVar{"AUTOPILOT NAV1 LOCK", "bool", func(s *Sample, v float64) { s.AP.Nav = v != 0 }},
		recVar{"AUTOPILOT APPROACH HOLD", "bool", func(s *Sample, v float64) { s.AP.Approach = v != 0 }},
		recVar{"AUTOPILOT GLIDESLOPE HOLD", "bool", func(s *Sample, v float64) { s.AP.GS = v != 0 }},
		recVar{"AUTOPILOT HEADING LOCK DIR", "degrees", func(s *Sample, v float64) { s.AP.HeadingSel = v }},
		recVar{"AUTOPILOT ALTITUDE LOCK VAR", "feet", func(s *Sample, v float64) { s.AP.AltitudeSel = v }},
		recVar{"AUTOPILOT VERTICAL HOLD VAR", "feet per minute", func(s *Sample, v float64) { s.AP.VSSel = v }},
		recVar{"AUTOPILOT AIRSPEED HOLD VAR", "knots", func(s *Sample, v float64) { s.AP.SpeedSel = v }},
	)
}()

// DefaultRecorderBase is the first data definition and request ID a
// Recorder takes (one definition; a request per object recorded).
const DefaultRecorderBase uint32 = 0x7C00

// RecorderIDs is how many request IDs a Recorder takes after its base (the
// objects it can record at once).
const RecorderIDs = 63

// Recorder records aircraft — the user's (types.SIMCONNECT_OBJECT_ID_USER)
// or AI objects — every sim frame into Tracks. Feed it every message
// (Handle); it never reads the connection itself.
type Recorder struct {
	client Client
	base   uint32

	mu      sync.Mutex
	defined bool
	byReq   map[uint32]*recording
	byObj   map[uint32]uint32
	// OnSample, when set, hears each sample as it comes (a live view, a
	// puppet's stream); it must not block.
	OnSample func(objectID uint32, s Sample)
}

type recording struct {
	obj      uint32
	track    *Track
	interval uint32
}

// NewRecorder returns a Recorder on client, its IDs from base
// (DefaultRecorderBase when 0).
func NewRecorder(client Client, base uint32) *Recorder {
	if base == 0 {
		base = DefaultRecorderBase
	}
	return &Recorder{client: client, base: base, byReq: map[uint32]*recording{}, byObj: map[uint32]uint32{}}
}

// Reset forgets the definition for a new connection on client (nil: the
// same); recordings going on are asked again.
func (r *Recorder) Reset(client Client) error {
	r.mu.Lock()
	if client != nil {
		r.client = client
	}
	r.defined = false
	recs := make([]*recording, 0, len(r.byReq))
	for _, rec := range r.byReq {
		recs = append(recs, rec)
	}
	r.mu.Unlock()
	for _, rec := range recs {
		if err := r.request(rec); err != nil {
			return err
		}
	}
	return nil
}

// RecordOptions say what a recording is.
type RecordOptions struct {
	Title, Model, Note string
	// EveryFrames: a sample every this many frames (0 or 1: every frame).
	EveryFrames uint32
}

// ErrRecording: the object is recorded already, or no request ID is left.
var ErrRecording = fmt.Errorf("flight: cannot record")

// Start records objectID from its next frame on.
func (r *Recorder) Start(objectID uint32, o RecordOptions) error {
	r.mu.Lock()
	if _, ok := r.byObj[objectID]; ok {
		r.mu.Unlock()
		return fmt.Errorf("%w: object %d recorded already", ErrRecording, objectID)
	}
	req := uint32(0)
	for id := r.base + 1; id <= r.base+RecorderIDs; id++ {
		if _, used := r.byReq[id]; !used {
			req = id
			break
		}
	}
	if req == 0 {
		r.mu.Unlock()
		return fmt.Errorf("%w: %d objects recorded already", ErrRecording, RecorderIDs)
	}
	interval := uint32(0)
	if o.EveryFrames > 1 {
		interval = o.EveryFrames - 1
	}
	rec := &recording{obj: objectID, interval: interval, track: &Track{Version: TrackVersion, Title: o.Title, Model: o.Model, Note: o.Note,
		User: objectID == types.SIMCONNECT_OBJECT_ID_USER, Started: time.Now()}}
	r.byReq[req], r.byObj[objectID] = rec, req
	r.mu.Unlock()
	return r.request(rec)
}

// request defines the frame (once a connection) and asks for rec's object.
func (r *Recorder) request(rec *recording) error {
	r.mu.Lock()
	client, defined, req := r.client, r.defined, r.byObj[rec.obj]
	r.defined = true
	r.mu.Unlock()
	if !defined {
		for i, v := range recVars {
			if err := client.AddToDataDefinition(r.base, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
				return fmt.Errorf("flight: define %s: %w", v.name, err)
			}
		}
	}
	return client.RequestDataOnSimObject(req, r.base, rec.obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, rec.interval, 0)
}

// Stop ends objectID's recording and returns its Track (nil when it was
// not recorded).
func (r *Recorder) Stop(objectID uint32) *Track {
	r.mu.Lock()
	req, ok := r.byObj[objectID]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	rec := r.byReq[req]
	delete(r.byObj, objectID)
	delete(r.byReq, req)
	client := r.client
	r.mu.Unlock()
	_ = client.RequestDataOnSimObject(req, r.base, objectID, types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	return rec.track
}

// Snapshot is a copy of objectID's Track so far (nil when not recorded).
func (r *Recorder) Snapshot(objectID uint32) *Track {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.byObj[objectID]
	if !ok {
		return nil
	}
	t := *r.byReq[req].track
	t.Samples = append([]Sample(nil), t.Samples...)
	return &t
}

// Handle takes a message: true when it was a frame of a recording.
func (r *Recorder) Handle(msg engine.Message) bool {
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return false
	}
	d := msg.AsSimObjectData()
	if d == nil {
		return false
	}
	r.mu.Lock()
	rec, ok := r.byReq[uint32(d.DwRequestID)]
	if !ok || uint32(d.DwDefineID) != r.base {
		r.mu.Unlock()
		return false
	}
	need := uint32(unsafe.Offsetof(d.DwData)) + uint32(len(recVars))*8
	if msg.Size != 0 && msg.Size < need {
		r.mu.Unlock()
		return true // ours, but cut short: not read past its end
	}
	vals := unsafe.Slice((*float64)(unsafe.Pointer(&d.DwData)), len(recVars))
	s := sampleOf(vals)
	rec.track.Samples = append(rec.track.Samples, s)
	on, obj := r.OnSample, rec.obj
	r.mu.Unlock()
	if on != nil {
		on(obj, s)
	}
	return true
}

// sampleOf is the sample of a frame's values, in recVars order.
func sampleOf(vals []float64) Sample {
	var s Sample
	for i, v := range recVars {
		if i < len(vals) {
			v.put(&s, vals[i])
		}
	}
	return s
}
