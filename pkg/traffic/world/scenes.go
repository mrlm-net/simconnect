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
// runway), "ROLE:heard" (a call to or from it is heard), "ROLE:rescue"
// (rescue vehicles sent for it) or "ROLE:rescue:with" (down, a vehicle
// within 80 m of it).
//
// A scene can set the airport up first (Jetways, Schedule), have a cast
// member declare an emergency (SceneCast.Emergency), run World actions at
// a beat (SceneBeat.Actions: the scenario actions, e.g. a runway closed or
// opened, "say" expectDelay or holdPosition) and once it ends (EndActions).
//
//	GET  /api/camera/scenes             — the scenes
//	POST /api/camera/scene?name=&icao=  — play one; no name: the Scene in the body

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
	// Jetways: switched on as the scene starts. Schedule: the airport's
	// scheduled traffic around the cast, switched on if it is off.
	Jetways  bool           `json:"jetways,omitempty"`
	Schedule *SceneSchedule `json:"schedule,omitempty"`
	// EndActions run however the scene ends (a closed runway opened again).
	EndActions []ScenarioAction `json:"endActions,omitempty"`
}

// SceneSchedule is the scheduled traffic a scene plays among.
type SceneSchedule struct {
	Density     float64 `json:"density"` // 1: the timetable as it is
	MaxAircraft int     `json:"maxAircraft,omitempty"`
}

// SceneCast is an aircraft of the scene.
type SceneCast struct {
	Role string `json:"role"` // what cues and shots call it: "dep"
	Kind string `json:"kind"` // departure | arrival
	Tail string `json:"tail"`
	// Models: title fragments to look for, in order ("csa-lines, a320").
	Models string `json:"models"`
	// AtSec: when it appears, seconds into the scene, or after After's cue
	// ("dep:pushback": the arrival tied to the departure's push, however
	// long its turnaround takes); PushAfterSec: a departure's push, after it
	// appears (at the earliest: its fuel truck, its tug first).
	AtSec        float64 `json:"atSec"`
	After        string  `json:"after,omitempty"`
	PushAfterSec float64 `json:"pushAfterSec"`
	Procedure    bool    `json:"procedure"` // a SID (departure)
	Tug          bool    `json:"tug"`
	// Stand: a departure's stand ("B9"); "" any free one. Fuel: refuelled
	// on it before the push.
	Stand string `json:"stand,omitempty"`
	Fuel  bool   `json:"fuel,omitempty"`
	// Emergency: an arrival declares one of this kind as it comes in
	// (engine-failure, medical, fuel, pressurisation, fire, hydraulic):
	// MAYDAY or PAN PAN, priority, the airport's rescue vehicles.
	Emergency string `json:"emergency,omitempty"`
}

// SceneBeat cuts to its shots when its cue comes.
type SceneBeat struct {
	Cue   string      `json:"cue"`
	Shots []SceneShot `json:"shots"`
	// Actions run as the beat starts: World actions on the cast (Who a
	// role), as a scenario's steps run them.
	Actions []ScenarioAction `json:"actions,omitempty"`
	// DelaySec: the beat starts this long after its cue came.
	DelaySec float64 `json:"delaySec,omitempty"`
	// End: the scene ends once its shots have played.
	End bool `json:"end"`
	// Listen switches the radio to this one's frequency from this beat on,
	// or to a frequency given ("132.89")
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
	Move string `json:"move"`
	// From, To: in place of Move, a slow glide from one picked frame to
	// another (compositions.go), To empty holding From.
	From string  `json:"from"`
	To   string  `json:"to"`
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
	// Long: Sec is exact and may run past camera.MaxShot, the move slowed to
	// fill it (camera.Stretch), for a shot that stays with an aircraft
	// through an event. Otherwise a shot is paced and capped.
	Long bool `json:"long,omitempty"`
	// Spot: in place of Move, the camera at a spot of the airport (spots.go),
	// following the aircraft for Sec exactly (0: DefaultSpotSec): "touchdown:24",
	// or with no runway the aircraft's ("touchdown"). Share and Lock change
	// the spot's share of the frame and aim at the object itself.
	Spot  string  `json:"spot,omitempty"`
	Share float64 `json:"share,omitempty"`
	Lock  bool    `json:"lock,omitempty"`
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
	return m.startScene(g, sc)
}

// startScene starts sc at g's airport, unless one is playing.
func (m *cameraMan) startScene(g *airport.Graph, sc Scene) error {
	if m.dir == nil {
		return errors.New("no camera on this connection")
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
	// beat, cue: what it has reached; events to the host until ended.
	beat   int
	cue    string
	events chan SceneEvent
	ended  bool
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
			if sent, with := r.m.cc.rescueStatus(v.Tail); sent {
				r.m.mu.Lock()
				s["rescue"] = true
				if with {
					s["rescue:with"] = true
				}
				r.m.mu.Unlock()
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (m *cameraMan) runScene(g *airport.Graph, sc Scene) {
	cc := m.cc
	r := &sceneRun{m: m, sc: sc, start: time.Now(), cast: map[string]*controlled{}, heard: map[string]bool{}, listening: sc.Listen, seen: map[string]map[string]bool{}, top: map[string]float64{}}
	r.listenEvents()
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
		r.finish(err.Error())
		return
	}
	m.mu.Lock()
	m.scene, m.subject, m.shot = r, "", sc.Name
	r.emit(SceneEvent{Kind: "start"})
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.scene = nil
		still := m.mode == "scene"
		m.mu.Unlock()
		if still {
			m.setMode("off", 0) // not over a mode picked meanwhile (#62)
		}
		r.act(g, sc.EndActions, logf)
		logf("ended after %s", time.Since(r.start).Round(time.Second))
		r.finish("")
	}()
	// The airport set up: jetways, the scheduled traffic around the cast.
	if m.w != nil && sc.Jetways {
		m.w.SetJetways(true)
	}
	if m.w != nil && sc.Schedule != nil {
		var now ScheduleSettings
		if err := m.w.Get("/api/schedule", &now); err == nil && !now.Enabled {
			icao := g.Layout.ICAO
			if err := m.w.SetSchedule(ScheduleSettings{Enabled: true, ICAO: icao, Airports: []string{icao}, Density: sc.Schedule.Density, MaxAircraft: sc.Schedule.MaxAircraft}); err != nil {
				logf("schedule: %v", err)
			}
		}
	}
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
			from := r.start
			if c.After != "" {
				for !r.cued(c.After) {
					m.mu.Lock()
					on := m.mode == "scene"
					m.mu.Unlock()
					if !on {
						return
					}
					time.Sleep(500 * time.Millisecond)
				}
				from = time.Now()
				logf("%s %s: %s came, appears in %.0f s", c.Role, c.Tail, c.After, c.AtSec)
			}
			time.Sleep(time.Until(from.Add(time.Duration(c.AtSec * float64(time.Second)))))
			req := SpawnRequest{Kind: c.Kind, ICAO: g.Layout.ICAO, Stand: -1, Model: cc.modelFor(c.Models), Tail: c.Tail, Tug: c.Tug, Procedure: c.Procedure,
				InjectApproach: c.Kind == "arrival", Fuel: c.Fuel}
			if c.Stand != "" {
				i, err := g.Layout.ParkingIndex(c.Stand)
				if err != nil {
					logf("%s %s: stand %s: %v", c.Role, c.Tail, c.Stand, err)
					return
				}
				req.Stand = i
			}
			if c.Kind == "departure" && c.PushAfterSec > 0 {
				req.pushAt = cc.clock.Now().Add(time.Duration(c.PushAfterSec * float64(time.Second)))
			}
			for i := 0; i < 6; i++ {
				var it *controlled
				err := cc.do(func() (e error) { it, e = cc.spawn(g, req); return e })
				if err == nil {
					it.mu.Lock()
					stand := it.view.Stand
					it.mu.Unlock()
					logf("%s %s at %s", c.Role, c.Tail, stand)
					m.mu.Lock()
					r.cast[c.Role] = it
					m.mu.Unlock()
					if c.Emergency != "" {
						go r.emergency(it, c)
					}
					return
				}
				logf("%s %s: %v", c.Role, c.Tail, err)
				if req.Stand >= 0 && !errors.Is(err, traffic.ErrSpawnBlocked) {
					req.Stand = -1 // its stand taken: any free one
					continue
				}
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
		var cuedAt time.Time // the cue come, the beat waits its delay
		for {
			if cuedAt.IsZero() && r.cued(b.Cue) && r.ready(b) {
				cuedAt = time.Now()
				r.event(SceneEvent{Kind: "cue", Beat: i + 1, Cue: b.Cue})
			}
			if !cuedAt.IsZero() && time.Since(cuedAt) >= time.Duration(b.DelaySec*float64(time.Second)) {
				break
			}
			// Waiting longer than the beat's shots: more of the same aircraft,
			// never a still held past camera.MaxShot.
			if cur, _ := m.dir.Current(time.Now()); cur == nil && m.dir.Remaining() == 0 && subject != nil {
				m.dir.Play(fillerGlides(g, subject, fill)...)
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
		r.act(g, b.Actions, logf)
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
		r.beat, r.cue = i+1, b.Cue
		r.emit(SceneEvent{Kind: "beat", Beat: i + 1, Cue: b.Cue, Subject: m.subject})
		m.mu.Unlock()
		if len(shots) > 0 {
			m.mu.Lock()
			subject = r.cast[b.Shots[0].Who]
			m.mu.Unlock()
			m.lockAirport(subject)
			m.dir.Play(shots...)
		}
		if b.End || i == len(sc.Beats)-1 { // the last beat plays out, End or not
			var total time.Duration
			for _, s := range shots {
				total += s.Length()
			}
			// A long shot can run for minutes: stopped, it ends now.
			for end := time.Now().Add(total); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
				m.mu.Lock()
				stopped := m.mode != "scene"
				m.mu.Unlock()
				if stopped {
					break
				}
			}
			return
		}
	}
}

// act runs a beat's World actions on the cast (Who a role), as a
// scenario's steps run them; each failure logged, the scene going on.
func (r *sceneRun) act(g *airport.Graph, list []ScenarioAction, logf func(string, ...any)) {
	if len(list) == 0 {
		return
	}
	if r.m.w == nil {
		logf("actions: no World on this connection")
		return
	}
	r.m.mu.Lock()
	cast := make(map[string]string, len(r.cast))
	for role, it := range r.cast {
		cast[role] = it.Tail
	}
	r.m.mu.Unlock()
	for _, a := range list {
		if err := r.m.w.Act(g.Layout.ICAO, a, cast); err != nil {
			logf("action %s %s: %v", a.Do, a.Who, err)
		}
	}
}

// emergency has cast member it declare c's emergency once it is flying:
// its MAYDAY or PAN PAN on the radio, priority, the rescue vehicles.
func (r *sceneRun) emergency(it *controlled, c SceneCast) {
	for it.object() == 0 {
		r.m.mu.Lock()
		on := r.m.mode == "scene"
		r.m.mu.Unlock()
		if !on {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := r.m.cc.declareEmergency(it, traffic.EmergencyKind(c.Emergency), "", false); err != nil {
		tlog.printf("scene  %s: %s emergency: %v", r.sc.Name, c.Tail, err)
	}
}

// ready reports whether the aircraft of b's shots are there to film.
func (r *sceneRun) ready(b SceneBeat) bool {
	for _, s := range b.Shots {
		r.m.mu.Lock()
		it := r.cast[s.Who]
		r.m.mu.Unlock()
		if it == nil || it.object() == 0 {
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
	it := r.cast[role]
	r.m.mu.Unlock()
	if it == nil || it.object() == 0 {
		return false
	}
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
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
	r.m.mu.Lock()
	cast := make(map[string]*controlled, len(r.cast))
	for k, v := range r.cast {
		cast[k] = v
	}
	r.m.mu.Unlock()
	return castShots(g, cast, list, r.sc.Name)
}

// castShots builds shots on cast (by role), for scene or scenario name.
func castShots(g *airport.Graph, cast map[string]*controlled, list []SceneShot, name string) []camera.Shot {
	var out []camera.Shot
	for _, s := range list {
		it := cast[s.Who]
		if it == nil {
			continue
		}
		it.mu.Lock()
		v, obj := it.view, it.objectID
		it.mu.Unlock()
		if obj == 0 {
			continue
		}
		// Slower than written: the moves read better unhurried (at most
		// camera.MaxShot).
		d := min(time.Duration(s.Sec*scenePace*float64(time.Second)), camera.MaxShot)
		long := time.Duration(s.Sec * float64(time.Second))
		if !s.Long {
			long = 0
		}
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
		label := it.Tail + " " + s.Move
		var shot camera.Shot
		switch {
		case s.Spot != "":
			spot := s.Spot
			if !strings.Contains(spot, ":") {
				spot += ":" + v.Runway
			}
			sh, err := it.cc.spotShot(g, spot, obj, false, size.Length, s.Sec, spotOptions{s.Share, s.Lock})
			if err != nil {
				tlog.printf("scene  %s: %v", name, err)
				continue
			}
			shot, label = sh, it.Tail+" "+spot
		case s.From != "":
			if gl, ok := glideShot(g, it, s.From, s.To, side, d); ok {
				shot, label = gl, it.Tail+" "+gl.Name()
			}
		case s.Move == "crane":
			shot = craneShot(obj, size, side, d)
		case s.Move == "runwaySide" || s.Move == "underApproach":
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
				shot = camera.TrackZoom(s.Move, eye, obj, fov, to, d)
			}
		default:
			if f, ok := sceneMoves[s.Move]; ok {
				shot = f(obj, size, side, d)
			}
		}
		if shot == nil {
			tlog.printf("scene  %s: no move %q", name, s.Move)
			continue
		}
		if long > 0 {
			shot = camera.Stretch(shot, long)
		}
		out = append(out, named{shot, label})
	}
	return out
}

// listen keeps the voice on the frequency of the role listened to: the
// scene's Listen, then each beat's.
func (r *sceneRun) listen() {
	for {
		r.m.mu.Lock()
		on := r.m.mode == "scene"
		lis := r.listening
		it := r.cast[lis]
		r.m.mu.Unlock()
		if !on {
			return
		}
		f := ""
		if it != nil {
			it.mu.Lock()
			f = it.view.Frequency
			it.mu.Unlock()
		} else if isFrequency(lis) {
			f = lis // a frequency of its own ("132.89": a station not ours)
		}
		if h := r.m.cc.core.hooks.SceneFrequency; h != nil && f != "" {
			h(f) // a voice follows the scene's radio
		}
		time.Sleep(time.Second)
	}
}

// isFrequency reports whether s is a VHF airband frequency ("132.89").
func isFrequency(s string) bool {
	f, err := strconv.ParseFloat(s, 64)
	return err == nil && f >= 108 && f <= 137
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
			if strings.Contains(lt, want) && !traffic.ModelNever(t) && !strings.Contains(lt, "stub") && !strings.Contains(lt, "force") && !strings.Contains(lt, "military") {
				return t
			}
		}
	}
	return ""
}
