package world

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Scenarios: a scripted situation played around the traffic (the app's
// Situation Creator; the camera's scenes are one kind). A scenario's cast
// are aircraft of ours pinned to a role, flown by the World like any other,
// on top of the schedule's traffic. Its steps each wait for their cue, then
// a delay, then run their actions once; they wait side by side, so one
// step can follow another ("after:ID") or come on its own cue.
//
// A cue is:
//
//	start                   at once
//	t:SEC                   seconds since the scenario began
//	after:ID                once step ID's actions have run
//	ROLE:STATE              the controller state reached (spawning, pushback,
//	                        taxiing, holding short, lining up, lined up,
//	                        departing, approaching, landing, rollout,
//	                        vacating, parking, parked, …)
//	ROLE:airborne           off the ground after its take-off
//	ROLE:heard              a call to or from it heard since the step began waiting
//	ROLE:lights:X           a light came on (N B S T L O W)
//	ROLE:height:FEET        above the runway after its take-off
//	ROLE:distance:NM        within NM of the airport; "ROLE:distance:>NM" beyond it
//
// and anything else is the host's (ScenarioHooks.Cue). Time is the World's
// clock: a pause stops it.

// Scenario is a scripted situation at an airport.
type Scenario struct {
	Name string         `json:"name"`
	ICAO string         `json:"icao"`
	Cast []ScenarioCast `json:"cast"`
	// Steps wait side by side, each for its own cue.
	Steps []ScenarioStep `json:"steps"`
	// LimitSec ends the scenario however far it got (0: 900).
	LimitSec float64 `json:"limitSec,omitempty"`
}

// ScenarioCast is an aircraft of the scenario.
type ScenarioCast struct {
	Role     string `json:"role"`
	Kind     string `json:"kind"` // departure | arrival
	Type     string `json:"type,omitempty"`
	Callsign string `json:"callsign"`
	// Models: title fragments to look for, in order ("csa-lines, a320");
	// "" the installed model of Type in the call sign's livery.
	Models string `json:"models,omitempty"`
	// Stand: "" any free one.
	Stand string `json:"stand,omitempty"`
	// AtSec: when it appears (negative: only on a "spawn" action);
	// PushAfterSec: a departure's push after it appears (0: its schedule).
	AtSec        float64 `json:"atSec"`
	PushAfterSec float64 `json:"pushAfterSec,omitempty"`
	Procedure    bool    `json:"procedure,omitempty"` // a SID (departure)
	Tug          bool    `json:"tug,omitempty"`
}

// ScenarioStep runs its actions once its cue has come and its delay
// passed.
type ScenarioStep struct {
	ID  string `json:"id,omitempty"`
	Cue string `json:"cue"`
	// Delay after the cue, a Go duration ("40s").
	Delay   string           `json:"delay,omitempty"`
	Actions []ScenarioAction `json:"actions,omitempty"`
	// End: the scenario ends once its actions have run.
	End bool `json:"end,omitempty"`
}

// ScenarioAction is one thing a step does. Do:
//
//	spawn      Who appears now
//	push       Who is cleared to push back
//	goAround   Who is sent around
//	hold       Who holds (On true) or leaves the hold (On false)
//	runway     Runway ("12/30" or an end) closed (On true) or open again (On false)
//	say        to Who, Kind: cancelTakeoff | holdPosition | goAround
//	camera     Shots on the cast (the camera's moves)
//
// and anything else is the host's (ScenarioHooks.Action).
type ScenarioAction struct {
	Do   string `json:"do"`
	Who  string `json:"who,omitempty"`
	Kind string `json:"kind,omitempty"`
	// Level: an emergency's call (mayday, panpan); "" by its kind.
	Level  string `json:"level,omitempty"`
	On     *bool  `json:"on,omitempty"`
	Runway string `json:"runway,omitempty"`
	// Use: Runway becomes the runway in use.
	Use   bool        `json:"use,omitempty"`
	Fix   string      `json:"fix,omitempty"`
	Shots []SceneShot `json:"shots,omitempty"`
	// Args are the host's, passed on untouched.
	Args map[string]string `json:"args,omitempty"`
}

// ScenarioHooks are the host's part of a scenario, each called outside the
// World's locks; Cue from the scenario's one goroutine.
type ScenarioHooks struct {
	// Cue answers a cue the World does not know: known false ends the
	// scenario with an error.
	Cue func(cue string) (known, met bool)
	// Action runs an action the World does not know.
	Action func(a ScenarioAction) (known bool, err error)
	// Event reports what happened.
	Event func(ScenarioEvent)
}

// ScenarioEvent is a step of a scenario run: Kind "cast" (Role appeared as
// Tail), "step" (step Step began its actions), "action" (Action done, or
// Err), "end" (Err why it ended early).
type ScenarioEvent struct {
	Kind   string    `json:"kind"`
	Step   int       `json:"step"`
	ID     string    `json:"id,omitempty"`
	Role   string    `json:"role,omitempty"`
	Tail   string    `json:"tail,omitempty"`
	Action string    `json:"action,omitempty"`
	Err    string    `json:"err,omitempty"`
	At     time.Time `json:"at"`
}

// ScenarioStatus is a scenario run now.
type ScenarioStatus struct {
	Name string `json:"name"`
	// Done: the steps whose actions have run, by index.
	Done  []int                         `json:"done"`
	Cast  map[string]ScenarioCastStatus `json:"cast"`
	Ended bool                          `json:"ended"`
	Err   string                        `json:"err,omitempty"`
}

// ScenarioCastStatus is a role's aircraft now ("" not there yet).
type ScenarioCastStatus struct {
	Tail  string `json:"tail,omitempty"`
	State string `json:"state,omitempty"`
}

// scenarioTick: how often the cues are looked at.
const scenarioTick = 200 * time.Millisecond

// ScenarioRun is a scenario playing.
type ScenarioRun struct {
	w  *World
	cc *controlCenter
	g  *airport.Graph
	sc Scenario
	h  ScenarioHooks

	start time.Time
	stop  chan struct{}
	once  sync.Once

	mu    sync.Mutex
	cast  map[string]*controlled
	track map[string]*castTrack // by role
	done  map[int]bool
	ids   map[string]bool // step IDs whose actions have run
	ended bool
	err   string
	// camera: the run set the camera to a scene, given back at the end.
	camera bool
}

// castTrack is what a cast aircraft has reached: each state, airborne and
// light once seen, so a cue holds once passed; top its greatest height
// above the runway, near its least distance from the airport and far its
// greatest; heard a call to or from it since the steps last began waiting.
type castTrack struct {
	seen      map[string]bool
	top       float64
	near, far float64
	heard     bool
	measured  bool
}

// scenarioRuns are the scenarios playing, for what is heard.
var scenarioRuns sync.Map // *ScenarioRun → struct{}

// scenarioHeard marks a call to or from an aircraft of a scenario.
func scenarioHeard(t traffic.Transmission) {
	scenarioRuns.Range(func(k, _ any) bool {
		k.(*ScenarioRun).heardCall(t.Callsign)
		return true
	})
}

// StartScenario plays sc at its airport with the host's hooks; it runs
// until its End step, its limit, or Stop.
func (w *World) StartScenario(sc Scenario, h ScenarioHooks) (*ScenarioRun, error) {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return nil, errors.New("scenario: not running")
	}
	sc.ICAO = strings.ToUpper(strings.TrimSpace(sc.ICAO))
	g, err := cc.graph(sc.ICAO)
	if err != nil {
		return nil, fmt.Errorf("scenario: %s: %w", sc.ICAO, err)
	}
	if err := sc.validate(h); err != nil {
		return nil, err
	}
	r := &ScenarioRun{w: w, cc: cc, g: g, sc: sc, h: h, start: cc.clock.Now(), stop: make(chan struct{}),
		cast: map[string]*controlled{}, track: map[string]*castTrack{}, done: map[int]bool{}, ids: map[string]bool{}}
	scenarioRuns.Store(r, struct{}{})
	cc.log.printf("scenario %s at %s: %d cast, %d steps", sc.Name, sc.ICAO, len(sc.Cast), len(sc.Steps))
	go r.run()
	return r, nil
}

// validate checks the cast and steps: roles once each, delays that parse,
// actions and roles the World knows (or a host to ask).
func (sc Scenario) validate(h ScenarioHooks) error {
	roles := map[string]bool{}
	for _, c := range sc.Cast {
		if c.Role == "" || roles[c.Role] {
			return fmt.Errorf("scenario: cast role %q empty or twice", c.Role)
		}
		if c.Kind != "departure" && c.Kind != "arrival" {
			return fmt.Errorf("scenario: %s: kind %q, want departure or arrival", c.Role, c.Kind)
		}
		roles[c.Role] = true
	}
	for i, s := range sc.Steps {
		if s.Delay != "" {
			if _, err := time.ParseDuration(s.Delay); err != nil {
				return fmt.Errorf("scenario: step %d: delay %q: %w", i+1, s.Delay, err)
			}
		}
		for _, a := range s.Actions {
			switch a.Do {
			case "spawn", "push", "goAround", "hold", "say":
				if !roles[a.Who] {
					return fmt.Errorf("scenario: step %d: %s: no cast role %q", i+1, a.Do, a.Who)
				}
			case "runway", "camera":
			default:
				if h.Action == nil {
					return fmt.Errorf("scenario: step %d: action %q unknown", i+1, a.Do)
				}
			}
		}
	}
	return nil
}

// Stop ends the scenario; its cast flies on as any of ours.
func (r *ScenarioRun) Stop() { r.end("") }

// Status is the scenario now.
func (r *ScenarioRun) Status() ScenarioStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := ScenarioStatus{Name: r.sc.Name, Cast: map[string]ScenarioCastStatus{}, Ended: r.ended, Err: r.err}
	for i := range r.sc.Steps {
		if r.done[i] {
			s.Done = append(s.Done, i)
		}
	}
	for _, c := range r.sc.Cast {
		cs := ScenarioCastStatus{}
		if it := r.cast[c.Role]; it != nil {
			it.mu.Lock()
			cs.Tail, cs.State = it.Tail, it.view.State
			it.mu.Unlock()
		}
		s.Cast[c.Role] = cs
	}
	return s
}

func (r *ScenarioRun) end(why string) {
	r.once.Do(func() {
		r.mu.Lock()
		r.ended, r.err = true, why
		cam := r.camera
		r.mu.Unlock()
		close(r.stop)
		scenarioRuns.Delete(r)
		if m := r.cameraMan(); cam && m != nil {
			m.mu.Lock()
			still := m.mode == "scene"
			m.mu.Unlock()
			if still {
				m.setMode("off", 0)
			}
		}
		if why == "" {
			r.cc.log.printf("scenario %s: ended after %s", r.sc.Name, r.cc.clock.Now().Sub(r.start).Round(time.Second))
		} else {
			r.cc.log.printf("scenario %s: ended: %s", r.sc.Name, why)
		}
		r.event(ScenarioEvent{Kind: "end", Err: why})
	})
}

func (r *ScenarioRun) event(e ScenarioEvent) {
	if r.h.Event == nil {
		return
	}
	e.At = r.cc.clock.Now()
	r.h.Event(e)
}

// run plays the scenario: the cast at their times, the steps as cued.
func (r *ScenarioRun) run() {
	limit := time.Duration(r.sc.LimitSec * float64(time.Second))
	if limit <= 0 {
		limit = 15 * time.Minute
	}
	spawned := map[string]bool{}
	fireAt := map[int]time.Time{} // cued steps, waiting out their delay
	tick := time.NewTicker(scenarioTick)
	defer tick.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-tick.C:
		}
		now := r.cc.clock.Now()
		if now.Sub(r.start) > limit {
			r.end("time is up")
			return
		}
		for _, c := range r.sc.Cast {
			if !spawned[c.Role] && c.AtSec >= 0 && now.Sub(r.start) >= time.Duration(c.AtSec*float64(time.Second)) {
				spawned[c.Role] = true
				go r.spawnCast(c)
			}
		}
		r.watch()
		for i, s := range r.sc.Steps {
			r.mu.Lock()
			done := r.done[i]
			r.mu.Unlock()
			if done {
				continue
			}
			if _, cued := fireAt[i]; !cued {
				met, err := r.cued(s.Cue)
				if err != nil {
					r.end(fmt.Sprintf("step %d: %v", i+1, err))
					return
				}
				if !met {
					continue
				}
				d, _ := time.ParseDuration(s.Delay)
				fireAt[i] = now.Add(d)
			}
			if now.Before(fireAt[i]) {
				continue
			}
			r.runStep(i, s, spawned)
			if s.End {
				r.end("")
				return
			}
		}
	}
}

// runStep runs step i's actions.
func (r *ScenarioRun) runStep(i int, s ScenarioStep, spawned map[string]bool) {
	r.cc.log.printf("scenario %s: step %d (%s)", r.sc.Name, i+1, s.Cue)
	r.event(ScenarioEvent{Kind: "step", Step: i, ID: s.ID})
	for _, a := range s.Actions {
		if a.Do == "spawn" {
			spawned[a.Who] = true
		}
		err := r.act(a)
		e := ScenarioEvent{Kind: "action", Step: i, ID: s.ID, Role: a.Who, Action: a.Do}
		if err != nil {
			e.Err = err.Error()
			r.cc.log.printf("scenario %s: step %d: %s %s: %v", r.sc.Name, i+1, a.Do, a.Who, err)
		}
		r.event(e)
	}
	r.mu.Lock()
	r.done[i] = true
	if s.ID != "" {
		r.ids[s.ID] = true
	}
	for _, t := range r.track {
		t.heard = false // "heard" counts from here for the steps still waiting
	}
	r.mu.Unlock()
}

// spawnCast brings c in, a few tries while its spawn point is in use.
func (r *ScenarioRun) spawnCast(c ScenarioCast) {
	for i := 0; i < 6; i++ {
		select {
		case <-r.stop:
			return
		default:
		}
		it, err := r.w.castSpawn(r.cc, r.g, c)
		if err == nil {
			r.mu.Lock()
			r.cast[c.Role] = it
			r.mu.Unlock()
			r.cc.log.printf("scenario %s: %s is %s", r.sc.Name, c.Role, it.Tail)
			r.event(ScenarioEvent{Kind: "cast", Role: c.Role, Tail: it.Tail})
			return
		}
		r.cc.log.printf("scenario %s: %s %s: %v", r.sc.Name, c.Role, c.Callsign, err)
		if !errors.Is(err, traffic.ErrSpawnBlocked) {
			r.event(ScenarioEvent{Kind: "cast", Role: c.Role, Err: err.Error()})
			return
		}
		select {
		case <-r.stop:
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// CastSpawn brings in an aircraft of a scenario's cast at icao now and
// returns its call sign: for a host that runs its steps itself.
func (w *World) CastSpawn(icao string, c ScenarioCast) (string, error) {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return "", errors.New("scenario: not running")
	}
	g, err := cc.graph(strings.ToUpper(icao))
	if err != nil {
		return "", err
	}
	it, err := w.castSpawn(cc, g, c)
	if err != nil {
		return "", err
	}
	return it.Tail, nil
}

func (w *World) castSpawn(cc *controlCenter, g *airport.Graph, c ScenarioCast) (*controlled, error) {
	if c.Kind != "departure" && c.Kind != "arrival" {
		return nil, fmt.Errorf("kind %q, want departure or arrival", c.Kind)
	}
	model := ""
	if c.Models != "" {
		model = cc.modelFor(c.Models)
	} else if t, ok := w.TitleFor(c.Type, c.Callsign); ok {
		model = t
	} else {
		model = cc.modelFor(c.Type)
	}
	stand := -1
	if c.Stand != "" {
		i, err := g.Layout.ParkingIndex(c.Stand)
		if err != nil {
			return nil, fmt.Errorf("stand %s: %w", c.Stand, err)
		}
		stand = i
	}
	// A call sign spawned again (the scenario played again): the old one goes.
	if old := cc.byTail(c.Callsign); old != nil && c.Callsign != "" {
		if err := cc.do(func() error { return cc.remove(old) }); err != nil {
			return nil, err
		}
	}
	req := SpawnRequest{Kind: c.Kind, ICAO: g.Layout.ICAO, Stand: stand, Model: model, Tail: c.Callsign, Tug: c.Tug, Procedure: c.Procedure,
		InjectApproach: c.Kind == "arrival"}
	if c.Kind == "departure" && c.PushAfterSec > 0 {
		req.pushAt = cc.clock.Now().Add(time.Duration(c.PushAfterSec * float64(time.Second)))
	}
	var it *controlled
	err := cc.do(func() (e error) { it, e = cc.spawn(g, req); return e })
	return it, err
}

// watch records what the cast has reached.
func (r *ScenarioRun) watch() {
	r.mu.Lock()
	cast := make(map[string]*controlled, len(r.cast))
	for k, v := range r.cast {
		cast[k] = v
	}
	r.mu.Unlock()
	for role, it := range cast {
		it.mu.Lock()
		v, h := it.view, it.heightFt
		it.mu.Unlock()
		nm := calc.HaversineNM(v.Position.Lat, v.Position.Lon, r.g.Layout.Latitude, r.g.Layout.Longitude)
		r.mu.Lock()
		t := r.track[role]
		if t == nil {
			t = &castTrack{seen: map[string]bool{}}
			r.track[role] = t
		}
		t.update(v, h, nm)
		r.mu.Unlock()
	}
}

func (t *castTrack) update(v ControlView, heightFt, nm float64) {
	if v.State != "" {
		t.seen[v.State] = true
	}
	if v.State == "departing" && !v.OnGround {
		t.seen["airborne"] = true
		t.top = max(t.top, heightFt)
	}
	for _, c := range v.Lights {
		if c != '.' {
			t.seen["lights:"+string(c)] = true
		}
	}
	if v.Position != (airport.LatLon{}) {
		if !t.measured {
			t.near, t.far, t.measured = nm, nm, true
		}
		t.near, t.far = min(t.near, nm), max(t.far, nm)
	}
}

// met reports whether what (a cue after its role) has come.
func (t *castTrack) met(what string) (bool, error) {
	switch {
	case what == "heard":
		return t.heard, nil
	case strings.HasPrefix(what, "height:"):
		ft, err := strconv.ParseFloat(strings.TrimPrefix(what, "height:"), 64)
		return err == nil && t.top >= ft, err
	case strings.HasPrefix(what, "distance:>"):
		nm, err := strconv.ParseFloat(strings.TrimPrefix(what, "distance:>"), 64)
		return err == nil && t.measured && t.far > nm, err
	case strings.HasPrefix(what, "distance:"):
		nm, err := strconv.ParseFloat(strings.TrimPrefix(what, "distance:"), 64)
		return err == nil && t.measured && t.near <= nm, err
	}
	return t.seen[what], nil
}

// heardCall marks a call to or from an aircraft of the cast.
func (r *ScenarioRun) heardCall(tail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for role, it := range r.cast {
		if it.Tail == tail && r.track[role] != nil {
			r.track[role].heard = true
		}
	}
}

// cued reports whether cue has come: the World's own, else the host's.
func (r *ScenarioRun) cued(cue string) (bool, error) {
	switch {
	case cue == "" || cue == "start":
		return true, nil
	case strings.HasPrefix(cue, "t:"):
		sec, err := strconv.ParseFloat(strings.TrimPrefix(cue, "t:"), 64)
		return err == nil && r.cc.clock.Now().Sub(r.start) >= time.Duration(sec*float64(time.Second)), err
	case strings.HasPrefix(cue, "after:"):
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.ids[strings.TrimPrefix(cue, "after:")], nil
	}
	if role, what, ok := strings.Cut(cue, ":"); ok && r.hasRole(role) {
		r.mu.Lock()
		defer r.mu.Unlock()
		t := r.track[role]
		if t == nil {
			return false, nil // not there yet
		}
		return t.met(what)
	}
	if r.h.Cue != nil {
		known, met := r.h.Cue(cue)
		if !known {
			return false, fmt.Errorf("cue %q unknown", cue)
		}
		return met, nil
	}
	return false, fmt.Errorf("cue %q unknown", cue)
}

func (r *ScenarioRun) hasRole(role string) bool {
	for _, c := range r.sc.Cast {
		if c.Role == role {
			return true
		}
	}
	return false
}

// CastCue reports whether cue (as a scenario's, without its role: "taxiing",
// "airborne", "height:3000", "distance:10") has come for one of ours now:
// for a host that runs its steps itself, looked at as it asks (a state
// passed between two looks is not seen; "heard" is not answered).
func (w *World) CastCue(tail, cue string) bool {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return false
	}
	it := cc.byTail(tail)
	if it == nil {
		return false
	}
	it.mu.Lock()
	v, h := it.view, it.heightFt
	it.mu.Unlock()
	g, err := cc.graph(it.ICAO)
	if err != nil {
		return false
	}
	t := &castTrack{seen: map[string]bool{}}
	t.update(v, h, calc.HaversineNM(v.Position.Lat, v.Position.Lon, g.Layout.Latitude, g.Layout.Longitude))
	met, _ := t.met(cue)
	return met
}

// act runs one action: the World's own, else the host's.
func (r *ScenarioRun) act(a ScenarioAction) error {
	r.mu.Lock()
	it := r.cast[a.Who]
	cast := map[string]string{}
	for role, c := range r.cast {
		cast[role] = c.Tail
	}
	r.mu.Unlock()
	switch a.Do {
	case "spawn":
		if it != nil {
			return nil // there already
		}
		for _, c := range r.sc.Cast {
			if c.Role == a.Who {
				go r.spawnCast(c)
			}
		}
		return nil
	case "camera":
		return r.playShots(a.Shots)
	}
	if known, err := r.w.act(r.sc.ICAO, a, cast); known {
		return err
	}
	if r.h.Action != nil {
		known, err := r.h.Action(a)
		if !known {
			return fmt.Errorf("action %q unknown", a.Do)
		}
		return err
	}
	return fmt.Errorf("action %q unknown", a.Do)
}

// Act runs a scenario action at icao (cast: role → call sign), for a host
// that runs its steps itself; "spawn" and "camera" are the runner's.
func (w *World) Act(icao string, a ScenarioAction, cast map[string]string) error {
	known, err := w.act(strings.ToUpper(icao), a, cast)
	if !known {
		return fmt.Errorf("action %q unknown", a.Do)
	}
	return err
}

// act is Act; known false for an action the World does not run.
func (w *World) act(icao string, a ScenarioAction, cast map[string]string) (bool, error) {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return true, errors.New("scenario: not running")
	}
	who := func() (*controlled, error) {
		tail := cast[a.Who]
		if tail == "" {
			return nil, fmt.Errorf("%s: not there", a.Who)
		}
		it := cc.byTail(tail)
		if it == nil {
			return nil, fmt.Errorf("%s: %s gone", a.Who, tail)
		}
		return it, nil
	}
	clear := func(action string) error {
		it, err := who()
		if err != nil {
			return err
		}
		it.mu.Lock()
		id := it.view.ID
		it.mu.Unlock()
		return w.Clear(id, action)
	}
	switch a.Do {
	case "push":
		return true, clear("pushback")
	case "goAround":
		return true, clear("goaround")
	case "hold":
		it, err := who()
		if err != nil {
			return true, err
		}
		if a.Fix != "" {
			return true, errors.New("hold: at a named fix not yet; its own STAR's hold")
		}
		action := "hold"
		if a.On != nil && !*a.On {
			action = "release"
		}
		return true, w.Approach(icao, it.Tail, action)
	case "runway":
		if a.Use {
			w.UseRunway(icao, a.Runway) // "" back to the wind
			return true, nil
		}
		if a.Runway == "" {
			return true, errors.New("runway: none named")
		}
		closed := a.On == nil || *a.On
		w.CloseRunway(icao, a.Runway, closed)
		return true, nil
	case "say":
		switch a.Kind {
		case "cancelTakeoff":
			return true, clear("abort")
		case "holdPosition":
			return true, clear("hold")
		case "goAround":
			return true, clear("goaround")
		}
		return true, fmt.Errorf("say %q not yet", a.Kind)
	case "emergency":
		return true, errors.New("emergency not yet")
	}
	return false, nil
}

// cameraMan is the live connection's camera (nil without).
func (r *ScenarioRun) cameraMan() *cameraMan {
	r.w.st.mu.Lock()
	defer r.w.st.mu.Unlock()
	return r.w.st.camera
}

// playShots films the cast: the camera taken for the scenario until it
// ends.
func (r *ScenarioRun) playShots(list []SceneShot) error {
	m := r.cameraMan()
	if m == nil || m.dir == nil {
		return errors.New("camera: none on this connection")
	}
	r.mu.Lock()
	cast := make(map[string]*controlled, len(r.cast))
	for k, v := range r.cast {
		cast[k] = v
	}
	r.mu.Unlock()
	shots := castShots(r.g, cast, list, r.sc.Name)
	if len(shots) == 0 {
		return errors.New("camera: no shot (the cast not there yet)")
	}
	m.mu.Lock()
	on := m.mode == "scene"
	m.mu.Unlock()
	if !on {
		if err := m.setMode("scene", 0); err != nil {
			return err
		}
		r.mu.Lock()
		r.camera = true
		r.mu.Unlock()
	}
	if subject := cast[list[0].Who]; subject != nil {
		m.lockAirport(subject)
	}
	m.dir.Play(shots...)
	return nil
}
