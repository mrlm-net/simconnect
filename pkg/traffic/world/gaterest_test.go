package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/jetway"
)

// TestGateResting: a gate whose dock failed (docking, then failed)
// JetwayGateFails times in a row, each counted once however many ticks see
// it failed, rests JetwayGateRest; failed ticks with no dock between
// (refused asks) count nothing; a dock resets it.
func TestGateResting(t *testing.T) {
	var s jetwayState
	now := time.Now()
	k := "LKPR 11"
	for i := 0; i < JetwayGateFails; i++ {
		at := now.Add(time.Duration(i) * time.Minute)
		s.gateResting(k, jetway.StatusDocking, at)
		for j := 0; j < 3; j++ { // seen failed on several ticks: one failure
			if s.gateResting(k, jetway.StatusFailed, at.Add(time.Duration(j+1)*time.Second)) != (i == JetwayGateFails-1) {
				t.Fatalf("failure %d, tick %d: resting %v", i+1, j, !(i == JetwayGateFails-1))
			}
		}
	}
	last := now.Add(time.Duration(JetwayGateFails-1)*time.Minute + time.Second)
	if !s.gateResting(k, jetway.StatusNone, last.Add(30*time.Minute)) {
		t.Error("not resting half way through the rest")
	}
	if s.gateResting(k, jetway.StatusNone, last.Add(JetwayGateRest+time.Second)) {
		t.Error("still resting after JetwayGateRest")
	}
	s.gateResting(k, jetway.StatusDocking, now.Add(2*time.Hour))
	s.gateResting(k, jetway.StatusFailed, now.Add(2*time.Hour))
	s.gateResting(k, jetway.StatusDocked, now.Add(2*time.Hour))
	if s.fails[k] != 0 {
		t.Errorf("fails %d after a dock, want 0", s.fails[k])
	}
	// Failed seen tick after tick with no dock between: nothing counted.
	for j := 0; j < 10; j++ {
		s.gateResting(k, jetway.StatusFailed, now.Add(3*time.Hour+time.Duration(j)*time.Second))
	}
	if s.fails[k] != 0 {
		t.Errorf("fails %d from failed ticks without a dock, want 0", s.fails[k])
	}
}
