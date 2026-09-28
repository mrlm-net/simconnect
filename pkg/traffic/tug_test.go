//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
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

// TestTaxiControllerTug: the tug is attached once, on the stand, when the
// pushback is cleared; it follows the aircraft through the push, gets
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
	run(TaxiPushback, 120) // held: no push, no tug
	if len(tug.attached) != 0 {
		t.Fatal("tug attached before the pushback was cleared")
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
	if d := alongHeading(nose, 90, airport.LatLon{Lat: at.Latitude, Lon: at.Longitude}); math.Abs(d-TugAheadMeters) > 0.1 || at.Heading != 90 || at.OnGround != 1 {
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
	for i := 0; i < 60*120 && !tug.Done(); i++ {
		if err := tug.Update(moved, false, 1.0/60); err != nil {
			t.Fatal(err)
		}
	}
	if !tug.Done() || len(c.removed) != 1 || c.removed[0] != 55 {
		t.Fatalf("done %v, removed %v", tug.Done(), c.removed)
	}
	if err := tug.Remove(); err != nil || len(c.removed) != 1 {
		t.Errorf("second removal: %v %v", err, c.removed)
	}
}
