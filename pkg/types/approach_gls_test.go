package types

import (
	"encoding/json"
	"testing"
)

// TestApproachTypeGLS: GLS, not a SimConnect value, is read and written by
// name like the others, and its number reads back too.
func TestApproachTypeGLS(t *testing.T) {
	b, err := json.Marshal(SIMCONNECT_FACILITY_APPROACH_TYPE_GLS)
	if err != nil || string(b) != `"GLS"` {
		t.Fatalf("written as %s %v", b, err)
	}
	for _, in := range []string{`"GLS"`, `12`} {
		var a SIMCONNECT_FACILITY_APPROACH_TYPE
		if err := json.Unmarshal([]byte(in), &a); err != nil || a != SIMCONNECT_FACILITY_APPROACH_TYPE_GLS {
			t.Errorf("%s read as %d %v", in, a, err)
		}
	}
	var a SIMCONNECT_FACILITY_APPROACH_TYPE
	if err := json.Unmarshal([]byte(`"GBAS"`), &a); err == nil {
		t.Error("an unknown approach type was taken")
	}
}
