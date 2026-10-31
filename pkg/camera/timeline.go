package camera

import (
	"cmp"
	"slices"
	"sync"
	"time"
)

// Cue is a shot placed on a film's timeline: it plays from At.
type Cue struct {
	At   time.Duration
	Shot Shot
}

// Timeline is a film of shots on a clock that is not the wall's, a
// replay's that pauses, seeks and changes rate: at any time on that clock
// the camera is where the shot cued for it says, so a shot cued at 545 s
// plays then even after a seek (Director.Follow).
type Timeline struct {
	cues []Cue
}

// NewTimeline is a film of cues (sorted by At; a cue's shot runs until its
// length or the next cue, whichever comes first).
func NewTimeline(cues ...Cue) *Timeline {
	c := slices.Clone(cues)
	slices.SortStableFunc(c, func(a, b Cue) int { return cmp.Compare(a.At, b.At) })
	return &Timeline{cues: c}
}

// PoseAt is the camera at clock: the shot cued then and its pose; false
// before the first cue, between shots and after the last (the camera holds
// or is given back).
func (tl *Timeline) PoseAt(clock time.Duration) (Pose, Shot, bool) {
	i, _ := slices.BinarySearchFunc(tl.cues, clock, func(c Cue, t time.Duration) int { return cmp.Compare(c.At, t) })
	if i < len(tl.cues) && tl.cues[i].At == clock {
		i++
	}
	i-- // the last cue at or before clock
	if i < 0 {
		return Pose{}, nil, false
	}
	c := tl.cues[i]
	into := clock - c.At
	if into > c.Shot.Length() {
		return Pose{}, nil, false
	}
	if w, ok := c.Shot.(*objectShot); ok && !w.ready() {
		return Pose{}, nil, false
	}
	return c.Shot.PoseAt(into), c.Shot, true
}

// OnObject is a shot on an object not known when the film is written, a replayed
// aircraft the simulator creates as the replay reaches it: on a Timeline, while
// object says 0 the camera holds as between shots, and the shot is made again
// when the object changes, after a seek brought the aircraft back.
func OnObject(length time.Duration, object func() uint32, shot func(o uint32) Shot) Shot {
	return &objectShot{length: length, object: object, shot: shot}
}

type objectShot struct {
	length time.Duration
	object func() uint32
	shot   func(o uint32) Shot

	mu  sync.Mutex
	o   uint32
	cur Shot
}

func (s *objectShot) Length() time.Duration { return s.length }

func (s *objectShot) ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o := s.object(); o != s.o || s.cur == nil && o != 0 {
		s.o, s.cur = o, nil
		if o != 0 {
			s.cur = s.shot(o)
		}
	}
	return s.cur != nil
}

// PoseAt: nothing while there is no object.
func (s *objectShot) PoseAt(t time.Duration) Pose {
	if !s.ready() {
		return Pose{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur.PoseAt(t)
}

func (s *objectShot) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == nil {
		return "on an object not there yet"
	}
	return s.cur.Name()
}

// End is when the last shot ends.
func (tl *Timeline) End() time.Duration {
	var end time.Duration
	for _, c := range tl.cues {
		end = max(end, c.At+c.Shot.Length())
	}
	return end
}

// Follow plays tl on clock (a replay's time, which may pause, seek or
// change rate) instead of the queued shots: at each Tick the camera is
// where tl says for clock(), holding its last pose between shots. Past
// tl's end, Tick reports ErrFilmEnded, for the host to Release. nil stops
// following.
func (d *Director) Follow(tl *Timeline, clock func() time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.film, d.clock = tl, clock
	if tl != nil {
		d.queue, d.cur = nil, nil
	}
}
