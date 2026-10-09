package flight

import (
	"sync"
	"time"
)

// Player is a Track's playback clock: where in the Track it is now, by the
// wall clock, with play, pause, seek and rate. Times are seconds from the
// Track's first sample. It applies nothing itself: an applier
// (UserReplay, Ghost) takes Sample each frame.
type Player struct {
	track *Track

	mu     sync.Mutex
	rate   float64
	paused bool
	at     float64   // track time at since
	since  time.Time // wall clock of at
}

// NewPlayer returns a Player at the start of t, paused, at real time.
func NewPlayer(t *Track) *Player {
	return &Player{track: t, rate: 1, paused: true}
}

// Track is the Track played.
func (p *Player) Track() *Track { return p.track }

// timeLocked is the track time at now; p.mu held.
func (p *Player) timeLocked(now time.Time) float64 {
	t := p.at
	if !p.paused && !p.since.IsZero() {
		t += now.Sub(p.since).Seconds() * p.rate
	}
	return min(max(t, 0), p.track.Duration())
}

// Time is the track time at now, 0 … Duration.
func (p *Player) Time(now time.Time) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.timeLocked(now)
}

// Play plays on from where it is.
func (p *Player) Play(now time.Time) {
	p.mu.Lock()
	p.at, p.since, p.paused = p.timeLocked(now), now, false
	p.mu.Unlock()
}

// Pause holds it where it is.
func (p *Player) Pause(now time.Time) {
	p.mu.Lock()
	p.at, p.since, p.paused = p.timeLocked(now), now, true
	p.mu.Unlock()
}

// Paused reports whether it is held.
func (p *Player) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// Seek moves it to track time t (clamped to the Track).
func (p *Player) Seek(t float64, now time.Time) {
	p.mu.Lock()
	p.at, p.since = min(max(t, 0), p.track.Duration()), now
	p.mu.Unlock()
}

// SetRate plays at rate times real time from now (0.5 half speed, 2
// double; 0 or less: 1).
func (p *Player) SetRate(rate float64, now time.Time) {
	if rate <= 0 {
		rate = 1
	}
	p.mu.Lock()
	p.at, p.since, p.rate = p.timeLocked(now), now, rate
	p.mu.Unlock()
}

// Sample is the aircraft at now (Track.At), and whether the end is reached.
func (p *Player) Sample(now time.Time) (Sample, bool) {
	p.mu.Lock()
	t := p.timeLocked(now)
	p.mu.Unlock()
	if len(p.track.Samples) == 0 {
		return Sample{}, true
	}
	s, _ := p.track.At(p.track.Samples[0].T + t)
	return s, t >= p.track.Duration()
}
