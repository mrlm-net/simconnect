//go:build windows
// +build windows

package traffic

import (
	"encoding/json"
	"io"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Movement is one controlled movement as a Recorder writes it: a JSON
// line per arrival or departure once it ends (#308).
type Movement struct {
	Time     time.Time `json:"time"`
	Kind     string    `json:"kind"` // "arrival" or "departure"
	ObjectID uint32    `json:"objectId"`
	Model    string    `json:"model"`
	Type     string    `json:"type"` // ICAO type designator, "" if unknown
	// Phase is the state it ended in ("parked", "complete", "failed", ...).
	Phase string `json:"phase"`
	Err   string `json:"error,omitempty"`
	// Arrivals: touchdown distance past the threshold and vertical speed,
	// the ground speed when leaving the runway, and how far from the
	// stand's stop mark the aircraft stopped.
	TouchdownM      float64 `json:"touchdownM,omitempty"`
	TouchdownFpm    float64 `json:"touchdownFpm,omitempty"`
	ExitKts         float64 `json:"exitKts,omitempty"`
	StandStopErrorM float64 `json:"standStopErrorM,omitempty"`
	// Departures: the take-off roll from its start to lift-off.
	LiftoffM float64 `json:"liftoffM,omitempty"`
	// Taxi speed while taxiing and moving: mean and maximum, knots.
	TaxiMeanKts float64 `json:"taxiMeanKts,omitempty"`
	TaxiMaxKts  float64 `json:"taxiMaxKts,omitempty"`
}

// MovementInfo describes the aircraft of a recorded movement.
type MovementInfo struct {
	Model string
	// Type is the ICAO type; "" resolves it from Model (ProfileFor).
	Type string
	// Stop is the stand's stop mark (ArrivalPlan.Stop) for the stop error
	// of an arrival; nil records none.
	Stop *airport.LatLon
}

// Recorder collects telemetry from controlled movements (#308): pass it
// every ArrivalEvent or TaxiEvent the application reads from a controller
// with Arrival or Departure; when the movement ends it writes one JSON line
// (Movement) to its writer and adds it to the Summary.
//
//	rec := traffic.NewRecorder(logFile)
//	for ev := range ctl.Events() {
//	    rec.Arrival(traffic.MovementInfo{Model: model, Stop: &ctl.Plan().Stop}, ev)
//	}
//
// A Recorder is safe for concurrent use.
type Recorder struct {
	mu    sync.Mutex
	w     io.Writer
	open  map[recKey]*recording
	done  []Movement
	now   func() time.Time
	wrErr error
}

type recKey struct {
	kind   string
	object uint32
}

type recording struct {
	m         Movement
	rollStart *airport.LatLon
	exited    bool
	taxiSum   float64
	taxiN     int
}

// NewRecorder creates a recorder writing JSON lines to w (nil keeps the
// movements for Summary only).
func NewRecorder(w io.Writer) *Recorder {
	return &Recorder{w: w, open: map[recKey]*recording{}, now: time.Now}
}

func (r *Recorder) start(kind string, obj uint32, info MovementInfo) *recording {
	k := recKey{kind, obj}
	rec := r.open[k]
	if rec == nil {
		typ := info.Type
		if typ == "" {
			typ = ProfileFor(info.Model).Type
		}
		rec = &recording{m: Movement{Kind: kind, ObjectID: obj, Model: info.Model, Type: typ}}
		r.open[k] = rec
	}
	return rec
}

func (rec *recording) taxi(kts float64) {
	if kts <= StoppedKts {
		return
	}
	rec.taxiSum += kts
	rec.taxiN++
	rec.m.TaxiMaxKts = math.Max(rec.m.TaxiMaxKts, kts)
}

// Arrival records one event of an arrival.
func (r *Recorder) Arrival(info MovementInfo, ev ArrivalEvent) {
	if ev.ObjectID == 0 {
		return // not spawned yet
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.start("arrival", ev.ObjectID, info)
	if ev.Touchdown != 0 && rec.m.TouchdownM == 0 {
		rec.m.TouchdownM, rec.m.TouchdownFpm = round1(ev.Touchdown), round1(ev.TouchdownFpm)
	}
	switch ev.State {
	case ArrivalVacating:
		if !rec.exited {
			rec.exited = true
			rec.m.ExitKts = round1(ev.GroundSpeed)
		}
	case ArrivalTaxiing, ArrivalParking:
		rec.taxi(ev.GroundSpeed)
	}
	if ev.State.Terminal() {
		if ev.State == ArrivalParked && info.Stop != nil {
			rec.m.StandStopErrorM = round1(localDist(*info.Stop, ev.Position))
		}
		r.finish(recKey{"arrival", ev.ObjectID}, rec, ev.State.String(), ev.Err)
	}
}

// Departure records one event of a departure.
func (r *Recorder) Departure(info MovementInfo, ev TaxiEvent) {
	if ev.ObjectID == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.start("departure", ev.ObjectID, info)
	switch ev.State {
	case TaxiTaxiing:
		rec.taxi(ev.GroundSpeed)
	case TaxiDeparting:
		if rec.rollStart == nil {
			p := ev.Position
			rec.rollStart = &p
		}
		if rec.m.LiftoffM == 0 && !ev.OnGround && ev.HeightFt > 0 {
			rec.m.LiftoffM = math.Round(localDist(*rec.rollStart, ev.Position))
		}
	}
	if ev.State.Terminal() {
		r.finish(recKey{"departure", ev.ObjectID}, rec, ev.State.String(), ev.Err)
	}
}

func (r *Recorder) finish(k recKey, rec *recording, phase string, err error) {
	delete(r.open, k)
	m := rec.m
	m.Time, m.Phase = r.now().UTC(), phase
	if err != nil {
		m.Err = err.Error()
	}
	if rec.taxiN > 0 {
		m.TaxiMeanKts = round1(rec.taxiSum / float64(rec.taxiN))
	}
	m.TaxiMaxKts = round1(m.TaxiMaxKts)
	r.done = append(r.done, m)
	if r.w != nil {
		b, _ := json.Marshal(m)
		if _, werr := r.w.Write(append(b, '\n')); werr != nil && r.wrErr == nil {
			r.wrErr = werr
		}
	}
}

// Err returns the first write error, if any.
func (r *Recorder) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.wrErr
}

// Movements returns the finished movements so far.
func (r *Recorder) Movements() []Movement {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Movement(nil), r.done...)
}

// TypeSummary aggregates the movements of one type: counts and medians
// (0 when none was recorded), to compare with and tune the profile.
type TypeSummary struct {
	Type       string `json:"type"`
	Arrivals   int    `json:"arrivals"`
	Departures int    `json:"departures"`
	Failed     int    `json:"failed"`

	TouchdownM      float64 `json:"touchdownM"`
	TouchdownFpm    float64 `json:"touchdownFpm"`
	ExitKts         float64 `json:"exitKts"`
	StandStopErrorM float64 `json:"standStopErrorM"`
	LiftoffM        float64 `json:"liftoffM"`
	TaxiMeanKts     float64 `json:"taxiMeanKts"`
}

// Summary aggregates the finished movements per type (by Type, then the
// model for unknown types), sorted by type.
func (r *Recorder) Summary() []TypeSummary {
	return Summarize(r.Movements())
}

// Summarize aggregates movements per type, e.g. read back from a
// Recorder's JSON lines.
func Summarize(ms []Movement) []TypeSummary {
	type acc struct {
		s                                  TypeSummary
		td, fpm, exit, stop, liftoff, taxi []float64
	}
	by := map[string]*acc{}
	for _, m := range ms {
		key := m.Type
		if key == "" {
			key = m.Model
		}
		a := by[key]
		if a == nil {
			a = &acc{s: TypeSummary{Type: key}}
			by[key] = a
		}
		if m.Kind == "arrival" {
			a.s.Arrivals++
		} else {
			a.s.Departures++
		}
		if m.Phase == "failed" {
			a.s.Failed++
		}
		add := func(xs *[]float64, x float64) {
			if x != 0 {
				*xs = append(*xs, x)
			}
		}
		add(&a.td, m.TouchdownM)
		add(&a.fpm, m.TouchdownFpm)
		add(&a.exit, m.ExitKts)
		add(&a.stop, m.StandStopErrorM)
		add(&a.liftoff, m.LiftoffM)
		add(&a.taxi, m.TaxiMeanKts)
	}
	out := make([]TypeSummary, 0, len(by))
	for _, a := range by {
		s := a.s
		s.TouchdownM, s.TouchdownFpm, s.ExitKts = median(a.td), median(a.fpm), median(a.exit)
		s.StandStopErrorM, s.LiftoffM, s.TaxiMeanKts = median(a.stop), median(a.liftoff), median(a.taxi)
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return round1((s[n/2-1] + s[n/2]) / 2)
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
