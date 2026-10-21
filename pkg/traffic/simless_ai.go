package traffic

import (
	"encoding/binary"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// SimlessAI stands in for MSFS AI where no simulator flies the aircraft
// (Vector, the ATC game: the World in WebAssembly on a loopback client).
// It wraps the client: an aircraft released to AI with a waypoint chain
// (a STAR and approach to the join point, a SID, an en-route plan) is
// flown along it here, at each point's speed, reaching each point's
// altitude by it, turning at the rate MSFS AI does, and its position
// written each Step as the simulator would move it. An aircraft placed by
// anyone else (the injected approach taking over at the join point) is no
// longer flown. Everything else passes through to the client unchanged;
// with a simulator attached it is not used at all.
type SimlessAI struct {
	engine.Client

	mu      sync.Mutex
	posDef  uint32 // the first data definition of "Initial Position": ours to write
	hasPos  bool
	posDefs map[uint32]bool
	wpDefs  map[uint32]bool
	objects map[uint32]*simlessObject
	writing bool // our own position write under way
	// freezeEvents: the client events that freeze an aircraft in place.
	freezeEvents map[uint32]bool
	last         time.Time
	// created: request → the pose an aircraft was created at
	// (AICreateNonATCAircraft), its object's once the simulator names it
	// (Handle).
	created map[uint32]types.SIMCONNECT_DATA_INITPOSITION
}

type simlessObject struct {
	wps      []types.SIMCONNECT_DATA_WAYPOINT
	next     int
	released bool
	// ai: released to the AI once (AIReleaseControl): unfrozen again
	// (Injector.Release) it is the AI's once more, as in MSFS.
	ai    bool
	posed bool // a pose known (placed, or flown)
	pos   airport.LatLon
	altFt float64
	hdg   float64
	kts   float64
	bank  float64
	vs    float64 // ft/min
}

// Simless flying tunables.
const (
	// simlessTurnDegPerSec: a standard rate turn.
	simlessTurnDegPerSec = 3.0
	// simlessAccelKts: speed changes at this many knots a second.
	simlessAccelKts = 1.5
	// simlessMaxVS: the steepest climb or descent, ft/min.
	simlessMaxVS = 3000.0
	// simlessStartKts: the speed of an aircraft released with none known.
	simlessStartKts = 250.0
)

// SimlessAIDefID is the data definition SimlessAI defines for the
// positions it writes when no other definition of a position came first.
var SimlessAIDefID uint32 = 0x7FFF_F000

// NewSimlessAI wraps client.
func NewSimlessAI(client engine.Client) *SimlessAI {
	return &SimlessAI{Client: client, wpDefs: map[uint32]bool{}, posDefs: map[uint32]bool{}, objects: map[uint32]*simlessObject{},
		created: map[uint32]types.SIMCONNECT_DATA_INITPOSITION{}}
}

// AICreateNonATCAircraft notes where the aircraft is created, then
// passes the call on.
func (s *SimlessAI) AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	s.noteCreated(initPos, RequestID)
	return s.Client.AICreateNonATCAircraft(szContainerTitle, szTailNumber, initPos, RequestID)
}

// AICreateNonATCAircraftEX1 notes where the aircraft is created, then
// passes the call on.
func (s *SimlessAI) AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	s.noteCreated(initPos, RequestID)
	return s.Client.AICreateNonATCAircraftEX1(szContainerTitle, szLivery, szTailNumber, initPos, RequestID)
}

func (s *SimlessAI) noteCreated(p types.SIMCONNECT_DATA_INITPOSITION, req uint32) {
	s.mu.Lock()
	s.created[req] = p
	s.mu.Unlock()
}

// Handle takes the object ID the simulator gives a created aircraft: its
// pose is where it was created, so released to the AI it is flown from
// there (a STAR from its first fix), not from its first waypoint, the
// next fix (MyCrew app, offline: an arrival on APRA2S appeared 6 NM out at
// 10,000 ft).
func (s *SimlessAI) Handle(msg engine.Message) {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		return
	}
	a := msg.AsAssignedObjectID()
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.created[uint32(a.DwRequestID)]
	if !ok {
		return
	}
	delete(s.created, uint32(a.DwRequestID))
	o := s.object(uint32(a.DwObjectID))
	if o.posed {
		return
	}
	o.posed, o.pos, o.altFt, o.hdg = true, airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}, p.Altitude, p.Heading
	if o.kts = float64(p.Airspeed); o.kts <= 0 {
		o.kts = simlessStartKts
	}
}

// AddToDataDefinition notes which definitions are positions and waypoint
// lists, then passes the call on.
func (s *SimlessAI) AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error {
	s.mu.Lock()
	switch datumType {
	case types.SIMCONNECT_DATATYPE_INITPOSITION:
		if !s.hasPos {
			s.posDef, s.hasPos = definitionID, true
		}
		s.posDefs[definitionID] = true
	case types.SIMCONNECT_DATATYPE_WAYPOINT:
		s.wpDefs[definitionID] = true
	}
	s.mu.Unlock()
	return s.Client.AddToDataDefinition(definitionID, datumName, unitsName, datumType, epsilon, datumID)
}

// SetDataOnSimObject takes a waypoint chain for the aircraft to fly and
// notes a position someone else placed it at (no longer flown here), then
// passes the call on.
func (s *SimlessAI) SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	s.mu.Lock()
	switch {
	case s.wpDefs[definitionID] && data != nil && arrayCount > 0 && cbUnitSize == engine.WaypointWireSize:
		o := s.object(objectID)
		o.wps = unpackWaypoints(unsafe.Slice((*byte)(data), int(arrayCount*cbUnitSize)), int(arrayCount))
		o.next = 0
	case s.posDefs[definitionID] && !s.writing && data != nil:
		p := *(*types.SIMCONNECT_DATA_INITPOSITION)(data)
		o := s.object(objectID)
		o.released = false // placed by its controller: not ours to fly
		o.posed, o.pos, o.altFt, o.hdg = true, airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}, p.Altitude, p.Heading
		o.kts = float64(p.Airspeed)
	}
	s.mu.Unlock()
	return s.Client.SetDataOnSimObject(definitionID, objectID, flags, arrayCount, cbUnitSize, data)
}

// AIReleaseControl hands the aircraft to the AI: flown here along its
// waypoints from now on.
func (s *SimlessAI) AIReleaseControl(objectID uint32, requestID uint32) error {
	s.mu.Lock()
	o := s.object(objectID)
	o.released, o.ai = true, true
	s.mu.Unlock()
	return s.Client.AIReleaseControl(objectID, requestID)
}

// MapClientEventToSimEvent notes the freeze event (an aircraft frozen is
// taken from the AI: Injector.Takeover), then passes the call on.
func (s *SimlessAI) MapClientEventToSimEvent(eventID uint32, eventName string) error {
	if eventName == "FREEZE_LATITUDE_LONGITUDE_SET" {
		s.mu.Lock()
		if s.freezeEvents == nil {
			s.freezeEvents = map[uint32]bool{}
		}
		s.freezeEvents[eventID] = true
		s.mu.Unlock()
	}
	return s.Client.MapClientEventToSimEvent(eventID, eventName)
}

// TransmitClientEvent stops flying an aircraft frozen in place (taken over
// by its controller) and flies one unfrozen again that the AI had (a
// departure handed back after its injected climb: live in Vector it stood
// there for good), then passes the call on.
func (s *SimlessAI) TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	s.mu.Lock()
	if s.freezeEvents[eventID] {
		if o := s.objects[objectID]; o != nil {
			if data != 0 {
				o.released = false
			} else if o.ai {
				o.released = true
			}
		}
	}
	s.mu.Unlock()
	return s.Client.TransmitClientEvent(objectID, eventID, data, groupID, flags)
}

// AIRemoveObject forgets the aircraft, then passes the call on.
func (s *SimlessAI) AIRemoveObject(objectID uint32, requestID uint32) error {
	s.mu.Lock()
	delete(s.objects, objectID)
	s.mu.Unlock()
	return s.Client.AIRemoveObject(objectID, requestID)
}

// object is objectID's state, made on first sight. s.mu held.
func (s *SimlessAI) object(objectID uint32) *simlessObject {
	o := s.objects[objectID]
	if o == nil {
		o = &simlessObject{}
		s.objects[objectID] = o
	}
	return o
}

// Flying reports whether objectID is flown here now.
func (s *SimlessAI) Flying(objectID uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.objects[objectID]
	return o != nil && o.released && len(o.wps) > 0
}

// Step moves every aircraft flown here to now (traffic time: the
// simulation rate, stopped while paused) and writes where each is.
func (s *SimlessAI) Step(now time.Time) {
	s.mu.Lock()
	dt := 0.0
	if !s.last.IsZero() {
		dt = now.Sub(s.last).Seconds()
	}
	s.last = now
	if dt <= 0 || dt > 5 {
		s.mu.Unlock()
		return
	}
	if !s.hasPos { // nobody defined a position yet: our own definition
		if err := s.Client.AddToDataDefinition(SimlessAIDefID, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0); err != nil {
			s.mu.Unlock()
			return
		}
		s.posDef, s.hasPos = SimlessAIDefID, true
	}
	type write struct {
		id uint32
		p  types.SIMCONNECT_DATA_INITPOSITION
	}
	var out []write
	for id, o := range s.objects {
		if !o.released || len(o.wps) == 0 {
			continue
		}
		o.fly(dt)
		out = append(out, write{id, types.SIMCONNECT_DATA_INITPOSITION{
			Latitude: o.pos.Lat, Longitude: o.pos.Lon, Altitude: o.altFt,
			Pitch:    -math.Max(-10, math.Min(15, o.vs/1000*2.5)), // nose up climbing (SimConnect: negative up)
			Bank:     -o.bank,
			Heading:  o.hdg,
			Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(o.kts)),
		}})
	}
	def := s.posDef
	s.writing = true
	s.mu.Unlock()
	for _, w := range out {
		p := w.p
		s.Client.SetDataOnSimObject(def, w.id, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
	}
	s.mu.Lock()
	s.writing = false
	s.mu.Unlock()
}

// fly moves o dt seconds along its waypoints.
func (o *simlessObject) fly(dt float64) {
	if !o.posed { // released with no pose known: at its first point, facing the next
		w := o.wps[0]
		o.pos, o.altFt, o.kts, o.posed = airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}, w.Altitude, w.KtsSpeed, true
		if o.kts <= 0 {
			o.kts = simlessStartKts
		}
		if len(o.wps) > 1 {
			o.next = 1
			n := o.wps[1]
			o.hdg = calc.BearingDegrees(w.Latitude, w.Longitude, n.Latitude, n.Longitude)
		}
	}
	if o.next < len(o.wps) {
		w := o.wps[o.next]
		d := calc.HaversineMeters(o.pos.Lat, o.pos.Lon, w.Latitude, w.Longitude)
		// Passed: within the turn's lead of it (the next leg's turn begun
		// early, as MSFS AI does), or close.
		lead := 300.0
		if o.next+1 < len(o.wps) {
			n := o.wps[o.next+1]
			turn := math.Abs(headingDiff(calc.BearingDegrees(w.Latitude, w.Longitude, n.Latitude, n.Longitude), o.hdg))
			lead = math.Max(lead, turnRadiusM(o.kts)*math.Tan(math.Min(turn, 150)/2*math.Pi/180))
		}
		if d < lead {
			o.next++
		}
	}
	// Steer for the point ahead, else on along the last leg.
	target := o.hdg
	if o.next < len(o.wps) {
		w := o.wps[o.next]
		target = calc.BearingDegrees(o.pos.Lat, o.pos.Lon, w.Latitude, w.Longitude)
		if w.Flags&uint32(types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED) != 0 && w.KtsSpeed > 0 {
			o.kts += math.Max(-simlessAccelKts*dt, math.Min(simlessAccelKts*dt, w.KtsSpeed-o.kts))
		}
		// The point's altitude by the point: the rate it takes, within
		// the steepest.
		d := calc.HaversineMeters(o.pos.Lat, o.pos.Lon, w.Latitude, w.Longitude)
		secs := d / math.Max(o.kts*1852/3600, 1)
		want := (w.Altitude - o.altFt) / math.Max(secs, 1) * 60
		o.vs = math.Max(-simlessMaxVS, math.Min(simlessMaxVS, want))
	} else {
		o.vs = 0
	}
	turn := headingDiff(o.hdg, target)
	step := math.Max(-simlessTurnDegPerSec*dt, math.Min(simlessTurnDegPerSec*dt, turn))
	o.hdg = math.Mod(o.hdg+step+360, 360)
	o.bank = 0
	if math.Abs(turn) > 1 {
		o.bank = math.Copysign(math.Min(25, math.Abs(turn)*2), turn)
	}
	o.altFt += o.vs / 60 * dt
	lat, lon := calc.DisplaceByHeading(o.pos.Lat, o.pos.Lon, o.hdg, o.kts*1852/3600*dt)
	o.pos = airport.LatLon{Lat: lat, Lon: lon}
}

// turnRadiusM is the radius of a standard rate turn at kts.
func turnRadiusM(kts float64) float64 {
	v := kts * 1852 / 3600
	return v / (simlessTurnDegPerSec * math.Pi / 180)
}

// unpackWaypoints reads n waypoints of the packed wire format
// (engine.PackWaypoints).
func unpackWaypoints(b []byte, n int) []types.SIMCONNECT_DATA_WAYPOINT {
	out := make([]types.SIMCONNECT_DATA_WAYPOINT, 0, n)
	f := func(o int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b[o:])) }
	for i := 0; i < n; i++ {
		o := i * engine.WaypointWireSize
		out = append(out, types.SIMCONNECT_DATA_WAYPOINT{
			Latitude: f(o), Longitude: f(o + 8), Altitude: f(o + 16),
			Flags:    binary.LittleEndian.Uint32(b[o+24:]),
			KtsSpeed: f(o + 28), PercentThrottle: f(o + 36),
		})
	}
	return out
}
