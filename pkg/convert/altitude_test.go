package convert

import (
	"math"
	"testing"
)

func TestFeetToMeters(t *testing.T) {
	tests := []struct {
		name string
		feet float64
		want float64
	}{
		{name: "zero", feet: 0, want: 0},
		{name: "FL350 (35000 ft)", feet: 35000, want: 10668.0},
		{name: "one foot", feet: 1, want: 0.3048},
		{name: "negative (below sea level)", feet: -1000, want: -304.8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FeetToMeters(tt.feet)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("FeetToMeters(%v) = %v, want %v", tt.feet, got, tt.want)
			}
		})
	}
}

func TestMetersToFeet(t *testing.T) {
	tests := []struct {
		name   string
		meters float64
		want   float64
	}{
		{name: "zero", meters: 0, want: 0},
		{name: "10668 m (FL350)", meters: 10668.0, want: 35000.0},
		{name: "one meter", meters: 1, want: 1.0 / 0.3048},
		{name: "negative", meters: -304.8, want: -1000.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetersToFeet(tt.meters)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("MetersToFeet(%v) = %v, want %v", tt.meters, got, tt.want)
			}
		})
	}
}

func TestAltitudeRoundtrip(t *testing.T) {
	values := []float64{0, 1, 100, 35000, -500, 45000.5}
	for _, v := range values {
		got := MetersToFeet(FeetToMeters(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip FeetToMeters->MetersToFeet(%v) = %v", v, got)
		}
	}
}

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
