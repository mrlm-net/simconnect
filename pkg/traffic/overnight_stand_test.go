package traffic

import (
	"slices"
	"testing"
)

// Overnight: an aircraft parked through the night takes a stand without a
// jetway (Bridged, the simulator's jetway list) while one is free, even
// where the airport types every airliner stand GATE (LKPR), and leaves the
// jetways to the day's turnarounds (#1298).
func TestOvernightPrefersRemote(t *testing.T) {
	g := lkprGraph(t)
	var gates []int
	for _, i := range g.Layout.SuitableStands(17) {
		if g.Layout.Parking[i].IsGate() {
			gates = append(gates, i)
		}
	}
	if len(gates) < 6 {
		t.Fatalf("%d gates for an A320", len(gates))
	}
	// The last three: airliner stands without a jetway.
	remote := gates[len(gates)-3:]
	bridged := gates[:len(gates)-3]
	for i := 0; i < 3; i++ {
		s, err := NewStandAllocator(&eventClient{}, g).Assign(StandRequirements{Owner: "NS" + string(rune('A'+i)), HalfSpan: 17, Overnight: true, Bridged: bridged})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(remote, s) {
			t.Errorf("overnight given %s (with a jetway), want one of the remote %v", g.Layout.Parking[s].Label(), remote)
		}
	}
	a := NewStandAllocator(&eventClient{}, g)
	for i := 0; i < 3; i++ {
		if s, err := a.Assign(StandRequirements{Owner: "NT" + string(rune('A'+i)), HalfSpan: 17, Overnight: true, Bridged: bridged}); err != nil || !slices.Contains(remote, s) {
			t.Errorf("overnight %d: stand %d %v, want one of the remote %v", i, s, err, remote)
		}
	}
	if s, err := a.Assign(StandRequirements{Owner: "NTD", HalfSpan: 17, Overnight: true, Bridged: bridged}); err != nil || slices.Contains(bridged, s) || g.Layout.Parking[s].IsGate() {
		t.Errorf("remote airliner stands full: stand %d %v, want a GA or cargo one before a jetway", s, err)
	}
}
