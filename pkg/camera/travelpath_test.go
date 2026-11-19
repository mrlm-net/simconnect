package camera

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// aimNow is where a spot looks on aim's subject now (a little above it) and
// the subject's velocity, as a host app would feed a TravelPath.
func aimNow(aim *Aim) (Point, [3]float64) {
	aim.mu.Lock()
	defer aim.mu.Unlock()
	now, _ := aim.update()
	lat, lon, alt := aim.at(now)
	return At(lat, lon, alt+spotLookUp), [3]float64{aim.vel.e, aim.vel.n, aim.vel.u}
}

// viewAngle is the angle between the views from eye to a and to b, degrees.
func viewAngle(eye, a, b vec3) float64 {
	h0, p0, _ := viewOf(eye, a)
	h1, p1, _ := viewOf(eye, b)
	return angleBetween(h0, p0, h1, p1)
}

// TestTravelPathMatchesTravelTo: with the ends where the spot and its
// subject are, a TravelPath flies as Spot.TravelTo does, frame by frame:
// the eye within a centimetre, the view within 0.01°, the lens too, the
// same length. NewTravelPath flies as the Observer's travel.
func TestTravelPathMatchesTravelTo(t *testing.T) {
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
	from := Pose{Eye: world(-50, -600, 6), Target: world(-50, -400, 0), FovDeg: 40}
	for _, c := range []struct {
		name string
		move TravelMove
		f    func() *flight
		obs  *Observer
	}{
		{"glide", TravelGlide, parked, nil}, {"slide", TravelSlide, parked, nil}, {"rise", TravelRise, parked, nil},
		{"pushIn", TravelPushIn, parked, nil}, {"dolly", TravelDolly, taxiing, nil}, {"auto", TravelAuto, taxiing, nil},
		{"observer", TravelAuto, parked, &Observer{TravelMaxM: 1000, SpeedMps: 30}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := c.f()
			aim := NewAim(7, func() (SpotSample, bool) { return f.sample, f.have }, func() time.Time { return f.now })
			f.advance(0)
			look, vel := aimNow(aim)
			var shot Shot
			var path *TravelPath
			if c.obs != nil {
				way := c.obs.Plan(Leg{From: from, Have: true, To: spot, Aim: aim, Size: 36})
				if way.Kind != MoveTravel {
					t.Fatalf("planned %q", way.Kind)
				}
				shot = way.Shots[0]
				path = NewTravelPath(from, spot.Eye, look, *c.obs)
			} else {
				tr := Travel{From: from, Move: c.move, LookVelMps: vel}
				shot = spot.TravelTo(aim, 36, tr)
				path = tr.Path(spot.Eye, look)
			}
			if shot.Length() != path.Length() {
				t.Fatalf("length %v, TravelTo %v", path.Length(), shot.Length())
			}
			const frame = time.Second / 60
			n := int(shot.Length() / frame)
			for i := 0; i <= n+1; i++ {
				at := min(time.Duration(i)*frame, shot.Length())
				if i > 0 {
					f.advance(frame)
				}
				want := shot.PoseAt(at)
				look, _ = aimNow(aim)
				got := path.At(at, spot.Eye, look)
				eye := local(got.Eye)
				if d := eye.sub(local(want.Eye)).len(); d > 0.01 {
					t.Fatalf("frame %d: eye %.3f m off", i, d)
				}
				if a := viewAngle(eye, local(got.Target), local(want.Target)); a > 0.01 {
					t.Fatalf("frame %d: view %.4f° off", i, a)
				}
				fov := path.Lens(at, spot.Framing(36, local(look).sub(eye).len()))
				if math.Abs(fov-want.FovDeg) > 0.01 {
					t.Fatalf("frame %d: lens %.3f, TravelTo %.3f", i, fov, want.FovDeg)
				}
			}
		})
	}
}

// TestTravelPathLiveEye: the end's eye creeping along a rail as the drone
// flies there: no jump, the speed gliding, the step a frame at most the
// cruise's and the rail's, and the drone lands exactly on the eye where it
// is at the end.
func TestTravelPathLiveEye(t *testing.T) {
	const rail = 2.0 // m/s
	for _, c := range []struct {
		name   string
		dir    vec3
		toLook vec3
	}{
		{"ahead", vec3{0, 1, 0}, vec3{150, 250, 2}},
		{"back", vec3{0, -1, 0}, vec3{150, 250, 2}},
		{"across", vec3{1, 0, 0}, vec3{-150, 100, 2}},
		{"climbing", vec3{0.6, 0.6, 0.2}, vec3{150, 250, 2}},
	} {
		t.Run(c.name, func(t *testing.T) {
			from := vec3{-50, -600, 6}
			path := NewTravelPath(Pose{Eye: world(from.e, from.n, from.u), Target: world(-50, -400, 0), FovDeg: 40},
				world(0, 0, 0), world(c.toLook.e, c.toLook.n, c.toLook.u), Observer{})
			dir := vec3{}.add(c.dir, 1/c.dir.len())
			endAt := func(sec float64) vec3 { return vec3{}.add(dir, rail*sec) }
			const frame = time.Second / 60
			// The cruise, and the rail twice: as it moves, and the share of
			// how far it has gone growing with the way.
			top := path.speed + 2*rail
			n := int(path.Length() / frame)
			var was vec3
			var wasAt time.Duration
			var wasV float64
			for i := 0; i <= n+1; i++ {
				at := min(time.Duration(i)*frame, path.Length())
				end := endAt(at.Seconds())
				p := path.At(at, world(end.e, end.n, end.u), world(c.toLook.e, c.toLook.n, c.toLook.u))
				eye := local(p.Eye)
				if i == 0 && eye.sub(from).len() > 0.01 {
					t.Fatalf("starts %.2f m from the camera", eye.sub(from).len())
				}
				if dt := (at - wasAt).Seconds(); i > 0 && dt > 0 {
					v := eye.sub(was).len() / dt
					if v > top {
						t.Fatalf("frame %d: %.2f m/s, at most %.2f", i, v, top)
					}
					if i > 1 && math.Abs(v-wasV) > 0.12 {
						t.Fatalf("frame %d: speed stepped %.2f → %.2f m/s", i, wasV, v)
					}
					wasV = v
				}
				was, wasAt = eye, at
			}
			if d := was.sub(endAt(path.Length().Seconds())).len(); d > 1e-6 {
				t.Errorf("ends %.6f m from the eye", d)
			}
			if !path.Done(path.Length()) || path.Done(path.Length()-time.Millisecond) {
				t.Errorf("Done not at Length %v", path.Length())
			}
		})
	}
}

// TestTravelPathTurn: the view turns to the look no faster than
// TravelTurnRate and over at least 3 s, then stays on it.
func TestTravelPathTurn(t *testing.T) {
	for _, c := range []struct {
		name       string
		from, look vec3
	}{
		{"quarter", vec3{0, -600, 6}, vec3{300, -600, 0}},
		{"about", vec3{0, -600, 6}, vec3{0, -900, 0}},
		{"small", vec3{0, -600, 6}, vec3{20, 300, 2}},
	} {
		t.Run(c.name, func(t *testing.T) {
			toLook := world(0, 300, 2)
			path := Travel{From: Pose{Eye: world(c.from.e, c.from.n, c.from.u), Target: world(c.look.e, c.look.n, c.look.u)}, Move: TravelGlide}.
				Path(world(0, 0, 0), toLook)
			const frame = time.Second / 60
			var was vec3
			var turned time.Duration
			for i := 0; time.Duration(i)*frame <= path.Length(); i++ {
				at := time.Duration(i) * frame
				p := path.At(at, world(0, 0, 0), toLook)
				eye, dir := local(p.Eye), local(p.Target).sub(local(p.Eye))
				dir = vec3{}.add(dir, 1/dir.len())
				if i > 0 {
					if r := math.Acos(max(-1, min(1, dir.e*was.e+dir.n*was.n+dir.u*was.u))) * 180 / math.Pi * 60; r > TravelTurnRate*1.01 {
						t.Fatalf("frame %d: view turns %.2f°/s", i, r)
					}
				}
				if viewAngle(eye, local(p.Target), local(toLook)) > 0.01 && turned < at {
					turned = at
				}
				was = dir
			}
			if c.name != "small" && turned < travelTurnMin {
				t.Errorf("turned in %v", turned)
			}
		})
	}
}
