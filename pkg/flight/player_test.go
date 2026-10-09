package flight

import (
	"math"
	"testing"
	"time"
)

// TestPlayer: paused at the start; played, real time; paused it holds;
// seek, rate; the end is reported and not passed.
func TestPlayer(t *testing.T) {
	tr := &Track{}
	for i := range 11 {
		tr.Samples = append(tr.Samples, Sample{T: 100 + float64(i), AltFt: float64(i) * 100})
	}
	p := NewPlayer(tr)
	t0 := time.Unix(1000, 0)
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if !p.Paused() || p.Time(t0.Add(time.Hour)) != 0 {
		t.Fatal("not paused at the start")
	}
	p.Play(t0)
	if s, done := p.Sample(t0.Add(2500 * time.Millisecond)); !near(s.AltFt, 250) || done {
		t.Errorf("2.5 s in: alt %v done %v", s.AltFt, done)
	}
	p.Pause(t0.Add(3 * time.Second))
	if !near(p.Time(t0.Add(time.Minute)), 3) {
		t.Errorf("paused at %v, want 3", p.Time(t0.Add(time.Minute)))
	}
	p.Seek(8, t0.Add(time.Minute))
	p.SetRate(2, t0.Add(time.Minute))
	p.Play(t0.Add(time.Minute))
	if got := p.Time(t0.Add(time.Minute + 500*time.Millisecond)); !near(got, 9) {
		t.Errorf("at 2×: %v, want 9", got)
	}
	if s, done := p.Sample(t0.Add(2 * time.Minute)); !done || !near(s.AltFt, 1000) {
		t.Errorf("past the end: alt %v done %v", s.AltFt, done)
	}
}
