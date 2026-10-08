package world

import "testing"

// TestTypeSaid: types said by name, not their ICAO code (live, "behind the
// landing DH8D").
func TestTypeSaid(t *testing.T) {
	for icao, want := range map[string]string{
		"DH8D": "Dash 8", "B38M": "Boeing 737 MAX", "B738": "Boeing 737", "BCS3": "Airbus A220",
		"PC12": "Pilatus PC-12", "A20N": "Airbus A320neo", "A321": "Airbus A321", "": "aircraft",
	} {
		if got := typeSaid(icao); got != want {
			t.Errorf("%q: %q, want %q", icao, got, want)
		}
	}
}
