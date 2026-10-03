//go:build windows
// +build windows

package traffic

import "testing"

func TestSquawks(t *testing.T) {
	var s Squawks
	a, err := s.Assign("CSA1")
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := s.Assign("CSA1"); again != a {
		t.Errorf("CSA1 got %s then %s", a, again)
	}
	if !validOctal(a) || a < "4001" || a > "4777" || SpecialSquawk(a) {
		t.Errorf("code %s", a)
	}
	// A bank of three with 7000 in it: two codes, then full.
	small := Squawks{First: "6777", Last: "7001"}
	got := map[string]bool{}
	for _, cs := range []string{"A", "B"} {
		c, err := small.Assign(cs)
		if err != nil || c == "7000" || got[c] {
			t.Fatalf("%s: %s %v", cs, c, err)
		}
		got[c] = true
	}
	if _, err := small.Assign("C"); err == nil {
		t.Error("full bank gave a code")
	}
	small.Release("A")
	if _, err := small.Assign("C"); err != nil {
		t.Errorf("after a release: %v", err)
	}
	if _, err := (&Squawks{First: "4008"}).Assign("X"); err == nil {
		t.Error("non-octal bank accepted")
	}
}

func validOctal(s string) bool {
	for _, c := range s {
		if c < '0' || c > '7' {
			return false
		}
	}
	return len(s) == 4
}

// CAP 413 Figure 24, with a route and a squawk.
func TestVFRDeparture(t *testing.T) {
	d := VFRDeparture{Turn: "left", MaxFt: 2500}
	if got := VFRDepartureInstructions(PosTower, "G-CD", d).Text; got != "G-CD, after departure, left turn approved, climb not above altitude 2500 feet until reaching the zone boundary" {
		t.Errorf("%q", got)
	}
	if got := VFRDepartureReadback(PosTower, "G-CD", d).Text; got != "Left turn approved, not above altitude 2500 feet until zone boundary, G-CD" {
		t.Errorf("readback %q", got)
	}
	d = VFRDeparture{Via: "Sierra", MaxFt: 2500, Squawk: "7000"}
	if got := VFRDepartureInstructions(PosTower, "OK-MYC", d).Text; got != "OK-MYC, after departure, route via Sierra, climb not above altitude 2500 feet until reaching the zone boundary, squawk 7000" {
		t.Errorf("%q", got)
	}
}
