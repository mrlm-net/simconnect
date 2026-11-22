package worldtest

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// Limits of TestBacktrackMeetsDeparture: how long both may take, how near
// ahead another facing an aircraft is head-on, and how long one may stand
// nose to nose before it is a standoff.
const (
	meetMaxTime     = 35 * time.Minute
	meetHeadOnM     = 80.0
	meetHeadOnDeg   = 120.0
	meetStandoffFor = 30 * time.Second
	meetNoseFactor  = 1.35
)

// meetWayOffStates are an arrival's from touchdown to its taxi clearance:
// on its way off the runway.
var meetWayOffStates = []string{"landing", "rollout", "vacating", "awaiting taxi"}

// noseToBody is how far the nose tip of the one at the origin, heading
// mine, is from the body, tail end to nose tip, of the other at (x, y)
// meters east and north, heading theirs: from the airframe, not from what
// the traffic picture samples.
func noseToBody(x, y, mine, theirs float64, p traffic.MotionProfile) float64 {
	nose := p.WheelbaseMeters*meetNoseFactor - p.RefAheadMeters
	tail := p.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	tail += p.RefAheadMeters
	ms, mc := math.Sincos(mine * math.Pi / 180)
	ts, tc := math.Sincos(theirs * math.Pi / 180)
	dx, dy := nose*ms-x, nose*mc-y
	along := math.Max(-tail, math.Min(nose, dx*ts+dy*tc))
	return math.Hypot(dx-along*ts, dy-along*tc)
}

// TestBacktrackMeetsDeparture (#1640): at a field with one way between
// runway and apron (TFFJ) an arrival backtracks to the exit a departure
// taxis out along or holds short on. At whatever moment of the arrival the
// departure appears it never stands nose to nose with one holding short,
// keeps the clearance from its true nose on the way off, and neither jumps.
// At XBAK, written by the test, wherever it runs (the two moments that
// always meet there); at TFFJ too where its data is present.
func TestBacktrackMeetsDeparture(t *testing.T) {
	meetCases(t, false)
}

// meetCases runs each case; apron: what comes of it on the apron fails too.
func meetCases(t *testing.T, apron bool) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	type meetCase struct {
		name  string
		first string
		until string
		after time.Duration
	}
	cases := []meetCase{
		{"holdingShort", "departure", "holding short", 0},
		{"taxiingOut", "departure", "taxiing", 0},
		{"together", "arrival", "", 0},
		{"onApproach", "arrival", "", time.Minute},
		{"onFinal", "arrival", "", 2 * time.Minute},
		{"landing", "arrival", "landing", 0},
		{"rollingOut", "arrival", "rollout", 0},
		{"vacating", "arrival", "vacating", 0},
	}
	checked := 0
	run := func(t *testing.T, at meetField, data string, cases []meetCase) {
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				meetRunAt(t, at, data, c.first, c.until, c.after, apron)
				checked++
			})
		}
	}
	// The departure first: it holds short as the arrival comes off. Only
	// those two, for the time the package may take on the Linux director.
	t.Run(meetXBAK.icao, func(t *testing.T) { run(t, meetXBAK, backtrackFieldDir(t), cases[:2]) })
	t.Run(meetTFFJ.icao, func(t *testing.T) { run(t, meetTFFJ, hasTFFJ(t), cases) })
	if checked == 0 {
		t.Error("no meeting checked")
	}
}

func meetRun(t *testing.T, data, first, until string, after time.Duration, apron bool) {
	meetRunAt(t, meetTFFJ, data, first, until, after, apron)
}

func meetRunAt(t *testing.T, at meetField, data, first, until string, after time.Duration, apron bool) {
	title := at.title
	log := &lockedBuf{}
	o := Options{DataDir: data, ICAO: at.icao, Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), title)}
	o.World.Output = log
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, end, ok := h.Layout.RunwayEnd(at.runway)
	if !ok {
		t.Fatalf("no runway %s at %s", at.runway, at.icao)
	}
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 25, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway(at.icao, at.runway)
	h.Advance(2 * time.Second)
	jumps := len(h.Sim.Jumps())
	tails := map[string]string{"arrival": "MTA1", "departure": "MTD1"}
	spawn := func(kind string) {
		t.Helper()
		r := world.SpawnRequest{Kind: kind, ICAO: at.icao, Tail: tails[kind], Runway: at.runway, Stand: at.standFor(kind), Model: title, InjectApproach: kind == "arrival"}
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	second := "departure"
	if first == second {
		second = "arrival"
	}
	start, spawned := h.Clock.Now(), false
	spawn(first)

	p := traffic.MotionProfileFor(title)
	var states [2][]string
	var lastAt [2][2]float64
	var out, in bool
	// What a built field's case is there for: the arrival seen coming back
	// along the runway, the departure holding short as it comes off.
	var backtracked, met bool
	closest, opposed, longest := math.Inf(1), time.Time{}, time.Duration(0)
	longestAt := ""
	var holdLongest time.Duration
	nearest, nearestAt := math.Inf(1), ""
	wayOffGap, wayOffAt := math.Inf(1), ""
	finished := h.Until(meetMaxTime, func() bool {
		now := h.Clock.Now()
		if !spawned {
			v, _ := h.Aircraft(tails[first])
			if until != "" && v.State == until || until == "" && now.Sub(start) >= after {
				spawn(second)
				spawned = true
			}
		}
		var on [2]bool
		var hdg [2]float64
		for i, kind := range []string{"arrival", "departure"} {
			v, ok := h.Aircraft(tails[kind])
			if !ok {
				continue
			}
			if n := len(states[i]); n == 0 || !strings.HasSuffix(states[i][n-1], " "+v.State) {
				states[i] = append(states[i], fmt.Sprintf("%d:%s", int(now.Sub(start).Seconds()), " "+v.State))
			}
			if i == 0 && v.State == "parked" {
				in = true
			}
			if i == 1 && !v.OnGround && slices.ContainsFunc(states[i], func(s string) bool {
				return strings.HasSuffix(s, " departing") || strings.HasSuffix(s, " lining up")
			}) {
				out = true
			}
			if v.OnGround && v.ObjectID != 0 {
				on[i], hdg[i], lastAt[i] = true, v.Heading, [2]float64{v.Position.Lat, v.Position.Lon}
			}
		}
		if on[0] {
			a, _ := h.Aircraft(tails["arrival"])
			if slices.Contains(meetWayOffStates, a.State) {
				backtracked = backtracked || math.Abs(math.Remainder(hdg[0]-end.Heading, 360)) >= meetHeadOnDeg
				if on[1] {
					b, _ := h.Aircraft(tails["departure"])
					met = met || backtracked && b.State == "holding short"
				}
			}
		}
		if on[0] && on[1] {
			k := math.Cos(lastAt[0][0] * math.Pi / 180)
			x, y := (lastAt[1][1]-lastAt[0][1])*111195*k, (lastAt[1][0]-lastAt[0][0])*111195
			dist := math.Hypot(x, y)
			closest = math.Min(closest, dist)
			a, _ := h.Aircraft(tails["arrival"])
			b, _ := h.Aircraft(tails["departure"])
			gap := math.Min(noseToBody(x, y, hdg[0], hdg[1], p), noseToBody(-x, -y, hdg[1], hdg[0], p))
			if gap < nearest {
				nearest = gap
				nearestAt = fmt.Sprintf("%s and %s, at %d s", a.State, b.State, int(now.Sub(start).Seconds()))
			}
			if slices.Contains(meetWayOffStates, a.State) && gap < wayOffGap {
				wayOffGap = gap
				wayOffAt = fmt.Sprintf("%s and %s, at %d s", a.State, b.State, int(now.Sub(start).Seconds()))
			}
			facing := math.Abs(math.Remainder(hdg[0]-hdg[1], 360)) >= meetHeadOnDeg
			s, c := math.Sincos(hdg[0] * math.Pi / 180)
			inFront := x*s+y*c > 0 && math.Abs(x*c-y*s) < p.SpanMeters
			s, c = math.Sincos(hdg[1] * math.Pi / 180)
			inFront = inFront && -(x*s+y*c) > 0
			if facing && inFront && dist < meetHeadOnM && !in {
				if opposed.IsZero() {
					opposed = now
				}
				if b.State == "holding short" {
					holdLongest = max(holdLongest, now.Sub(opposed))
				}
				if d := now.Sub(opposed); d > longest {
					longest = d
					a, _ := h.Aircraft(tails["arrival"])
					b, _ := h.Aircraft(tails["departure"])
					longestAt = fmt.Sprintf("%.0f m apart, %s and %s, at %d s", dist, a.State, b.State, int(now.Sub(start).Seconds()))
				}
			} else {
				opposed = time.Time{}
			}
		}
		return in && out
	})
	t.Logf("arrival %q", states[0])
	t.Logf("departure %q", states[1])
	t.Logf("closest %.1f m between reference points, nose to nose for %v (%s)", closest, longest, longestAt)
	// What comes of it on the apron is logged here and fails only where
	// apron is set.
	onApron := t.Logf
	if apron {
		onApron = t.Errorf
	}
	if !finished {
		onApron("not both finished in %v: arrival parked %v, departure away %v", meetMaxTime, in, out)
		a, _ := h.Aircraft(tails["arrival"])
		b, _ := h.Aircraft(tails["departure"])
		if b.State == "holding short" || slices.Contains(meetWayOffStates, a.State) {
			t.Errorf("standoff on the way off the runway: %s and %s after %v", a.State, b.State, meetMaxTime)
		}
	}
	if longest > meetStandoffFor {
		onApron("head-on for %v: %s", longest, longestAt)
	}
	if holdLongest > meetStandoffFor {
		t.Errorf("nose to nose with the departure holding short for %v", holdLongest)
	}
	if at.built && (!spawned || !backtracked || !met) {
		t.Errorf("nothing checked: both spawned %v, backtracked %v, departure holding short as the arrival came off %v", spawned, backtracked, met)
	}
	t.Logf("true nose to body %.2f m (%s); on the way off %.2f m (%s)", nearest, nearestAt, wayOffGap, wayOffAt)
	if least := traffic.TrafficGapMeters - traffic.TrafficOverrunMeters; wayOffGap < least {
		t.Errorf("on the way off a nose came to %.2f m of the other's body, less than %.0f m: %s", wayOffGap, least, wayOffAt)
	}
	if closest < p.SpanMeters {
		onApron("came to %.1f m of each other, less than a span (%.1f m)", closest, p.SpanMeters)
	}
	for _, j := range h.Sim.Jumps()[jumps:] {
		if j.Tail == tails["arrival"] || j.Tail == tails["departure"] {
			t.Errorf("%s jumped: %+v", j.Tail, j)
		}
	}
	if t.Failed() {
		for _, l := range strings.Split(log.String(), "\n") {
			if strings.Contains(l, "ground") || strings.Contains(l, "held") || strings.Contains(l, "route") || strings.Contains(l, "hold") {
				t.Log(l)
			}
		}
	}
}
