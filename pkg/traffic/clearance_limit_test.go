package traffic

import "testing"

// TestClearanceLimit: the city by default, the airport's own name at a
// city of several, EHAM Schiphol; a local table wins, and an empty local
// name says the city again.
func TestClearanceLimit(t *testing.T) {
	defer SetClearanceLimits(nil)
	for icao, want := range map[string]string{"LOWW": "Vienna", "EGLL": "Heathrow", "LFPO": "Orly", "EHAM": "Schiphol", "ZZZZ": "ZZZZ"} {
		city := map[string]string{"LOWW": "Vienna", "EGLL": "London", "LFPO": "Paris", "EHAM": "Amsterdam"}[icao]
		if got := ClearanceLimit(icao, city); got != want {
			t.Errorf("%s: %q, want %q", icao, got, want)
		}
	}
	SetClearanceLimits(map[string]string{"loww": "Schwechat", "EHAM": ""})
	if got := ClearanceLimit("LOWW", "Vienna"); got != "Schwechat" {
		t.Errorf("local LOWW: %q", got)
	}
	if got := ClearanceLimit("EHAM", "Amsterdam"); got != "Amsterdam" {
		t.Errorf("EHAM removed locally: %q", got)
	}
	if got := ClearanceLimit("EGKK", "London"); got != "Gatwick" {
		t.Errorf("built-in kept: %q", got)
	}
}
