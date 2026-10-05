package traffic

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// varietyRadio is a radio with variety v whose transmissions are collected.
func varietyRadio(v *Variety) (*Radio, *[]Transmission) {
	var said []Transmission
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	r := NewRadio(RadioOptions{Now: func() time.Time { now = now.Add(20 * time.Second); return now }, ReadBack: true, Variety: v,
		FrequencyOf:    func(string, Position) string { return "121.910" },
		OnTransmission: func(t Transmission) { said = append(said, t) }})
	return r, &said
}

func TestVarietyOffSaysAsBefore(t *testing.T) {
	r, said := varietyRadio(nil)
	r.Transmit("LKPR", Handoff("CSA1", PosGround, PosTower, "Ruzyne Tower", "118.105"))
	if len(*said) != 2 || (*said)[0].Text != "CSA1, contact Ruzyne Tower 118.105" || (*said)[1].Text != "Ruzyne Tower 118.105, CSA1" {
		t.Fatalf("said %+v", *said)
	}
}

func TestVarietySeededAndExact(t *testing.T) {
	run := func(seed uint64) string {
		r, said := varietyRadio(&Variety{Seed: seed, SayAgain: 0.1, ReadbackError: 0.1})
		for i := range 40 {
			cs := fmt.Sprintf("CSA%d", i)
			r.Transmit("LKPR", Handoff(cs, PosGround, PosTower, "Ruzyne Tower", "118.105"))
			r.Transmit("LKPR", Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHeading,
				Params: map[string]string{ParamTurn: "left", ParamHeading: "270"}})
		}
		var b strings.Builder
		for _, s := range *said {
			fmt.Fprintf(&b, "%s %s %s\n", s.At.Format("15:04:05.000"), s.Intent, s.Text)
			if s.Intent == IntentContact && s.Params[ParamFreq] != "118.105" {
				t.Errorf("params changed: %+v", s)
			}
		}
		return b.String()
	}
	a, b := run(7), run(7)
	if a != b {
		t.Fatal("same seed, different radio")
	}
	if a == run(8) {
		t.Error("another seed says the same")
	}
	for _, want := range []string{"good day", string(IntentPilotSayAgain), "negative", "heading 280"} {
		if !strings.Contains(a, want) {
			t.Errorf("no %q in 80 clearances:\n%s", want, a)
		}
	}
}

func TestVarietyCorrectionThenRightReadback(t *testing.T) {
	r, said := varietyRadio(&Variety{Seed: 1, SayAgain: -1, ReadbackError: 1})
	r.Transmit("LKPR", Transmission{Position: PosApproach, Callsign: "CSA1", Intent: IntentHeading,
		Params: map[string]string{ParamTurn: "left", ParamHeading: "270"}})
	var texts []string
	for _, s := range *said {
		texts = append(texts, s.Text)
	}
	want := []string{"CSA1, turn left heading 270", "Turn left heading 280, CSA1",
		"CSA1, negative, turn left heading 270", "Turn left heading 270, CSA1"}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Fatalf("said %q, want %q", texts, want)
	}
	for i := 1; i < len(*said); i++ {
		if (*said)[i].At.Before((*said)[i-1].At.Add(SpeakingTime((*said)[i-1].Text))) {
			t.Errorf("%q stepped on %q", (*said)[i].Text, (*said)[i-1].Text)
		}
	}
}

func TestWrongDigit(t *testing.T) {
	for in, want := range map[string]string{"270": "280", "121.910": "121.920", "4521": "4531", "FL240": "FL250", "5": "5", "090": "000"} {
		if got := wrongDigit(in); got != want {
			t.Errorf("wrongDigit(%q) = %q, want %q", in, got, want)
		}
	}
}
