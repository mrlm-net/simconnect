package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestGroundUnderDoesNotFlip: rolling down a slope over cells seen every other
// frame, with its own sample two frames old, the ground under an aircraft keeps
// falling. Between the cell and the sample it went up and down by their
// difference every frame (live, TVS148 at 117 kt on LKPR 06: 0.11 ft).
func TestGroundUnderDoesNotFlip(t *testing.T) {
	inj := NewInjector(&eventClient{})
	const perFrame = 0.05 // ft, 3 ft/s at 60 frames a second
	at := func(n int) airport.LatLon {
		return offsetHeading(airport.LatLon{Lat: 50.1, Lon: 14.2}, 90, float64(n)*2)
	}
	real := func(n int) float64 { return 1170 - float64(n)*perFrame }
	for n := 0; n < 300; n += 2 {
		inj.ground.put(at(n), real(n))
	}
	o := &injected{}
	prev := math.Inf(1)
	for n := 2; n < 300; n++ {
		o.groundFt = real(n - 2)
		got := inj.groundUnder(o, at(n))
		if n > 2 && got > prev+0.001 {
			t.Fatalf("frame %d: ground up %.3f ft, from %.3f to %.3f", n, got-prev, prev, got)
		}
		if math.Abs(got-real(n)) > 0.02 {
			t.Fatalf("frame %d: ground %.3f, there %.3f", n, got, real(n))
		}
		prev = got
	}
}

// TestGroundUnderFadesTheCell: past the last cell the height leaves it smoothly
// for the aircraft's own sample, and a standing aircraft doesn't keep it.
func TestGroundUnderFadesTheCell(t *testing.T) {
	now := time.Unix(1000, 0)
	inj := NewInjector(&eventClient{})
	inj.now = func() time.Time { return now }
	base := airport.LatLon{Lat: 50.1, Lon: 14.2}
	inj.ground.put(base, 1200.5)
	o := &injected{groundFt: 1200}
	if got := inj.groundUnder(o, base); got != 1200.5 {
		t.Fatalf("on the cell: %.2f", got)
	}
	// Half the fade's distance on: half the difference.
	if got := inj.groundUnder(o, offsetHeading(base, 90, groundOffFadeM/2)); math.Abs(got-1200.25) > 0.01 {
		t.Errorf("halfway: %.2f, want 1200.25", got)
	}
	if got := inj.groundUnder(o, offsetHeading(base, 90, groundOffFadeM*2)); got != 1200 {
		t.Errorf("far on: %.2f, want its sample", got)
	}
	// Standing just off the cell: gone with time.
	inj.ground.put(base, 1200.5)
	inj.groundUnder(o, base)
	off := offsetHeading(base, 90, 5)
	now = now.Add(time.Duration(groundOffFadeSec * float64(time.Second)))
	if got := inj.groundUnder(o, off); got != 1200 {
		t.Errorf("standing, later: %.2f, want its sample", got)
	}
}
