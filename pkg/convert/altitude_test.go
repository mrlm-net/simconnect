package convert

import (
	"math"
	"testing"
)

func TestFPMConversions(t *testing.T) {
	tests := []struct {
		name string
		fpm  float64
		fps  float64
	}{
		{name: "zero", fpm: 0, fps: 0},
		{name: "1800 fpm", fpm: 1800, fps: 30},
		{name: "negative (descent)", fpm: -600, fps: -10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FeetPerMinuteToFeetPerSecond(tt.fpm); math.Abs(got-tt.fps) > epsilon {
				t.Errorf("FPMtoFPS(%v) = %v, want %v", tt.fpm, got, tt.fps)
			}
			if got := FeetPerSecondToFeetPerMinute(tt.fps); math.Abs(got-tt.fpm) > epsilon {
				t.Errorf("FPStoFPM(%v) = %v, want %v", tt.fps, got, tt.fpm)
			}
		})
	}
}
