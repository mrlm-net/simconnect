package traffic

import (
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestSelfAnnounce: the calls to a field's traffic, said to "<Field>
// Traffic" and closed with the field's name, nobody answering; the IFR
// release and cancellation with approach (#1296).
func TestSelfAnnounce(t *testing.T) {
	for _, c := range []struct {
		tx   Transmission
		want string
	}{
		{SelfTaxiing("Henderson", "OKABC", "24"), "Henderson Traffic, OKABC, taxiing to runway 24, Henderson"},
		{SelfDeparting("Henderson", "OKABC", "24", "to the north"), "Henderson Traffic, OKABC, departing runway 24, to the north, Henderson"},
		{SelfDeparting("Henderson", "OKABC", "24", PatternStay), "Henderson Traffic, OKABC, departing runway 24, remaining in the pattern, Henderson"},
		{SelfInbound("Henderson", "OKABC", 10, "south"), "Henderson Traffic, OKABC, 10 miles south, inbound for landing, Henderson"},
		{SelfInbound("Henderson", "OKABC", 1, ""), "Henderson Traffic, OKABC, 1 mile, inbound for landing, Henderson"},
		{SelfPattern("Henderson", "OKABC", "left downwind", "24"), "Henderson Traffic, OKABC, left downwind runway 24, Henderson"},
		{SelfClear("Henderson", "OKABC", "24"), "Henderson Traffic, OKABC, clear of runway 24, Henderson"},
		{ReleasedForDeparture("OKABC"), "OKABC, released for departure, report airborne"},
		{CancelIFR("Portland Approach", "OKABC"), "Portland Approach, OKABC, cancel IFR"},
		{IFRCancelled("OKABC"), "OKABC, IFR cancellation received"},
	} {
		if c.tx.Text != c.want {
			t.Errorf("%q, want %q", c.tx.Text, c.want)
		}
	}
	tx := SelfTaxiing("Henderson", "OKABC", "24")
	if tx.Position != PosTraffic || !tx.Pilot || tx.Intent != IntentSelfAnnounce || tx.Params[ParamSelf] != "taxiing" || tx.Params[ParamRunway] != "24" {
		t.Errorf("%+v", tx)
	}
}

// withCircuit is t with its position as said.
func withCircuit(t Transmission, where string) Transmission {
	t.Params[ParamCircuit] = where
	return t
}

// withRunway is t with its runway.
func withRunway(t Transmission, rwy string) Transmission {
	t.Params[ParamRunway] = rwy
	return t
}

// TestRadioUncontrolled: at a field without a tower the radio says the
// crews' own announcements for the clearances our controllers still decide,
// on the common frequency, nobody answering; the other calls of tower and
// ground are not said; approach stays as it is (#1296).
func TestRadioUncontrolled(t *testing.T) {
	var heard []Transmission
	r := NewRadio(RadioOptions{ReadBack: true, OnTransmission: func(tx Transmission) { heard = append(heard, tx) },
		Uncontrolled: func(a string) (UncontrolledField, bool) {
			return UncontrolledField{Name: "Henderson", Freq: "118.10"}, a == "AGGH"
		}})
	taxi := Transmission{Position: PosGround, Callsign: "OKABC", Intent: IntentTaxi, Params: map[string]string{ParamRunway: "24"}}
	toStand := Transmission{Position: PosGround, Callsign: "OKABC", Intent: IntentTaxi, Params: map[string]string{ParamStand: "A1"}}
	for _, c := range []struct {
		tx   Transmission
		want []string
		freq string
	}{
		{taxi, []string{"Henderson Traffic, OKABC, taxiing to runway 24, Henderson"}, "118.10"},
		{ClearedTakeoff("OKABC", "24", ""), []string{"Henderson Traffic, OKABC, departing runway 24, Henderson"}, "118.10"},
		{ClearedToLand("OKABC", "24", ""), []string{"Henderson Traffic, OKABC, final runway 24, Henderson"}, "118.10"},
		{Vacated("OKABC", "24"), []string{"Henderson Traffic, OKABC, clear of runway 24, Henderson"}, "118.10"},
		{CheckIn(PosGround, "Ground", "OKABC", "runway 24 vacated at D", ""), []string{"Henderson Traffic, OKABC, clear of runway 24, Henderson"}, "118.10"},
		{CheckIn(PosTower, "Tower", "OKABC", "final runway 24", ""), nil, ""},
		{VFRForLanding("Tower", "OKABC", "Cessna", "10 miles south", "2000 feet", ""), []string{"Henderson Traffic, OKABC, 10 miles south, inbound for landing, Henderson"}, "118.10"},
		{withCircuit(CheckIn(PosTower, "Tower", "OKABC", "final runway 24", ""), "8 miles east"), []string{"Henderson Traffic, OKABC, 8 miles east, inbound for landing, Henderson"}, "118.10"},
		{withRunway(CircuitReport("OKABC", "left downwind"), "24"), []string{"Henderson Traffic, OKABC, left downwind runway 24, Henderson"}, "118.10"},
		{withRunway(CircuitReport("OKABC", "right base"), "24"), []string{"Henderson Traffic, OKABC, right base runway 24, Henderson"}, "118.10"},
		{withRunway(CircuitReport("OKABC", "left downwind, touch and go"), "24"), []string{"Henderson Traffic, OKABC, left downwind runway 24, touch and go, Henderson"}, "118.10"},
		{CircuitReport("OKABC", "downwind"), nil, ""},
		{toStand, nil, ""},
		{HoldPosition("OKABC"), nil, ""},
		{pilotTx(PosTower, "OKABC", IntentCheckIn, nil, "Tower, OKABC"), nil, ""},
		{Handoff("OKABC", PosApproach, PosTower, "Henderson Tower", "118.10"), []string{"OKABC, change to advisory frequency approved", "Change to advisory frequency, OKABC"}, ""},
	} {
		heard = nil
		said := r.Transmit("AGGH", c.tx)
		var got []string
		for _, tx := range heard {
			got = append(got, tx.Text)
			if c.freq != "" && tx.Frequency != c.freq {
				t.Errorf("%q on %q, want %s", tx.Text, tx.Frequency, c.freq)
			}
		}
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%v: said %q, want %q", c.tx.Intent, got, c.want)
		}
		if len(c.want) == 0 && said.Text != "" {
			t.Errorf("%v: returned %q, nothing said", c.tx.Intent, said.Text)
		}
	}
	heard = nil
	r.Transmit("LKPR", ClearedTakeoff("CSA1", "24", ""))
	if len(heard) != 2 || heard[0].Intent != IntentTakeoff {
		t.Errorf("a towered airport: %+v", heard)
	}
}

// TestFieldCallName: the name a field's traffic is called by.
func TestFieldCallName(t *testing.T) {
	for _, c := range []struct {
		l    airport.Layout
		want string
	}{
		{airport.Layout{ICAO: "AGGH", Frequencies: []airport.Frequency{{Kind: airport.FreqCTAF, MHz: 118.1, Name: "HENDERSON INFORMATION"}}}, "Henderson"},
		{airport.Layout{ICAO: "KVUO", Name: "Pearson Field"}, "Pearson Field"},
		{airport.Layout{ICAO: "XXXX", Name: "Some Town International Airport"}, "Some Town"},
		{airport.Layout{ICAO: "XXXX"}, "XXXX"},
	} {
		if got := FieldCallName(&c.l); got != c.want {
			t.Errorf("%s: %q, want %q", c.l.ICAO, got, c.want)
		}
	}
}
