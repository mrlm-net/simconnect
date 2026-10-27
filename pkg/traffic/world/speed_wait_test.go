package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestSpeedWait: told a slower speed, an arrival is not told another for
// speedCallEvery (live, WZZ1042: 230, 210, 180 kt within 3 minutes); one
// closing on the one ahead is slowed at once; never slowed, no wait.
func TestSpeedWait(t *testing.T) {
	now := time.Now()
	e := traffic.SequenceEntry{Callsign: "WZZ1042"}
	if !speedWait(now, now.Add(-90*time.Second), e) {
		t.Error("90 s after a speed: told another")
	}
	if speedWait(now, now.Add(-speedCallEvery), e) {
		t.Error("speedCallEvery after: still waiting")
	}
	if speedWait(now, time.Time{}, e) {
		t.Error("never slowed: waiting")
	}
	e.ShortBy = 30 * time.Second // settles with the next cut (offline: most re-cuts came this way)
	if !speedWait(now, now.Add(-90*time.Second), e) {
		t.Error("30 s short of its leader: told another")
	}
	e.ShortBy = speedWaitShortBy
	if speedWait(now, now.Add(-30*time.Second), e) {
		t.Error("closing on its leader: waiting")
	}
}
