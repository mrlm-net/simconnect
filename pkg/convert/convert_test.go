//go:build windows
// +build windows

package convert

import (
	"math"
	"testing"
)

// epsilon for simple unit conversions (linear multiplications/divisions).
const epsilon = 1e-9

// epsilonDeg for geographic degree comparisons (OffsetToLatLon).
const epsilonDeg = 1e-4

// --- Altitude ---

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

// --- Distance ---

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

func TestDistanceRoundtrip(t *testing.T) {
	values := []float64{0, 1, 0.25, 100, 5000}
	for _, v := range values {
		got := MetersToNM(NMToMeters(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip NMToMeters->MetersToNM(%v) = %v", v, got)
		}
	}
}

// --- Speed: Knots <-> KPH ---

func TestKnotsToKilometersPerHour(t *testing.T) {
	tests := []struct {
		name  string
		knots float64
		want  float64
	}{
		{name: "zero", knots: 0, want: 0},
		{name: "250 kts cruise", knots: 250, want: 463.0},
		{name: "one knot", knots: 1, want: 1.852},
		{name: "high speed", knots: 500, want: 926.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KnotsToKilometersPerHour(tt.knots)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KnotsToKilometersPerHour(%v) = %v, want %v", tt.knots, got, tt.want)
			}
		})
	}
}

func TestKilometersPerHourToKnots(t *testing.T) {
	tests := []struct {
		name string
		kph  float64
		want float64
	}{
		{name: "zero", kph: 0, want: 0},
		{name: "463 kph", kph: 463.0, want: 250.0},
		{name: "1.852 kph", kph: 1.852, want: 1.0},
		{name: "high speed", kph: 926.0, want: 500.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KilometersPerHourToKnots(tt.kph)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KilometersPerHourToKnots(%v) = %v, want %v", tt.kph, got, tt.want)
			}
		})
	}
}

func TestKnotsKPHRoundtrip(t *testing.T) {
	values := []float64{0, 1, 100, 250, 500}
	for _, v := range values {
		got := KilometersPerHourToKnots(KnotsToKilometersPerHour(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip KnotsToKPH->KPHToKnots(%v) = %v", v, got)
		}
	}
}

// --- Speed: Knots <-> Mach ---

func TestKnotsToMach(t *testing.T) {
	tests := []struct {
		name  string
		knots float64
		want  float64
	}{
		{name: "zero", knots: 0, want: 0},
		{name: "Mach 1", knots: 661.4788, want: 1.0},
		{name: "Mach 0.82 cruise", knots: 661.4788 * 0.82, want: 0.82},
		{name: "subsonic 250 kts", knots: 250, want: 250.0 / 661.4788},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KnotsToMach(tt.knots)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KnotsToMach(%v) = %v, want %v", tt.knots, got, tt.want)
			}
		})
	}
}

func TestMachToKnots(t *testing.T) {
	tests := []struct {
		name string
		mach float64
		want float64
	}{
		{name: "zero", mach: 0, want: 0},
		{name: "Mach 1", mach: 1.0, want: 661.4788},
		{name: "Mach 0.82", mach: 0.82, want: 661.4788 * 0.82},
		{name: "Mach 2", mach: 2.0, want: 661.4788 * 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MachToKnots(tt.mach)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("MachToKnots(%v) = %v, want %v", tt.mach, got, tt.want)
			}
		})
	}
}

func TestKnotsMachRoundtrip(t *testing.T) {
	values := []float64{0, 100, 250, 661.4788, 1000}
	for _, v := range values {
		got := MachToKnots(KnotsToMach(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip KnotsToMach->MachToKnots(%v) = %v", v, got)
		}
	}
}

// --- Speed: KPH <-> Mach ---

func TestKilometersPerHourToMach(t *testing.T) {
	tests := []struct {
		name string
		kph  float64
		want float64
	}{
		{name: "zero", kph: 0, want: 0},
		{name: "Mach 1", kph: 1225.044, want: 1.0},
		{name: "Mach 0.82 cruise", kph: 1225.044 * 0.82, want: 0.82},
		{name: "low speed", kph: 300, want: 300.0 / 1225.044},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KilometersPerHourToMach(tt.kph)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KilometersPerHourToMach(%v) = %v, want %v", tt.kph, got, tt.want)
			}
		})
	}
}

func TestMachToKilometersPerHour(t *testing.T) {
	tests := []struct {
		name string
		mach float64
		want float64
	}{
		{name: "zero", mach: 0, want: 0},
		{name: "Mach 1", mach: 1.0, want: 1225.044},
		{name: "Mach 0.82", mach: 0.82, want: 1225.044 * 0.82},
		{name: "Mach 2", mach: 2.0, want: 1225.044 * 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MachToKilometersPerHour(tt.mach)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("MachToKilometersPerHour(%v) = %v, want %v", tt.mach, got, tt.want)
			}
		})
	}
}

func TestKPHMachRoundtrip(t *testing.T) {
	values := []float64{0, 300, 900, 1225.044, 2500}
	for _, v := range values {
		got := MachToKilometersPerHour(KilometersPerHourToMach(v))
		if math.Abs(got-v) > epsilon {
			t.Errorf("roundtrip KPHToMach->MachToKPH(%v) = %v", v, got)
		}
	}
}

// --- ICAO Validation ---

func TestIsICAOCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		// valid codes
		{name: "KJFK (US)", code: "KJFK", want: true},
		{name: "EGLL (UK)", code: "EGLL", want: true},
		{name: "LKPR (Czech)", code: "LKPR", want: true},
		{name: "YSSY (Australia)", code: "YSSY", want: true},
		{name: "ZBAA (China)", code: "ZBAA", want: true},
		// invalid codes
		{name: "empty string", code: "", want: false},
		{name: "3 chars", code: "KJF", want: false},
		{name: "5 chars", code: "KJFKX", want: false},
		{name: "lowercase", code: "kjfk", want: false},
		{name: "ZZZZ pseudo-code", code: "ZZZZ", want: false},
		{name: "Q prefix (reserved)", code: "QXYZ", want: false},
		{name: "X prefix (reserved)", code: "XABC", want: false},
		{name: "I prefix (excluded)", code: "IABC", want: false},
		{name: "J prefix (excluded)", code: "JABC", want: false},
		{name: "numeric", code: "1234", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsICAOCode(tt.code)
			if got != tt.want {
				t.Errorf("IsICAOCode(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

// --- ICAO Region ---

func TestICAORegion(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "KJFK US", code: "KJFK", want: "Contiguous United States"},
		{name: "EGLL Northern Europe", code: "EGLL", want: "Northern Europe"},
		{name: "YSSY Australia", code: "YSSY", want: "Australia"},
		{name: "LKPR Southern Europe", code: "LKPR", want: "Southern Europe, Israel, Turkey"},
		{name: "ZBAA China", code: "ZBAA", want: "China, North Korea, Mongolia"},
		{name: "CYYZ Canada", code: "CYYZ", want: "Canada"},
		{name: "empty string", code: "", want: ""},
		{name: "9 prefix unknown", code: "9XXX", want: "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ICAORegion(tt.code)
			if got != tt.want {
				t.Errorf("ICAORegion(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// --- ICAO Country ---

func TestICAOCountry(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "EGLL via EG prefix", code: "EGLL", want: "United Kingdom (and Crown Dependencies)"},
		{name: "KJFK via K fallback", code: "KJFK", want: "Contiguous United States"},
		{name: "LKPR via LK prefix", code: "LKPR", want: "Czech Republic"},
		{name: "YSSY via Y fallback", code: "YSSY", want: "Australia (including Norfolk Island, Christmas Island, Cocos (Keeling) Islands and Australian Antarctic Territory)"},
		{name: "CYYZ via C fallback", code: "CYYZ", want: "Canada"},
		{name: "empty string", code: "", want: ""},
		{name: "single char", code: "K", want: ""},
		{name: "unknown prefix", code: "99XX", want: "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ICAOCountry(tt.code)
			if got != tt.want {
				t.Errorf("ICAOCountry(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// --- Position ---

func TestOffsetToLatLon(t *testing.T) {
	tests := []struct {
		name      string
		latRef    float64
		lonRef    float64
		xEast     float64
		zNorth    float64
		wantLat   float64
		wantLon   float64
		tolerance float64
	}{
		{
			name:   "zero offset at origin",
			latRef: 0, lonRef: 0,
			xEast: 0, zNorth: 0,
			wantLat: 0, wantLon: 0,
			tolerance: epsilon,
		},
		{
			name:   "1000m north at equator",
			latRef: 0, lonRef: 0,
			xEast: 0, zNorth: 1000,
			// 1000m north at equator: ~0.00904 degrees latitude
			wantLat:   0.009043,
			wantLon:   0,
			tolerance: epsilonDeg,
		},
		{
			name:   "1000m east at equator",
			latRef: 0, lonRef: 0,
			xEast: 1000, zNorth: 0,
			// 1000m east at equator: ~0.00899 degrees longitude
			wantLat:   0,
			wantLon:   0.008983,
			tolerance: epsilonDeg,
		},
		{
			name:   "1000m north at 60N",
			latRef: 60, lonRef: 10,
			xEast: 0, zNorth: 1000,
			// At 60N the meridian radius is slightly different
			wantLat:   60.008992,
			wantLon:   10,
			tolerance: epsilonDeg,
		},
		{
			name:   "1000m east at 60N",
			latRef: 60, lonRef: 10,
			xEast: 1000, zNorth: 0,
			// At 60N the parallel is smaller, so 1000m east produces a larger degree shift
			wantLat:   60,
			wantLon:   10.017966,
			tolerance: epsilonDeg,
		},
		{
			name:   "negative offsets at equator",
			latRef: 0, lonRef: 0,
			xEast: -1000, zNorth: -1000,
			wantLat:   -0.009043,
			wantLon:   -0.008983,
			tolerance: epsilonDeg,
		},
		{
			name:   "combined offset at mid-latitude",
			latRef: 45, lonRef: -90,
			xEast: 5000, zNorth: 5000,
			// At 45N: ~0.04499 lat, ~0.06341 lon for 5000m
			wantLat:   45.04499,
			wantLon:   -89.93659,
			tolerance: epsilonDeg,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLat, gotLon := OffsetToLatLon(tt.latRef, tt.lonRef, tt.xEast, tt.zNorth)
			if math.Abs(gotLat-tt.wantLat) > tt.tolerance {
				t.Errorf("OffsetToLatLon lat: got %v, want %v (diff %v)",
					gotLat, tt.wantLat, math.Abs(gotLat-tt.wantLat))
			}
			if math.Abs(gotLon-tt.wantLon) > tt.tolerance {
				t.Errorf("OffsetToLatLon lon: got %v, want %v (diff %v)",
					gotLon, tt.wantLon, math.Abs(gotLon-tt.wantLon))
			}
		})
	}
}
