package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/jetway"
)

// TestGateResting: a gate whose dock failed JetwayGateFails times in a
// row, each ask counted once, rests JetwayGateRest; a dock resets it.
func TestGateResting(t *testing.T) {
	var s jetwayState
	now := time.Now()
	k := "LKPR 11"
	for i := 0; i < JetwayGateFails; i++ {
		ask := now.Add(time.Duration(i) * time.Minute)
		for j := 0; j < 3; j++ { // seen failed on several ticks: one failure
			if s.gateResting(k, jetway.StatusFailed, ask, ask.Add(time.Duration(j)*time.Second)) != (i == JetwayGateFails-1) {
				t.Fatalf("failure %d, tick %d: resting %v", i+1, j, !(i == JetwayGateFails-1))
			}
		}
	}
	last := now.Add(time.Duration(JetwayGateFails-1) * time.Minute)
	if !s.gateResting(k, jetway.StatusNone, last, last.Add(30*time.Minute)) {
		t.Error("not resting half way through the rest")
	}
	if s.gateResting(k, jetway.StatusNone, last, last.Add(JetwayGateRest+time.Second)) {
		t.Error("still resting after JetwayGateRest")
	}
	s.gateResting(k, jetway.StatusFailed, now.Add(2*time.Hour), now.Add(2*time.Hour))
	s.gateResting(k, jetway.StatusDocked, now.Add(2*time.Hour), now.Add(2*time.Hour))
	if s.fails[k] != 0 {
		t.Errorf("fails %d after a dock, want 0", s.fails[k])
	}
}
