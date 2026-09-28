//go:build windows
// +build windows

package traffic

import (
	"bytes"
	"fmt"
	"math"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// EngineType is the ENGINE TYPE SimVar.
type EngineType int

const (
	EnginePiston      EngineType = 0
	EngineJet         EngineType = 1
	EngineNone        EngineType = 2
	EngineHeloTurbine EngineType = 3
	EngineUnsupported EngineType = 4
	EngineTurboprop   EngineType = 5
	EngineElectric    EngineType = 6
)

// SimVarData is what the simulator reports about an aircraft's type
// (#325), read by ProfileReader. Zero means not reported.
type SimVarData struct {
	ObjectID uint32
	// WingspanM is WING SPAN; CGHeightM is STATIC CG TO GROUND.
	WingspanM, CGHeightM float64
	// Design speeds, knots: DESIGN SPEED VS0 (stall, landing
	// configuration), VS1 (stall, clean), DESIGN TAKEOFF SPEED, DESIGN
	// SPEED CLIMB and DESIGN SPEED VC (cruise).
	VS0Kts, VS1Kts, TakeoffKts, ClimbKts, CruiseKts float64
	Engines                                         int
	EngineType                                      EngineType
	// MaxGrossKg and TotalWeightKg are MAX GROSS WEIGHT and TOTAL WEIGHT.
	MaxGrossKg, TotalWeightKg float64
	// FlapPositions is FLAPS NUM HANDLE POSITIONS.
	FlapPositions int
	// ATCModel, ATCType and Category are ATC MODEL, ATC TYPE and the
	// object CATEGORY ("Airplane").
	ATCModel, ATCType, Category string
}

// profileVars are the SimVars of the reader's data definition; the order,
// units and types must match profileWire. Names and units as documented in
// the MSFS 2024 SDK (Aircraft Flight Model, Engine, Control and Radio
// Navigation SimVars; CATEGORY in Miscellaneous). None is written.
var profileVars = [...]struct {
	name, unit string
	typ        types.SIMCONNECT_DATATYPE
}{
	{"WING SPAN", "feet", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"DESIGN SPEED VS0", "knots", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"DESIGN SPEED VS1", "knots", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"DESIGN TAKEOFF SPEED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"DESIGN SPEED CLIMB", "knots", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"DESIGN SPEED VC", "knots", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"NUMBER OF ENGINES", "number", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"ENGINE TYPE", "enum", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"MAX GROSS WEIGHT", "pounds", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"TOTAL WEIGHT", "pounds", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"FLAPS NUM HANDLE POSITIONS", "number", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"STATIC CG TO GROUND", "feet", types.SIMCONNECT_DATATYPE_FLOAT64},
	{"ATC MODEL", "", types.SIMCONNECT_DATATYPE_STRING128},
	{"ATC TYPE", "", types.SIMCONNECT_DATATYPE_STRING128},
	{"CATEGORY", "", types.SIMCONNECT_DATATYPE_STRING128},
}

type profileWire struct {
	SpanFt, VS0, VS1, TakeoffKts, ClimbKts, CruiseKts float64
	Engines, EngineType, MaxGrossLb, TotalLb          float64
	FlapPositions, CGFt                               float64
	ATCModel, ATCType, Category                       [128]byte
}

const lbToKg = 0.45359237

func decodeProfileWire(obj uint32, w *profileWire) SimVarData {
	str := func(b []byte) string {
		if i := bytes.IndexByte(b, 0); i >= 0 {
			b = b[:i]
		}
		return string(b)
	}
	return SimVarData{
		ObjectID:  obj,
		WingspanM: w.SpanFt * 0.3048, CGHeightM: w.CGFt * 0.3048,
		VS0Kts: w.VS0, VS1Kts: w.VS1, TakeoffKts: w.TakeoffKts, ClimbKts: w.ClimbKts, CruiseKts: w.CruiseKts,
		Engines: int(math.Round(w.Engines)), EngineType: EngineType(math.Round(w.EngineType)),
		MaxGrossKg: w.MaxGrossLb * lbToKg, TotalWeightKg: w.TotalLb * lbToKg,
		FlapPositions: int(math.Round(w.FlapPositions)),
		ATCModel:      str(w.ATCModel[:]), ATCType: str(w.ATCType[:]), Category: str(w.Category[:]),
	}
}

// ProfileDataClient is the part of engine.Client (and manager.Manager) a
// ProfileReader uses.
type ProfileDataClient interface {
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
}

// ProfileReader reads the type SimVars (SimVarData) of spawned aircraft
// through the application's message loop, like nav.WeatherReader: call
// Request for an object once it exists, pass every message to Handle and
// Refine the profile with what comes back.
//
//	pr := traffic.NewProfileReader(client, 8400, 8401)
//	pr.Request(objectID)
//	for msg := range client.Stream() {
//	    if v, ok := pr.Handle(msg); ok {
//	        prof := traffic.Refine(traffic.ProfileFor(v.ATCModel), v)
//	    }
//	}
//
// All requests share one request ID; answers carry the object ID. A
// ProfileReader is safe for concurrent use; after a reconnect call Reset.
type ProfileReader struct {
	mu         sync.Mutex
	client     ProfileDataClient
	defID      uint32
	reqID      uint32
	registered bool
	last       map[uint32]SimVarData
}

// NewProfileReader creates a reader that uses one data definition ID and
// one request ID; keep both clear of the application's own IDs.
func NewProfileReader(client ProfileDataClient, defID, reqID uint32) *ProfileReader {
	return &ProfileReader{client: client, defID: defID, reqID: reqID, last: map[uint32]SimVarData{}}
}

// Reset forgets the registered definition and the answers, e.g. after the
// simulator reconnects. A non-nil client replaces the current one.
func (r *ProfileReader) Reset(client ProfileDataClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if client != nil {
		r.client = client
	}
	r.registered = false
	r.last = map[uint32]SimVarData{}
}

// Request asks once for the type SimVars of an object (0 is the user
// aircraft; its answer carries the user's own object ID).
func (r *ProfileReader) Request(objectID uint32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client == nil {
		return ErrNotConnected
	}
	if !r.registered {
		for i, v := range profileVars {
			if err := r.client.AddToDataDefinition(r.defID, v.name, v.unit, v.typ, 0, uint32(i)); err != nil {
				return fmt.Errorf("traffic: define %s: %w", v.name, err)
			}
		}
		r.registered = true
	}
	if err := r.client.RequestDataOnSimObject(r.reqID, r.defID, objectID, types.SIMCONNECT_PERIOD_ONCE, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0); err != nil {
		return fmt.Errorf("traffic: request aircraft data: %w", err)
	}
	return nil
}

// Handle processes one message. It returns ok=true with the data when msg
// is an answer to the reader's request; other messages are ignored.
func (r *ProfileReader) Handle(msg engine.Message) (SimVarData, bool) {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return SimVarData{}, false
	}
	d := msg.AsSimObjectData()
	if uint32(d.DwRequestID) != r.reqID {
		return SimVarData{}, false
	}
	v := decodeProfileWire(uint32(d.DwObjectID), engine.CastDataAs[profileWire](&d.DwData))
	r.mu.Lock()
	r.last[v.ObjectID] = v
	r.mu.Unlock()
	return v, true
}

// Data returns the last data Handle decoded for an object.
func (r *ProfileReader) Data(objectID uint32) (SimVarData, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.last[objectID]
	return v, ok
}

// Reference weights of the table's speeds, as fractions of the maximum
// gross weight: approach speeds at a typical landing weight, take-off
// speeds at a typical take-off weight.
const (
	refLandingWeight = 0.78
	refTakeoffWeight = 0.90
)

// Refine adjusts a profile with what the simulator reports for the
// aircraft (#325):
//
//   - A generic profile (Type "") takes the reported span and engine type
//     (GenericProfile) and its speeds from the design speeds: the final
//     approach at 1.3 VS0 + 5, rotation at DESIGN TAKEOFF SPEED, the
//     initial climb at DESIGN SPEED CLIMB. A known type keeps its
//     published airframe and speeds: the sim's design figures vary by
//     model and are often placeholders on AI models.
//   - Both scale the speeds by weight (√(TOTAL WEIGHT / reference
//     weight), within -12 % and +8 %) and the take-off acceleration
//     inversely with the weight, unless the weight exceeds MAX GROSS
//     WEIGHT (FSLTL models report such weights).
//   - A generic profile's flap settings snap to the reported handle
//     detents; a reported STATIC CG TO GROUND replaces the table's.
//
// Measured live (MSFS 2024, FSLTL A320 AI): span 31.7 m, VS1 165 kt,
// 4 flap positions and 87 t of a 68 t maximum — hence known types keep
// their table figures.
//
// Values not reported (zero) leave the profile as it is.
func Refine(p AircraftProfile, v SimVarData) AircraftProfile {
	if p.Type == "" && v.WingspanM > 0 {
		cat := p.Category
		switch v.EngineType {
		case EngineTurboprop:
			cat = CategoryTurboprop
		case EnginePiston:
			cat = CategoryPiston
		case EngineJet:
			cat = CategoryJet
		}
		p = GenericProfile(v.WingspanM, cat)
	}
	if p.Type == "" {
		if v.VS0Kts > 30 {
			vapp := math.Round(1.3*v.VS0Kts) + 5
			p.Approach.ApproachKts, p.Approach.TouchdownKts, p.Approach.StartKts = vapp, vapp-5, vapp+15
		}
		if v.TakeoffKts > 30 {
			p.Takeoff.RotateKts = v.TakeoffKts
		}
		if v.ClimbKts > p.Takeoff.RotateKts {
			p.Takeoff.ClimbKts = v.ClimbKts
		}
	}
	if v.CGHeightM > 0.5 {
		p.CGHeightM = v.CGHeightM
	}
	// FSLTL AI models report a TOTAL WEIGHT above their MAX GROSS WEIGHT
	// (87 t of 68 t on the A320): inconsistent weights are ignored.
	if v.MaxGrossKg > 0 && v.TotalWeightKg > 0 && v.TotalWeightKg <= v.MaxGrossKg*1.02 {
		w := v.TotalWeightKg / v.MaxGrossKg
		land := clamp(math.Sqrt(w/refLandingWeight), 0.88, 1.08)
		to := clamp(math.Sqrt(w/refTakeoffWeight), 0.88, 1.08)
		a := &p.Approach
		a.StartKts, a.ApproachKts, a.TouchdownKts = math.Round(a.StartKts*land), math.Round(a.ApproachKts*land), math.Round(a.TouchdownKts*land)
		t := &p.Takeoff
		t.RotateKts, t.ClimbKts = math.Round(t.RotateKts*to), math.Round(t.ClimbKts*to)
		t.RollAccel /= to * to
	}
	// Known types keep their published flap schedule: FSLTL's A320 reports
	// 4 handle positions, the real one has 5.
	if n := v.FlapPositions; n >= 2 && p.Type == "" {
		f := &p.Flaps
		f.TakeoffPct, f.ApproachPct, f.LandingPct = snapDetent(f.TakeoffPct, n), snapDetent(f.ApproachPct, n), snapDetent(f.LandingPct, n)
	}
	return p
}

// snapDetent moves a handle percentage to the nearest of n handle
// positions (0 %, …, 100 %), keeping a set flap above the up position.
func snapDetent(pct float64, n int) float64 {
	step := 100 / float64(n-1)
	s := math.Round(pct/step) * step
	if pct > 0 && s == 0 {
		s = step
	}
	return math.Round(s*10) / 10
}

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
