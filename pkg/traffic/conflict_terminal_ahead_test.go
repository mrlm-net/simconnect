package traffic

import "testing"

// An arrival above the terminal area now, descending into it, passes a
// go-around there 4 NM off: the terminal 3 NM applies where they pass, not
// the en-route 5 NM of its level now (offline LKPR 06: AFR971 at 12 000 ft
// and DLH299 going around at 5000 ft, the go-around vectored for nothing,
// #1454).
func TestTerminalMinimumWhereTheyPass(t *testing.T) {
	a := air(1, "AFR971", 0, 0, 12000, 90, 250, -1500, true)
	b := air(2, "DLH299", 40, 4, 5000, 270, 250, 0, true)
	a.Airport, b.Airport = "LKPR", "LKPR"
	if cs := PredictConflicts([]TrackedAircraft{a, b}, ConflictOptions{}); len(cs) != 0 {
		t.Errorf("4 NM apart below 10 000 ft, both at LKPR: %+v", cs)
	}
	a.Airport, b.Airport = "", ""
	if cs := PredictConflicts([]TrackedAircraft{a, b}, ConflictOptions{}); len(cs) == 0 {
		t.Error("4 NM apart en route: no conflict")
	}
}
