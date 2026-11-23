package worldtest

import "testing"

// TestBacktrackDepartureOnApron (#1641): the same meetings end well on the
// apron too. Both finish in bounded time, neither stands nose to nose and
// they never come nearer each other than a span.
func TestBacktrackDepartureOnApron(t *testing.T) {
	meetCases(t, true)
}
