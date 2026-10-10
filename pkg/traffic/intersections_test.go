package traffic

import (
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

func TestIntersectionClass(t *testing.T) {
	for model, want := range map[string]string{
		"A320": ClassJet, "B738": ClassJet, "CRJ9": ClassRegional, "E190": ClassRegional, "AT76": ClassTurboprop,
		"DH8D": ClassTurboprop, "C172": ClassLight, "B77W": ClassHeavy, "A388": ClassHeavy,
	} {
		if got := IntersectionClass(model); got != want {
			t.Errorf("%s: %s, want %s", model, got, want)
		}
	}
}

// The rules by class, and a local override per value (pkg/dict).
func TestIntersectionRule(t *testing.T) {
	if m, full := IntersectionRule("A320"); full || m != 2500 {
		t.Errorf("A320: %.0f m, full length %v", m, full)
	}
	if _, full := IntersectionRule("B77W"); !full {
		t.Error("B77W not full length only")
	}
	if err := dict.Use("traffic.intersectionClasses", []byte(`[{"class":"jet","minRemainingM":2700}]`)); err != nil {
		t.Fatal(err)
	}
	defer dict.Reset("traffic.intersectionClasses")
	if m, _ := IntersectionRule("A320"); m != 2700 {
		t.Errorf("A320 overridden: %.0f m", m)
	}
	if DeclineChance("A320", "CSA123") == 0 {
		t.Error("overriding the runway left lost the decline share")
	}
}

// The chance stays within the class's share times 0.5–1.5 and 0.5–2, the
// same for a flight every time; widebodies never decline (never offered).
func TestDeclineChance(t *testing.T) {
	share := intersectionClassesNow.Load()[ClassJet].DeclineShare
	for _, cs := range []string{"CSA123", "DLH4AB", "RYR1527", "EZY77", "WZZ1"} {
		c := DeclineChance("A320", cs)
		if c < share*0.25 || c > share*3 || c != DeclineChance("A320", cs) {
			t.Errorf("%s: %.3f (share %.2f)", cs, c, share)
		}
	}
	if c := DeclineChance("B77W", "BAW1"); c != 0 {
		t.Errorf("B77W declines %.3f", c)
	}
}

func TestIntersectionPhrases(t *testing.T) {
	tx := Say(Transmission{Position: PosGround, Callsign: "AFR1910", Intent: IntentTaxi,
		Params: map[string]string{ParamRunway: "26L", ParamEntry: "B12", ParamRemaining: "2800", ParamTaxiways: "B"}})
	if !strings.Contains(tx.Text, "runway 26L at B12, 2800 metres available") {
		t.Errorf("taxi with the runway left: %q", tx.Text)
	}
	tx.Params[ParamNoReadback] = "1"
	if _, ok := Readback(tx); ok {
		t.Error("read back with ParamNoReadback")
	}
	if u := UnableIntersection("AFR1910"); !u.Pilot || u.Text != "Unable intersection, request full length, AFR1910" {
		t.Errorf("unable: %+v", u)
	}
}
