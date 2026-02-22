package calc

import (
	"math"
	"testing"
)

func TestWindCorrectionAngle(t *testing.T) {
	const epsilon = 1e-9
	const epsilonDeg = 1e-4 // degrees tolerance for trigonometric results

	tests := []struct {
		name      string
		windDir   float64
		windSpeed float64
		tas       float64
		course    float64
		wantWCA   float64
		tolerance float64
	}{
		{
			name:    "zero TAS — undefined, returns 0",
			windDir: 270, windSpeed: 20, tas: 0, course: 360,
			wantWCA:   0,
			tolerance: epsilon,
		},
		{
			name:    "no wind — WCA is 0",
			windDir: 270, windSpeed: 0, tas: 100, course: 360,
			wantWCA:   0,
			tolerance: epsilon,
		},
		{
			name:    "direct headwind — wind from ahead, no correction needed",
			windDir: 360, windSpeed: 30, tas: 120, course: 360,
			// windTo = 360+180 = 540; sin(540-360)=sin(180)=0 → WCA=0
			wantWCA:   0,
			tolerance: epsilonDeg,
		},
		{
			name:    "direct tailwind — wind from behind, no correction needed",
			windDir: 180, windSpeed: 30, tas: 120, course: 360,
			// windTo = 180+180 = 360; sin(360-360)=sin(0)=0 → WCA=0
			wantWCA:   0,
			tolerance: epsilonDeg,
		},
		{
			name: "90-degree crosswind from left — positive WCA (correct right)",
			// wind from 270°, course 360°: windTo=450; sin(450-360)=sin(90)=1
			// WCA = asin(20/100) ≈ 11.537°
			windDir: 270, windSpeed: 20, tas: 100, course: 360,
			wantWCA:   math.Asin(20.0/100.0) * 180.0 / math.Pi,
			tolerance: epsilonDeg,
		},
		{
			name: "90-degree crosswind from right — negative WCA (correct left)",
			// wind from 90°, course 360°: windTo=270; sin(270-360)=sin(-90)=-1
			// WCA = asin(-20/100) ≈ -11.537°
			windDir: 90, windSpeed: 20, tas: 100, course: 360,
			wantWCA:   math.Asin(-20.0/100.0) * 180.0 / math.Pi,
			tolerance: epsilonDeg,
		},
		{
			name: "windSpeed exceeds TAS — clamp guards asin domain",
			// wind from 270°, windSpeed=150, tas=100, course=360
			// ratio = 150/100 = 1.5 → sin(90°)=1 → clamped to 1 → WCA = asin(1) = 90°
			windDir: 270, windSpeed: 150, tas: 100, course: 360,
			wantWCA:   90.0,
			tolerance: epsilonDeg,
		},
		{
			name:    "near-zero TAS — treated as zero, returns 0",
			windDir: 270, windSpeed: 20, tas: 1e-15, course: 360,
			wantWCA:   0,
			tolerance: epsilon,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WindCorrectionAngle(tt.windDir, tt.windSpeed, tt.tas, tt.course)
			if math.Abs(got-tt.wantWCA) > tt.tolerance {
				t.Errorf("WindCorrectionAngle() = %v, want %v", got, tt.wantWCA)
			}
		})
	}
}
