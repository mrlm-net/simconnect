package convert

import (
	"math"
	"testing"
)

func TestKilometerDistanceConversions(t *testing.T) {
	tests := []struct {
		name string
		nm   float64
		km   float64
	}{
		{name: "zero", nm: 0, km: 0},
		{name: "1 NM = 1.852 km", nm: 1, km: 1.852},
		{name: "100 NM", nm: 100, km: 185.2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NMToKilometers(tt.nm); math.Abs(got-tt.km) > epsilon {
				t.Errorf("NMToKilometers(%v) = %v, want %v", tt.nm, got, tt.km)
			}
			if got := KilometersToNM(tt.km); math.Abs(got-tt.nm) > epsilon {
				t.Errorf("KilometersToNM(%v) = %v, want %v", tt.km, got, tt.nm)
			}
		})
	}
}

func TestNMKilometerConsistency(t *testing.T) {
	values := []float64{1, 10, 100, 250}
	for _, nm := range values {
		viaKM := KilometersToNM(NMToKilometers(nm))
		viaM := MetersToNM(NMToMeters(nm))
		if math.Abs(viaKM-viaM) > epsilon {
			t.Errorf("NM %v: via km = %v, via m = %v", nm, viaKM, viaM)
		}
	}
}

