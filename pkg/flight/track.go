// Package flight records how an aircraft flies — the user's or an AI
// object, every sim frame — as a Track, and reads, writes and interpolates
// Tracks. A Track holds what an aircraft looks and moves like: position,
// attitude, speeds, gear, flaps, spoilers, control surfaces, lights,
// engines and the autopilot, so a replay of it (the applicator) or a
// profile learned from it looks as the real flight did.
package flight

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// TrackVersion is the version of the Track format written.
const TrackVersion = 1

// Engines are the engines a sample holds.
const Engines = 4

// Sample is an aircraft at one moment. Angles are degrees (heading true),
// altitudes feet above sea level, speeds knots, vertical speed feet per
// minute, percentages 0–100.
type Sample struct {
	// T is the simulation time, seconds (SIMULATION TIME): it runs with the
	// sim's rate and stops while paused.
	T float64 `json:"t"`

	Lat, Lon float64
	AltFt    float64
	GroundFt float64 // the ground under it (GROUND ALTITUDE)
	// CGFt is the centre of gravity's height above the ground standing
	// (STATIC CG TO GROUND): where the wheels touch.
	CGFt                 float64
	Pitch, Bank, Heading float64 // pitch up positive, bank right positive
	IAS, GS, VS          float64
	OnGround             bool

	GearHandle bool    // down
	GearPct    float64 // the gear's extension (centre gear)
	FlapsIndex int     // the flap lever's detent
	FlapsPct   float64 // trailing edge flaps
	// FlapsHandle is the flap lever, percent of its travel.
	FlapsHandle   float64
	Spoilers      float64 // spoiler handle
	SpoilersArmed bool

	// Control surfaces, -100…100 (elevator up, aileron right, rudder right
	// positive): seen on a replayed aircraft.
	Elevator, Aileron, Rudder float64
	Brakes                    float64 // left and right, the more applied
	ParkingBrake              bool

	// Lights is LIGHT ON STATES: nav 0x1, beacon 0x2, landing 0x4, taxi
	// 0x8, strobe 0x10, panel 0x20, recognition 0x40, wing 0x80, logo
	// 0x100, cabin 0x200.
	Lights int

	EngineCount int
	Throttle    [Engines]float64 // lever position
	N1          [Engines]float64
	Reverser    [Engines]float64 // reverse nozzle deployed

	AP Autopilot
}

// Autopilot is the autopilot's state as the standard SimVars give it.
type Autopilot struct {
	Master, FD, Autothrottle bool // armed (AUTOPILOT THROTTLE ARM)
	// Holds engaged.
	Heading, Altitude, VS, Speed, Nav, Approach, GS bool
	// Selected values: heading (magnetic), altitude, vertical speed,
	// airspeed.
	HeadingSel, AltitudeSel, VSSel, SpeedSel float64
}

// Light bits of Sample.Lights.
const (
	LightNav = 1 << iota
	LightBeacon
	LightLanding
	LightTaxi
	LightStrobe
	LightPanel
	LightRecognition
	LightWing
	LightLogo
	LightCabin
)

// Track is a recorded flight of one aircraft.
type Track struct {
	Version int `json:"version"`
	// Title is the aircraft's sim title, Model its ICAO type ("A320", ""
	// unknown); User whether it is the user aircraft.
	Title string `json:"title,omitempty"`
	Model string `json:"model,omitempty"`
	User  bool   `json:"user,omitempty"`
	// Started is the wall clock at the first sample.
	Started time.Time `json:"started"`
	// Note is the recorder's free text (the flight, the airport).
	Note    string   `json:"note,omitempty"`
	Samples []Sample `json:"-"`
}

// Duration is the simulation time from the first sample to the last.
func (t *Track) Duration() float64 {
	if len(t.Samples) < 2 {
		return 0
	}
	return t.Samples[len(t.Samples)-1].T - t.Samples[0].T
}

// At is the aircraft at simulation time at, interpolated between the
// samples around it: numbers linearly, headings and longitudes the short
// way round, what is on or off (gear handle, lights, flap detent, the
// autopilot's modes) as the earlier sample has it. Before the first sample
// it is the first, after the last the last; false without samples.
func (t *Track) At(at float64) (Sample, bool) {
	n := len(t.Samples)
	if n == 0 {
		return Sample{}, false
	}
	if at <= t.Samples[0].T {
		return t.Samples[0], true
	}
	if at >= t.Samples[n-1].T {
		return t.Samples[n-1], true
	}
	i := sort.Search(n, func(i int) bool { return t.Samples[i].T > at }) // first after
	a, b := t.Samples[i-1], t.Samples[i]
	f := 0.0
	if b.T > a.T {
		f = (at - a.T) / (b.T - a.T)
	}
	return Lerp(a, b, f), true
}

// Lerp is the sample a fraction f of the way from a to b (see At).
func Lerp(a, b Sample, f float64) Sample {
	l := func(x, y float64) float64 { return x + (y-x)*f }
	s := a
	s.T = l(a.T, b.T)
	s.Lat = l(a.Lat, b.Lat)
	s.Lon = lerpAngle(a.Lon, b.Lon, f)
	s.AltFt, s.GroundFt, s.CGFt = l(a.AltFt, b.AltFt), l(a.GroundFt, b.GroundFt), l(a.CGFt, b.CGFt)
	s.Pitch, s.Bank = l(a.Pitch, b.Pitch), lerpAngle(a.Bank, b.Bank, f)
	s.Heading = math.Mod(lerpAngle(a.Heading, b.Heading, f)+360, 360)
	s.IAS, s.GS, s.VS = l(a.IAS, b.IAS), l(a.GS, b.GS), l(a.VS, b.VS)
	s.GearPct, s.FlapsPct, s.FlapsHandle, s.Spoilers = l(a.GearPct, b.GearPct), l(a.FlapsPct, b.FlapsPct), l(a.FlapsHandle, b.FlapsHandle), l(a.Spoilers, b.Spoilers)
	s.Elevator, s.Aileron, s.Rudder, s.Brakes = l(a.Elevator, b.Elevator), l(a.Aileron, b.Aileron), l(a.Rudder, b.Rudder), l(a.Brakes, b.Brakes)
	for i := range s.Throttle {
		s.Throttle[i], s.N1[i], s.Reverser[i] = l(a.Throttle[i], b.Throttle[i]), l(a.N1[i], b.N1[i]), l(a.Reverser[i], b.Reverser[i])
	}
	s.AP.HeadingSel = math.Mod(lerpAngle(a.AP.HeadingSel, b.AP.HeadingSel, f)+360, 360)
	s.AP.AltitudeSel, s.AP.VSSel, s.AP.SpeedSel = l(a.AP.AltitudeSel, b.AP.AltitudeSel), l(a.AP.VSSel, b.AP.VSSel), l(a.AP.SpeedSel, b.AP.SpeedSel)
	return s
}

// lerpAngle goes from a to b the short way round (degrees).
func lerpAngle(a, b, f float64) float64 {
	d := math.Mod(b-a+540, 360) - 180
	return a + d*f
}

// ── The file ──────────────────────────────────────────────────────────────
//
// JSON lines: the first is the Track's header with the sample fields'
// names in order ("fields"), each next one a sample as an array of numbers
// in that order (bools 0 or 1). A reader takes the fields by name: one it
// does not know is skipped, one missing stays zero, so fields can be added
// without breaking older files or readers. A name ending in ".gz" is
// gzipped.

// field is one number of a sample in the file.
type field struct {
	name string
	get  func(*Sample) float64
	set  func(*Sample, float64)
}

func num(name string, p func(*Sample) *float64) field {
	return field{name, func(s *Sample) float64 { return *p(s) }, func(s *Sample, v float64) { *p(s) = v }}
}

func flag(name string, p func(*Sample) *bool) field {
	return field{name, func(s *Sample) float64 { return b2f(*p(s)) }, func(s *Sample, v float64) { *p(s) = v != 0 }}
}

func integer(name string, p func(*Sample) *int) field {
	return field{name, func(s *Sample) float64 { return float64(*p(s)) }, func(s *Sample, v float64) { *p(s) = int(v) }}
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

var fields = func() []field {
	fs := []field{
		num("t", func(s *Sample) *float64 { return &s.T }),
		num("lat", func(s *Sample) *float64 { return &s.Lat }),
		num("lon", func(s *Sample) *float64 { return &s.Lon }),
		num("alt", func(s *Sample) *float64 { return &s.AltFt }),
		num("ground", func(s *Sample) *float64 { return &s.GroundFt }),
		num("cg", func(s *Sample) *float64 { return &s.CGFt }),
		num("pitch", func(s *Sample) *float64 { return &s.Pitch }),
		num("bank", func(s *Sample) *float64 { return &s.Bank }),
		num("hdg", func(s *Sample) *float64 { return &s.Heading }),
		num("ias", func(s *Sample) *float64 { return &s.IAS }),
		num("gs", func(s *Sample) *float64 { return &s.GS }),
		num("vs", func(s *Sample) *float64 { return &s.VS }),
		flag("onGround", func(s *Sample) *bool { return &s.OnGround }),
		flag("gearHandle", func(s *Sample) *bool { return &s.GearHandle }),
		num("gearPct", func(s *Sample) *float64 { return &s.GearPct }),
		integer("flapsIndex", func(s *Sample) *int { return &s.FlapsIndex }),
		num("flapsPct", func(s *Sample) *float64 { return &s.FlapsPct }),
		num("flapsHandle", func(s *Sample) *float64 { return &s.FlapsHandle }),
		num("spoilers", func(s *Sample) *float64 { return &s.Spoilers }),
		flag("spoilersArmed", func(s *Sample) *bool { return &s.SpoilersArmed }),
		num("elevator", func(s *Sample) *float64 { return &s.Elevator }),
		num("aileron", func(s *Sample) *float64 { return &s.Aileron }),
		num("rudder", func(s *Sample) *float64 { return &s.Rudder }),
		num("brakes", func(s *Sample) *float64 { return &s.Brakes }),
		flag("parkingBrake", func(s *Sample) *bool { return &s.ParkingBrake }),
		integer("lights", func(s *Sample) *int { return &s.Lights }),
		integer("engines", func(s *Sample) *int { return &s.EngineCount }),
	}
	for i := range Engines {
		n := fmt.Sprint(i + 1)
		fs = append(fs,
			num("throttle"+n, func(s *Sample) *float64 { return &s.Throttle[i] }),
			num("n1_"+n, func(s *Sample) *float64 { return &s.N1[i] }),
			num("reverser"+n, func(s *Sample) *float64 { return &s.Reverser[i] }))
	}
	return append(fs,
		flag("ap", func(s *Sample) *bool { return &s.AP.Master }),
		flag("fd", func(s *Sample) *bool { return &s.AP.FD }),
		flag("athr", func(s *Sample) *bool { return &s.AP.Autothrottle }),
		flag("apHdg", func(s *Sample) *bool { return &s.AP.Heading }),
		flag("apAlt", func(s *Sample) *bool { return &s.AP.Altitude }),
		flag("apVS", func(s *Sample) *bool { return &s.AP.VS }),
		flag("apSpd", func(s *Sample) *bool { return &s.AP.Speed }),
		flag("apNav", func(s *Sample) *bool { return &s.AP.Nav }),
		flag("apApp", func(s *Sample) *bool { return &s.AP.Approach }),
		flag("apGS", func(s *Sample) *bool { return &s.AP.GS }),
		num("apHdgSel", func(s *Sample) *float64 { return &s.AP.HeadingSel }),
		num("apAltSel", func(s *Sample) *float64 { return &s.AP.AltitudeSel }),
		num("apVSSel", func(s *Sample) *float64 { return &s.AP.VSSel }),
		num("apSpdSel", func(s *Sample) *float64 { return &s.AP.SpeedSel }),
	)
}()

type header struct {
	Track
	Fields []string `json:"fields"`
}

// Write writes t to w as JSON lines.
func (t *Track) Write(w io.Writer) error {
	bw := bufio.NewWriter(w)
	h := header{Track: *t, Fields: make([]string, len(fields))}
	h.Version = TrackVersion
	for i, f := range fields {
		h.Fields[i] = f.name
	}
	enc := json.NewEncoder(bw)
	if err := enc.Encode(h); err != nil {
		return err
	}
	row := make([]float64, len(fields))
	for i := range t.Samples {
		for j, f := range fields {
			row[j] = f.get(&t.Samples[i])
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// SampleFields are the names of a sample's numbers in Row order, as the
// file's header lists them.
func SampleFields() []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.name
	}
	return out
}

// Row is s as numbers in SampleFields order: a compact sample to send
// (a puppet's stream, #964).
func (s Sample) Row() []float64 {
	row := make([]float64, len(fields))
	for i, f := range fields {
		row[i] = f.get(&s)
	}
	return row
}

// RowDecoder decodes rows in the order of names (another build's
// SampleFields): a name it does not know is skipped, one missing stays
// zero.
func RowDecoder(names []string) func(row []float64) Sample {
	byName := map[string]field{}
	for _, f := range fields {
		byName[f.name] = f
	}
	set := make([]func(*Sample, float64), len(names))
	for i, n := range names {
		if f, ok := byName[n]; ok {
			set[i] = f.set
		}
	}
	return func(row []float64) Sample {
		var s Sample
		for i, v := range row {
			if i < len(set) && set[i] != nil {
				set[i](&s, v)
			}
		}
		return s
	}
}

// ErrTrackVersion: a Track written by a newer format than this reader's.
var ErrTrackVersion = errors.New("flight: track from a newer version")

// ReadTrack reads a Track written by Write.
func ReadTrack(r io.Reader) (*Track, error) {
	dec := json.NewDecoder(bufio.NewReader(r))
	var h header
	if err := dec.Decode(&h); err != nil {
		return nil, fmt.Errorf("flight: track header: %w", err)
	}
	if h.Version > TrackVersion {
		return nil, fmt.Errorf("%w: %d", ErrTrackVersion, h.Version)
	}
	byName := map[string]field{}
	for _, f := range fields {
		byName[f.name] = f
	}
	set := make([]func(*Sample, float64), len(h.Fields))
	for i, n := range h.Fields {
		if f, ok := byName[n]; ok {
			set[i] = f.set
		}
	}
	t := h.Track
	var row []float64
	for {
		row = row[:0]
		if err := dec.Decode(&row); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("flight: track sample %d: %w", len(t.Samples)+1, err)
		}
		var s Sample
		for i, v := range row {
			if i < len(set) && set[i] != nil {
				set[i](&s, v)
			}
		}
		t.Samples = append(t.Samples, s)
	}
	return &t, nil
}

// WriteFile writes t to path (gzipped when it ends in ".gz").
func (t *Track) WriteFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	var w io.Writer = f
	var gz *gzip.Writer
	if strings.HasSuffix(path, ".gz") {
		gz = gzip.NewWriter(f)
		w = gz
	}
	err = t.Write(w)
	if gz != nil {
		if cerr := gz.Close(); err == nil {
			err = cerr
		}
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// ReadTrackFile reads a Track from path (gzipped when it ends in ".gz").
func ReadTrackFile(path string) (*Track, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return ReadTrack(r)
}
