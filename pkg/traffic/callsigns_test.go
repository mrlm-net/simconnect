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
		"LOT844": "Pollot 844", "BAW452": "Speedbird 452", "TVS795": "Skytravel 795", "EZY12AB": "Easy 12AB",
		"AFR903": "Airfrans 903", "QTR1489": "Qatari 1489", "KAL1573": "Koreanair 1573", "ENT464": "Enter 464",
		"QQQ123": "QQQ123", "ZZZ1": "ZZZ1",
		// Registrations, letter by letter (live, "OKVQY" was read as a word).
		"OKABC": "Oscar Kilo Alpha Bravo Charlie", "OK-VQY": "Oscar Kilo Victor Quebec Yankee", "N123AB": "November 1 2 3 Alpha Bravo",
		// Not in the schedule: the built-in designators (Telephony).
		"CEF001": "Czech Air Force 001", "GAF615": "German Air Force 615", "RCH4021": "Reach 4021", "UPS2938": "UPS 2938",
		"BMW1": "BMW Flight 1", "XYZ123": "Rainbird 123",
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
	r.Transmit("LKPR", ClearedTakeoff("DLH1675", "24", ""))
	if len(said) != 2 {
		t.Fatalf("%d transmissions, want the clearance and its readback", len(said))
	}
	for _, tx := range said {
		if strings.Contains(tx.Text, "DLH1675") || !strings.Contains(tx.Text, "Lufthansa 1675") || tx.Callsign != "DLH1675" {
			t.Errorf("%+v", tx)
		}
	}
}
