package world

import (
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestLimitAlongATaxiway: a limit part-way along taxiway A is not "hold short
// of A", the taxiway it is on, but of the one crossing there, or a marked point
// with none (#1331).
func TestLimitAlongATaxiway(t *testing.T) {
	a := airport.Edge{To: 1, Name: "A"}
	route := &airport.Route{Nodes: []airport.NodeID{0, 1, 2}, Edges: []airport.Edge{a, {To: 2, Name: "A"}}}
	for _, tc := range []struct {
		name string
		adj  [][]airport.Edge
		want string
	}{
		{"a crossing", [][]airport.Edge{{a}, {{To: 2, Name: "A"}, {To: 3, Name: "B"}}, {}, {}}, "hold short of B"},
		{"none", [][]airport.Edge{{a}, {{To: 2, Name: "A"}}, {}, {}}, "marked point"},
	} {
		it := &controlled{Tail: "CSA1", graph: &airport.Graph{Adj: tc.adj}}
		got := it.phraseView(ControlView{Tail: "CSA1"}, route, "upto", 1)
		if !strings.Contains(got.Text, tc.want) {
			t.Errorf("%s: said %q, want %q", tc.name, got.Text, tc.want)
		}
	}
}
