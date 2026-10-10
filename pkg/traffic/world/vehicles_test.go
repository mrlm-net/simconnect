package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestGroundVehicles: all on by default; Options turn kinds off from the
// start; a kind off gives no vehicle of it (a departure's tug, a
// follow-me); turned on again, it does.
func TestGroundVehicles(t *testing.T) {
	w := New(Options{DataDir: t.TempDir(), GroundVehicles: map[traffic.VehicleKind]bool{traffic.VehicleFollowMe: false}})
	got := w.GroundVehicles()
	if len(got) != len(VehicleKinds) || !got[traffic.VehicleTug] || got[traffic.VehicleFollowMe] {
		t.Fatalf("at the start %v", got)
	}
	w.SetGroundVehicles(map[traffic.VehicleKind]bool{traffic.VehicleTug: false})
	cc := &controlCenter{core: w.st.core}
	if tug := cc.tug(SpawnRequest{Kind: "departure", Tug: true}, 0, traffic.MotionProfile{}, 34); tug != nil {
		t.Error("a tug with tugs off")
	}
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	if fm := cc.followMe(SpawnRequest{Kind: "arrival"}, g, 0, traffic.MotionProfile{}); fm != nil {
		t.Error("a follow-me with follow-me cars off")
	}
	w.SetGroundVehicles(map[traffic.VehicleKind]bool{traffic.VehicleTug: true})
	if !w.GroundVehicles()[traffic.VehicleTug] || !cc.core.vehicleOn(traffic.VehicleTug) {
		t.Error("tugs not on again")
	}
}
