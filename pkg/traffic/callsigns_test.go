//go:build windows
// +build windows

package traffic

import (
	"strings"
	"testing"
)

// Call signs as said, from the verified telephony designators (#462).
func TestSaidCallsign(t *testing.T) {
	c := DefaultScheduleConfig()
	for cs, want := range map[string]string{
		"DLH1675": "Lufthansa 1675", "CSA273": "CSA Lines 273", "KLM594": "KLM 594", "WZZ1529": "Wizzair 1529",
		"LOT844": "LOT 844", "BAW452": "Speedbird 452", "TVS795": "Skytravel 795", "EZY12AB": "Easy 12AB",
		"AFR903": "Airfrans 903", "QTR1489": "Qatari 1489", "KAL1573": "Koreanair 1573", "ENT464": "Enter 464",
		"XYZ123": "XYZ123", "OKABC": "OKABC", "TST1": "TST1",
	} {
		if got := c.SaidCallsign(cs); got != want {
			t.Errorf("%s: %q, want %q", cs, got, want)
		}
	}
	for _, a := range c.Airlines {
		if a.Telephony == "" {
			t.Errorf("%s has no telephony designator", a.ICAO)
		}
	}
	for _, a := range c.Airports {
		if a.Name == "" {
			t.Errorf("%s has no name", a.ICAO)
		}
	}
	if got := c.AirportName("EDDF"); got != "Frankfurt" {
		t.Errorf("EDDF: %q", got)
	}
}

// The radio writes the call sign as said in every transmission, the
// readback included; Callsign keeps the ICAO form.
func TestRadioSaysCallsign(t *testing.T) {
	c := DefaultScheduleConfig()
	var said []Transmission
	r := NewRadio(RadioOptions{ReadBack: true, SaidCallsign: c.SaidCallsign, OnTransmission: func(tx Transmission) { said = append(said, tx) }})
	r.Transmit("LKPR", ClearedTakeoff("DLH1675", "24", false))
	if len(said) != 2 {
		t.Fatalf("%d transmissions, want the clearance and its readback", len(said))
	}
	for _, tx := range said {
		if strings.Contains(tx.Text, "DLH1675") || !strings.Contains(tx.Text, "Lufthansa 1675") || tx.Callsign != "DLH1675" {
			t.Errorf("%+v", tx)
		}
	}
}
