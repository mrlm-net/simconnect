package convert

import (
	"math"
	"testing"
)

func TestDegreesRadians(t *testing.T) {
	tests := []struct {
		name string
		deg  float64
		rad  float64
	}{
		{name: "zero", deg: 0, rad: 0},
		{name: "90 degrees", deg: 90, rad: math.Pi / 2},
		{name: "180 degrees", deg: 180, rad: math.Pi},
		{name: "360 degrees", deg: 360, rad: 2 * math.Pi},
		{name: "negative", deg: -90, rad: -math.Pi / 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DegreesToRadians(tt.deg); math.Abs(got-tt.rad) > epsilon {
				t.Errorf("DegreesToRadians(%v) = %v, want %v", tt.deg, got, tt.rad)
			}
			if got := RadiansToDegrees(tt.rad); math.Abs(got-tt.deg) > epsilon {
				t.Errorf("RadiansToDegrees(%v) = %v, want %v", tt.rad, got, tt.deg)
			}
		})
	}
}

func TestNormalizeHeading(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "zero", in: 0, want: 0},
		{name: "180", in: 180, want: 180},
		{name: "359", in: 359, want: 359},
		{name: "360 wraps to 0", in: 360, want: 0},
		{name: "720 wraps to 0", in: 720, want: 0},
		{name: "negative wraps to 270", in: -90, want: 270},
		{name: "negative wraps to 350", in: -10, want: 350},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeHeading(tt.in)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("NormalizeHeading(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeAngle(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "zero", in: 0, want: 0},
		{name: "180 stays 180", in: 180, want: 180},
		{name: "181 wraps to -179", in: 181, want: -179},
		{name: "-180 wraps to 180", in: -180, want: 180},
		{name: "360 wraps to 0", in: 360, want: 0},
		{name: "-361 wraps to -1", in: -361, want: -1},
		{name: "90 stays 90", in: 90, want: 90},
		{name: "-90 stays -90", in: -90, want: -90},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAngle(tt.in)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("NormalizeAngle(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestAngleDifference(t *testing.T) {
	tests := []struct {
		name     string
		from, to float64
		want     float64
	}{
		{name: "from=0 to=90 → 90", from: 0, to: 90, want: 90},
		{name: "from=350 to=10 → 20 (short way)", from: 350, to: 10, want: 20},
		{name: "from=10 to=350 → -20", from: 10, to: 350, want: -20},
		{name: "from=0 to=180 → 180", from: 0, to: 180, want: 180},
		{name: "from=0 to=-180 → 180", from: 0, to: -180, want: 180},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AngleDifference(tt.from, tt.to)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("AngleDifference(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
