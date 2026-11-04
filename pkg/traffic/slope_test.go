package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// slopeRoll rolls title at kts along heading hdg over ground rising grade
// (ft per ft along hdg 65, LKPR 06/24's) for 300 m, the ground map holding
// every cell there, and is the pitch placed each frame and the profile.
func slopeRoll(t *testing.T, title string, hdg, kts, grade float64) ([]float64, MotionProfile) {
	t.Helper()
	ec := &eventClient{}
	inj := NewInjector(ec)
	w := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	inj.now = func() time.Time { return w }
	const obj = 42
	inj.Takeover(obj)
	inj.SetModel(obj, title)
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, obj, 1200, 12))
	mid := airport.LatLon{Lat: 50.1, Lon: 14.25}
	// The ground: 1200 ft at mid, rising grade along 065, every cell.
	ground := func(p airport.LatLon) float64 { return 1200 + grade*alongHeading(mid, 65, p)/0.3048 }
	for e := -200.0; e <= 200; e += 0.5 {
		for n := -200.0; n <= 200; n += 0.5 {
			p := offset(mid, e, n)
			inj.ground.put(p, ground(p))
		}
	}
	v := kts * ktsToMS
	var out []float64
	for s := -150.0; s <= 150; s += v / 60 {
		pose := GroundPose{Position: offsetHeading(mid, hdg, s), Heading: hdg, GroundSpeedKts: kts}
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, obj, ground(pose.Position), 12)) // sampled every frame
		n := len(placements(ec))
		if err := inj.Place(obj, pose); err != nil {
			t.Fatal(err)
		}
		if p := placements(ec); len(p) > n {
			out = append(out, p[len(p)-1].Pitch)
		}
		w = w.Add(time.Second / 60)
	}
	return out, MotionProfileFor(title)
}

// On a sloping runway every type sits on all its wheels: pitched by the
// slope from its main gear to its nose gear, nose up uphill (SimConnect:
// negative), never jumping a frame. Placed at its static pitch whatever
// the slope, an A220 rolling down LKPR 06/24's 0.6 % had its nose wheel
// 10 cm off the runway, read as a wheelie after landing and before the
// take-off roll (live, CSA164).
func TestGroundPitchFollowsTheSlope(t *testing.T) {
	const grade = 0.006
	slopeDeg := math.Atan(grade) * 180 / math.Pi
	for _, title := range []string{
		"FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery",
		"FSLTL_FAIB_B738_TVS-Skytravel_TSOC",
		"FSLTL A20N WZZ Wizzair",
		"FSLTL_BCS3_CSA",
		"FSLTL_B77W_UAE",
		"Asobo C172",
		"some unknown model", // the default profile
	} {
		for _, c := range []struct {
			hdg, want float64 // want: the slope, nose up positive
		}{{65, slopeDeg}, {245, -slopeDeg}, {155, 0}} { // up, down, across
			pitch, prof := slopeRoll(t, title, c.hdg, 30, grade)
			if len(pitch) < 60 {
				t.Fatalf("%s: %d placements", title, len(pitch))
			}
			// Settled (the first placement takes it at once; moving on,
			// the cells read about a metre apart): the slope's.
			for i, p := range pitch[10:] {
				if got := -p; math.Abs(got-c.want) > 0.03 {
					t.Fatalf("%s (wheelbase %.1f m) facing %03.0f: pitch %.3f° nose up at frame %d, want %.3f° (the slope)", title, prof.WheelbaseMeters, c.hdg, got, i+10, c.want)
				}
			}
			for i := 1; i < len(pitch); i++ {
				if d := math.Abs(pitch[i] - pitch[i-1]); d > slopeDegPerSec/60+1e-9 {
					t.Fatalf("%s facing %03.0f: pitch jumped %.4f° at frame %d", title, c.hdg, d, i)
				}
			}
		}
	}
}

// Level ground: exactly its static pitch, as before.
func TestGroundPitchLevel(t *testing.T) {
	pitch, _ := slopeRoll(t, "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", 65, 20, 0)
	for i, p := range pitch {
		if p != 0 {
			t.Fatalf("level ground: pitch %.4f at frame %d, want the static 0", p, i)
		}
	}
}

// Where the ground map has not seen the ground under both gears, the slope
// is not guessed: the last one stays (here none, the static pitch).
func TestGroundPitchUnknownGround(t *testing.T) {
	inj := NewInjector(&eventClient{})
	o := &injected{groundFt: 1200}
	o.wheelbase, o.refAhead = 12.6, 1
	p := airport.LatLon{Lat: 50.1, Lon: 14.25}
	inj.ground.put(offsetHeading(p, 90, -1), 1200) // the mains only
	if d := inj.slopeShown(o, p, 90); d != 0 {
		t.Errorf("nose gear's ground unknown: slope %.3f, want 0", d)
	}
	inj.ground.put(offsetHeading(p, 90, 11.6), 1210) // 10 ft up: not the ground
	if d := inj.slopeShown(o, p, 90); d != 0 {
		t.Errorf("nose gear's cell 10 ft off: slope %.3f, want 0", d)
	}
}
