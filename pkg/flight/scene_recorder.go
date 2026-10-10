package flight

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

// SceneRecorder records the aircraft near the player into a Scene (#1012):
// the host gives it, about once a second, the objects near the player
// (nearest first) with who they are; each is recorded at a low rate from
// when it comes until it leaves, at most Max at once. It shares a
// Recorder (its 63 objects the hard cap) with the player's own recording,
// so the scene's times are the player's Track's clock.

// DefaultSceneMax and DefaultSceneEveryFrames: aircraft recorded at once,
// and a sample every this many frames (about 1 Hz at 60 fps).
const (
	DefaultSceneMax         = 40
	DefaultSceneEveryFrames = 60
)

// SceneObject is an aircraft near the player: its object and who it is
// (SceneAircraft without Key and Track; Key may be given, else one is
// made from the callsign or the object).
type SceneObject struct {
	ObjectID uint32
	Identity SceneAircraft
}

// SceneOptions tune a SceneRecorder; zero values take the defaults.
type SceneOptions struct {
	Max         int    // aircraft at once (DefaultSceneMax), never past what the Recorder has left
	EveryFrames uint32 // DefaultSceneEveryFrames
	Note        string
}

type sceneLive struct {
	a *SceneAircraft
}

// SceneRecorder is one scene being recorded.
type SceneRecorder struct {
	rec *Recorder
	opt SceneOptions

	mu      sync.Mutex
	started time.Time
	live    map[uint32]*sceneLive // by object
	done    []*SceneAircraft
	keys    map[string]int // keys given, for a fresh one on return
	stopped bool
}

// NewSceneRecorder records a scene on rec.
func NewSceneRecorder(rec *Recorder, o SceneOptions) *SceneRecorder {
	if o.Max <= 0 {
		o.Max = DefaultSceneMax
	}
	o.Max = min(o.Max, RecorderIDs)
	if o.EveryFrames == 0 {
		o.EveryFrames = DefaultSceneEveryFrames
	}
	return &SceneRecorder{rec: rec, opt: o, started: time.Now(), live: map[uint32]*sceneLive{}, keys: map[string]int{}}
}

// Update takes the aircraft near the player now, nearest first: those new
// are recorded (while under Max and the Recorder has room), those gone
// stop (their stretch kept in the scene). The player's own object is not
// one of them.
func (s *SceneRecorder) Update(near []SceneObject) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return fmt.Errorf("flight: scene stopped")
	}
	here := map[uint32]bool{}
	for _, o := range near {
		here[o.ObjectID] = true
	}
	for obj, l := range s.live {
		if !here[obj] {
			s.finishLocked(obj, l)
		}
	}
	var first error
	for _, o := range near {
		if _, ok := s.live[o.ObjectID]; ok || len(s.live) >= s.opt.Max {
			continue
		}
		a := o.Identity
		a.Key = s.keyLocked(a, o.ObjectID)
		if err := s.rec.Start(o.ObjectID, RecordOptions{Title: a.Title, Model: a.Type, Note: a.Key, EveryFrames: s.opt.EveryFrames}); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		s.live[o.ObjectID] = &sceneLive{a: &a}
	}
	return first
}

// keyLocked is a key for a: its own, else its callsign, else the object;
// one seen before gets "#2", "#3"…; s.mu held.
func (s *SceneRecorder) keyLocked(a SceneAircraft, obj uint32) string {
	k := a.Key
	if k == "" {
		k = a.Callsign
	}
	if k == "" {
		k = fmt.Sprintf("obj-%d", obj)
	}
	s.keys[k]++
	if n := s.keys[k]; n > 1 {
		k = fmt.Sprintf("%s#%d", k, n)
	}
	return k
}

// finishLocked stops obj's recording into the scene; s.mu held.
func (s *SceneRecorder) finishLocked(obj uint32, l *sceneLive) {
	delete(s.live, obj)
	if t := s.rec.Stop(obj); t != nil && len(t.Samples) > 0 {
		l.a.Track = t
		s.done = append(s.done, l.a)
	}
}

// Recording reports how many aircraft are recorded now.
func (s *SceneRecorder) Recording() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.live)
}

// Snapshot is the scene so far (the aircraft still recorded up to now).
func (s *SceneRecorder) Snapshot() *Scene {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := &Scene{Version: SceneVersion, Started: s.started, Note: s.opt.Note, Aircraft: slices.Clone(s.done)}
	for obj, l := range s.live {
		if t := s.rec.Snapshot(obj); t != nil && len(t.Samples) > 0 {
			a := *l.a
			a.Track = t
			sc.Aircraft = append(sc.Aircraft, &a)
		}
	}
	return sc
}

// Stop ends every recording and returns the Scene.
func (s *SceneRecorder) Stop() *Scene {
	s.mu.Lock()
	for obj, l := range s.live {
		s.finishLocked(obj, l)
	}
	s.stopped = true
	s.mu.Unlock()
	return s.Snapshot()
}
