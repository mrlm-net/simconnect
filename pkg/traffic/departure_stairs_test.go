package traffic

import (
	"testing"
	"time"
)

// Stairs at the door (#831): sent StairsStartDelay after the aircraft
// waits, gone StairsClearMargin before the tug is due; cleared to push
// while they stand there, they back away first and the push waits.
func TestTaxiControllerStairs(t *testing.T) {
	stairs := &fakeFuel{arriveAfter: 60 * 30, leaveAfter: 60 * 20}
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{Stairs: stairs, PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v", ctl.State())
	}
	start := *now
	run(TaxiPushback, 60*5)
	if stairs.attached != 0 {
		t.Fatalf("stairs sent %v after waiting began", now.Sub(start))
	}
	run(TaxiPushback, 60*20)
	if stairs.attached != 1 {
		t.Fatalf("stairs attached %d times after %v", stairs.attached, now.Sub(start))
	}
	for i := 0; i < 60*60*20 && stairs.leaveAt == 0; i++ {
		run(TaxiPushback, 1)
	}
	if stairs.leaveAt == 0 {
		t.Fatal("never left")
	}
	if by := ctl.gateAt.Add(-TugLeadTime - StairsClearMargin); now.After(by.Add(time.Second)) {
		t.Errorf("left %v after it should (%v)", now.Sub(by), by.Sub(start))
	}

	// Cleared to push with the stairs at the door: they leave, the push waits.
	s2 := &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 20}
	ctl2, _, run2, _ := injectedDeparture(t, TaxiRequest{Stairs: s2, PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true})
	go func() {
		for range ctl2.Events() {
		}
	}()
	run2(TaxiPushback, 60*40)
	if s2.attached != 1 || !s2.Fuelling() {
		t.Fatalf("stairs not at the door: attached %d", s2.attached)
	}
	ctl2.ClearPushback()
	run2(TaxiPushback, 60*5)
	if ctl2.State() == TaxiPushback {
		t.Error("pushing with the stairs at the door")
	}
	if !run2(TaxiPushback, 60*120) {
		t.Fatalf("no push after the stairs left: %v", ctl2.State())
	}
}

// A GPU at the nose (#832) runs with the stairs: both from the fleet, both
// gone before the push; GPUSpot is ahead of the nose gear, right of the
// axis.
func TestTaxiControllerGPU(t *testing.T) {
	fleet := NewVehicleFleet(map[VehicleKind]int{VehicleGPU: 1, VehicleStairs: 1})
	gpu, stairs := &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}, &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{GPU: gpu, Stairs: stairs, PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true}, TaxiWithServices(fleet))
	go func() {
		for range ctl.Events() {
		}
	}()
	run(TaxiPushback, 60*40)
	if gpu.attached != 1 || stairs.attached != 1 {
		t.Fatalf("GPU %d, stairs %d", gpu.attached, stairs.attached)
	}
	if out, _ := fleet.Out(VehicleGPU); out != 1 {
		t.Errorf("GPU not taken from the fleet: %d out", out)
	}
	ctl.ClearPushback()
	if !run(TaxiPushback, 60*120) {
		t.Fatalf("no push: %v", ctl.State())
	}
	if gpu.leaveAt == 0 || stairs.leaveAt == 0 {
		t.Error("pushing with the GPU or the stairs still there")
	}

	pose := GroundPose{Heading: 90}
	prof := DefaultMotionProfile()
	p := GPUSpot(pose, prof)
	main := offsetHeading(pose.Position, 270, prof.RefAheadMeters)
	nose := offsetHeading(main, 90, prof.WheelbaseMeters)
	if d := localDist(nose, p.Position); d < 2 || d > 5 {
		t.Errorf("GPU %.1f m from the nose gear", d)
	}
	if p.Position.Lat >= nose.Lat {
		t.Error("GPU not right of the axis (south, heading east)")
	}
}

// The GPU stays until the APU is on (#1025): APUStartBeforeTug before the
// tug comes, after the stairs have gone.
func TestTaxiControllerGPUUntilAPU(t *testing.T) {
	fleet := NewVehicleFleet(map[VehicleKind]int{VehicleGPU: 1, VehicleStairs: 1})
	gpu, stairs := &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}, &fakeFuel{arriveAfter: 60, leaveAfter: 60 * 10}
	ctl, _, run, now := injectedDeparture(t, TaxiRequest{GPU: gpu, Stairs: stairs, PushbackAt: time.Now().Add(20 * time.Minute), HoldForClearances: true}, TaxiWithServices(fleet))
	go func() {
		for range ctl.Events() {
		}
	}()
	var stairsLeft, gpuLeft time.Time
	for i := 0; i < 60*60*25 && gpuLeft.IsZero(); i++ {
		run(TaxiPushback, 1)
		if stairsLeft.IsZero() && stairs.leaveAt > 0 {
			stairsLeft = *now
		}
		if gpu.leaveAt > 0 {
			gpuLeft = *now
		}
	}
	if gpuLeft.IsZero() || stairsLeft.IsZero() {
		t.Fatalf("GPU left %v, stairs left %v", !gpuLeft.IsZero(), !stairsLeft.IsZero())
	}
	apu := ctl.gateAt.Add(-TugLeadTime - APUStartBeforeTug)
	if d := gpuLeft.Sub(apu); d < 0 || d > 2*time.Second {
		t.Errorf("GPU left %v from the APU start", d)
	}
	if !gpuLeft.After(stairsLeft) {
		t.Errorf("GPU left %v before the stairs", stairsLeft.Sub(gpuLeft))
	}
}
