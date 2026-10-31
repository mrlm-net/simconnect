package world

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A scene from the host (#1324): World.PlayScene takes the Scene itself,
// not the name of a file; progress comes as SceneEvents and on GET
// /api/camera.

// SceneEvent is what a scene playing has reached: Kind "start", "cue" (a
// beat's cue came, its delay still to run), "beat" (Beat began, numbered
// from 1), "shot" (Shot began), "radio" (a transmission), "end" (Err why, if it
// could not go on).
type SceneEvent struct {
	Kind    string  `json:"kind"`
	Scene   string  `json:"scene"`
	Beat    int     `json:"beat,omitempty"`
	Cue     string  `json:"cue,omitempty"`
	Shot    string  `json:"shot,omitempty"`
	Subject string  `json:"subject,omitempty"` // the aircraft on screen
	AtSec   float64 `json:"atSec"`             // since the scene began
	Err     string  `json:"err,omitempty"`
	// Kind "radio": a transmission. AtSec is when it starts, later than now when
	// it waits for the frequency; DurSec how long it takes to say. Speaker is the
	// call sign of a crew or the station, Callsign who a station talks to.
	Speaker   string  `json:"speaker,omitempty"`
	Callsign  string  `json:"callsign,omitempty"`
	Frequency string  `json:"frequency,omitempty"`
	Text      string  `json:"text,omitempty"`
	DurSec    float64 `json:"durSec,omitempty"`
	Scripted  bool    `json:"scripted,omitempty"`
	Muted     bool    `json:"muted,omitempty"`
}

// SceneProgress is the scene playing, on GET /api/camera.
type SceneProgress struct {
	Name       string  `json:"name"`
	Beat       int     `json:"beat"` // the beat begun, from 1; 0 before the first
	Beats      int     `json:"beats"`
	Cue        string  `json:"cue,omitempty"` // the cue of the beat begun
	ElapsedSec float64 `json:"elapsedSec"`
}

// validLines checks a scene's lines: each said by someone, on a frequency, and
// none over another on the same one.
func validLines(sc Scene, roles map[string]bool) error {
	ends := map[string]float64{}
	lines := slices.Clone(sc.Lines)
	slices.SortStableFunc(lines, func(a, b SceneLine) int { return cmp.Compare(a.AtSec, b.AtSec) })
	for _, l := range lines {
		switch {
		case l.Who == "":
			return fmt.Errorf("scene: line %q: nobody says it", l.Text)
		case l.Text == "":
			return fmt.Errorf("scene: line of %s: no text", l.Who)
		case l.AtSec < 0:
			return fmt.Errorf("scene: line %q: at %.0f s", l.Text, l.AtSec)
		case l.Frequency != "" && !isFrequency(l.Frequency):
			return fmt.Errorf("scene: line %q: frequency %q", l.Text, l.Frequency)
		case l.Frequency == "" && !roles[l.Who]:
			return fmt.Errorf("scene: line %q: station %s without a frequency", l.Text, l.Who)
		}
		if l.Frequency == "" {
			continue
		}
		if end := ends[l.Frequency]; l.AtSec < end {
			return fmt.Errorf("scene: line %q at %.1f s: the one before on %s lasts until %.1f s", l.Text, l.AtSec, l.Frequency, end)
		}
		ends[l.Frequency] = l.AtSec + traffic.SpeakingTime(l.Text).Seconds()
	}
	return nil
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
		if isMoment(event) && role != "" && (role == "player" || role == strings.ToUpper(role)) {
			return nil // the player, or any aircraft by its call sign
		}
		if !roles[role] || event == "" {
			return fmt.Errorf("scene: %s: cue %q: want start, t:SECONDS or ROLE:EVENT of the cast", what, c)
		}
		return nil
	}
	if err := validLines(sc, roles); err != nil {
		return err
	}
	if sc.Local != "" {
		if _, err := localSeconds(sc.Local); err != nil {
			return fmt.Errorf("scene: %w", err)
		}
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
			if s.From == "" && s.Spot == "" && !moves[s.Move] {
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
	e.Scene = r.sc.Name
	if e.Kind != "radio" {
		e.AtSec = time.Since(r.start).Seconds()
	}
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

// lineLeadSec: a line takes its frequency this long before it starts, so that what
// else is said on it is over by then or waits.
const lineLeadSec = 15.0

// sayLines says the scene's lines to the timetable.
func (r *sceneRun) sayLines(icao string) {
	lines := slices.Clone(r.sc.Lines)
	slices.SortStableFunc(lines, func(a, b SceneLine) int { return cmp.Compare(a.AtSec, b.AtSec) })
	tails := map[string]string{}
	for _, c := range r.sc.Cast {
		tails[c.Role] = c.Tail
	}
	cc := r.m.cc
	held := 0
	for said := 0; said < len(lines); {
		r.m.mu.Lock()
		on := r.m.mode == "scene" && r.m.scene == r
		r.m.mu.Unlock()
		if !on {
			return
		}
		sec := time.Since(r.start).Seconds()
		for ; held < len(lines) && sec >= lines[held].AtSec-lineLeadSec; held++ {
			if l := lines[held]; !r.sc.Mute && l.Frequency != "" {
				until := time.Duration((l.AtSec - sec) * float64(time.Second))
				cc.radio.Occupy(icao, l.Frequency, cc.clock.Now().Add(until+traffic.SpeakingTime(l.Text)+time.Second))
			}
		}
		for ; said < len(lines) && sec >= lines[said].AtSec; said++ {
			l := lines[said]
			tx := traffic.Transmission{Frequency: l.Frequency, Text: l.Text}
			if tail, ok := tails[l.Who]; ok {
				tx.Pilot, tx.Callsign = true, tail
				if tx.Frequency == "" {
					r.m.mu.Lock()
					it := r.cast[l.Who]
					r.m.mu.Unlock()
					if it != nil {
						it.mu.Lock()
						tx.Frequency = it.view.Frequency
						it.mu.Unlock()
					}
				}
			} else {
				tx.Controller, tx.Callsign = l.Who, l.To
			}
			cc.radio.Script(icao, tx)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// localSeconds is a time of day "HH:MM" in seconds since midnight.
func localSeconds(s string) (float64, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("time of day %q: want HH:MM", s)
	}
	return float64(t.Hour()*3600 + t.Minute()*60), nil
}

// sceneTime sets the simulator's clock to local and gives what puts it back to
// where it would have been by then; nil if it could not. Measured live (MSFS 2024,
// LKPR, 2026-10-31): ZULU_HOURS_SET takes effect within the second and the part of
// the day follows; our traffic runs on its own clock and takes no notice.
func (m *cameraMan) sceneTime(local string, logf func(string, ...any)) func() {
	sec, err := localSeconds(local)
	if err != nil || m.simTime == nil || m.setZulu == nil {
		return nil
	}
	zulu, now, ok := m.simTime()
	if !ok {
		logf("time of day: the simulator's not known yet")
		return nil
	}
	set := func(z float64) error {
		day := int(math.Mod(math.Mod(z, 86400)+86400, 86400))
		return m.setZulu(uint32(day/3600), uint32(day%3600/60))
	}
	if err := set(sec - (now - zulu)); err != nil {
		logf("time of day: %v", err)
		return nil
	}
	began := m.cc.clock.Now()
	return func() {
		if err := set(zulu + m.cc.clock.Now().Sub(began).Seconds()); err != nil {
			logf("time of day back: %v", err)
		}
	}
}

// sceneRadio tells the host of a transmission and says whether the scene mutes it.
func (m *cameraMan) sceneRadio(t traffic.Transmission) (muted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.scene
	if r == nil || m.mode != "scene" {
		return false
	}
	muted = r.sc.Mute && !t.Scripted
	speaker := t.Controller
	switch {
	case t.Pilot:
		speaker = t.Callsign
	case speaker == "":
		speaker = string(t.Position)
	}
	callsign := t.Callsign
	if t.Pilot {
		callsign = ""
	}
	r.emit(SceneEvent{Kind: "radio", AtSec: time.Since(r.start).Seconds() + t.At.Sub(m.cc.clock.Now()).Seconds(), Speaker: speaker, Callsign: callsign,
		Frequency: t.Frequency, Text: t.Text, DurSec: t.SpeakingTime().Seconds(), Scripted: t.Scripted, Muted: muted})
	return muted
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
