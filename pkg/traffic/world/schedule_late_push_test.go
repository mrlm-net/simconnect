package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestDeparturePushAtRetimed: a departure the manager retimed, spawned late
// by a limit, pushes back no earlier than its estimate; an on-time one, a
// turnaround, a night stop and a start stand at their STD (#1679).
func TestDeparturePushAtRetimed(t *testing.T) {
	std := time.Date(2026, 11, 22, 14, 0, 0, 0, time.UTC)
	est := std.Add(7 * time.Minute)
	dep := func(mod func(*traffic.ManagedFlight)) traffic.ManagedFlight {
		f := traffic.ManagedFlight{Kind: "departure", Flight: traffic.Flight{Callsign: "DLH785", STD: std}}
		mod(&f)
		return f
	}
	for _, c := range []struct {
		name string
		f    traffic.ManagedFlight
		want time.Time
	}{
		{"on time", dep(func(*traffic.ManagedFlight) {}), std},
		{"retimed", dep(func(f *traffic.ManagedFlight) { f.Estimated = est }), est},
		{"estimate before STD", dep(func(f *traffic.ManagedFlight) { f.Estimated = std.Add(-time.Minute) }), std},
		{"turnaround", dep(func(f *traffic.ManagedFlight) { f.Estimated, f.TurnFrom = est, "DLH784" }), std},
		{"night stop", dep(func(f *traffic.ManagedFlight) { f.Estimated, f.NightStop = est, true }), std},
		{"start stand", dep(func(f *traffic.ManagedFlight) { f.Estimated, f.StartStand = est, true }), std},
	} {
		if got := departurePushAt(c.f); !got.Equal(c.want) {
			t.Errorf("%s: push at %v, want %v", c.name, got, c.want)
		}
	}
}
