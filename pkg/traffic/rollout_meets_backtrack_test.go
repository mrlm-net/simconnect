package traffic

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestCrewGoesAroundForBacktrack (#1680): one arrival comes in on its
// injected final while the one before it backtracks on the same runway
// after its landing roll, both driven, with no tower. The crew looks at the
// runway before touching down: with the other still on it, it goes around,
// and it never touches down with the other on the runway, so the two never
// meet on it. At least one case must go around. Before, it landed into the
// backtrack and the two stood nose to nose.
func TestCrewGoesAroundForBacktrack(t *testing.T) {
	around := 0
	for _, after := range []float64{0, 15, 30, 45, 60} {
		t.Run(fmt.Sprintf("%.0fs", after), func(t *testing.T) {
			if r := rolloutMeetsBacktrackRun(t, midExitGraph(t, 10), after); r.around {
				around++
			}
		})
	}
	if around == 0 {
		t.Error("no case went around: the crew's look at the runway was never tested")
	}
}

// TestHeadOnFieldNeverMeets (#1680): as TestCrewGoesAroundForBacktrack, on
// a field where the way off of both lies behind the touchdown, so one on
// its landing roll would have no way off short of the backtrack. It never
// gets there: the crew goes around, and the backtrack turns off at its own
// way off.
func TestHeadOnFieldNeverMeets(t *testing.T) {
	around := 0
	for _, after := range []float64{0, 10, 20, 30} {
		t.Run(fmt.Sprintf("%.0fs", after), func(t *testing.T) {
			r := rolloutMeetsBacktrackRun(t, headOnField(t), after)
			t.Logf("the first off by %s", r.firstExit)
			if r.around {
				around++
			}
		})
	}
	if around == 0 {
		t.Error("no case went around: the head-on was never kept off")
	}
}

// headOnField is runway 09/27 of 2500 m (x from -1250 to 1250 m) whose
// one exit to the stand that does not turn back, C, is at x 1000, behind
// the touchdown on 27: an arrival on 27 rolls to the end, turns round and
// backtracks to C. D at the west end leans back for 27. E, every 300 m
// from x -900 to 600, leans back for 27 (none for it) and forward for 09,
// but onto a hairpin back west (a turn of more than airport.UTurnAngle) to
// the parallel taxiway: never planned, still a way off the runway.
func headOnField(t *testing.T) *airport.Graph {
	t.Helper()
	var pts []airport.RawTaxiPoint
	var paths []airport.RawTaxiPath
	pt := func(x, z float32, typ int32) int32 {
		pts = append(pts, airport.RawTaxiPoint{Type: typ, BiasX: x, BiasZ: z})
		return int32(len(pts) - 1)
	}
	link := func(a, b, typ int32, name uint32, width float32, rwy int32) {
		paths = append(paths, airport.RawTaxiPath{Type: typ, Width: width, RunwayNumber: rwy, Start: a, End: b, NameIndex: name})
	}
	const nA, nC, nD, nE, nS, nZ = 1, 2, 3, 4, 5, 6
	xs := []float32{-1250, -900, -600, -300, 0, 300, 600, 1000, 1250}
	rwy := map[float32]int32{}
	for i, x := range xs {
		rwy[x] = pt(x, 0, 1)
		if i > 0 {
			link(rwy[xs[i-1]], rwy[x], 2, 0, 45, 9)
		}
	}
	// D: the west end, leaning back for 27, onto A.
	a0 := pt(-1240, 180, 1)
	hd := pt(-1240, 75, 2)
	link(rwy[-1250], hd, 1, nD, 23, 0)
	link(hd, a0, 1, nD, 23, 0)
	// C: a right angle at x 1000 onto A, the apron and the stand.
	a1 := pt(1000, 180, 1)
	hc := pt(1000, 75, 2)
	link(rwy[1000], hc, 1, nC, 23, 0)
	link(hc, a1, 1, nC, 23, 0)
	link(a0, a1, 1, nA, 23, 0)
	apron := pt(1100, 180, 1)
	link(a1, apron, 1, nA, 23, 0)
	lead := pt(1100, 260, 1)
	link(apron, lead, 1, nS, 20, 0)
	// E: out to a hold, then a hairpin back onto Z, which joins A only at
	// the west end.
	w := pt(-1240, 100, 1)
	link(w, a0, 1, nZ, 23, 0)
	prev := w
	for _, x := range []float32{-900, -600, -300, 0, 300, 600} {
		top := pt(x+20, 170, 2)
		back := pt(x-10, 100, 1)
		link(rwy[x], top, 1, nE, 23, 0)
		link(top, back, 1, nE, 23, 0)
		link(prev, back, 1, nZ, 23, 0)
		prev = back
	}
	link(lead, 0, 3, 0, 20, 0)
	raw := airport.RawAirport{ICAO: "XHON", Name: "Head-on", Latitude: 50.1214, Longitude: 13.8124, Altitude: 300,
		Runways:    []airport.RawRunway{{Latitude: 50.1214, Longitude: 13.8124, Altitude: 300, Heading: 90, Length: 2500, Width: 45, PrimaryNumber: 9, SecondaryNumber: 27}},
		Parking:    []airport.RawParking{{Name: 14, Number: 1, Type: 9, Heading: 0, Radius: 24, BiasX: 1100, BiasZ: 310}},
		TaxiPoints: pts, TaxiPaths: paths, TaxiNames: []string{"", "A", "C", "D", "E", "S", "Z"}}
	layout, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(layout)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// smoothTurnDegS: the most an aircraft's heading may change in a second on
// the ground in these tests (airliner taxi turns peak around 10–15°/s,
// TestGroundMoverTurnEasesInAndOut), and smoothSlipM how far one frame's
// move may differ from its speed (m) before it is a jump.
const (
	smoothTurnDegS = 20.0
	smoothSlipM    = 0.05
	// smoothTightDeg: rounding in a frame's heading change, not a jump.
	smoothTightDeg = 0.05
)

// frameWatch checks one aircraft frame by frame on the ground, across
// paths planned again: its speed changes by at most 0.5 m/s a frame, its
// heading turns no faster than smoothTurnDegS nor tighter than its
// wheelbase allows for the distance moved, and it moves as far as its
// speed takes it (no position jump).
type frameWatch struct {
	name             string
	ok               bool
	pos              airport.LatLon
	hdg, v           float64
	speed, turn, gap float64 // the worst: m/s a frame, °/s, metres
	tight            float64 // the worst heading change over what the distance allows, °
}

func (w *frameWatch) see(p GroundPose, wheelbase float64) {
	v := p.GroundSpeedKts * ktsToMS
	if w.ok {
		dt := 1.0 / backtrackTrafficFps
		d := localDist(w.pos, p.Position)
		dh := math.Abs(headingDiff(w.hdg, p.Heading))
		w.speed = math.Max(w.speed, math.Abs(v-w.v))
		w.turn = math.Max(w.turn, dh/dt)
		w.gap = math.Max(w.gap, math.Abs(d-(v+w.v)/2*dt))
		w.tight = math.Max(w.tight, dh-(d+smoothSlipM)/math.Max(wheelbase, 1)*180/math.Pi)
	}
	w.pos, w.hdg, w.v, w.ok = p.Position, p.Heading, v, true
}

func (w *frameWatch) reset() { w.ok = false }

func (w *frameWatch) check(t *testing.T) {
	t.Helper()
	t.Logf("%s: up to %.2f m/s a frame, %.1f°/s, %.3f m off its speed, %.2f° tighter than its wheelbase", w.name, w.speed, w.turn, w.gap, w.tight)
	if w.speed > 0.5 {
		t.Errorf("%s: speed changed %.2f m/s in one frame: a dead stop", w.name, w.speed)
	}
	if w.turn > smoothTurnDegS {
		t.Errorf("%s: heading turned %.1f°/s, want at most %.0f", w.name, w.turn, smoothTurnDegS)
	}
	if w.tight > smoothTightDeg {
		t.Errorf("%s: heading turned %.2f° more in a frame than its wheelbase allows for the distance", w.name, w.tight)
	}
	if w.gap > smoothSlipM {
		t.Errorf("%s: moved %.3f m off its speed in one frame: a position jump", w.name, w.gap)
	}
}

// rolloutMeetsBacktrackLimit: both off the runway (or the second gone
// around) within this of the second one's start, or one is stuck.
const rolloutMeetsBacktrackLimit = 15 * time.Minute

// meetResult: whether the second went around; the first's way off the
// runway.
type meetResult struct {
	around    bool
	firstExit string
}

// meetDump is where c stands and what stops it, for a stuck run.
func meetDump(c *ArrivalController) string {
	if c.mover == nil {
		return fmt.Sprintf("%s, no mover", c.State())
	}
	p, m := c.mover.Pose(), c.mover
	return fmt.Sprintf("%s at %.5f,%.5f hdg %.0f | s %.1f/%.1f v %.2f kt clear %.1f backFrom %.1f backtracking %v exit %s | blockedBy %d follow %v | traffic %v at %.1f runOn %.1f",
		c.State(), p.Position.Lat, p.Position.Lon, p.Heading, p.Distance, m.Path().Length(), p.GroundSpeedKts, c.clearDist, c.backFrom, c.backtracking, c.plan.Exit.Taxiway,
		c.blockedBy, c.followTraffic, m.hasTraffic, m.trafficAt, m.runOnTo)
}

// injectGroundRequest is the injector's ground height request of obj, as
// the injector numbered it.
func injectGroundRequest(t *testing.T, inj *Injector, obj uint32) uint32 {
	t.Helper()
	inj.mu.Lock()
	defer inj.mu.Unlock()
	o, ok := inj.objects[obj]
	if !ok {
		t.Fatalf("object %d not with the injector", obj)
	}
	return inj.reqBase + 2*uint32(o.slot) + 1
}

// pictureOnRunway reports whether any point along the body of aircraft id,
// as pic has it, is on runway r's surface.
func pictureOnRunway(pic *GroundPicture, id uint32, r airport.Runway) bool {
	pic.mu.Lock()
	e, ok := pic.aircraft[id]
	pic.mu.Unlock()
	if !ok {
		return false
	}
	a, b := r.Primary.Threshold, r.Secondary.Threshold
	for k := range e.axisPoints() {
		q := offsetHeading(e.pos, e.hdg, e.axisAt(k))
		along := calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, q.Lat, q.Lon)
		off := math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, q.Lat, q.Lon))
		if along > 0 && along < r.Length && off < r.Width/2 {
			return true
		}
	}
	return false
}

func rolloutMeetsBacktrackRun(t *testing.T, g *airport.Graph, after float64) meetResult {
	ec := &eventClient{}
	inj, pic := NewInjector(ec), NewGroundPicture()
	now := time.Now()
	clock := func() time.Time { return now }
	newArrival := func(defBase, reqBase uint32, obj uint32) *ArrivalController {
		c := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj), ArrivalWithGroundPicture(pic), ArrivalWithIDs(defBase, reqBase))
		if err := c.Start(ArrivalRequest{Graph: g, Runway: "27", Parking: 0, Model: "A321", InjectApproach: true, SpawnNm: 2,
			HoldForClearance: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range c.Events() {
			}
		}()
		c.now = clock
		c.Handle(assignedMsg(reqBase, obj))
		inj.Handle(groundMsg(injectGroundRequest(t, inj, obj), obj, 1000, 12))
		return c
	}
	const first, second = 77, 78
	a := newArrival(DefaultArrivalDefinitionBase, DefaultArrivalRequestBase, first)
	var b *ArrivalController
	const bDef, bReq = DefaultArrivalDefinitionBase + 50, DefaultArrivalRequestBase + 50
	var backAt, bStart time.Time
	least := TrafficGapMeters - TrafficOverrunMeters
	closest, met, around, downOnIt := math.Inf(1), false, false, false
	// The first from its backtrack until off the runway, through its turn
	// off; the second on its landing roll.
	wa, wb := &frameWatch{name: "the backtrack"}, &frameWatch{name: "the landing roll"}
	step := func(c *ArrivalController, reqBase, obj uint32) {
		if c == nil || c.State().Terminal() {
			return
		}
		// Where the simulator has it: on the ground where it is placed, so
		// the other sees it there (onPosition reports it to the picture).
		pos, hdg, kts := g.Layout.Parking[0].Position, 0.0, 0.0
		if c.mover != nil {
			p := c.mover.Pose()
			pos, hdg, kts = p.Position, p.Heading, p.GroundSpeedKts
		}
		c.Handle(arrivalPositionMsg(reqBase+arrReqMonitor, obj, pos, 0, hdg, kts, false))
		if c.State() == ArrivalAwaitingTaxi {
			c.ClearToTaxi() // off the runway: taxis on, out of the other's way
		}
	}
	off := func(c *ArrivalController) bool { return c != nil && c.State() >= ArrivalAwaitingTaxi }
	for i := 0; i < backtrackTrafficFps*3600; i++ {
		now = now.Add(time.Second / backtrackTrafficFps)
		step(a, DefaultArrivalRequestBase, first)
		if !around {
			step(b, bReq, second)
		}
		if a.mover != nil && a.backtracking && a.mover.Pose().Distance >= a.backFrom && backAt.IsZero() {
			backAt = now
		}
		if b == nil && !backAt.IsZero() && now.Sub(backAt) >= time.Duration(after*float64(time.Second)) {
			if off(a) {
				t.Skipf("the first was off the runway before the second started (%.0f s)", after)
			}
			b, bStart = newArrival(bDef, bReq, second), now
		}
		if a.mover != nil && !backAt.IsZero() && (a.State() == ArrivalRollout || a.State() == ArrivalVacating) {
			wa.see(a.mover.Pose(), a.profile().WheelbaseMeters)
		} else {
			wa.reset()
		}
		if b == nil {
			continue
		}
		if !around {
			b.mu.Lock()
			around = b.goArounds > 0
			down := b.mover != nil || b.approach != nil && b.approach.Pose().OnGround
			b.mu.Unlock()
			if down && !downOnIt && pictureOnRunway(pic, first, b.plan.Runway) {
				downOnIt = true
				t.Errorf("touched down %v after the start with the first on the runway: %s", now.Sub(bStart).Round(time.Second), meetDump(a))
			}
		}
		if off(a) && (around || off(b)) {
			break
		}
		if now.Sub(bStart) > rolloutMeetsBacktrackLimit {
			t.Fatalf("stuck: the first %s, the second %s, %v after the second started\nfirst: %s\nsecond: %s", a.State(), b.State(), rolloutMeetsBacktrackLimit, meetDump(a), meetDump(b))
		}
		if around || a.mover == nil || b.mover == nil || b.State() != ArrivalRollout || b.backtracking && b.mover.Pose().Distance >= b.backFrom {
			wb.reset()
			continue
		}
		// The second on its landing roll, the first on the runway or turning
		// off it.
		pa, pb := a.mover.Pose(), b.mover.Pose()
		met = met || b.blockedBy == first && pb.GroundSpeedKts < 1
		if a.State() <= ArrivalVacating {
			closest = math.Min(closest, noseGap(pb, b.profile(), pa.Position, pa.Heading, a.profile()))
		}
		wb.see(pb, b.profile().WheelbaseMeters)
	}
	if b == nil {
		t.Fatal("the first never backtracked")
	}
	if !off(a) || !around && !off(b) {
		t.Fatalf("not both off the runway: the first %s, the second %s, gone around %v", a.State(), b.State(), around)
	}
	r := meetResult{around: around}
	a.mu.Lock()
	if a.plan != nil {
		r.firstExit = a.plan.Exit.Taxiway
	}
	a.mu.Unlock()
	t.Logf("closest %.2f m, met %v, gone around %v", closest, met, around)
	if met {
		t.Error("stopped on its landing roll for the backtrack: they met on the runway")
	}
	if closest < least-0.05 {
		t.Errorf("nose %.2f m from the other, want its %.0f m clearance", closest, least)
	}
	wa.check(t)
	wb.check(t)
	return r
}
