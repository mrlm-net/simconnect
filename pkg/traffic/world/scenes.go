package world

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Scenes: scripted films of our traffic. A scene spawns its cast, listens
// to one of them on the radio, and plays beats: each waits for its cue and
// cuts to its shots. Scenes are JSON files in -scenes (read on every run,
// so a scene can be edited and played again); the built-in ones are the
// fallback.
//
// A cue is "start", "t:SECONDS" (since the scene began), "ROLE:STATE" (the
// controller state, e.g. "dep:pushback", "dep:taxiing"; "dep:airborne" once
// off the ground), "ROLE:lights:X" (a light comes on: N nav, B beacon, S
// strobe, T taxi, L landing, O logo, W wing), "ROLE:height:FEET" (above the
// runway) or "ROLE:heard" (a call to or from it is heard).
//
//	GET  /api/camera/scenes             — the scenes
//	POST /api/camera/scene?name=&icao=  — play one

//go:embed scenes/*.json
var builtinScenes embed.FS

// scenePace stretches every scene shot (the moves were found a little
// fast).
const scenePace = 1.3

// scenesDir is where scenes are read from first (-scenes).
var scenesDir = "scenes"

// Scene is a scripted film.
type Scene struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Cast        []SceneCast `json:"cast"`
	// Listen: the voice follows this one's frequency.
	Listen string      `json:"listen"`
	Beats  []SceneBeat `json:"beats"`
	// LimitSec ends the scene however far it got (default 900).
	LimitSec float64 `json:"limitSec"`
}

// SceneCast is an aircraft of the scene.
type SceneCast struct {
	Role string `json:"role"` // what cues and shots call it: "dep"
	Kind string `json:"kind"` // departure | arrival
	Tail string `json:"tail"`
	// Models: title fragments to look for, in order ("csa-lines, a320").
	Models string `json:"models"`
	// AtSec: when it appears; PushAfterSec: a departure's push, after it
	// appears.
	AtSec        float64 `json:"atSec"`
	PushAfterSec float64 `json:"pushAfterSec"`
	Procedure    bool    `json:"procedure"` // a SID (departure)
	Tug          bool    `json:"tug"`
}

// SceneBeat cuts to its shots when its cue comes.
type SceneBeat struct {
	Cue   string      `json:"cue"`
	Shots []SceneShot `json:"shots"`
	// End: the scene ends once its shots have played.
	End bool `json:"end"`
	// Listen switches the radio to this one's frequency from this beat on
	// ("" keeps it).
	Listen string `json:"listen"`
}

// SceneShot is one shot: a drone move on an aircraft of the cast, or a
// camera fixed beside its runway.
type SceneShot struct {
	// Move: crane, revealRise, flyover, spiralDescend, leadChase,
	// parallaxTrack, heroPushIn, pullBackReveal, topDown, topOrbit,
	// sideDolly, headOnPass, chaseRise, engine, noseGear, mainGear,
	// cockpit, tail, wingtip, lights; runwaySide and underApproach (fixed,
	// zooming: Along, Right, Up, Fov, FovTo).
	Move string  `json:"move"`
	Who  string  `json:"who"`
	Side float64 `json:"side"` // +1 its right (default), -1 its left
	Sec  float64 `json:"sec"`
	// runwaySide, underApproach: meters along the runway from its threshold,
	// to its right, above it; the zoom.
	Along float64 `json:"along"`
	Right float64 `json:"right"`
	Up    float64 `json:"up"`
	Fov   float64 `json:"fov"`
	FovTo float64 `json:"fovTo"`
}

var sceneMoves = map[string]camera.MoveFunc{
	"revealRise": camera.RevealRise, "flyover": camera.Flyover, "spiralDescend": camera.SpiralDescend,
	"leadChase": camera.LeadChase, "parallaxTrack": camera.ParallaxTrack, "heroPushIn": camera.HeroLowPushIn,
	"pullBackReveal": camera.PullBackReveal, "topDown": camera.TopDown, "topOrbit": camera.TopOrbit,
	"sideDolly": camera.SideDolly, "headOnPass": camera.HeadOnPass, "chaseRise": camera.ChaseRise,
	"engine": camera.EngineDetail, "noseGear": camera.NoseGearDetail, "mainGear": camera.MainGearDetail,
	"cockpit": camera.CockpitDetail, "tail": camera.TailDetail, "wingtip": camera.WingtipAlong, "lights": camera.LightsDetail,
}

// craneShot opens a scene: from high behind down onto the aircraft.
func craneShot(o uint32, s camera.Size, side float64, d time.Duration) camera.Shot {
	w, l := s.Span/2, s.Length
	return camera.Path("opening crane", d, nil,
		camera.Key{At: 0, Pose: camera.Pose{Eye: camera.On(o, side*w*4, l*4, -l*6.5), Target: camera.On(o, 0, 0, 0), FovDeg: 60}},
		camera.Key{At: 1, Pose: camera.Pose{Eye: camera.On(o, side*w*1.3, l*0.3, -l*1.6), Target: camera.On(o, 0, 2, 0), FovDeg: 50}})
}

// loadScenes reads the scenes: -scenes first, the built-in ones for the
// names not there.
func loadScenes() (map[string]Scene, error) {
	out := map[string]Scene{}
	read := func(fsys fs.FS, dir string) error {
		files, err := fs.Glob(fsys, dir+"/*.json")
		if err != nil {
			return err
		}
		for _, f := range files {
			b, err := fs.ReadFile(fsys, f)
			if err != nil {
				return err
			}
			var sc Scene
			if err := json.Unmarshal(b, &sc); err != nil {
				return fmt.Errorf("%s: %w", f, err)
			}
			key := strings.TrimSuffix(filepath.Base(f), ".json")
			if _, ok := out[key]; !ok {
				out[key] = sc
			}
		}
		return nil
	}
	if fi, err := os.Stat(scenesDir); err == nil && fi.IsDir() {
		if err := read(os.DirFS(scenesDir), "."); err != nil {
			return nil, err
		}
	}
	if err := read(builtinScenes, "scenes"); err != nil {
		return nil, err
	}
	return out, nil
}

type sceneInfo struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// listScenes are the scenes for the map's picker, by key.
func listScenes() ([]sceneInfo, error) {
	m, err := loadScenes()
	if err != nil {
		return nil, err
	}
	var out []sceneInfo
	for k, s := range m {
		out = append(out, sceneInfo{k, s.Name, s.Description})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// playScene starts scene key at g's airport.
func (m *cameraMan) playScene(g *airport.Graph, key string) error {
	if m.dir == nil {
		return errors.New("no camera on this connection")
	}
	all, err := loadScenes()
	if err != nil {
		return err
	}
	sc, ok := all[key]
	if !ok {
		return fmt.Errorf("no scene %q", key)
	}
	m.mu.Lock()
	running := m.mode == "scene" || m.sceneStarting
	m.sceneStarting = !running || m.sceneStarting
	m.mu.Unlock()
	if running {
		return errors.New("a scene is playing")
	}
	go m.runScene(g, sc)
	return nil
}

// sceneRun is a scene playing: its cast and what has happened to it.
type sceneRun struct {
	m     *cameraMan
	sc    Scene
	start time.Time
	cast  map[string]*controlled
	heard map[string]bool // roles heard since the last beat
	// listening: the role whose frequency the radio follows.
	listening string
	// seen: what each role has reached ("pushback", "airborne",
	// "lights:B"), watched every 100 ms, so a cue holds once passed; top
	// its greatest height above the runway.
	seen map[string]map[string]bool
	top  map[string]float64
}

// watch records what the cast reaches.
func (r *sceneRun) watch() {
	for {
		r.m.mu.Lock()
		on := r.m.mode == "scene"
		cast := make(map[string]*controlled, len(r.cast))
		for k, v := range r.cast {
			cast[k] = v
		}
		r.m.mu.Unlock()
		if !on {
			return
		}
		for role, it := range cast {
			it.mu.Lock()
			v, h := it.view, it.heightFt
			it.mu.Unlock()
			r.m.mu.Lock()
			s := r.seen[role]
			if s == nil {
				s = map[string]bool{}
				r.seen[role] = s
			}
			s[v.State] = true
			if v.State == "departing" && !v.OnGround {
				s["airborne"] = true
				r.top[role] = max(r.top[role], h)
			}
			for _, c := range v.Lights {
				if c != '.' {
					s["lights:"+string(c)] = true
				}
			}
			r.m.mu.Unlock()
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (m *cameraMan) runScene(g *airport.Graph, sc Scene) {
	cc := m.cc
	r := &sceneRun{m: m, sc: sc, start: time.Now(), cast: map[string]*controlled{}, heard: map[string]bool{}, listening: sc.Listen, seen: map[string]map[string]bool{}, top: map[string]float64{}}
	logf := func(f string, a ...any) { tlog.printf("scene  "+sc.Name+": "+f, a...) }
	limit := time.Duration(sc.LimitSec * float64(time.Second))
	if limit <= 0 {
		limit = 15 * time.Minute
	}
	err := m.setMode("scene", 0)
	m.mu.Lock()
	m.sceneStarting = false
	m.mu.Unlock()
	if err != nil {
		logf("camera: %v", err)
		return
	}
	m.mu.Lock()
	m.scene, m.subject, m.shot = r, "", sc.Name
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.scene = nil
		still := m.mode == "scene"
		m.mu.Unlock()
		if still {
			m.setMode("off", 0) // not over a mode picked meanwhile (#62)
		}
		logf("ended after %s", time.Since(r.start).Round(time.Second))
	}()
	// A scene played again: its cast from last time goes first.
	for _, c := range sc.Cast {
		if old := cc.byTail(c.Tail); old != nil {
			if err := cc.do(func() error { return cc.remove(old) }); err != nil {
				logf("%s: %v", c.Tail, err)
			}
		}
	}
	// The cast, each at its time.
	for _, c := range sc.Cast {
		go func(c SceneCast) {
			time.Sleep(time.Until(r.start.Add(time.Duration(c.AtSec * float64(time.Second)))))
			req := SpawnRequest{Kind: c.Kind, ICAO: g.Layout.ICAO, Stand: -1, Model: cc.modelFor(c.Models), Tail: c.Tail, Tug: c.Tug, Procedure: c.Procedure,
				InjectApproach: c.Kind == "arrival"}
			if c.Kind == "departure" && c.PushAfterSec > 0 {
				req.pushAt = cc.clock.Now().Add(time.Duration(c.PushAfterSec * float64(time.Second)))
			}
			for i := 0; i < 6; i++ {
				var it *controlled
				err := cc.do(func() (e error) { it, e = cc.spawn(g, req); return e })
				if err == nil {
					logf("%s %s at %s", c.Role, c.Tail, it.view.Stand)
					m.mu.Lock()
					r.cast[c.Role] = it
					m.mu.Unlock()
					return
				}
				logf("%s %s: %v", c.Role, c.Tail, err)
				if !errors.Is(err, traffic.ErrSpawnBlocked) {
					return
				}
				time.Sleep(10 * time.Second)
			}
		}(c)
	}
	go r.listen()
	go r.watch()
	var subject *controlled // on screen: the first aircraft of the last beat
	fill := 0
	for i, b := range sc.Beats {
		for !r.cued(b.Cue) || !r.ready(b) {
			// Waiting longer than the beat's shots: more of the same aircraft,
			// never a still held past camera.MaxShot.
			if cur, _ := m.dir.Current(time.Now()); cur == nil && m.dir.Remaining() == 0 && subject != nil {
				m.dir.Play(sequenceFor(subject, fill)...)
				fill++
			}
			if time.Since(r.start) > limit {
				logf("time is up at beat %d (%s)", i+1, b.Cue)
				return
			}
			m.mu.Lock()
			stopped := m.mode != "scene"
			m.mu.Unlock()
			if stopped {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		shots := r.shots(g, b.Shots)
		logf("beat %d (%s): %d shots", i+1, b.Cue, len(shots))
		m.mu.Lock()
		r.heard = map[string]bool{}
		if b.Listen != "" {
			r.listening = b.Listen
		}
		if len(b.Shots) > 0 && r.cast[b.Shots[0].Who] != nil {
			m.subject = r.cast[b.Shots[0].Who].Tail
		}
		m.mu.Unlock()
		if len(shots) > 0 {
			m.mu.Lock()
			subject = r.cast[b.Shots[0].Who]
			m.mu.Unlock()
			m.lockAirport(subject)
			m.dir.Play(shots...)
		}
		if b.End {
			var total time.Duration
			for _, s := range shots {
				total += s.Length()
			}
			time.Sleep(total)
			return
		}
	}
}

// ready reports whether the aircraft of b's shots are there to film.
func (r *sceneRun) ready(b SceneBeat) bool {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, s := range b.Shots {
		if it := r.cast[s.Who]; it == nil || it.objectID == 0 {
			return false
		}
	}
	return true
}

// cued reports whether cue has come.
func (r *sceneRun) cued(cue string) bool {
	if cue == "" || cue == "start" {
		return true
	}
	if s, ok := strings.CutPrefix(cue, "t:"); ok {
		sec, _ := strconv.ParseFloat(s, 64)
		return time.Since(r.start) >= time.Duration(sec*float64(time.Second))
	}
	role, what, _ := strings.Cut(cue, ":")
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	if it := r.cast[role]; it == nil || it.objectID == 0 {
		return false
	}
	switch {
	case what == "heard":
		return r.heard[role]
	case strings.HasPrefix(what, "height:"):
		ft, _ := strconv.ParseFloat(strings.TrimPrefix(what, "height:"), 64)
		return r.top[role] >= ft
	}
	return r.seen[role][what]
}

// shots builds a beat's shots on the cast.
func (r *sceneRun) shots(g *airport.Graph, list []SceneShot) []camera.Shot {
	var out []camera.Shot
	for _, s := range list {
		r.m.mu.Lock()
		it := r.cast[s.Who]
		r.m.mu.Unlock()
		if it == nil || it.objectID == 0 {
			continue
		}
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		// Slower than written: the moves read better unhurried (at most
		// camera.MaxShot).
		d := min(time.Duration(s.Sec*scenePace*float64(time.Second)), camera.MaxShot)
		if d <= 0 {
			d = wideShot
		}
		side := s.Side
		if side == 0 {
			side = 1
		}
		p := traffic.ProfileFor(strings.SplitN(v.Model, liverySep, 2)[0]).Motion
		size := camera.Size{Span: p.SpanMeters, Length: p.SpanMeters * 1.05}
		if size.Span <= 0 {
			size = camera.Size{Span: 36, Length: 38}
		}
		name := it.Tail + " " + s.Move
		var shot camera.Shot
		switch s.Move {
		case "crane":
			shot = craneShot(it.objectID, size, side, d)
		case "runwaySide", "underApproach":
			along, right, up := s.Along, s.Right, s.Up
			if s.Move == "underApproach" && along == 0 {
				along, right = -350, 110
			}
			if along == 0 && right == 0 {
				along, right = 700, 90
			}
			if up == 0 {
				up = 3
			}
			fov, to := s.Fov, s.FovTo
			if fov == 0 {
				fov = 40
			}
			if to == 0 {
				to = fov * 0.6
			}
			if eye, ok := runwaySide(g.Layout, v.Runway, along, right*side, up); ok {
				shot = camera.TrackZoom(s.Move, eye, it.objectID, fov, to, d)
			}
		default:
			if f, ok := sceneMoves[s.Move]; ok {
				shot = f(it.objectID, size, side, d)
			}
		}
		if shot == nil {
			tlog.printf("scene  %s: no move %q", r.sc.Name, s.Move)
			continue
		}
		out = append(out, named{shot, name})
	}
	return out
}

// listen keeps the voice on the frequency of the role listened to: the
// scene's Listen, then each beat's.
func (r *sceneRun) listen() {
	for {
		r.m.mu.Lock()
		on := r.m.mode == "scene"
		it := r.cast[r.listening]
		r.m.mu.Unlock()
		if !on {
			return
		}
		if it != nil {
			it.mu.Lock()
			f := it.view.Frequency
			it.mu.Unlock()
			if h := r.m.cc.core.hooks.SceneFrequency; h != nil && f != "" {
				h(f) // a voice follows the scene's radio
			}
		}
		time.Sleep(time.Second)
	}
}

// heardCall marks a call to or from an aircraft of the scene. m.mu held.
func (r *sceneRun) heardCall(tail string) {
	for role, it := range r.cast {
		if it.Tail == tail {
			r.heard[role] = true
		}
	}
}

// modelFor is an airliner title the simulator has, by fragments in order
// ("csa-lines, a320"); never a stub or a military one; "" the default.
func (cc *controlCenter) modelFor(hints string) string {
	list := cc.modelList()
	for _, want := range strings.Split(hints+",a320,a319,737", ",") {
		want = strings.ToLower(strings.TrimSpace(want))
		if want == "" {
			continue
		}
		for _, t := range list {
			lt := strings.ToLower(t)
			if strings.Contains(lt, want) && !strings.Contains(lt, "stub") && !strings.Contains(lt, "force") && !strings.Contains(lt, "military") {
				return t
			}
		}
	}
	return ""
}
