package world

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// A scene from the host (#1324): World.PlayScene takes the Scene itself,
// not the name of a file; progress comes as SceneEvents and on GET
// /api/camera.

// SceneEvent is what a scene playing has reached: Kind "start", "cue" (a
// beat's cue came, its delay still to run), "beat" (Beat began, numbered
// from 1), "shot" (Shot began), "end" (Err why, if it could not go on).
type SceneEvent struct {
	Kind    string  `json:"kind"`
	Scene   string  `json:"scene"`
	Beat    int     `json:"beat,omitempty"`
	Cue     string  `json:"cue,omitempty"`
	Shot    string  `json:"shot,omitempty"`
	Subject string  `json:"subject,omitempty"` // the aircraft on screen
	AtSec   float64 `json:"atSec"`             // since the scene began
	Err     string  `json:"err,omitempty"`
}

// SceneProgress is the scene playing, on GET /api/camera.
type SceneProgress struct {
	Name       string  `json:"name"`
	Beat       int     `json:"beat"` // the beat begun, from 1; 0 before the first
	Beats      int     `json:"beats"`
	Cue        string  `json:"cue,omitempty"` // the cue of the beat begun
	ElapsedSec float64 `json:"elapsedSec"`
}

// sceneEventQueue is how many events wait for a slow host; more are dropped,
// as the camera never waits for it.
const sceneEventQueue = 256

// OnSceneEvent has f hear the scenes' events, in order, off the camera's
// goroutine (nil: nobody). It may be slow; it must not wait for the scene.
func (w *World) OnSceneEvent(f func(SceneEvent)) {
	if f == nil {
		w.sceneEvents.Store(nil)
		return
	}
	w.sceneEvents.Store(&f)
}

// PlayScene plays sc at airport icao. Refused if it does not validate, with
// no camera on this connection or with a scene already playing.
func (w *World) PlayScene(icao string, sc Scene) error {
	_, err := w.Do(http.MethodPost, "/api/camera/scene?icao="+url.QueryEscape(icao), sc)
	return err
}

// StopScene stops the scene playing and gives the camera back; nothing
// playing, it does nothing.
func (w *World) StopScene() error {
	var v cameraView
	if err := w.Get("/api/camera", &v); err != nil {
		return err
	}
	if v.Mode != "scene" {
		return nil
	}
	_, err := w.Do(http.MethodPost, "/api/camera", map[string]string{"mode": "off"})
	return err
}

// validate reports what is wrong with a scene from a host: an unknown role
// or move would only be found, as a shot missing, while it plays.
func (sc Scene) validate() error {
	roles := map[string]bool{}
	for _, c := range sc.Cast {
		switch {
		case c.Role == "":
			return errors.New("scene: a cast member without a role")
		case roles[c.Role]:
			return fmt.Errorf("scene: role %q twice", c.Role)
		case c.Kind != "departure" && c.Kind != "arrival":
			return fmt.Errorf("scene: %s: kind %q, want departure or arrival", c.Role, c.Kind)
		case c.Tail == "":
			return fmt.Errorf("scene: %s: no tail", c.Role)
		}
		roles[c.Role] = true
	}
	cue := func(what, c string) error {
		if c == "" || c == "start" {
			return nil
		}
		if s, ok := strings.CutPrefix(c, "t:"); ok {
			if _, err := strconv.ParseFloat(s, 64); err != nil {
				return fmt.Errorf("scene: %s: cue %q: seconds", what, c)
			}
			return nil
		}
		role, event, _ := strings.Cut(c, ":")
		if !roles[role] || event == "" {
			return fmt.Errorf("scene: %s: cue %q: want start, t:SECONDS or ROLE:EVENT of the cast", what, c)
		}
		return nil
	}
	listen := func(what, l string) error {
		if l != "" && !roles[l] && !isFrequency(l) {
			return fmt.Errorf("scene: %s: listen %q: a role or a frequency", what, l)
		}
		return nil
	}
	if err := listen("scene", sc.Listen); err != nil {
		return err
	}
	for _, c := range sc.Cast {
		if c.After != "" {
			if err := cue(c.Role, c.After); err != nil {
				return err
			}
		}
	}
	moves := map[string]bool{}
	for _, name := range sceneMoveNames() {
		moves[name] = true
	}
	for i, b := range sc.Beats {
		what := fmt.Sprintf("beat %d", i+1)
		if err := cue(what, b.Cue); err != nil {
			return err
		}
		if err := listen(what, b.Listen); err != nil {
			return err
		}
		for _, s := range b.Shots {
			if !roles[s.Who] {
				return fmt.Errorf("scene: %s: shot of %q, not in the cast", what, s.Who)
			}
			if s.From == "" && !moves[s.Move] {
				return fmt.Errorf("scene: %s: no move %q", what, s.Move)
			}
		}
	}
	return nil
}

// event sends a scene's event to the host; the scene over, nothing more.
// Under m.mu.
func (r *sceneRun) emit(e SceneEvent) {
	if r.ended {
		return
	}
	e.Scene, e.AtSec = r.sc.Name, time.Since(r.start).Seconds()
	select {
	case r.events <- e:
	default:
	}
}

// event is emit outside m.mu.
func (r *sceneRun) event(e SceneEvent) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.emit(e)
}

// listenEvents has the host hear r's events until the scene ends.
func (r *sceneRun) listenEvents() {
	r.events = make(chan SceneEvent, sceneEventQueue)
	w := r.m.w
	go func() {
		for e := range r.events {
			if w == nil {
				continue
			}
			if f := w.sceneEvents.Load(); f != nil {
				(*f)(e)
			}
		}
	}()
}

// finish is the scene's last event, why if it could not go on.
func (r *sceneRun) finish(why string) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.emit(SceneEvent{Kind: "end", Err: why})
	if !r.ended {
		r.ended = true
		close(r.events)
	}
}

// progress is what the scene has reached. Under m.mu.
func (r *sceneRun) progress() *SceneProgress {
	return &SceneProgress{Name: r.sc.Name, Beat: r.beat, Beats: len(r.sc.Beats), Cue: r.cue, ElapsedSec: time.Since(r.start).Seconds()}
}
