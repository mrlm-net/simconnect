package traffic

import (
	_ "embed"
	"strings"
	"sync"
	"unicode"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Callsigns and destinations as said on the radio (#462): what the text of
// a transmission shows is exactly what a voice says, so the call sign is
// written with the airline's telephony designator ("Lufthansa 1675", not
// "DLH1675") and a destination with its name ("Frankfurt", not "EDDF").

// SaidTelephony writes a telephony designator as said: each word
// capitalised ("SPEEDBIRD" → "Speedbird", "CZECH AIR FORCE" → "Czech Air
// Force"), except initialisms said letter by letter ("KLM", "CSA Lines"):
// short words without a vowel, and the few with one (initialisms).
func SaidTelephony(tel string) string {
	words := strings.Fields(strings.ReplaceAll(tel, "-", " "))
	for i, w := range words {
		if initialism(w) {
			words[i] = strings.ToUpper(w)
			continue
		}
		r := []rune(strings.ToLower(w))
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// initialisms with a vowel, said letter by letter.
var initialisms = map[string]bool{"CSA": true, "VIP": true, "ABC": true, "UPS": true, "TWA": true, "USA": true, "LTU": true, "AMC": true, "CGI": true, "ATA": true}

// initialism: a word said letter by letter: up to three letters with no
// vowel ("KLM", "BMW"), a single letter, or a known one ("CSA", "UPS").
func initialism(w string) bool {
	w = strings.ToUpper(w)
	if len(w) > 3 {
		return false
	}
	return initialisms[w] || !strings.ContainsAny(w, "AEIOUY") || len(w) == 1
}

// SaidCallsign is flight call sign cs as said: the airline's telephony
// designator, then the flight number ("DLH1675" → "Lufthansa 1675",
// "CEF001" → "Czech Air Force 001"). The schedule's airlines first, then
// the ICAO designators of the world (Telephony). A call sign of an
// operator without one is returned as it is. One not of the form three
// letters and a number is a registration, said letter by letter in the
// phonetic alphabet, digits as digits ("OKVQY" → "Oscar Kilo Victor Quebec
// Yankee", "N123AB" → "November 1 2 3 Alpha Bravo").
func (c ScheduleConfig) SaidCallsign(cs string) string {
	if strings.Contains(cs, " ") {
		return cs // as said already: "Tug 3" (#752)
	}
	if len(cs) < 4 || cs[3] < '0' || cs[3] > '9' || strings.ContainsAny(cs[:3], "0123456789") {
		return SaidRegistration(cs)
	}
	for _, a := range c.Airlines {
		if strings.EqualFold(a.ICAO, cs[:3]) && a.Telephony != "" {
			return SaidTelephony(a.Telephony) + " " + cs[3:]
		}
	}
	if tel, _, ok := Telephony(cs[:3]); ok {
		return SaidTelephony(tel) + " " + cs[3:]
	}
	return cs
}

// telephonyTSV: ICAO designator, telephony designator, operator; from
// Wikipedia's List of airline codes (see the file's head).
//
//go:embed telephony.tsv
var telephonyTSV string

type telephonyEntry struct{ tel, name string }

var telephonyTable = sync.OnceValue(func() map[string]telephonyEntry {
	m := make(map[string]telephonyEntry, 6000)
	for _, line := range strings.Split(telephonyTSV, "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		f := strings.SplitN(strings.TrimRight(line, ""), "	", 3)
		if len(f) == 3 {
			m[f[0]] = telephonyEntry{tel: f[1], name: f[2]}
		}
	}
	return m
})

// Telephony is the radio call sign of the operator with ICAO designator
// icao ("CEF" → "CZECH AIR FORCE", operator "Czech Air Force"), from the
// built-in list of about 5500 airlines and air forces.
func Telephony(icao string) (tel, operator string, ok bool) {
	e, ok := telephonyTable()[strings.ToUpper(icao)]
	return e.tel, e.name, ok
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

// SaidRegistration is a registration as said: each letter in the phonetic
// alphabet, each digit on its own, the dash left out ("OK-VQY" → "Oscar
// Kilo Victor Quebec Yankee").
func SaidRegistration(reg string) string {
	var words []string
	for i := 0; i < len(reg); i++ {
		switch b := reg[i]; {
		case b >= '0' && b <= '9':
			words = append(words, string(b))
		case b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z':
			words = append(words, nav.Phonetic(b))
		}
	}
	if len(words) == 0 {
		return reg
	}
	return strings.Join(words, " ")
}
