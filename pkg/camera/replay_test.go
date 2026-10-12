package camera

import (
	"math"
	"testing"
	"time"
)

// Every move listed, named and kinded.
func TestMoves(t *testing.T) {
	ms := Moves()
	if len(ms) != 19 {
		t.Fatalf("%d moves", len(ms))
	}
	for _, m := range ms {
		if m.Name == "" || m.Doc == "" || m.Func == nil {
			t.Errorf("move %+v", m)
		}
		if s := m.Func(5, Size{Span: 34, Length: 37}, 1, 6*time.Second); s.Length() != 6*time.Second {
			t.Errorf("%s: %v long", m.Name, s.Length())
		}
	}
}

// An object-relative point in the world: ahead along the heading, right
// across it, up; and a nose-up pitch lifts a point ahead.
func TestPlaced(t *testing.T) {
	o := ObjectPose{Lat: 50, Lon: 14, AltM: 300, Heading: 90}
	pose := func(uint32) (ObjectPose, bool) { return o, true }
	m := func(lat, lon float64) (n, e float64) {
		return (lat - 50) * mPerDegLat, (lon - 14) * mPerDegLat * math.Cos(50*math.Pi/180)
	}
	lat, lon, alt, ok := Placed(On(1, 0, 10, 100), pose)
	n, e := m(lat, lon)
	if !ok || math.Abs(e-100) > 0.1 || math.Abs(n) > 0.1 || math.Abs(alt-310) > 0.1 {
		t.Errorf("100 m ahead east: %.1f N, %.1f E, %.1f m", n, e, alt)
	}
	lat, lon, _, _ = Placed(On(1, 50, 0, 0), pose)
	if n, e := m(lat, lon); math.Abs(n+50) > 0.1 || math.Abs(e) > 0.1 {
		t.Errorf("50 m right of east-facing: %.1f N, %.1f E (want 50 S)", n, e)
	}
	o.Pitch = 10
	_, _, alt, _ = Placed(On(1, 0, 0, 100), pose)
	if math.Abs(alt-(300+100*math.Sin(10*math.Pi/180))) > 0.1 {
		t.Errorf("pitched 10°: %.1f m", alt)
	}
	if _, _, _, ok := Placed(Point{Frame: Eyepoint}, pose); ok {
		t.Error("an eyepoint point placed")
	}
	ws := WorldPoses(Flyover(1, Size{}, 1, 2*time.Second), 500*time.Millisecond, func(time.Duration, uint32) (ObjectPose, bool) { return o, true })
	if len(ws) != 5 || ws[4].T != 2*time.Second {
		t.Errorf("world poses %d, last at %v", len(ws), ws[len(ws)-1].T)
	}
}

// A film on the replay's clock: the cued shot at its time, held between,
// over past the end; a seek back plays the earlier shot again.
func TestFollow(t *testing.T) {
	a := Hold("a", Pose{Eye: At(50, 14, 400), Target: At(50, 14.01, 300)}, 10*time.Second)
	b := Hold("b", Pose{Eye: At(51, 14, 400), Target: At(51, 14.01, 300)}, 10*time.Second)
	tl := NewTimeline(Cue{At: 545 * time.Second, Shot: a}, Cue{At: 600 * time.Second, Shot: b})
	if _, s, ok := tl.PoseAt(550 * time.Second); !ok || s.Name() != "a" {
		t.Fatalf("at 550 s: %v %v", s, ok)
	}
	if _, _, ok := tl.PoseAt(570 * time.Second); ok {
		t.Error("between shots: a pose")
	}
	f := &fakeAPI{}
	d := NewDirector(f, "test")
	clock := 550 * time.Second
	d.Follow(tl, func() time.Duration { return clock })
	d.Tick(time.Now()) // asks for the camera
	d.mu.Lock()
	d.acquired = true
	d.mu.Unlock()
	if err := d.Tick(time.Now()); err != nil || len(f.set) == 0 || f.set[len(f.set)-1].Position.X != 50 {
		t.Fatalf("at 550 s: %v, %d sets", err, len(f.set))
	}
	clock = 605 * time.Second
	d.Tick(time.Now())
	if f.set[len(f.set)-1].Position.X != 51 {
		t.Error("seeked to 605 s: not on b")
	}
	clock = 548 * time.Second // seek back
	d.Tick(time.Now())
	if f.set[len(f.set)-1].Position.X != 50 {
		t.Error("seeked back to 548 s: not on a")
	}
	clock = 700 * time.Second
	if err := d.Tick(time.Now()); err != ErrFilmEnded {
		t.Errorf("past the end: %v", err)
	}
}
