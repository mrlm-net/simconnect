package flight

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// A Scene is the aircraft around a flight, recorded with it (#1011): each
// with its identity and its own Track, coming and going as it does, on the
// same clock as the player's Track (SIMULATION TIME), so a replay shows
// the scene as it was.

// SceneVersion is the scene file format.
const SceneVersion = 1

// Scene aircraft sources.
const (
	SourceOurs   = "ours"   // the add-on's own traffic
	SourceSimAI  = "sim-ai" // the simulator's AI traffic
	SourcePlayer = "player" // another player (multiplayer)
)

// SceneAircraft is one aircraft of a scene: Key is stable within the
// scene (another stretch of the same aircraft has another key).
type SceneAircraft struct {
	Key      string `json:"key"`
	Callsign string `json:"callsign,omitempty"`
	Title    string `json:"title,omitempty"`  // the sim title (model)
	Livery   string `json:"livery,omitempty"` // the livery title
	Type     string `json:"type,omitempty"`   // ICAO type
	Source   string `json:"source,omitempty"` // SourceOurs, SourceSimAI, SourcePlayer
	Track    *Track `json:"-"`
}

// First and Last are its first and last sample's simulation time (0
// without samples).
func (a *SceneAircraft) First() float64 {
	if a.Track == nil || len(a.Track.Samples) == 0 {
		return 0
	}
	return a.Track.Samples[0].T
}

func (a *SceneAircraft) Last() float64 {
	if a.Track == nil || len(a.Track.Samples) == 0 {
		return 0
	}
	return a.Track.Samples[len(a.Track.Samples)-1].T
}

// Scene is the aircraft recorded around a flight.
type Scene struct {
	Version  int              `json:"version"`
	Started  time.Time        `json:"started"`
	Note     string           `json:"note,omitempty"`
	Aircraft []*SceneAircraft `json:"-"`
}

// SceneSample is an aircraft of a scene at a time.
type SceneSample struct {
	Aircraft *SceneAircraft
	Sample   Sample
}

// At is every aircraft there at simulation time t (between its first and
// last sample), interpolated (Track.At).
func (s *Scene) At(t float64) []SceneSample {
	var out []SceneSample
	for _, a := range s.Aircraft {
		if a.Track == nil || len(a.Track.Samples) == 0 || t < a.First() || t > a.Last() {
			continue
		}
		if smp, ok := a.Track.At(t); ok {
			out = append(out, SceneSample{Aircraft: a, Sample: smp})
		}
	}
	return out
}

// Find is the aircraft with key, nil none.
func (s *Scene) Find(key string) *SceneAircraft {
	for _, a := range s.Aircraft {
		if a.Key == key {
			return a
		}
	}
	return nil
}

// Span is the scene's first and last simulation time.
func (s *Scene) Span() (first, last float64) {
	have := false
	for _, a := range s.Aircraft {
		if a.Track == nil || len(a.Track.Samples) == 0 {
			continue
		}
		if !have || a.First() < first {
			first = a.First()
		}
		if !have || a.Last() > last {
			last = a.Last()
		}
		have = true
	}
	return first, last
}

// The file: a header line ({"version", "started", "note", "fields"}), then
// per aircraft a line {"aircraft": {...}} and its samples as lines
// {"k": key, "r": [numbers in "fields" order]}.
type sceneHeader struct {
	Version int       `json:"version"`
	Started time.Time `json:"started"`
	Note    string    `json:"note,omitempty"`
	Fields  []string  `json:"fields"`
}

type sceneLine struct {
	Aircraft *SceneAircraft `json:"aircraft,omitempty"`
	K        string         `json:"k,omitempty"`
	R        []float64      `json:"r,omitempty"`
}

// Write writes s to w as JSON lines.
func (s *Scene) Write(w io.Writer) error {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	if err := enc.Encode(sceneHeader{Version: SceneVersion, Started: s.Started, Note: s.Note, Fields: SampleFields()}); err != nil {
		return err
	}
	for _, a := range s.Aircraft {
		if err := enc.Encode(sceneLine{Aircraft: a}); err != nil {
			return err
		}
		if a.Track == nil {
			continue
		}
		for _, smp := range a.Track.Samples {
			if err := enc.Encode(sceneLine{K: a.Key, R: smp.Row()}); err != nil {
				return err
			}
		}
	}
	return bw.Flush()
}

// ErrSceneVersion: a scene written by a newer format than this reader's.
var ErrSceneVersion = errors.New("flight: scene from a newer version")

// ReadScene reads a Scene written by Write.
func ReadScene(r io.Reader) (*Scene, error) {
	dec := json.NewDecoder(bufio.NewReader(r))
	var h sceneHeader
	if err := dec.Decode(&h); err != nil {
		return nil, fmt.Errorf("flight: scene header: %w", err)
	}
	if h.Version > SceneVersion {
		return nil, fmt.Errorf("%w: %d", ErrSceneVersion, h.Version)
	}
	decode := RowDecoder(h.Fields)
	s := &Scene{Version: h.Version, Started: h.Started, Note: h.Note}
	byKey := map[string]*SceneAircraft{}
	for n := 1; ; n++ {
		var l sceneLine
		if err := dec.Decode(&l); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("flight: scene line %d: %w", n+1, err)
		}
		switch {
		case l.Aircraft != nil:
			a := l.Aircraft
			a.Track = &Track{Version: TrackVersion, Title: a.Title, Model: a.Type, Started: h.Started}
			byKey[a.Key] = a
			s.Aircraft = append(s.Aircraft, a)
		case l.K != "":
			a := byKey[l.K]
			if a == nil {
				return nil, fmt.Errorf("flight: scene line %d: aircraft %q not declared", n+1, l.K)
			}
			a.Track.Samples = append(a.Track.Samples, decode(l.R))
		}
	}
	for _, a := range s.Aircraft {
		sort.SliceStable(a.Track.Samples, func(i, j int) bool { return a.Track.Samples[i].T < a.Track.Samples[j].T })
	}
	return s, nil
}

// WriteFile writes s to path (gzipped when it ends in ".gz").
func (s *Scene) WriteFile(path string) error {
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
	err = s.Write(w)
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

// ReadSceneFile reads a Scene from path (gzipped when it ends in ".gz").
func ReadSceneFile(path string) (*Scene, error) {
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
	return ReadScene(r)
}
