package convert

import (
	"math"
	"testing"
)

func TestNMToMeters(t *testing.T) {
	tests := []struct {
		name string
		nm   float64
		want float64
	}{
		{name: "zero", nm: 0, want: 0},
		{name: "1 NM", nm: 1, want: 1852.0},
		{name: "fractional", nm: 0.5, want: 926.0},
		{name: "large", nm: 100, want: 185200.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NMToMeters(tt.nm)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("NMToMeters(%v) = %v, want %v", tt.nm, got, tt.want)
			}
		})
	}
}

func TestMetersToNM(t *testing.T) {
	tests := []struct {
		name   string
		meters float64
		want   float64
	}{
		{name: "zero", meters: 0, want: 0},
		{name: "1852 m", meters: 1852.0, want: 1.0},
		{name: "fractional", meters: 926.0, want: 0.5},
		{name: "large", meters: 185200.0, want: 100.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetersToNM(tt.meters)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("MetersToNM(%v) = %v, want %v", tt.meters, got, tt.want)
			}
		})
	}
}

func TestNMMetersRoundtrip(t *testing.T) {
	values := []float64{0, 1, 0.25, 100, 5000}
	for _, v := range values {
		got := MetersToNM(NMToMeters(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip NMToMeters->MetersToNM(%v) = %v", v, got)
		}
	}
}

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

func TestKilometersMetersRoundtrip(t *testing.T) {
	values := []float64{0, 1, 0.5, 100}
	for _, km := range values {
		if got := MetersToKilometers(KilometersToMeters(km)); math.Abs(got-km) > epsilon {
			t.Errorf("roundtrip km->m->km(%v) = %v", km, got)
		}
	}
}

// NMKilometerConsistency verifies that NM->km->NM and NM->m->NM agree.
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
