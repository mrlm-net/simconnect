package traffic

import (
	"strings"
	"time"
)

// abbreviatedClashWindow: another aircraft with the same abbreviated call sign
// heard on the frequency this recently keeps both on the full one.
const abbreviatedClashWindow = 30 * time.Minute

// AbbreviatedRegistration is a registration's abbreviated call sign as said:
// its first character and its last two (SERA.14050(b): "OKXSP" → "Oscar Sierra
// Papa"). "" for anything else: a flight number's call sign has no abbreviated
// form, and a registration under four characters nothing to leave out.
func AbbreviatedRegistration(cs string) string {
	if !isRegistration(cs) {
		return ""
	}
	var chars []byte
	for i := 0; i < len(cs); i++ {
		if b := cs[i]; b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' {
			chars = append(chars, b)
		}
	}
	if len(chars) < 4 {
		return ""
	}
	n := len(chars)
	return SaidRegistration(string([]byte{chars[0], chars[n-2], chars[n-1]}))
}

// isRegistration: not "as said already" ("Tug 3"), nor three letters and a
// number (ScheduleConfig.SaidCallsign's test).
func isRegistration(cs string) bool {
	if cs == "" || strings.Contains(cs, " ") {
		return false
	}
	return len(cs) < 4 || cs[3] < '0' || cs[3] > '9' || strings.ContainsAny(cs[:3], "0123456789")
}

// radioContact is one registration's exchange on one frequency.
type radioContact struct {
	freq                string
	pilotSaid, atcSaid  bool
	abbreviatedByTheATC bool
}

// abbreviate reports whether t says its registration in the abbreviated form,
// and notes the exchange. Full on first contact both ways; the controller
// abbreviates once two-way contact is established, and only then may the crew
// follow. A new frequency starts over, and an aircraft with the same abbreviated
// call sign on the frequency keeps both on the full one. Under r.mu.
func (r *Radio) abbreviate(t Transmission) bool {
	short := AbbreviatedRegistration(t.Callsign)
	if short == "" || t.Frequency == "" {
		return false
	}
	on := t.Airport + " " + t.Frequency + " " + short
	heard := r.abbreviations[on]
	if heard == nil {
		heard = map[string]time.Time{}
		r.abbreviations[on] = heard
	}
	clash := false
	for cs, at := range heard {
		if cs != t.Callsign && t.At.Sub(at) < abbreviatedClashWindow {
			clash = true
		}
	}
	heard[t.Callsign] = t.At

	key := t.Airport + " " + t.Callsign
	c := r.contacts[key]
	if c == nil || c.freq != t.Frequency {
		c = &radioContact{freq: t.Frequency}
		r.contacts[key] = c
	}
	if clash {
		c.abbreviatedByTheATC = false
		return false
	}
	if t.Pilot {
		c.pilotSaid = true
		return c.abbreviatedByTheATC
	}
	if c.pilotSaid && c.atcSaid {
		c.abbreviatedByTheATC = true
	}
	c.atcSaid = true
	return c.abbreviatedByTheATC
}
