package worldtest

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestOpposedTaxiNoDeadlock: an arrival in along LKPR's F to C24 and a
// departure out of C28 along H1 and F the other way, on routes the reverse
// of each other for 1.4 km (live: TVS681 and CSA398 met nose to nose in
// the middle of F, neither able to give way). The departure holds on H1
// short of where the arrival turns in, and both get where they go.
func TestOpposedTaxiNoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	stand := func(label string) int {
		for i, p := range h.Layout.Parking {
			if p.Label() == label {
				return i
			}
		}
		t.Fatalf("no stand %s", label)
		return -1
	}
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "arrival", ICAO: "LKPR", Stand: stand("C24"), Runway: "24",
		Model: "FSLTL_FAIB_B738_TVS-Smartwings_NC", Tail: "TVS681", InjectApproach: true}); err != nil {
		t.Fatal(err)
	}
	state := map[string]string{}
	spawned := false
	for range 30 * 6 {
		h.Advance(10 * time.Second)
		for _, v := range h.World.Snapshot().Aircraft {
			state[v.Tail] = v.State
		}
		if state["TVS681"] == "rollout" && !spawned {
			spawned = true
			if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "departure", ICAO: "LKPR", Stand: stand("C28"), Runway: "06",
				Model: "FSLTL_FAIB_A320_SmartWings_CzechAirlinesLivery", Tail: "CSA398"}); err != nil {
				t.Fatal(err)
			}
		}
		if state["TVS681"] == "parked" && (state["CSA398"] == "departing" || state["CSA398"] == "complete") {
			return
		}
	}
	t.Fatalf("not both through in 30 minutes: TVS681 %s, CSA398 %s", state["TVS681"], state["CSA398"])
}
