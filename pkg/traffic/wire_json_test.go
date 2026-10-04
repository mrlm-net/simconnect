package traffic

import (
	"encoding/json"
	"testing"
)

// TestRequestsMarshal: a departure's and an arrival's request go over a
// wire as JSON (the World's actuator, #710): the graph, the tug, the fuel
// truck and the stand check stay out.
func TestRequestsMarshal(t *testing.T) {
	g := lkprGraph(t)
	tr := TaxiRequest{Graph: g, Parking: 3, Runway: "24", Model: "FSLTL_B738_RYR", StandOccupied: func(int) bool { return false }}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	var back TaxiRequest
	if err := json.Unmarshal(b, &back); err != nil || back.Runway != "24" || back.Parking != 3 || back.Graph != nil {
		t.Errorf("%+v %v", back, err)
	}
	if _, err := json.Marshal(ArrivalRequest{Graph: g, Runway: "24", Parking: 5}); err != nil {
		t.Error(err)
	}
}
