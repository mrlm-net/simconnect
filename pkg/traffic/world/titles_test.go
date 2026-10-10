package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/flight"
)

// Without a connection there are no models: none.
func TestTitleForNotConnected(t *testing.T) {
	w := New(Options{})
	if title, ok := w.TitleFor("A320", "CSA123"); ok || title != "" {
		t.Errorf("TitleFor before connecting: %q, %v", title, ok)
	}
	if got := w.FleetFallback()(&flight.SceneAircraft{Type: "A320", Callsign: "CSA123"}); got != "" {
		t.Errorf("fallback before connecting: %q", got)
	}
}

// Connected: the type in the airline's livery, else the type in another.
func TestTitleFor(t *testing.T) {
	w := New(Options{})
	w.st.control = &controlCenter{models: map[string]bool{
		"FSLTL_FAIB_A320_CSA-Czech Airlines": true,
		"FSLTL_FAIB_A320_DLH-Lufthansa":      true,
		"FSLTL_FAIB_B738_RYR-Ryanair":        true,
	}}
	for _, c := range []struct{ typ, cs, want string }{
		{"A320", "CSA123", "FSLTL_FAIB_A320_CSA-Czech Airlines"},
		{"A320", "DLH4AB", "FSLTL_FAIB_A320_DLH-Lufthansa"},
		{"B738", "OKABC", "FSLTL_FAIB_B738_RYR-Ryanair"},
	} {
		if got, ok := w.TitleFor(c.typ, c.cs); !ok || got != c.want {
			t.Errorf("TitleFor(%s, %s) = %q, %v; want %q", c.typ, c.cs, got, ok, c.want)
		}
	}
	if got := w.FleetFallback()(&flight.SceneAircraft{Type: "A320", Callsign: "CSA9"}); got != "FSLTL_FAIB_A320_CSA-Czech Airlines" {
		t.Errorf("fallback %q", got)
	}
}
