package nav

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestPLNApproachTypeGLS: a .pln names no GLS approach (the simulator has
// none); the ILS keeps its spelling.
func TestPLNApproachTypeGLS(t *testing.T) {
	if s := plnApproachType(types.SIMCONNECT_FACILITY_APPROACH_TYPE_GLS); s != "" {
		t.Errorf("GLS spelled %q", s)
	}
	if s := plnApproachType(types.SIMCONNECT_FACILITY_APPROACH_TYPE_ILS); s != "ILS" {
		t.Errorf("ILS spelled %q", s)
	}
}
