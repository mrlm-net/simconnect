package world

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestSpawnWaitLoggedOnce: a spawn waiting on its place at every retry is
// logged once, again when the reason changes or after waitSayEvery.
func TestSpawnWaitLoggedOnce(t *testing.T) {
	l := &trafficLog{}
	s := &scheduler{cc: &controlCenter{log: l}}
	f := traffic.ManagedFlight{Flight: traffic.Flight{Callsign: "DLH644"}}
	t0 := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	blocked := func(at time.Duration, why string) {
		s.event(traffic.ManagerEvent{Kind: traffic.EventBlocked, Time: t0.Add(at), Flight: f, Reason: why})
	}
	for i := range 5 {
		blocked(time.Duration(i)*20*time.Second, "spawn point in use: would meet THY1729")
		s.event(traffic.ManagerEvent{Kind: traffic.EventEstimated, Time: t0, Flight: f, Reason: "landing sequence"})
	}
	blocked(2*time.Minute, "spawn point in use: would meet AUA1")
	blocked(3*time.Minute, "spawn point in use: would meet AUA1")
	blocked(8*time.Minute, "spawn point in use: would meet AUA1")
	n := 0
	for _, line := range l.lines {
		if strings.Contains(line, "waiting —") {
			n++
		}
	}
	if n != 3 {
		t.Errorf("%d wait lines, want 3", n)
	}
}
