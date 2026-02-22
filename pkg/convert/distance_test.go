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

func TestNMToStatuteMiles(t *testing.T) {
	tests := []struct {
		name string
		nm   float64
		want float64
	}{
		{name: "zero", nm: 0, want: 0},
		{name: "1 NM ≈ 1.15078 statute miles", nm: 1, want: 1852.0 / statuteMileToMeters},
		{name: "10 NM", nm: 10, want: 10 * 1852.0 / statuteMileToMeters},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NMToStatuteMiles(tt.nm)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("NMToStatuteMiles(%v) = %v, want %v", tt.nm, got, tt.want)
			}
		})
	}
}

func TestStatuteMilesToNM(t *testing.T) {
	tests := []struct {
		name string
		mi   float64
		want float64
	}{
		{name: "zero", mi: 0, want: 0},
		{name: "1 statute mile ≈ 0.86898 NM", mi: 1, want: statuteMileToMeters / 1852.0},
		{name: "10 statute miles", mi: 10, want: 10 * statuteMileToMeters / 1852.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatuteMilesToNM(tt.mi)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("StatuteMilesToNM(%v) = %v, want %v", tt.mi, got, tt.want)
			}
		})
	}
}

func TestKilometersToStatuteMiles(t *testing.T) {
	tests := []struct {
		name string
		km   float64
		want float64
	}{
		{name: "zero", km: 0, want: 0},
		{name: "1 km ≈ 0.62137 statute miles", km: 1, want: 1000.0 / statuteMileToMeters},
		{name: "1.609344 km = 1 statute mile", km: 1.609344, want: 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KilometersToStatuteMiles(tt.km)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KilometersToStatuteMiles(%v) = %v, want %v", tt.km, got, tt.want)
			}
		})
	}
}

func TestStatuteMilesToKilometers(t *testing.T) {
	tests := []struct {
		name string
		mi   float64
		want float64
	}{
		{name: "zero", mi: 0, want: 0},
		{name: "1 statute mile = 1.609344 km", mi: 1, want: 1.609344},
		{name: "2 statute miles", mi: 2, want: 2 * 1.609344},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatuteMilesToKilometers(tt.mi)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("StatuteMilesToKilometers(%v) = %v, want %v", tt.mi, got, tt.want)
			}
		})
	}
}

func TestStatuteMilesRoundTrips(t *testing.T) {
	// NM -> SM -> NM
	nmValues := []float64{0, 1, 0.5, 100, 5000}
	for _, nm := range nmValues {
		got := StatuteMilesToNM(NMToStatuteMiles(nm))
		if math.Abs(got-nm) > epsilon {
			t.Errorf("roundtrip NMToSM->SMToNM(%v) = %v", nm, got)
		}
	}
	// km -> SM -> km
	kmValues := []float64{0, 1, 1.609344, 100}
	for _, km := range kmValues {
		got := StatuteMilesToKilometers(KilometersToStatuteMiles(km))
		if math.Abs(got-km) > epsilon {
			t.Errorf("roundtrip kmToSM->SMToKm(%v) = %v", km, got)
		}
	}
	// m -> SM -> m
	mValues := []float64{0, 1609.344, 5000, 100000}
	for _, m := range mValues {
		got := StatuteMilesToMeters(MetersToStatuteMiles(m))
		if math.Abs(got-m) > epsilon {
			t.Errorf("roundtrip mToSM->SMToM(%v) = %v", m, got)
		}
	}
}
