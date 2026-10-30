package camera

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// flight is a subject flying straight at speed over the eye's neighbourhood,
// sampled at period with noise on its position and on the sample's time.
type flight struct {
	now, start time.Time
	period     time.Duration
	rng        *rand.Rand
	sample     SpotSample
	have       bool
	next       time.Time
	// at is the true position at t into the flight.
	at func(t float64) (e, n, u float64)
}

const spotLat, spotLon, spotAlt = 50.0, 14.0, 300.0

func geo(e, n, u float64) ObjectPose {
	return ObjectPose{Lat: spotLat + n/mPerDegLat, Lon: spotLon + e/(mPerDegLat*math.Cos(spotLat*math.Pi/180)), AltM: spotAlt + u}
}

func (f *flight) advance(d time.Duration) {
	f.now = f.now.Add(d)
	if !f.now.Before(f.next) {
		t := f.now.Sub(f.start).Seconds()
		e, n, u := f.at(t)
		noise := func() float64 { return (f.rng.Float64()*2 - 1) * 0.5 }
		f.sample = SpotSample{Pose: geo(e+noise(), n+noise(), u+noise()), At: f.now}
		f.have = true
		f.next = f.next.Add(f.period + time.Duration(f.rng.Intn(40))*time.Millisecond)
	}
}

// angleTo is the bearing and elevation from the eye (0, 0, 0) to a world point,
// degrees.
func angleTo(p Point) (bearing, elev float64) {
	e := (p.Lon - spotLon) * mPerDegLat * math.Cos(spotLat*math.Pi/180)
	n := (p.Lat - spotLat) * mPerDegLat
	u := p.AltM - spotAlt
	return math.Atan2(e, n) * 180 / math.Pi, math.Atan2(u, math.Hypot(e, n)) * 180 / math.Pi
}

// TestSpotAimSmooth: samples once a second, noisy, on a long lens: the aim
// stays on a straight flight and turns evenly from frame to frame, where
// following the samples themselves would step 70 m each second.
func TestSpotAimSmooth(t *testing.T) {
	start := time.Unix(1000, 0)
	f := &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(1)),
		at: func(t float64) (float64, float64, float64) { return 200, -2000 + 70*t, 50 }}
	s := Spot{Name: "s", Eye: At(spotLat, spotLon, spotAlt), Share: 0.35}
	shot := s.Track(7, 36, time.Minute, func() (SpotSample, bool) { return f.sample, f.have }, func() time.Time { return f.now })

	const frame = time.Second / 60
	var was float64
	var worstStep, worstMiss float64
	for i := 0; i < 50*60; i++ {
		f.advance(frame)
		p := shot.PoseAt(0)
		if p.Target.Frame != World || p.Eye.Frame != World {
			t.Fatalf("frames %v %v", p.Eye.Frame, p.Target.Frame)
		}
		b, _ := angleTo(p.Target)
		tt := f.now.Sub(start).Seconds()
		e, n, _ := f.at(tt)
		truth := math.Atan2(e, n) * 180 / math.Pi
		if i > 5*60 {
			step := math.Abs(math.Remainder(b-was, 360))
			want := 70 * 200 / (e*e + n*n) * 180 / math.Pi / 60 // true rate a frame
			worstStep = max(worstStep, math.Abs(step-want))
			// What is off, as a share of the frame.
			worstMiss = max(worstMiss, math.Abs(math.Remainder(b-truth, 360))/p.FovDeg)
		}
		was = b
	}
	// A second's samples would step 70 m: at 2 km with this lens some 0.3
	// frames at once.
	if worstStep > 0.02 {
		t.Errorf("aim stepped %.4f° off its rate in a frame", worstStep)
	}
	if worstMiss > 0.12 {
		t.Errorf("aim off by %.2f of the frame", worstMiss)
	}
}

// TestSpotLens: the lens keeps the aircraft at its share, inside the range,
// and does not jump.
func TestSpotLens(t *testing.T) {
	start := time.Unix(1000, 0)
	f := &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(2)),
		at: func(t float64) (float64, float64, float64) { return 100, -6000 + 70*t, 30 }}
	s := Spot{Eye: At(spotLat, spotLon, spotAlt), Share: 0.35, FovMin: 8, FovMax: 40}
	shot := s.Track(7, 36, 3*time.Minute, func() (SpotSample, bool) { return f.sample, f.have }, func() time.Time { return f.now })
	const frame = time.Second / 60
	was := 0.0
	for i := 0; i < 100*60; i++ {
		f.advance(frame)
		p := shot.PoseAt(0)
		if p.FovDeg < 8-1e-9 || p.FovDeg > 40+1e-9 {
			t.Fatalf("frame %d: fov %.2f outside the lens", i, p.FovDeg)
		}
		if was > 0 && math.Abs(p.FovDeg-was)/was > 0.01 {
			t.Fatalf("frame %d: fov jumped %.3f → %.3f", i, was, p.FovDeg)
		}
		was = p.FovDeg
		if i == 40*60 { // 3200 m away: 36/0.35 = 103 m wide, 1.84°: the long end
			if p.FovDeg > 9 {
				t.Errorf("far: fov %.2f, want the long end", p.FovDeg)
			}
		}
	}
	// 100 s on: 1000 m out, 103 m of frame is 5.9°, the long end again.
	// Close by: within the range and at the share.
	f2 := &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(3)),
		at: func(t float64) (float64, float64, float64) { return 150, 0, 0 }}
	shot = s.Track(7, 36, time.Minute, func() (SpotSample, bool) { return f2.sample, f2.have }, func() time.Time { return f2.now })
	var p Pose
	for range 60 {
		f2.advance(frame)
		p = shot.PoseAt(0)
	}
	want := 2 * math.Atan(36/0.35/(2*150)) * 180 / math.Pi
	if math.Abs(p.FovDeg-want) > 0.5 {
		t.Errorf("at 150 m fov %.2f, want %.2f", p.FovDeg, want)
	}
}

// TestSpotRail: on a rail the camera goes abeam the subject, within the rail,
// no faster than its speed and with no jump; fixed, it stays put.
func TestSpotRail(t *testing.T) {
	start := time.Unix(1000, 0)
	f := &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(4)),
		at: func(t float64) (float64, float64, float64) { return 120, -300 + 60*t, 0 }}
	end := At(spotLat+300/mPerDegLat, spotLon, spotAlt) // 300 m north
	s := Spot{Eye: At(spotLat, spotLon, spotAlt), RailEnd: &end, RailSpeed: 5}
	shot := s.Track(7, 36, 2*time.Minute, func() (SpotSample, bool) { return f.sample, f.have }, func() time.Time { return f.now })
	const frame = time.Second / 60
	north := func(p Pose) float64 { return (p.Eye.Lat - spotLat) * mPerDegLat }
	was, wasV := 0.0, 0.0
	for i := 0; i < 90*60; i++ {
		f.advance(frame)
		n := north(shot.PoseAt(0))
		if n < -1e-6 || n > 300+1e-6 {
			t.Fatalf("frame %d: %.1f m off the rail", i, n)
		}
		v := (n - was) * 60
		if i > 0 && (math.Abs(v) > 5.01 || math.Abs(v-wasV) > 0.05) {
			t.Fatalf("frame %d: %.2f m/s after %.2f", i, v, wasV)
		}
		was, wasV = n, v
	}
	if math.Abs(was-300) > 1 {
		t.Errorf("ended %.0f m along, want the end of the rail", was)
	}
	fixed := Spot{Eye: At(spotLat, spotLon, spotAlt)}.Track(7, 36, time.Minute, func() (SpotSample, bool) { return f.sample, f.have }, func() time.Time { return f.now })
	f.advance(frame)
	if p := fixed.PoseAt(0); p.Eye != At(spotLat, spotLon, spotAlt) {
		t.Errorf("fixed moved: %+v", p.Eye)
	}
}

// TestSpotCoast: samples stop (the simulator paused), the aim runs on no
// further than spotCoast; no sample at all, it aims at the object; LockAim
// always does.
func TestSpotCoast(t *testing.T) {
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }
	on := On(7, 0, spotLookUp, 0)
	var have bool
	var sm SpotSample
	feed := func() (SpotSample, bool) { return sm, have }
	s := Spot{Eye: At(spotLat, spotLon, spotAlt)}
	shot := s.Track(7, 36, time.Minute, feed, clock)
	if p := shot.PoseAt(0); p.Target != on {
		t.Errorf("no sample: target %+v", p.Target)
	}
	have = true
	sm = SpotSample{Pose: geo(100, 0, 0), At: now, MpsNorth: 50}
	shot.PoseAt(0)
	now = now.Add(20 * time.Second)
	a := shot.PoseAt(0).Target
	now = now.Add(20 * time.Second)
	b := shot.PoseAt(0).Target
	if a != b {
		t.Errorf("still moving with no samples: %+v %+v", a, b)
	}
	if n := (a.Lat - spotLat) * mPerDegLat; math.Abs(n-50*spotCoast.Seconds()) > 1 {
		t.Errorf("coasted %.0f m, want %.0f", n, 50*spotCoast.Seconds())
	}
	s.LockAim = true
	if p := s.Track(7, 36, time.Minute, feed, clock).PoseAt(0); p.Target != on {
		t.Errorf("locked: target %+v", p.Target)
	}
}
