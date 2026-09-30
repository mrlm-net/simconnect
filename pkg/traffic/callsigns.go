//go:build windows
// +build windows

package traffic

import (
	"strings"
	"unicode"
)

// Callsigns and destinations as said on the radio (#462): what the text of
// a transmission shows is exactly what a voice says, so the call sign is
// written with the airline's telephony designator ("Lufthansa 1675", not
// "DLH1675") and a destination with its name ("Frankfurt", not "EDDF").

// SaidTelephony writes a telephony designator as said: each word
// capitalised ("SPEEDBIRD" → "Speedbird", "CSA LINES" → "CSA Lines"),
// except words of up to three letters, which are initialisms said letter by
// letter ("KLM", "CSA").
func SaidTelephony(tel string) string {
	words := strings.Fields(tel)
	for i, w := range words {
		if len(w) <= 3 {
			words[i] = strings.ToUpper(w)
			continue
		}
		r := []rune(strings.ToLower(w))
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// SaidCallsign is flight call sign cs as said: the airline's telephony
// designator, then the flight number ("DLH1675" → "Lufthansa 1675"). A call
// sign of an airline without a telephony designator, or not of the form
// three letters and a number, is returned as it is.
func (c ScheduleConfig) SaidCallsign(cs string) string {
	if len(cs) < 4 || cs[3] < '0' || cs[3] > '9' {
		return cs
	}
	for _, a := range c.Airlines {
		if strings.EqualFold(a.ICAO, cs[:3]) && a.Telephony != "" {
			return SaidTelephony(a.Telephony) + " " + cs[3:]
		}
	}
	return cs
}

// AirportName is airport icao as ATC names it in a clearance ("Frankfurt");
// its ICAO code when the schedule has no name for it.
func (c ScheduleConfig) AirportName(icao string) string {
	for _, a := range c.Airports {
		if strings.EqualFold(a.ICAO, icao) && a.Name != "" {
			return a.Name
		}
	}
	return icao
}
