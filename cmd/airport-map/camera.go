//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The camera operator: the simulator's add-on camera on our traffic.
// Follow keeps it on the aircraft picked on the map; auto cuts to the
// aircraft on the radio as it is heard, with a shot fitting what it does
// (orbit on the stand and the push, side tracking on the taxiway, a camera
// beside the runway for the take-off roll and the landing, a chase on the
// climb-out); the demo plays a planned scene with it. Every shot is at most
// camera.MaxShot and moves smoothly.
//
//	GET  /api/camera            — mode, subject, shot
//	POST /api/camera {mode, id} — off | follow (id: the card) | auto
//	GET  /api/camera/scenes            — the scripted scenes (scenes.go)
//	POST /api/camera/scene?name=&icao= — play one

// cameraHeard is the live connection's camera operator, for the radio and
// the voice to tell what is heard.
var cameraHeard atomic.Pointer[*cameraMan]

// heardOnCamera tells the camera a call is heard. Never in the caller's
// goroutine: calls are said with the aircraft's lock held, and the cut
// takes it again (a deadlock that froze the whole map).
func heardOnCamera(t traffic.Transmission) {
	if p := cameraHeard.Load(); p != nil {
		go (*p).heard(t)
	}
}

// cameraRate is how often the camera is set while a shot plays.
const cameraRate = time.Second / 60

// minShotBeforeCut: a shot runs at least this long before a call on the
// radio cuts to another aircraft.
const minShotBeforeCut = 5 * time.Second

type cameraMan struct {
	cc  *controlCenter
	dir *camera.Director

	mu      sync.Mutex
	mode    string // off, follow, auto, demo
	follow  int    // the card followed
	subject string // the aircraft on screen
	since   time.Time
	shots   int // shots of the subject in a row
	shot    string
	err     string
	locked  string // the airport the world is kept loaded around
	scene   *sceneRun
	picking atomic.Bool // a next shot being picked
}

func newCameraMan(cc *controlCenter, client engine.Client) *cameraMan {
	m := &cameraMan{cc: cc, mode: "off"}
	if e, ok := client.(*engine.Engine); ok {
		m.dir = camera.NewDirector(e, "airport-map")
		m.dir.OnShot(func(s camera.Shot) {
			m.mu.Lock()
			m.shot = s.Name()
			m.mu.Unlock()
		})
	}
	return m
}

// handle takes the camera's status messages.
func (m *cameraMan) handle(msg engine.Message) bool {
	return m.dir != nil && m.dir.Handle(msg)
}

// tick sets the camera; in the connection's goroutine at cameraRate.
func (m *cameraMan) tick(now time.Time) {
	if m.dir == nil {
		return
	}
	m.mu.Lock()
	mode := m.mode
	m.mu.Unlock()
	if mode == "off" {
		return
	}
	// The next shot is picked off the connection's goroutine: it looks at
	// the aircraft (their locks), and a goroutine holding one may be
	// waiting on this one.
	if cur, _ := m.dir.Current(now); cur == nil && m.dir.Remaining() == 0 && mode != "scene" && m.picking.CompareAndSwap(false, true) {
		go func() {
			defer m.picking.Store(false)
			m.next(now)
		}()
	}
	if err := m.dir.Tick(now); err != nil && !errors.Is(err, camera.ErrNotAcquired) {
		m.mu.Lock()
		m.err = err.Error()
		m.mu.Unlock()
	}
}

// setMode switches the camera: off gives it back to the simulator.
func (m *cameraMan) setMode(mode string, follow int) error {
	if m.dir == nil {
		return errors.New("no camera on this connection")
	}
	switch mode {
	case "off", "follow", "auto", "scene":
	default:
		return errors.New("mode: off, follow or auto")
	}
	m.mu.Lock()
	m.mode, m.follow, m.subject, m.shots, m.err = mode, follow, "", 0, ""
	m.mu.Unlock()
	if mode == "off" {
		return m.cc.do(m.dir.Release)
	}
	m.dir.Play() // the next tick picks a shot
	return nil
}

// heard is a call on the radio, as it is heard: in auto and the demo the
// camera cuts to the aircraft talking or talked to.
func (m *cameraMan) heard(t traffic.Transmission) {
	if m.dir == nil || t.Callsign == "" {
		return
	}
	m.mu.Lock()
	mode, subject, since := m.mode, m.subject, m.since
	if m.scene != nil {
		m.scene.heardCall(t.Callsign) // a cue; the scene cuts itself
	}
	m.mu.Unlock()
	if mode != "auto" || t.Callsign == subject {
		return
	}
	now := time.Now()
	if subject != "" && now.Sub(since) < minShotBeforeCut {
		return
	}
	it := m.cc.byTail(t.Callsign)
	if it == nil || it.objectID == 0 {
		return
	}
	m.cut(it, now)
}

// cut puts it on screen now with the shot for what it does.
func (m *cameraMan) cut(it *controlled, now time.Time) {
	m.mu.Lock()
	if m.subject != it.Tail {
		m.shots = 0
	}
	n := m.shots
	m.subject, m.since, m.shots = it.Tail, now, n+1
	m.mu.Unlock()
	m.lockAirport(it)
	m.dir.Play(sequenceFor(it, n)...)
}

// lockAirport keeps the airport of it loaded while the camera is on it: no
// scenery loading on a cut across it.
func (m *cameraMan) lockAirport(it *controlled) {
	m.mu.Lock()
	same := m.locked == it.ICAO
	m.locked = it.ICAO
	m.mu.Unlock()
	if !same {
		l := it.graph.Layout
		m.dir.LockWorld(camera.At(l.Latitude, l.Longitude, l.Altitude))
	}
}

// next picks the shot after one has ended: the aircraft followed; in auto
// the subject again for a second look, else the busiest aircraft.
func (m *cameraMan) next(now time.Time) {
	m.mu.Lock()
	mode, follow, subject, shots := m.mode, m.follow, m.subject, m.shots
	m.mu.Unlock()
	m.cc.mu.Lock()
	var items []*controlled
	for _, it := range m.cc.items {
		items = append(items, it)
	}
	m.cc.mu.Unlock()
	var pick *controlled
	best := -1
	for _, it := range items {
		it.mu.Lock()
		done, id := it.view.Done, it.ID
		it.mu.Unlock()
		if done || it.objectID == 0 {
			continue
		}
		if mode == "follow" {
			if id == follow {
				pick = it
			}
			continue
		}
		score := interest(it)
		if it.Tail == subject && shots < 2 {
			score += 3 // a second look before moving on
		}
		if score > best {
			best, pick = score, it
		}
	}
	if pick != nil {
		m.cut(pick, now)
	}
}

// interest ranks what an aircraft is doing for the auto camera.
func interest(it *controlled) int {
	it.mu.Lock()
	v := it.view
	it.mu.Unlock()
	switch v.State {
	case "departing", "landing", "rollout":
		return 6
	case "lining up", "lined up", "pushback":
		return 4
	case "approaching":
		return 3
	case "holding short", "taxiing", "vacating":
		return 2
	}
	return 1
}

// Shot lengths: a wide shot long enough to read the move, a detail one a
// beat.
const (
	wideShot   = 7 * time.Second
	detailShot = 4500 * time.Millisecond
)

// phaseMoves are the drone moves for what an aircraft is doing, wide and
// detail, taken in turn so a second look is another angle.
var phaseMoves = map[string]struct{ wide, detail []camera.MoveFunc }{
	"stand":   {[]camera.MoveFunc{camera.RevealRise, camera.SpiralDescend, camera.TopDown}, []camera.MoveFunc{camera.CockpitDetail, camera.TailDetail, camera.WingtipAlong}},
	"push":    {[]camera.MoveFunc{camera.TopDown, camera.PullBackReveal, camera.Flyover}, []camera.MoveFunc{camera.NoseGearDetail, camera.WingtipAlong, camera.CockpitDetail}},
	"start":   {[]camera.MoveFunc{camera.HeroLowPushIn, camera.PullBackReveal}, []camera.MoveFunc{camera.EngineDetail, camera.CockpitDetail}},
	"taxi":    {[]camera.MoveFunc{camera.ParallaxTrack, camera.LeadChase, camera.Flyover, camera.SpiralDescend}, []camera.MoveFunc{camera.TailDetail, camera.CockpitDetail, camera.NoseGearDetail}},
	"holding": {[]camera.MoveFunc{camera.HeroLowPushIn, camera.SpiralDescend, camera.RevealRise}, []camera.MoveFunc{camera.CockpitDetail, camera.EngineDetail, camera.TailDetail}},
	"roll":    {[]camera.MoveFunc{camera.LeadChase, camera.ParallaxTrack}, []camera.MoveFunc{camera.EngineDetail, camera.NoseGearDetail}},
	"climb":   {[]camera.MoveFunc{camera.ChaseRise, camera.LeadChase, camera.Flyover}, []camera.MoveFunc{camera.TailDetail, camera.EngineDetail}},
	"final":   {[]camera.MoveFunc{camera.LeadChase, camera.ChaseRise, camera.SpiralDescend}, []camera.MoveFunc{camera.CockpitDetail, camera.NoseGearDetail, camera.EngineDetail}},
}

// phaseOf is the camera phase of what an aircraft does.
func phaseOf(v ControlView) string {
	switch v.State {
	case "spawning", "awaiting pushback", "parked":
		return "stand"
	case "pushback":
		return "push"
	case "awaiting taxi":
		return "start"
	case "taxiing", "vacating":
		return "taxi"
	case "holding short", "lining up", "lined up":
		return "holding"
	case "departing":
		if v.OnGround {
			return "roll"
		}
		return "climb"
	case "rollout":
		return "roll"
	case "approaching", "landing":
		if v.OnGround {
			return "roll"
		}
		return "final"
	}
	return "taxi"
}

// sequenceFor is a short sequence on it for what it is doing: wide, a
// detail, wide again, cut against each other; on the runway and on final a
// camera fixed beside the runway opens it. Look n gives other moves and
// the other side.
func sequenceFor(it *controlled, n int) []camera.Shot {
	it.mu.Lock()
	v := it.view
	it.mu.Unlock()
	obj, l := it.objectID, it.graph.Layout
	m := traffic.ProfileFor(strings.SplitN(v.Model, liverySep, 2)[0]).Motion
	size := camera.Size{Span: m.SpanMeters, Length: m.SpanMeters * 1.05}
	side := 1.0
	if n%2 == 1 {
		side = -1
	}
	ph := phaseOf(v)
	moves := phaseMoves[ph]
	pick := func(list []camera.MoveFunc, k int) camera.MoveFunc { return list[(n+k)%len(list)] }
	name := func(s camera.Shot) camera.Shot { return named{s, it.Tail + " " + s.Name()} }
	var out []camera.Shot
	switch {
	case ph == "roll" && n%2 == 0:
		along := 700.0
		if it.arr != nil {
			along = 900
		}
		if eye, ok := runwaySide(l, v.Runway, along, 90*side, 3); ok {
			out = append(out, name(camera.TrackZoom("runway side", eye, obj, 40, 22, wideShot)))
		}
	case ph == "final" && n%2 == 0:
		if eye, ok := runwaySide(l, v.Runway, -350, 110*side, 3); ok {
			out = append(out, name(camera.TrackZoom("under the approach", eye, obj, 45, 25, wideShot)))
		}
	}
	if len(out) == 0 {
		out = append(out, name(pick(moves.wide, 0)(obj, size, side, wideShot)))
	}
	out = append(out, name(pick(moves.detail, 0)(obj, size, -side, detailShot)))
	out = append(out, name(pick(moves.wide, 1)(obj, size, -side, wideShot)))
	return out
}

// named gives a shot the call sign of its aircraft.
type named struct {
	camera.Shot
	name string
}

func (s named) Name() string { return s.name }

// runwaySide is a camera point along runway end name's centreline (meters
// from its threshold, negative before it), off to its right (left when
// negative) and up above the field.
func runwaySide(l *airport.Layout, name string, along, right, up float64) (camera.Point, bool) {
	_, end, ok := l.RunwayEnd(name)
	if !ok {
		return camera.Point{}, false
	}
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, along)
	lat, lon = calc.DisplaceByHeading(lat, lon, math.Mod(end.Heading+90, 360), right)
	return camera.At(lat, lon, l.Altitude+up), true
}

type cameraView struct {
	Mode     string `json:"mode"`
	Subject  string `json:"subject,omitempty"`
	Shot     string `json:"shot,omitempty"`
	Acquired bool   `json:"acquired"`
	Error    string `json:"error,omitempty"`
}

func (m *cameraMan) view() cameraView {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := cameraView{Mode: m.mode, Subject: m.subject, Shot: m.shot, Error: m.err}
	if m.dir != nil {
		v.Acquired = m.dir.Acquired()
	}
	return v
}

// registerCamera serves the camera API.
func registerCamera(mux *http.ServeMux, st *state) {
	cam := func(w http.ResponseWriter) *cameraMan {
		st.mu.Lock()
		m := st.camera
		st.mu.Unlock()
		if m == nil {
			http.Error(w, "not connected to the simulator", http.StatusServiceUnavailable)
		}
		return m
	}
	mux.HandleFunc("GET /api/camera", func(w http.ResponseWriter, r *http.Request) {
		if m := cam(w); m != nil {
			writeJSON(w, m.view())
		}
	})
	mux.HandleFunc("POST /api/camera", func(w http.ResponseWriter, r *http.Request) {
		m := cam(w)
		if m == nil {
			return
		}
		var req struct {
			Mode string `json:"mode"`
			ID   int    `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.setMode(strings.ToLower(req.Mode), req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, m.view())
	})
	mux.HandleFunc("GET /api/camera/scenes", func(w http.ResponseWriter, r *http.Request) {
		list, err := listScenes()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	})
	mux.HandleFunc("POST /api/camera/scene", func(w http.ResponseWriter, r *http.Request) {
		m := cam(w)
		if m == nil {
			return
		}
		g, err := m.cc.graph(strings.ToUpper(r.URL.Query().Get("icao")))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.playScene(g, r.URL.Query().Get("name")); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, m.view())
	})
}

