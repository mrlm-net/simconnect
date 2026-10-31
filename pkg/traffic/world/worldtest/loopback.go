// Package worldtest runs a World with no simulator, for tests: a loopback
// engine.Client (Loopback) stands in for MSFS — it keeps the objects
// created, takes their positions as they are written and answers every
// data request from them — and Harness runs a World on it with SimlessAI,
// an airport loaded from an offline dump (simconnect-data), the radio
// heard and a scripted user aircraft.
//
// Time runs on the wall clock (the World's own tickers), so a scenario
// takes as long as it would in the simulator; Advance waits.
package worldtest

import (
	"encoding/binary"
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Object is a simulated object as the loopback keeps it.
type Object struct {
	ID               uint32
	Title, ATCID     string
	Lat, Lon         float64 // degrees
	AltFt            float64 // above mean sea level
	Heading          float64 // degrees true
	Pitch, Bank      float64 // degrees, SimConnect's sign (positive nose down, left wing down)
	GroundKts, VSFpm float64
	OnGround         bool
	// COM1 is the active COM1 frequency, MHz (the user aircraft).
	COM1 float64
	// EnginesRunning: its engines run (GENERAL ENG COMBUSTION).
	EnginesRunning bool
	// Vehicle: a simulated object (AICreateSimulatedObject: a tug, a truck),
	// not an aircraft: by-type scans tell them apart as the simulator does.
	Vehicle bool
	// GearUp: its gear is up.
	GearUp bool
	// Lights: the lights on, by SimVar ("LIGHT BEACON"), as events set them.
	Lights map[string]bool
}

type datum struct {
	name, unit string
	typ        types.SIMCONNECT_DATATYPE
}

type request struct {
	req, def, obj uint32
	period        types.SIMCONNECT_PERIOD
	interval      uint32
	frames        uint32
	last          time.Time
}

// UserObjectID is the user aircraft's object ID.
const UserObjectID = 1

// Loopback is an engine.Client with no simulator behind it: objects
// created get IDs, positions written are kept, and data requests are
// answered from them on Step, each SimVar by its name and unit (one it
// does not know is 0). Facility, flight-plan and other calls do nothing.
type Loopback struct {
	mu sync.Mutex
	// cameraOwned, cameraSets: the add-on camera (camera.go).
	cameraOwned bool
	cameraPose  types.SIMCONNECT_DATA_CAMERA
	events      map[uint32]string // client event ID → its simulator event (lights.go)
	cameraSets  int
	out         chan engine.Message
	defs        map[uint32][]datum
	reqs        map[uint32]*request
	objects     map[uint32]*Object
	nextID      uint32
	sent        uint32
	frameEvt    uint32
	hasFrame    bool
	// GroundFt is the ground elevation everywhere (the airport's), feet.
	groundFt float64
	// Spawn is where an aircraft created with no position goes (ParkedATC,
	// enroute): the airport.
	spawnLat, spawnLon float64
	clock              func() time.Time
	aircraftTitles     []string
	groundTitles       []string
}

// NewLoopback returns a loopback with the user aircraft at lat, lon on
// ground groundFt (feet), messages buffered up to queue.
func NewLoopback(lat, lon, groundFt float64, queue int) *Loopback {
	l := &Loopback{
		out:      make(chan engine.Message, queue),
		defs:     map[uint32][]datum{},
		reqs:     map[uint32]*request{},
		objects:  map[uint32]*Object{},
		nextID:   1000,
		groundFt: groundFt, spawnLat: lat, spawnLon: lon,
		clock: traffic.Now,
	}
	l.objects[UserObjectID] = &Object{ID: UserObjectID, Title: "User Aircraft", ATCID: "PLAYER", Lat: lat, Lon: lon, AltFt: groundFt, OnGround: true, COM1: 118.1}
	return l
}

// User sets the user aircraft with f (under the loopback's lock).
func (l *Loopback) User(f func(o *Object)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f(l.objects[UserObjectID])
}

// Objects are copies of the objects now, the user aircraft included.
func (l *Loopback) Objects() []Object {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Object, 0, len(l.objects))
	for _, o := range l.objects {
		out = append(out, *o)
	}
	return out
}

func (l *Loopback) Connect() error    { return nil }
func (l *Loopback) Disconnect() error { return nil }

func (l *Loopback) GetLastSentPacketID() (uint32, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sent++
	return l.sent, nil
}

func (l *Loopback) Stream() <-chan engine.Message { return l.out }

func (l *Loopback) AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.defs[definitionID] = append(l.defs[definitionID], datum{strings.ToUpper(strings.TrimSpace(datumName)), strings.ToLower(unitsName), datumType})
	return nil
}

func (l *Loopback) ClearDataDefinition(definitionID uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.defs, definitionID)
	return nil
}

func (l *Loopback) RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if objectID == types.SIMCONNECT_OBJECT_ID_USER {
		objectID = UserObjectID
	}
	if period == types.SIMCONNECT_PERIOD_NEVER {
		delete(l.reqs, requestID)
		return nil
	}
	r := &request{req: requestID, def: definitionID, obj: objectID, period: period, interval: interval}
	if period == types.SIMCONNECT_PERIOD_ONCE {
		l.sendData(r, false, 1, 1)
		return nil
	}
	l.reqs[requestID] = r
	return nil
}

func (l *Loopback) RequestDataOnSimObjectType(requestID uint32, definitionID uint32, dwRadiusMeters uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	var ids []uint32
	for id, o := range l.objects {
		switch objectType {
		case types.SIMCONNECT_SIMOBJECT_TYPE_USER:
			if id != UserObjectID {
				continue
			}
		case types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT:
			if o.Vehicle {
				continue // a tug or truck is no aircraft
			}
		case types.SIMCONNECT_SIMOBJECT_TYPE_GROUND:
			if !o.Vehicle {
				continue
			}
		}
		ids = append(ids, id)
	}
	for i, id := range ids {
		l.sendData(&request{req: requestID, def: definitionID, obj: id}, true, uint32(i+1), uint32(len(ids)))
	}
	return nil
}

func (l *Loopback) SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if objectID == types.SIMCONNECT_OBJECT_ID_USER {
		objectID = UserObjectID
	}
	o := l.objects[objectID]
	ds := l.defs[definitionID]
	if o == nil || data == nil || len(ds) == 0 {
		return nil
	}
	if ds[0].typ == types.SIMCONNECT_DATATYPE_INITPOSITION {
		p := *(*types.SIMCONNECT_DATA_INITPOSITION)(data)
		if p.Latitude != 0 || p.Longitude != 0 {
			o.Lat, o.Lon, o.AltFt = p.Latitude, p.Longitude, p.Altitude
			o.Pitch, o.Bank, o.Heading = p.Pitch, p.Bank, p.Heading
			o.OnGround = p.OnGround != 0
			o.GroundKts = float64(p.Airspeed)
		}
		return nil
	}
	// Plain values (a definition of FLOAT64s): the position and attitude
	// ones taken, the rest ignored.
	b := unsafe.Slice((*byte)(data), int(cbUnitSize))
	off := 0
	for _, d := range ds {
		if d.typ != types.SIMCONNECT_DATATYPE_FLOAT64 || off+8 > len(b) {
			return nil
		}
		v := math.Float64frombits(binary.LittleEndian.Uint64(b[off:]))
		off += 8
		switch d.name {
		case "PLANE LATITUDE":
			o.Lat = deg(v, d.unit)
		case "PLANE LONGITUDE":
			o.Lon = deg(v, d.unit)
		case "PLANE ALTITUDE":
			o.AltFt = v
		case "PLANE HEADING DEGREES TRUE":
			o.Heading = deg(v, d.unit)
		}
	}
	return nil
}

func (l *Loopback) create(title, tail string, lat, lon, altFt, hdg float64, onGround bool, requestID uint32) error {
	return l.createObject(title, tail, lat, lon, altFt, hdg, onGround, false, requestID)
}

func (l *Loopback) createObject(title, tail string, lat, lon, altFt, hdg float64, onGround, vehicle bool, requestID uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextID++
	id := l.nextID
	if lat == 0 && lon == 0 {
		lat, lon, altFt, onGround = l.spawnLat, l.spawnLon, l.groundFt, true
	}
	l.objects[id] = &Object{ID: id, Title: title, ATCID: tail, Lat: lat, Lon: lon, AltFt: altFt, Heading: hdg, OnGround: onGround, Vehicle: vehicle}
	m := types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID{DwRequestID: types.DWORD(requestID), DwObjectID: types.DWORD(id)}
	m.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID)
	l.emit(unsafe.Slice((*byte)(unsafe.Pointer(&m)), unsafe.Sizeof(m)))
	return nil
}

func (l *Loopback) AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error {
	return l.create(szContainerTitle, szTailNumber, 0, 0, 0, 0, true, RequestID)
}

func (l *Loopback) AICreateParkedATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, szAirportID string, RequestID uint32) error {
	return l.create(szContainerTitle, szTailNumber, 0, 0, 0, 0, true, RequestID)
}

func (l *Loopback) AICreateEnrouteATCAircraft(szContainerTitle string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	return l.create(szContainerTitle, szTailNumber, 0, 0, 0, 0, true, RequestID)
}

func (l *Loopback) AICreateEnrouteATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, iFlightNumber uint32, szFlightPlanPath string, dFlightPlanPosition float64, bTouchAndGo bool, RequestID uint32) error {
	return l.create(szContainerTitle, szTailNumber, 0, 0, 0, 0, true, RequestID)
}

func (l *Loopback) AICreateNonATCAircraft(szContainerTitle string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	return l.create(szContainerTitle, szTailNumber, initPos.Latitude, initPos.Longitude, initPos.Altitude, initPos.Heading, initPos.OnGround != 0, RequestID)
}

func (l *Loopback) AICreateNonATCAircraftEX1(szContainerTitle string, szLivery string, szTailNumber string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	return l.create(szContainerTitle, szTailNumber, initPos.Latitude, initPos.Longitude, initPos.Altitude, initPos.Heading, initPos.OnGround != 0, RequestID)
}

func (l *Loopback) AICreateSimulatedObject(szContainerTitle string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	return l.createObject(szContainerTitle, "", initPos.Latitude, initPos.Longitude, initPos.Altitude, initPos.Heading, initPos.OnGround != 0, true, RequestID)
}

func (l *Loopback) AICreateSimulatedObjectEX1(szContainerTitle string, szLivery string, initPos types.SIMCONNECT_DATA_INITPOSITION, RequestID uint32) error {
	return l.createObject(szContainerTitle, "", initPos.Latitude, initPos.Longitude, initPos.Altitude, initPos.Heading, initPos.OnGround != 0, true, RequestID)
}

func (l *Loopback) AIRemoveObject(objectID uint32, requestID uint32) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if objectID != UserObjectID {
		delete(l.objects, objectID)
	}
	return nil
}

func (l *Loopback) SubscribeToSystemEvent(eventID uint32, eventName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if strings.EqualFold(eventName, "Frame") {
		l.frameEvt, l.hasFrame = eventID, true
	}
	return nil
}

// Step sends what the simulator would in a frame: the Frame event and the
// data of every request due (each frame, or each second).
func (l *Loopback) Step() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	if l.hasFrame {
		var m types.SIMCONNECT_RECV_EVENT_FRAME
		m.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_EVENT_FRAME)
		m.UGroupID = types.DWORD(0xFFFFFFFF)
		m.UEventID = types.DWORD(l.frameEvt)
		m.FFrameRate, m.FSimSpeed = 60, 1
		l.emit(unsafe.Slice((*byte)(unsafe.Pointer(&m)), unsafe.Sizeof(m)))
	}
	for id, r := range l.reqs {
		if _, ok := l.objects[r.obj]; !ok {
			delete(l.reqs, id)
			continue
		}
		switch r.period {
		case types.SIMCONNECT_PERIOD_SECOND:
			if now.Sub(r.last) < time.Duration(r.interval+1)*time.Second {
				continue
			}
		default: // each frame, every interval+1 of them
			r.frames++
			if r.frames <= r.interval {
				continue
			}
			r.frames = 0
		}
		r.last = now
		l.sendData(r, false, 1, 1)
	}
}

// sendData sends r's data for its object. l.mu held.
func (l *Loopback) sendData(r *request, byType bool, entry, outOf uint32) {
	o := l.objects[r.obj]
	ds, ok := l.defs[r.def]
	if o == nil || !ok {
		return
	}
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	body := l.encode(o, ds)
	buf := make([]byte, off+len(body)+4)
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	if byType {
		h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE)
	}
	h.DwRequestID, h.DwObjectID, h.DwDefineID = types.DWORD(r.req), types.DWORD(o.ID), types.DWORD(r.def)
	h.DwEntryNumber, h.DwOutOf, h.DwDefineCount = types.DWORD(entry), types.DWORD(outOf), types.DWORD(len(ds))
	copy(buf[off:], body)
	l.emit(buf)
}

// emit sends a message built in b, its size set. l.mu held. A full queue
// drops it, as a slow reader of the simulator would lose nothing but
// time: the World asks again.
func (l *Loopback) emit(b []byte) {
	h := (*types.SIMCONNECT_RECV)(unsafe.Pointer(&b[0]))
	h.DwSize, h.DwVersion = types.DWORD(len(b)), 1
	select {
	case l.out <- engine.Message{SIMCONNECT_RECV: h, Size: uint32(len(b))}:
	default:
	}
}

// encode is o's values for definition ds, as the simulator packs them.
func (l *Loopback) encode(o *Object, ds []datum) []byte {
	var b []byte
	for _, d := range ds {
		switch d.typ {
		case types.SIMCONNECT_DATATYPE_FLOAT64:
			b = binary.LittleEndian.AppendUint64(b, math.Float64bits(l.value(o, d)))
		case types.SIMCONNECT_DATATYPE_FLOAT32:
			b = binary.LittleEndian.AppendUint32(b, math.Float32bits(float32(l.value(o, d))))
		case types.SIMCONNECT_DATATYPE_INT32:
			b = binary.LittleEndian.AppendUint32(b, uint32(int32(l.value(o, d))))
		case types.SIMCONNECT_DATATYPE_INT64:
			b = binary.LittleEndian.AppendUint64(b, uint64(int64(l.value(o, d))))
		case types.SIMCONNECT_DATATYPE_STRING8, types.SIMCONNECT_DATATYPE_STRING32, types.SIMCONNECT_DATATYPE_STRING64,
			types.SIMCONNECT_DATATYPE_STRING128, types.SIMCONNECT_DATATYPE_STRING256, types.SIMCONNECT_DATATYPE_STRING260:
			s := make([]byte, stringSize[d.typ])
			copy(s[:len(s)-1], l.text(o, d))
			b = append(b, s...)
		case types.SIMCONNECT_DATATYPE_STRINGV:
			s := append([]byte(l.text(o, d)), 0)
			for len(s)%4 != 0 {
				s = append(s, 0)
			}
			b = append(b, s...)
		case types.SIMCONNECT_DATATYPE_INITPOSITION:
			p := types.SIMCONNECT_DATA_INITPOSITION{Latitude: o.Lat, Longitude: o.Lon, Altitude: o.AltFt, Pitch: o.Pitch, Bank: o.Bank, Heading: o.Heading}
			if o.OnGround {
				p.OnGround = 1
			}
			b = append(b, unsafe.Slice((*byte)(unsafe.Pointer(&p)), unsafe.Sizeof(p))...)
		case types.SIMCONNECT_DATATYPE_LATLONALT, types.SIMCONNECT_DATATYPE_XYZ:
			for _, v := range []float64{o.Lat, o.Lon, o.AltFt} {
				b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
			}
		default: // waypoints, marker state: none
		}
	}
	return b
}

var stringSize = map[types.SIMCONNECT_DATATYPE]int{
	types.SIMCONNECT_DATATYPE_STRING8: 8, types.SIMCONNECT_DATATYPE_STRING32: 32, types.SIMCONNECT_DATATYPE_STRING64: 64,
	types.SIMCONNECT_DATATYPE_STRING128: 128, types.SIMCONNECT_DATATYPE_STRING256: 256, types.SIMCONNECT_DATATYPE_STRING260: 260,
}

// text is a string SimVar of o.
func (l *Loopback) text(o *Object, d datum) string {
	switch base(d.name) {
	case "TITLE":
		return o.Title
	case "ATC ID":
		return o.ATCID
	case "AI TRAFFIC STATE":
		if o.ID == UserObjectID {
			return ""
		}
		return "STATE_SIMPLE_FLIGHT"
	}
	return ""
}

// value is a numeric SimVar of o in d's unit (degrees, feet, knots, ...;
// one not known is 0).
func (l *Loopback) value(o *Object, d datum) float64 {
	b2f := func(v bool) float64 {
		if v {
			return 1
		}
		return 0
	}
	now := l.clock().UTC()
	switch base(d.name) {
	case "PLANE LATITUDE":
		return fromDeg(o.Lat, d.unit)
	case "PLANE LONGITUDE":
		return fromDeg(o.Lon, d.unit)
	case "PLANE ALTITUDE", "INDICATED ALTITUDE":
		return feet(o.AltFt, d.unit)
	case "PLANE ALT ABOVE GROUND", "PLANE ALT ABOVE GROUND MINUS CG":
		return feet(math.Max(0, o.AltFt-l.groundFt), d.unit)
	case "GROUND ALTITUDE":
		return feet(l.groundFt, d.unit)
	case "PLANE HEADING DEGREES TRUE", "PLANE HEADING DEGREES MAGNETIC", "GPS GROUND TRUE TRACK":
		return fromDeg(o.Heading, d.unit)
	case "PLANE PITCH DEGREES":
		return fromDeg(o.Pitch, d.unit)
	case "PLANE BANK DEGREES":
		return fromDeg(o.Bank, d.unit)
	case "GROUND VELOCITY", "AIRSPEED TRUE", "AIRSPEED INDICATED":
		return o.GroundKts
	case "VERTICAL SPEED":
		return o.VSFpm
	case "LIGHT NAV", "LIGHT BEACON", "LIGHT STROBE", "LIGHT TAXI", "LIGHT LANDING", "LIGHT LOGO", "LIGHT WING":
		return b2f(o.Lights[d.name])
	case "SIM ON GROUND":
		return b2f(o.OnGround)
	case "GENERAL ENG COMBUSTION":
		return b2f(o.EnginesRunning)
	case "GEAR TOTAL PCT EXTENDED", "GEAR HANDLE POSITION":
		if o.GearUp {
			return 0
		}
		return 1
	case "SIMULATION RATE":
		return 1
	case "COM ACTIVE FREQUENCY":
		return o.COM1
	case "CAMERA STATE":
		return 2
	case "ZULU TIME", "LOCAL TIME":
		return float64(now.Hour()*3600+now.Minute()*60+now.Second()) + float64(now.Nanosecond())/1e9
	case "ZULU DAY OF MONTH":
		return float64(now.Day())
	case "ZULU MONTH OF YEAR":
		return float64(now.Month())
	case "ZULU YEAR":
		return float64(now.Year())
	case "TIME OF DAY":
		return 1 // day
	case "WING SPAN":
		return 112
	case "STATIC CG TO GROUND":
		return 8.5
	}
	return 0
}

// base is a SimVar name without its index ("COM ACTIVE FREQUENCY:1").
func base(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[:i]
	}
	return name
}

func fromDeg(v float64, unit string) float64 {
	if strings.HasPrefix(unit, "radian") {
		return v * math.Pi / 180
	}
	return v
}

func deg(v float64, unit string) float64 {
	if strings.HasPrefix(unit, "radian") {
		return v * 180 / math.Pi
	}
	return v
}

func feet(v float64, unit string) float64 {
	if strings.HasPrefix(unit, "meter") {
		return v * 0.3048
	}
	return v
}

// DefaultAircraftTitles and DefaultGroundTitles are the models the loopback
// lists (EnumerateSimObjectsAndLiveries) unless SetTitles gives others:
// the FSLTL and stock traffic of the live tests, one or two per type.
var (
	DefaultAircraftTitles = []string{
		"FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", "FSLTL A320 Air France SL", "FSLTL_FAIB_A321_WZZ-Wizz Air",
		"FSLTL A20N WZZ Wizzair", "FSLTL_FAIB_B738_TVS-Smartwings_NC", "FSLTL_B738_RYR", "FSLTL_B738_KLM",
		"FSLTL_FAIB_B739_THY-Turkish Airlines", "FSLTL_SBAI_BCS3_CSA-Lines", "FSLTL_E190_KLM", "FSLTL_E190_LOT",
		"Asobo PassiveAircraft Citation XLS :: CITATION_XLS_OFFICIAL", "Asobo PassiveAircraft Praetor 500 :: PRAETOR_500_0",
		"Asobo PassiveAircraft PC24", "DA62 Passengers :: Private 02",
	}
	DefaultGroundTitles = []string{
		"FSDT_Pushback_03", "FSDT_Pushback_Mototok_Spacer200", "FSDT_Pushback_Mototok_8600MA",
		"Truck Fire Airport Medium", "Truck Utility Europe Medic Box",
	}
)

// SetTitles replaces the aircraft and ground vehicle models listed (nil
// keeps that list).
func (l *Loopback) SetTitles(aircraft, ground []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if aircraft != nil {
		l.aircraftTitles = aircraft
	}
	if ground != nil {
		l.groundTitles = ground
	}
}

// EnumerateSimObjectsAndLiveries lists the models (SetTitles) as the
// simulator does, one entry a model, in one message.
func (l *Loopback) EnumerateSimObjectsAndLiveries(requestID uint32, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	titles := l.aircraftTitles
	if objectType == types.SIMCONNECT_SIMOBJECT_TYPE_GROUND {
		titles = l.groundTitles
	}
	if titles == nil {
		titles = DefaultAircraftTitles
		if objectType == types.SIMCONNECT_SIMOBJECT_TYPE_GROUND {
			titles = DefaultGroundTitles
		}
	}
	var hdr types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST
	off := int(unsafe.Offsetof(hdr.RgData))
	entry := int(unsafe.Sizeof(types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY{}))
	buf := make([]byte, off+len(titles)*entry)
	h := (*types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST)
	h.DwRequestID, h.DwArraySize, h.DwEntryNumber, h.DwOutOf = types.DWORD(requestID), types.DWORD(len(titles)), 0, 1
	for i, t := range titles {
		copy(buf[off+i*entry:off+i*entry+255], t)
	}
	l.emit(buf)
	return nil
}
