package types

import (
	"encoding/json"
	"testing"
)

// TestFacilityEnumsJSON: the enums are written by name, read by name or
// by the number older files have, and a value without a name stays a
// number.
func TestFacilityEnumsJSON(t *testing.T) {
	type rec struct {
		Parking SIMCONNECT_FACILITY_TAXI_PARKING_TYPE   `json:"parking"`
		Name    SIMCONNECT_FACILITY_TAXI_PARKING_NAME   `json:"name"`
		Point   SIMCONNECT_FACILITY_TAXI_POINT_TYPE     `json:"point"`
		Path    SIMCONNECT_FACILITY_TAXI_PATH_TYPE      `json:"path"`
		Leg     SIMCONNECT_FACILITY_LEG_TYPE            `json:"leg"`
		Alt     SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR `json:"alt,omitempty"`
		Surface SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE `json:"surface"`
		Des     SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR   `json:"des"`
		Appr    SIMCONNECT_FACILITY_APPROACH_TYPE       `json:"appr"`
	}
	in := rec{SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_HEAVY, SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_B,
		SIMCONNECT_FACILITY_TAXI_POINT_TYPE_HOLD_SHORT, SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PAINTED_LINE,
		SIMCONNECT_FACILITY_LEG_TYPE_TF, 0, SIMCONNECT_FACILITY_RUNWAY_SURFACE_TYPE_UNKNOWN,
		SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR_LEFT, SIMCONNECT_FACILITY_APPROACH_TYPE_ILS}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"parking":"GATE_HEAVY","name":"GATE_B","point":"HOLD_SHORT","path":"PAINTED_LINE","leg":"TF","surface":"UNKNOWN","des":"LEFT","appr":"ILS"}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
	var out rec
	if err := json.Unmarshal(b, &out); err != nil || out != in {
		t.Fatalf("by name: %+v, %v", out, err)
	}
	legacy := `{"parking":10,"name":13,"point":2,"path":8,"leg":18,"alt":2,"surface":254,"des":1,"appr":4}`
	out = rec{}
	if err := json.Unmarshal([]byte(legacy), &out); err != nil {
		t.Fatal(err)
	}
	in.Alt = SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_ABOVE
	if out != in {
		t.Fatalf("by number: %+v", out)
	}
	// No name: a number, kept as it is.
	if b, _ := json.Marshal(SIMCONNECT_FACILITY_TAXI_POINT_TYPE(3)); string(b) != "3" {
		t.Errorf("unnamed point type: %s", b)
	}
	var p SIMCONNECT_FACILITY_TAXI_POINT_TYPE
	if err := json.Unmarshal([]byte(`"NOPE"`), &p); err == nil {
		t.Error("an unknown name was taken")
	}
}
