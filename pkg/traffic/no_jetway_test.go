package traffic

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Types boarding by their own airstairs (the user: CRJ, Citation): no
// jetway, no stairs vehicle; a local rule takes one back.
func TestNoJetway(t *testing.T) {
	for model, want := range map[string]bool{
		"FSLTL_FAIB_CRJ7_CLH-Lufthansa CityLine": true, "CRJ9": true, "AT76": true, "DH8D": true, "C25C": true,
		"Cessna Citation Longitude Variation 18": true, "A320": false, "E190": false, "B738": false,
	} {
		if got := NoJetway(model); got != want {
			t.Errorf("%s: %v, want %v", model, got, want)
		}
	}
	if err := dict.Use("traffic.noJetway", []byte(`[{"type":"CRJ9","noJetway":false}]`)); err != nil {
		t.Fatal(err)
	}
	defer dict.Reset("traffic.noJetway")
	if NoJetway("CRJ9") {
		t.Error("CRJ9 not taken back")
	}
}

// A type with its own airstairs is given a stand without a jetway while one
// is free, a gate when none is.
func TestNoJetwayPrefersRemote(t *testing.T) {
	g := lkprGraph(t)
	a := NewStandAllocator(&eventClient{}, g)
	for i := 0; i < 5; i++ {
		s, err := a.Assign(StandRequirements{Owner: "CRJ" + string(rune('A'+i)), HalfSpan: 12, NoJetway: true})
		if err != nil {
			t.Fatal(err)
		}
		if g.Layout.Parking[s].IsGate() {
			t.Errorf("own airstairs given gate %s with remote stands free", g.Layout.Parking[s].Label())
		}
	}
}

// Prefer: the gates whose jetway can dock are taken first (#572).
func TestStandPrefer(t *testing.T) {
	g := lkprGraph(t)
	a := NewStandAllocator(&eventClient{}, g)
	prefer := []int{26, 23}
	for i := 0; i < 2; i++ {
		s, err := a.Assign(StandRequirements{Owner: "PF" + string(rune('A'+i)), HalfSpan: 17, Prefer: prefer})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(prefer, s) {
			t.Errorf("stand %d, want one of %v", s, prefer)
		}
	}
	if s, _ := a.Assign(StandRequirements{Owner: "PFC", HalfSpan: 17, Prefer: prefer}); slices.Contains(prefer, s) || s < 0 {
		t.Errorf("third: stand %d, want another free one", s)
	}
}
