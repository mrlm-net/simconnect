package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// fakeTug records what the departure asks of its tug.
type fakeTug struct {
	attached       []GroundPose
	pushing, after int
	pushPoses      []GroundPose
	removed        int
	doneAfter      int // Done once this many updates came after the push
}

func (f *fakeTug) Handle(engine.Message) bool { return false }
func (f *fakeTug) Attach(p GroundPose) error  { f.attached = append(f.attached, p); return nil }
func (f *fakeTug) Done() bool                 { return f.doneAfter > 0 && f.after >= f.doneAfter }
func (f *fakeTug) Remove() error              { f.removed++; return nil }
func (f *fakeTug) Update(p GroundPose, pushing bool, _ float64) error {
	if pushing {
		f.pushing++
		f.pushPoses = append(f.pushPoses, p)
	} else {
		f.after++
	}
	return nil
}

// TestTaxiControllerTug: the tug is attached once, on the stand, while the
// aircraft waits for pushback; it follows it through the push, gets
// updates after it until Done, and is removed on Cancel.
func TestTaxiControllerTug(t *testing.T) {
	tug := &fakeTug{doneAfter: 30}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{Tug: tug, HoldForClearances: true})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v", ctl.State())
	}
	run(TaxiPushback, 120) // held: no push, but the tug is connected
	if len(tug.attached) != 1 || tug.pushing == 0 {
		t.Fatalf("waiting for pushback: attached %d, updates %d", len(tug.attached), tug.pushing)
	}
	ctl.ClearPushback()
	if !run(TaxiAwaitingTaxi, 60*600) {
		t.Fatalf("state %v", ctl.State())
	}
	run(TaxiTaxiing, 60*5)
	if len(tug.attached) != 1 {
		t.Fatalf("attached %d times", len(tug.attached))
	}
	stand := ctl.req.Graph.Layout.Parking[ctl.req.Parking]
	if d := localDist(tug.attached[0].Position, StandPoint(stand, 0)); d > 1 {
		t.Errorf("tug attached %.1f m from the stand", d)
	}
	if tug.pushing < 60 || len(tug.pushPoses) < 2 {
		t.Fatalf("%d pushing updates", tug.pushing)
	}
	last := tug.pushPoses[len(tug.pushPoses)-1]
	if d := localDist(last.Position, ctl.mover.Pose().Position); d > 1 && ctl.State() == TaxiAwaitingTaxi {
		t.Errorf("last push update %.1f m from the aircraft", d)
	}
	if tug.after < tug.doneAfter {
		t.Errorf("%d updates after the push, want %d (until Done)", tug.after, tug.doneAfter)
	}
	if n := tug.after; n > tug.doneAfter {
		t.Errorf("updated %d times after Done", n-tug.doneAfter)
	}
	if err := ctl.Cancel(); err != nil || tug.removed != 1 {
		t.Errorf("cancel: %v, removed %d", err, tug.removed)
	}
}

// tugClient records simulated object creation and removal.
type tugClient struct {
	eventClient
	created []types.SIMCONNECT_DATA_INITPOSITION
	titles  []string
}

func (c *tugClient) AICreateSimulatedObject(title string, p types.SIMCONNECT_DATA_INITPOSITION, _ uint32) error {
	c.titles, c.created = append(c.titles, title), append(c.created, p)
	return nil
}

// TestSimObjectTug: the tug is created on the aircraft's nose gear, frozen
// once assigned, placed with the aircraft while pushing, then waits, drives
// off and is removed.
func TestSimObjectTug(t *testing.T) {
	c := &tugClient{}
	inj := NewInjector(c)
	prof := DefaultMotionProfile()
	tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
	pose := GroundPose{Position: lkpr, Heading: 90}
	if err := tug.Attach(pose); err != nil {
		t.Fatal(err)
	}
	if len(c.created) != 1 || c.titles[0] != DefaultTugTitle {
		t.Fatalf("created %v %v", c.titles, c.created)
	}
	nose := NoseGear(lkpr, 90, prof)
	at := c.created[0]
	if d := alongHeading(nose, 90, airport.LatLon{Lat: at.Latitude, Lon: at.Longitude}); math.Abs(d-TugAheadMeters) > 0.1 || at.Heading != normDeg(90+TugYawDeg) || at.OnGround != 1 {
		t.Errorf("tug %.2f m ahead of the nose gear, heading %.0f, on ground %d", d, at.Heading, at.OnGround)
	}
	if !tug.Handle(assignedMsg(9001, 55)) || tug.ObjectID() != 55 {
		t.Fatal("assignment not handled")
	}
	if len(c.events) == 0 {
		t.Error("tug not frozen")
	}
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
	before := len(c.waypoints)
	moved := GroundPose{Position: offsetHeading(lkpr, 270, 5), Heading: 90, GroundSpeedKts: 2}
	if err := tug.Update(moved, true, 1.0/60); err != nil {
		t.Fatal(err)
	}
	if len(c.waypoints) != before+1 {
		t.Fatal("tug not placed while pushing")
	}
	// It never comes closer to the nose gear than while connected: it backs
	// away first (it faces the aircraft), then turns off.
	moveNose := NoseGear(moved.Position, moved.Heading, prof)
	start := localDist(tug.at(moved).Position, moveNose)
	for i := 0; i < 60*120 && !tug.Done(); i++ {
		if err := tug.Update(moved, false, 1.0/60); err != nil {
			t.Fatal(err)
		}
		if d := localDist(tug.pose.Position, moveNose); d < start-0.2 {
			t.Fatalf("tug %.1f m from the nose gear, closer than connected (%.1f m)", d, start)
		}
	}
	if !tug.Done() || len(c.removed) != 1 || c.removed[0] != 55 {
		t.Fatalf("done %v, removed %v", tug.Done(), c.removed)
	}
	if err := tug.Remove(); err != nil || len(c.removed) != 1 {
		t.Errorf("second removal: %v %v", err, c.removed)
	}
}

// TestSimObjectTugSteers: through a pushback arc the tow bar swings off the
// aircraft axis with the nose wheel's travel (not locked on the axis), stays
// within TugMaxBarDeg and keeps the tug on the bar, AheadMeters from the
// nose gear.
func TestSimObjectTugSteers(t *testing.T) {
	prof := DefaultMotionProfile()
	prof.CruiseKts, prof.MinTurnKts = PushbackSpeedKts, 1
	gear := offset(lkpr, 0, -prof.RefAheadMeters)
	path, err := NewArcPath([]airport.LatLon{gear, offset(gear, 0, -40), offset(gear, 50, -40)}, prof, 25)
	if err != nil {
		t.Fatal(err)
	}
	m := NewPushbackMover(path, prof, 0)
	c := &tugClient{}
	inj := NewInjector(c)
	tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
	if err := tug.Attach(m.Pose()); err != nil {
		t.Fatal(err)
	}
	tug.Handle(assignedMsg(9001, 55))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
	maxRel := 0.0
	for i := 0; i < 60*300; i++ {
		pose := m.Step(1.0 / 60)
		if err := tug.Update(pose, true, 1.0/60); err != nil {
			t.Fatal(err)
		}
		nose := NoseGear(pose.Position, pose.Heading, prof)
		if d := localDist(tug.pose.Position, nose); math.Abs(d-TugAheadMeters) > 0.01 {
			t.Fatalf("tug %.2f m from the nose gear", d)
		}
		rel := headingDiff(pose.Heading, normDeg(tug.pose.Heading-TugYawDeg))
		if math.Abs(rel) > TugMaxBarDeg+1e-9 {
			t.Fatalf("bar %.1f° off the axis", rel)
		}
		if math.Abs(rel) > math.Abs(maxRel) {
			maxRel = rel
		}
		if pose.Arrived {
			break
		}
	}
	if math.Abs(maxRel) < 10 {
		t.Errorf("bar at most %.1f° off the axis in the arc: locked on the axis", maxRel)
	}
	t.Logf("bar up to %.1f° off the aircraft axis", maxRel)
}

// TestTaxiWaitsForTug: cleared to taxi right after the push, the aircraft
// stays put until the tug has driven off.
func TestTaxiWaitsForTug(t *testing.T) {
	tug := &fakeTug{doneAfter: 600} // ten seconds of driving off
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{Tug: tug, HoldForClearances: true})
	go func() {
		for range ctl.Events() {
		}
	}()
	run(TaxiAwaitingPushback, 600)
	run(TaxiPushback, 120)
	ctl.ClearPushback()
	if !run(TaxiAwaitingTaxi, 60*600) {
		t.Fatalf("state %v", ctl.State())
	}
	ctl.ClearToTaxi()
	// The tug drives off, then the engines start (EngineStartTime each).
	for i := 0; i < 60*180 && ctl.State() == TaxiAwaitingTaxi; i++ {
		run(TaxiTaxiing, 1)
		if ctl.State() != TaxiAwaitingTaxi && !tug.Done() {
			t.Fatalf("taxiing with the tug still there (%d of %d updates)", tug.after, tug.doneAfter)
		}
	}
	if ctl.State() != TaxiTaxiing || !tug.Done() {
		t.Fatalf("state %v, tug done %v", ctl.State(), tug.Done())
	}
}

// After the push the tug backs away and drives off without a jump: each
// placement moves it no more than its speed allows in a frame (live it
// jumped about its wheelbase as it started backing off, and again as it
// turned away).
func TestSimObjectTugLeavesSmoothly(t *testing.T) {
	c := &tugClient{}
	inj := NewInjector(c)
	prof := DefaultMotionProfile()
	tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
	pose := GroundPose{Position: lkpr, Heading: 90}
	if err := tug.Attach(pose); err != nil {
		t.Fatal(err)
	}
	tug.Handle(assignedMsg(9001, 55))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
	if err := tug.Update(pose, true, 1.0/60); err != nil {
		t.Fatal(err)
	}
	prev := tug.pose.Position
	worst := 0.0
	for i := 0; i < 60*120 && !tug.Done(); i++ {
		if err := tug.Update(pose, false, 1.0/60); err != nil {
			t.Fatal(err)
		}
		if tug.Done() {
			break
		}
		worst = math.Max(worst, localDist(prev, tug.pose.Position))
		prev = tug.pose.Position
	}
	if limit := TugDriveOffKts * ktsToMS / 60 * 1.5; worst > limit {
		t.Errorf("tug moved %.2f m in one frame, want at most %.2f", worst, limit)
	}
}

// With the airport known, a tug appears at its depot, drives in along the
// vehicle roads to the nose (the push waits: Connected), and after the push
// drives home to the depot, where it is removed.
func TestSimObjectTugFromDepot(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	i, err := l.ParkingIndex("B9")
	if err != nil {
		t.Fatal(err)
	}
	c := &tugClient{}
	inj := NewInjector(c)
	prof := DefaultMotionProfile()
	tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
	tug.Layout = l
	stand := l.Parking[i]
	pose := GroundPose{Position: StandPoint(stand, prof.RefAheadMeters), Heading: stand.Heading}
	if err := tug.Attach(pose); err != nil {
		t.Fatal(err)
	}
	nose := NoseGear(pose.Position, pose.Heading, prof)
	at := airport.LatLon{Lat: c.created[0].Latitude, Lon: c.created[0].Longitude}
	if d := localDist(at, nose); d < 50 {
		t.Fatalf("appeared %.0f m from the nose: not at a depot", d)
	}
	depot, _ := nearestDepot(l, nose)
	if localDist(at, depot) > 15 {
		t.Errorf("appeared %.0f m from its depot", localDist(at, depot))
	}
	if tug.Connected() {
		t.Fatal("connected before it was even created")
	}
	tug.Handle(assignedMsg(9001, 55))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
	steps, maxStep := 0, 0.0
	for ; steps < 60*600 && !tug.Connected(); steps++ {
		before := tug.pose.Position
		if err := tug.Update(pose, true, 1.0/60); err != nil {
			t.Fatal(err)
		}
		if steps > 0 {
			maxStep = math.Max(maxStep, localDist(before, tug.pose.Position))
		}
	}
	if !tug.Connected() {
		t.Fatal("never reached the nose")
	}
	// Driving in and connecting without a hop (live: 3 m at the nose).
	if maxStep > 0.3 {
		t.Errorf("moved %.2f m in one frame driving in or connecting", maxStep)
	}
	if d := localDist(tug.pose.Position, tug.at(pose).Position); d > 0.5 {
		t.Errorf("connected %.1f m off the tow point", d)
	}
	t.Logf("drove in in %.0f s", float64(steps)/60)
	// The push done (no movement here): home to the depot, then removed.
	for i := 0; i < 60*900 && !tug.Done(); i++ {
		if err := tug.Update(pose, false, 1.0/60); err != nil {
			t.Fatal(err)
		}
	}
	if !tug.Done() || len(c.removed) != 1 {
		t.Fatalf("done %v, removed %v", tug.Done(), c.removed)
	}
	if d := localDist(tug.pose.Position, depot); d > 30 {
		t.Errorf("removed %.0f m from its depot", d)
	}
}

// A tug never created is created once more, further along its way in
// (another tug may stand where it was to appear); not a third time.
func TestTugRetryCreate(t *testing.T) {
	g := lkprGraph(t)
	c := &tugClient{}
	tug := NewSimObjectTug(c, NewInjector(c), DefaultTugTitle, 9001, DefaultMotionProfile())
	tug.Layout = g.Layout
	i, _ := g.Layout.ParkingIndex("S16")
	st := g.Layout.Parking[i]
	if err := tug.Attach(GroundPose{Position: st.Position, Heading: st.Heading}); err != nil {
		t.Fatal(err)
	}
	if !tug.RetryCreate() {
		t.Fatal("not created again")
	}
	if len(c.created) != 2 {
		t.Fatalf("%d creations", len(c.created))
	}
	a, b := c.created[0], c.created[1]
	if d := calc.HaversineMeters(a.Latitude, a.Longitude, b.Latitude, b.Longitude); d < TugRetryAheadMeters-10 || d > TugRetryAheadMeters+10 {
		t.Errorf("created again %.0f m from the first place, want about %.0f", d, TugRetryAheadMeters)
	}
	if tug.RetryCreate() {
		t.Error("created a third time")
	}
}
