//go:build windows
// +build windows

package traffic

import "testing"

func TestIDBlocks(t *testing.T) {
	b := NewIDBlocks(20000, 30000, 10, 3)
	var defs []uint32
	for i := 0; i < 3; i++ {
		d, r, err := b.Acquire()
		if err != nil || d != 20000+uint32(i)*10 || r != 30000+uint32(i)*10 {
			t.Fatalf("block %d: %d %d %v", i, d, r, err)
		}
		defs = append(defs, d)
	}
	if _, _, err := b.Acquire(); err != ErrNoIDs {
		t.Fatalf("fourth block: %v", err)
	}
	b.Release(defs[1])
	b.Release(defs[1]) // twice: ignored
	b.Release(12345)   // unknown: ignored
	if b.InUse() != 2 {
		t.Fatalf("%d in use", b.InUse())
	}
	if d, _, _ := b.Acquire(); d != defs[1] {
		t.Fatalf("reused %d, want %d", d, defs[1])
	}
}

// TestRedefineClears: a controller on a reused ID block clears the
// definitions first, so its fields are not appended twice.
func TestRedefineClears(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	fleet := NewFleet(ec)
	c22, _ := g.Layout.ParkingIndex("C22")
	for i := 0; i < 2; i++ {
		ctl := NewTaxiController(fleet, TaxiWithIDs(20000, 30000))
		if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: "24", Model: "FSLTL A320 Air France SL", Tail: "CSA1"}); err != nil {
			t.Fatal(err)
		}
	}
	if n := ec.cleared(); n != 2 {
		t.Fatalf("%d definitions cleared on the second start, want 2", n)
	}
}
