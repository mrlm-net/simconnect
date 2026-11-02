package traffic

import (
	"strings"
	"testing"
	"time"
)

func TestAbbreviatedRegistration(t *testing.T) {
	for cs, want := range map[string]string{
		"OKXSP":   "Oscar Sierra Papa",
		"OK-XSP":  "Oscar Sierra Papa",
		"N123AB":  "November Alpha Bravo",
		"DLH1675": "", // a flight number: no abbreviated form
		"CSA12":   "",
		"Tug 3":   "",
		"ABC":     "",
		"":        "",
	} {
		if got := AbbreviatedRegistration(cs); got != want {
			t.Errorf("%q: %q, want %q", cs, got, want)
		}
	}
}

// exchange is a radio whose transmissions say call signs, with a frequency by
// position.
func exchange(t *testing.T) (say func(pilot bool, pos Position, cs, text string) string) {
	t.Helper()
	at := time.Date(2026, 11, 2, 22, 0, 0, 0, time.UTC)
	c := ScheduleConfig{}
	r := NewRadio(RadioOptions{SaidCallsign: c.SaidCallsign, Now: func() time.Time { return at },
		FrequencyOf: func(_ string, p Position) string {
			if p == PosApproach {
				return "120.525"
			}
			return "118.105"
		}})
	return func(pilot bool, pos Position, cs, text string) string {
		at = at.Add(time.Minute)
		return r.Transmit("LKPR", Transmission{Pilot: pilot, Position: pos, Callsign: cs, Text: cs + ", " + text}).Text
	}
}

// Full on first contact both ways; the controller abbreviates first, the crew
// only after it; a new frequency starts over (#1412).
func TestRegistrationAbbreviatedByTheControllerFirst(t *testing.T) {
	say := exchange(t)
	const full, short = "Oscar Kilo X-ray Sierra Papa", "Oscar Sierra Papa"
	for i, c := range []struct {
		pilot bool
		pos   Position
		want  string
	}{
		{true, PosTower, full},   // established
		{false, PosTower, full},  // good day, continue
		{true, PosTower, full},   // readback: nobody abbreviated yet
		{false, PosTower, short}, // the controller does
		{true, PosTower, short},  // and the crew follows
		{false, PosTower, short},
		{true, PosApproach, full}, // going around, on approach's frequency
		{false, PosApproach, full},
		{false, PosApproach, short},
	} {
		got := say(c.pilot, c.pos, "OKXSP", "text")
		if !strings.HasPrefix(got, c.want+",") {
			t.Errorf("transmission %d: %q, want %q", i, got, c.want)
		}
	}
}

// The same abbreviation on the frequency: the full call sign for both.
func TestRegistrationFullWhenAbbreviationsClash(t *testing.T) {
	say := exchange(t)
	const full = "Oscar Kilo X-ray Sierra Papa"
	say(true, PosTower, "OKXSP", "text")
	say(false, PosTower, "OKXSP", "text")
	if got := say(false, PosTower, "OKXSP", "text"); !strings.HasPrefix(got, "Oscar Sierra Papa,") {
		t.Fatalf("alone on the frequency: %q", got)
	}
	if got := say(true, PosTower, "OMASP", "text"); !strings.HasPrefix(got, "Oscar Mike Alpha Sierra Papa,") {
		t.Errorf("the other: %q", got)
	}
	if got := say(false, PosTower, "OKXSP", "text"); !strings.HasPrefix(got, full+",") {
		t.Errorf("with OMASP on the frequency: %q", got)
	}
	if got := say(true, PosTower, "OKXSP", "text"); !strings.HasPrefix(got, full+",") {
		t.Errorf("the crew with OMASP on the frequency: %q", got)
	}
}

// A flight number's call sign is said in full throughout.
func TestAirlineCallSignNotAbbreviated(t *testing.T) {
	say := exchange(t)
	for range 4 {
		if got := say(false, PosTower, "XXX123", "text"); !strings.HasPrefix(got, "XXX123,") {
			t.Errorf("%q", got)
		}
	}
}
