package convert

import (
	"math"
	"testing"
)

func TestCelsiusToFahrenheit(t *testing.T) {
	tests := []struct {
		name    string
		celsius float64
		want    float64
	}{
		{name: "freezing point", celsius: 0, want: 32},
		{name: "boiling point", celsius: 100, want: 212},
		{name: "negative forty — same in both scales", celsius: -40, want: -40},
		{name: "body temperature", celsius: 37, want: 98.6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CelsiusToFahrenheit(tt.celsius)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("CelsiusToFahrenheit(%v) = %v, want %v", tt.celsius, got, tt.want)
			}
		})
	}
}

func TestFahrenheitToCelsius(t *testing.T) {
	tests := []struct {
		name       string
		fahrenheit float64
		want       float64
	}{
		{name: "freezing point", fahrenheit: 32, want: 0},
		{name: "boiling point", fahrenheit: 212, want: 100},
		{name: "negative forty — same in both scales", fahrenheit: -40, want: -40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FahrenheitToCelsius(tt.fahrenheit)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("FahrenheitToCelsius(%v) = %v, want %v", tt.fahrenheit, got, tt.want)
			}
		})
	}
}

func TestCelsiusToKelvin(t *testing.T) {
	tests := []struct {
		name    string
		celsius float64
		want    float64
	}{
		{name: "absolute zero in Celsius", celsius: -273.15, want: 0},
		{name: "freezing point", celsius: 0, want: 273.15},
		{name: "boiling point", celsius: 100, want: 373.15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CelsiusToKelvin(tt.celsius)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("CelsiusToKelvin(%v) = %v, want %v", tt.celsius, got, tt.want)
			}
		})
	}
}

func TestKelvinToCelsius(t *testing.T) {
	tests := []struct {
		name   string
		kelvin float64
		want   float64
	}{
		{name: "absolute zero", kelvin: 0, want: -273.15},
		{name: "freezing point", kelvin: 273.15, want: 0},
		{name: "boiling point", kelvin: 373.15, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KelvinToCelsius(tt.kelvin)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KelvinToCelsius(%v) = %v, want %v", tt.kelvin, got, tt.want)
			}
		})
	}
}

func TestFahrenheitToKelvin(t *testing.T) {
	tests := []struct {
		name       string
		fahrenheit float64
		want       float64
	}{
		{name: "freezing point", fahrenheit: 32, want: 273.15},
		{name: "boiling point", fahrenheit: 212, want: 373.15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FahrenheitToKelvin(tt.fahrenheit)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("FahrenheitToKelvin(%v) = %v, want %v", tt.fahrenheit, got, tt.want)
			}
		})
	}
}

func TestKelvinToFahrenheit(t *testing.T) {
	tests := []struct {
		name   string
		kelvin float64
		want   float64
	}{
		{name: "freezing point", kelvin: 273.15, want: 32},
		{name: "boiling point", kelvin: 373.15, want: 212},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KelvinToFahrenheit(tt.kelvin)
			if math.Abs(got-tt.want) > epsilon {
				t.Errorf("KelvinToFahrenheit(%v) = %v, want %v", tt.kelvin, got, tt.want)
			}
		})
	}
}

func TestTemperatureRoundTrips(t *testing.T) {
	inputs := []float64{-273.15, -40, 0, 20, 37, 100, 500}
	for _, c := range inputs {
		// Celsius → Fahrenheit → Celsius
		if got := FahrenheitToCelsius(CelsiusToFahrenheit(c)); math.Abs(got-c) > epsilon {
			t.Errorf("C→F→C round-trip for %v: got %v", c, got)
		}
		// Celsius → Kelvin → Celsius
		if got := KelvinToCelsius(CelsiusToKelvin(c)); math.Abs(got-c) > epsilon {
			t.Errorf("C→K→C round-trip for %v: got %v", c, got)
		}
	}
}
