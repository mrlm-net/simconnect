package calc

import (
	"math"
	"testing"
)

func TestHeadwindCrosswind(t *testing.T) {
	const epsilon = 1e-9
	const epsilonSpeed = 1e-6 // for trigonometric results

	tests := []struct {
		name          string
		windDir       float64
		windSpeed     float64
		runwayHeading float64
		wantHeadwind  float64
		wantCrosswind float64
		tolerance     float64
	}{
		{
			name:    "pure headwind — wind from runway direction",
			windDir: 360, windSpeed: 20, runwayHeading: 360,
			wantHeadwind: 20, wantCrosswind: 0,
			tolerance: epsilonSpeed,
		},
		{
			name:    "pure tailwind — wind from behind",
			windDir: 180, windSpeed: 10, runwayHeading: 360,
			wantHeadwind: -10, wantCrosswind: 0,
			tolerance: epsilonSpeed,
		},
		{
			name:    "pure crosswind from right — 090 wind on 360 runway",
			windDir: 90, windSpeed: 15, runwayHeading: 360,
			wantHeadwind: 0, wantCrosswind: 15,
			tolerance: epsilonSpeed,
		},
		{
			name:    "pure crosswind from left — 270 wind on 360 runway",
			windDir: 270, windSpeed: 15, runwayHeading: 360,
			wantHeadwind: 0, wantCrosswind: -15,
			tolerance: epsilonSpeed,
		},
		{
			name:    "45-degree crosswind — equal headwind and crosswind",
			windDir: 45, windSpeed: 10, runwayHeading: 360,
			wantHeadwind:  10 * math.Cos(45*math.Pi/180),
			wantCrosswind: 10 * math.Sin(45*math.Pi/180),
			tolerance:     epsilonSpeed,
		},
		{
			name:    "zero wind speed",
			windDir: 270, windSpeed: 0, runwayHeading: 90,
			wantHeadwind: 0, wantCrosswind: 0,
			tolerance: epsilon,
		},
		{
			name:    "non-north runway — 090 runway, wind from 090",
			windDir: 90, windSpeed: 12, runwayHeading: 90,
			wantHeadwind: 12, wantCrosswind: 0,
			tolerance: epsilonSpeed,
		},
		{
			name:    "non-north runway — 090 runway, wind from 360",
			windDir: 360, windSpeed: 10, runwayHeading: 90,
			wantHeadwind: 0, wantCrosswind: -10,
			tolerance: epsilonSpeed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hw, xw := HeadwindCrosswind(tt.windDir, tt.windSpeed, tt.runwayHeading)
			if math.Abs(hw-tt.wantHeadwind) > tt.tolerance {
				t.Errorf("headwind = %v, want %v", hw, tt.wantHeadwind)
			}
			if math.Abs(xw-tt.wantCrosswind) > tt.tolerance {
				t.Errorf("crosswind = %v, want %v", xw, tt.wantCrosswind)
			}
		})
	}
}

func TestCrosswindComponent(t *testing.T) {
	const epsilonSpeed = 1e-6

	tests := []struct {
		name          string
		windDir       float64
		windSpeed     float64
		runwayHeading float64
		want          float64
		tolerance     float64
	}{
		{
			name:    "pure crosswind from right",
			windDir: 90, windSpeed: 15, runwayHeading: 360,
			want: 15, tolerance: epsilonSpeed,
		},
		{
			name:    "pure crosswind from left",
			windDir: 270, windSpeed: 15, runwayHeading: 360,
			want: -15, tolerance: epsilonSpeed,
		},
		{
			name:    "pure headwind — zero crosswind",
			windDir: 360, windSpeed: 20, runwayHeading: 360,
			want: 0, tolerance: epsilonSpeed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CrosswindComponent(tt.windDir, tt.windSpeed, tt.runwayHeading)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("CrosswindComponent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeadwindComponent(t *testing.T) {
	const epsilonSpeed = 1e-6

	tests := []struct {
		name          string
		windDir       float64
		windSpeed     float64
		runwayHeading float64
		want          float64
		tolerance     float64
	}{
		{
			name:    "pure headwind",
			windDir: 360, windSpeed: 20, runwayHeading: 360,
			want: 20, tolerance: epsilonSpeed,
		},
		{
			name:    "pure tailwind — negative headwind",
			windDir: 180, windSpeed: 10, runwayHeading: 360,
			want: -10, tolerance: epsilonSpeed,
		},
		{
			name:    "pure crosswind — zero headwind",
			windDir: 90, windSpeed: 15, runwayHeading: 360,
			want: 0, tolerance: epsilonSpeed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HeadwindComponent(tt.windDir, tt.windSpeed, tt.runwayHeading)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Errorf("HeadwindComponent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHeadwindCrosswindPythagoras verifies that decomposed components
// reconstruct the original wind speed via Pythagorean theorem.
func TestHeadwindCrosswindPythagoras(t *testing.T) {
	cases := [][3]float64{
		{360, 20, 360},
		{45, 15, 360},
		{270, 10, 90},
		{180, 25, 250},
	}
	for _, c := range cases {
		hw, xw := HeadwindCrosswind(c[0], c[1], c[2])
		got := math.Sqrt(hw*hw + xw*xw)
		if math.Abs(got-c[1]) > 1e-9 {
			t.Errorf("windDir=%.0f windSpeed=%.0f runway=%.0f: sqrt(hw²+xw²) = %v, want %v",
				c[0], c[1], c[2], got, c[1])
		}
	}
}
