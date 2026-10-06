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

// Crews greet on their first call to a station, in varied words and
// places; a call that is not a first call is not greeted (#721).
func TestCrewsGreetOnFirstCalls(t *testing.T) {
	r, said := varietyRadio(&Variety{Seed: 3, SayAgain: -1, ReadbackError: -1})
	for i := range 60 {
		cs := fmt.Sprintf("CSA%d", i)
		r.Transmit("LKPR", CheckIn(PosApproach, "Ruzyne Radar", cs, "flight level 100", ""))
		r.Transmit("LKPR", RequestTaxi(cs))
	}
	words := []string{"good morning", "good afternoon", "good evening", "good day", "hello", "morning", "afternoon", "evening"}
	forms, greeted := map[string]bool{}, 0
	for _, s := range *said {
		var w string
		for _, x := range words {
			if strings.Contains(strings.ToLower(s.Text), x) {
				w = x
				break
			}
		}
		if s.Intent == IntentRequestTaxi {
			if w != "" {
				t.Errorf("not a first call, greeted: %q", s.Text)
			}
			continue
		}
		if s.Params[ParamStation] != "Ruzyne Radar" || s.Intent != IntentCheckIn || !strings.Contains(s.Text, "flight level 100") {
			t.Errorf("changed: %+v", s)
		}
		if w == "" {
			continue
		}
		greeted++
		cs := s.Callsign
		switch {
		case strings.HasPrefix(s.Text, "Ruzyne Radar, "+cs):
			forms["after the call sign"] = true
		case strings.HasPrefix(s.Text, "Ruzyne Radar, "):
			forms["after the station"] = true
		default:
			forms["first"] = true
		}
		forms[w] = true
	}
	if greeted < 30 || greeted == 60 {
		t.Errorf("%d of 60 greeted: most crews should, not all", greeted)
	}
	for _, f := range []string{"after the call sign", "after the station", "first", "good afternoon", "good day", "hello"} {
		if !forms[f] {
			t.Errorf("no greeting %q in 60 calls: %v", f, forms)
		}
	}
}

// The controller's first answer to a first call is often greeted back,
// after the call sign; later ones are not (#721).
func TestControllersGreetBack(t *testing.T) {
	r, said := varietyRadio(&Variety{Seed: 5, SayAgain: -1, ReadbackError: -1})
	for i := range 40 {
		cs := fmt.Sprintf("CSA%d", i)
		r.Transmit("LKPR", CheckIn(PosApproach, "Ruzyne Radar", cs, "flight level 100", ""))
		r.Transmit("LKPR", Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHeading, Params: map[string]string{ParamTurn: "left", ParamHeading: "270"}})
		r.Transmit("LKPR", Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHeading, Params: map[string]string{ParamTurn: "left", ParamHeading: "250"}})
	}
	back := 0
	for _, s := range *said {
		if s.Pilot {
			continue
		}
		g := strings.Contains(s.Text, "good ") || strings.Contains(s.Text, "hello")
		if g && s.Params[ParamHeading] == "250" {
			t.Errorf("a later call greeted: %q", s.Text)
		}
		if g {
			back++
			if !strings.HasPrefix(s.Text, s.Callsign+", ") || !strings.HasSuffix(s.Text, "turn left heading 270") {
				t.Errorf("greeting not after the call sign: %q", s.Text)
			}
		}
	}
	if back < 12 || back == 40 {
		t.Errorf("%d of 40 first answers greeted back", back)
	}
}

// A crew that misses a handoff: no readback, the controller calls again
// after a silence, then the readback (#721).
func TestMissedCall(t *testing.T) {
	r, said := varietyRadio(&Variety{Seed: 2, SayAgain: -1, ReadbackError: -1, MissedCall: 1})
	r.Transmit("LKPR", Handoff("CSA1", PosGround, PosTower, "Ruzyne Tower", "118.105"))
	s := *said
	if len(s) != 3 || s[0].Pilot || s[1].Pilot || !s[2].Pilot || s[1].Params[ParamRepeat] != "1" {
		t.Fatalf("said %+v", s)
	}
	if gap := s[1].At.Sub(s[0].At.Add(SpeakingTime(s[0].Text))); gap < 6*time.Second {
		t.Errorf("called again %v after the first call", gap)
	}
}

// A wrong altitude is a thousand feet off, never "6010" (live, TVS1539).
func TestWrongValue(t *testing.T) {
	for _, c := range []struct{ k, v, want string }{
		{ParamAltitude, "6000", "7000"},
		{ParamAltitude, "12000", "13000"},
		{ParamAltitude, "900", "900"}, // too few digits: not made wrong
		{ParamHeading, "270", "280"},
		{ParamSquawk, "4521", "4531"},
	} {
		if got := wrongValue(c.k, c.v); got != c.want {
			t.Errorf("%s %s: %s, want %s", c.k, c.v, got, c.want)
		}
	}
}
