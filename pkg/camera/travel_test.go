package camera

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// local is p in metres east, north and up of the spot test's origin.
func local(p Point) vec3 {
	return vec3{(p.Lon - spotLon) * mPerDegLat * math.Cos(spotLat*math.Pi/180), (p.Lat - spotLat) * mPerDegLat, p.AltM - spotAlt}
}

func world(e, n, u float64) Point {
	g := geo(e, n, u)
	return At(g.Lat, g.Lon, g.AltM)
}

// travelTrace flies tr from a pose at from (local metres, looking at look)
// to a spot at the origin, the subject flying f, frame by frame, and checks
// what a viewer would see: no jump of the eye, no step in its speed, the
// cruise and the height kept, the view turning slowly, the lens gliding.
// It gives the last pose and the cruise.
func travelTrace(t *testing.T, f *flight, from, look vec3, tr Travel, spot Spot) (*Aim, Pose, float64) {
	t.Helper()
	aim := NewAim(7, func() (SpotSample, bool) { return f.sample, f.have }, func() time.Time { return f.now })
	f.advance(0)
	tr.From = Pose{Eye: world(from.e, from.n, from.u), Target: world(look.e, look.n, look.u), FovDeg: 40}
	shot := spot.TravelTo(aim, 36, tr)
	speed := tr.SpeedMps
	if speed == 0 {
		speed = DefaultTravelSpeed
	}
	const frame = time.Second / 60
	if p := shot.PoseAt(0); local(p.Eye).sub(from).len() > 0.01 {
		t.Fatalf("starts %.2f m from the camera", local(p.Eye).sub(from).len())
	}
	var was, eye vec3
	var wasV, wasHead, wasPitch, wasRate, wasFov, top, fastest float64
	var p Pose
	n := int(shot.Length() / frame)
	if n < 10 {
		t.Fatalf("travel of %v", shot.Length())
	}
	for i := 0; i <= n; i++ {
		at := time.Duration(i) * frame
		if i == n {
			at = shot.Length()
		}
		if i > 0 {
			f.advance(frame)
		}
		p = shot.PoseAt(at)
		eye = local(p.Eye)
		tg := local(p.Target)
		head, pitch, _ := viewOf(eye, tg)
		if i > 0 {
			v := eye.sub(was).len() * 60
			if i < n && v > speed*1.02 {
				t.Fatalf("frame %d: %.2f m/s, cruise %.1f", i, v, speed)
			}
			if i > 1 && i < n && math.Abs(v-wasV) > 0.12 {
				t.Fatalf("frame %d: speed stepped %.2f → %.2f m/s", i, wasV, v)
			}
			rate := math.Abs(wrap180(head-wasHead)) * 60
			if rate > 10 || math.Abs(pitch-wasPitch)*60 > 10 {
				t.Fatalf("frame %d: view turns %.1f°/s heading, %.1f°/s pitch", i, rate, math.Abs(pitch-wasPitch)*60)
			}
			if i > 1 && math.Abs(rate-wasRate) > 0.6 {
				t.Fatalf("frame %d: turn rate stepped %.2f → %.2f °/s", i, wasRate, rate)
			}
			if math.Abs(p.FovDeg-wasFov) > 0.15 {
				t.Fatalf("frame %d: lens jumped %.2f → %.2f", i, wasFov, p.FovDeg)
			}
			wasV, wasRate, fastest = v, rate, max(fastest, v)
		}
		if eye.u < min(from.u, 0)-0.01 {
			t.Fatalf("frame %d: %.1f m, below both ends", i, eye.u)
		}
		top = max(top, eye.u)
		was, wasHead, wasPitch, wasFov = eye, head, pitch, p.FovDeg
	}
	if fastest < 0.9*speed {
		t.Errorf("cruised at %.1f m/s, want about %.1f", fastest, speed)
	}
	if eye.len() > 1e-6 {
		t.Errorf("ends %.3f m from the spot's eye", eye.len())
	}
	margin := tr.MarginM
	if margin == 0 {
		margin = DefaultTravelMargin
	}
	if top < max(from.u, 0)+margin-0.01 {
		t.Errorf("cruised %.1f m up, want %.1f", top, max(from.u, 0)+margin)
	}
	return aim, p, top
}

// TestTravelSmooth (#1474): every move of a travel flies with no jump or
// speed step, cruising at its speed over the higher end plus the margin,
// turning slowly, and ends at the spot's eye.
func TestTravelSmooth(t *testing.T) {
	start := time.Unix(1000, 0)
	parked := func() *flight {
		return &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(5)),
			at: func(float64) (float64, float64, float64) { return 150, 250, 0 }}
	}
	taxiing := func() *flight {
		return &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(6)),
			at: func(t float64) (float64, float64, float64) { return 120, -500 + 8*t, 0 }}
	}
	spot := Spot{Name: "s", Eye: At(spotLat, spotLon, spotAlt)}
	from := vec3{-50, -600, 6}
	for _, c := range []struct {
		move  TravelMove
		f     func() *flight
		speed float64
	}{
		{TravelGlide, parked, 0}, {TravelSlide, parked, 0}, {TravelRise, parked, 0},
		{TravelPushIn, parked, 10}, {TravelDolly, taxiing, 20}, {TravelAuto, taxiing, 0},
	} {
		name := string(c.move)
		if name == "" {
			name = "auto"
		}
		t.Run(name, func(t *testing.T) {
			_, _, top := travelTrace(t, c.f(), from, vec3{-50, -400, 0}, Travel{Move: c.move, SpeedMps: c.speed}, spot)
			if c.move == TravelRise && top < 6+DefaultTravelMargin+10-0.01 {
				t.Errorf("rise cruised only %.1f m up", top)
			}
		})
	}
}

// TestTravelLength: the way at the cruise with a ramp at each end; a view
// with far to turn makes it longer.
func TestTravelLength(t *testing.T) {
	start := time.Unix(1000, 0)
	f := &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(7)),
		at: func(float64) (float64, float64, float64) { return 0, 300, 0 }}
	aim := NewAim(7, func() (SpotSample, bool) { return f.sample, f.have }, func() time.Time { return f.now })
	f.advance(0)
	spot := Spot{Name: "s", Eye: At(spotLat, spotLon, spotAlt)}
	// 600 m south, already looking north at the subject.
	shot := spot.TravelTo(aim, 36, Travel{Move: TravelGlide, From: Pose{Eye: world(0, -600, 0), Target: world(0, 300, 2)}})
	if got, want := shot.Length().Seconds(), 600/DefaultTravelSpeed+TravelRamp.Seconds(); got < want || got > want*1.02 {
		t.Errorf("travel %.1f s, want %.1f", got, want)
	}
	// Looking the other way: the turn takes longer than the way.
	shot = spot.TravelTo(aim, 36, Travel{Move: TravelGlide, From: Pose{Eye: world(0, -100, 0), Target: world(0, -900, 2)}})
	if got, want := shot.Length().Seconds(), 180*1.875/TravelTurnRate; got < want {
		t.Errorf("travel with a U-turn %.1f s, want at least %.1f", got, want)
	}
}

// TestTravelJoin: the spot after a travel on the same aim starts where the
// travel ended, eye, aim and lens: no cut. The spot taken again joins too.
func TestTravelJoin(t *testing.T) {
	start := time.Unix(1000, 0)
	f := &flight{now: start, start: start, next: start, period: time.Second, rng: rand.New(rand.NewSource(8)),
		at: func(t float64) (float64, float64, float64) { return 100, -800 + 10*t, 0 }}
	end := At(spotLat+400/mPerDegLat, spotLon, spotAlt)
	spot := Spot{Name: "s", Eye: At(spotLat, spotLon, spotAlt), RailEnd: &end}
	aim, last, _ := travelTrace(t, f, vec3{-300, -300, 8}, vec3{0, -900, 0}, Travel{}, spot)
	const frame = time.Second / 60
	follow := spot.Follow(aim, 36, time.Minute, true)
	f.advance(frame)
	p := follow.PoseAt(0)
	if d := local(p.Eye).sub(local(last.Eye)).len(); d > 0.01 {
		t.Errorf("eye jumped %.2f m at the join", d)
	}
	if d := local(p.Target).sub(local(last.Target)).len(); d > 1 {
		t.Errorf("aim jumped %.2f m at the join", d)
	}
	if math.Abs(p.FovDeg-last.FovDeg) > 0.15 {
		t.Errorf("lens jumped %.2f → %.2f at the join", last.FovDeg, p.FovDeg)
	}
	// On along the rail, then the spot again, joined.
	for range 30 * 60 {
		f.advance(frame)
		last = follow.PoseAt(0)
	}
	again := spot.Follow(aim, 36, time.Minute, true)
	f.advance(frame)
	p = again.PoseAt(0)
	if d := local(p.Eye).sub(local(last.Eye)).len(); d > 0.2 {
		t.Errorf("taken again: eye jumped %.2f m", d)
	}
	if math.Abs(p.FovDeg-last.FovDeg) > 0.15 {
		t.Errorf("taken again: lens jumped %.2f → %.2f", last.FovDeg, p.FovDeg)
	}
}

// TestTravelAuto: the move picked from the subject.
func TestTravelAuto(t *testing.T) {
	a := vec3{0, -600, 0}
	for _, c := range []struct {
		subj, vel vec3
		want      TravelMove
	}{
		{vec3{100, -300, 0}, vec3{0, 8, 0}, TravelDolly},
		{vec3{60, -300, 0}, vec3{}, TravelSlide},
		{vec3{20, 300, 0}, vec3{}, TravelPushIn},
		{vec3{900, 0, 0}, vec3{}, TravelGlide},
	} {
		if got := autoTravel(a, c.subj, c.vel, 0, 1, 600); got != c.want {
			t.Errorf("subject %+v moving %+v: %q, want %q", c.subj, c.vel, got, c.want)
		}
	}
}

// TestPan: a look round turns no faster than PanRate, the eye still.
func TestPan(t *testing.T) {
	eye := world(0, 0, 10)
	from, to := world(0, 500, 0), world(-500, -100, 0)
	d := PanLength(eye, from, to, 10*time.Second)
	shot := Pan("look", eye, from, to, 40, 20, d)
	const frame = time.Second / 60
	var was float64
	for i := 0; i <= int(d/frame); i++ {
		p := shot.PoseAt(time.Duration(i) * frame)
		if p.Eye != eye {
			t.Fatal("the eye moved")
		}
		h, _, _ := viewOf(local(eye), local(p.Target))
		if i > 0 && math.Abs(wrap180(h-was))*60 > PanRate+0.01 {
			t.Fatalf("frame %d: %.2f°/s", i, math.Abs(wrap180(h-was))*60)
		}
		was = h
	}
	if p := shot.PoseAt(d); local(p.Target).sub(local(to)).len() > 0.01 || p.FovDeg != 20 {
		t.Errorf("ends at %+v, fov %.1f", p.Target, p.FovDeg)
	}
}
