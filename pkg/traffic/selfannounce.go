package traffic

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Self-announce (#1296): at a field without a tower pilots announce what
// they do on its common frequency (CTAF, UNICOM, MULTICOM:
// airport.Layout.SelfAnnounceFrequency) to the traffic around, "<Field>
// Traffic", and end with the field's name again; nobody answers or clears
// them. An IFR flight there gets its clearance and release from approach
// or center, and cancels IFR with them.

// PosTraffic is a field's common frequency: the traffic around, no
// station.
const PosTraffic Position = "traffic"

// Self-announce and untowered IFR intents.
const (
	IntentSelfAnnounce Intent = "self_announce" // pilot, to the traffic: taxiing, departing, inbound, a leg of the pattern, clear of the runway
	IntentReleased     Intent = "released"      // approach/center: released for departure at an untowered field
	IntentCancelIFR    Intent = "cancel_ifr"    // pilot: cancelling IFR; ATC: cancellation received
)

// ParamSelf is what a self-announce says (SelfTaxiing, …): "taxiing",
// "departing", "inbound", "pattern", "clear".
const ParamSelf = "self"

// TrafficStation is a field's common frequency as called: "Ruzyne
// Traffic".
func TrafficStation(field string) string {
	return strings.TrimSpace(field) + " Traffic"
}

// selfTx is a self-announce: "<Field> Traffic, <cs>, <what>, <Field>".
func selfTx(field, cs, kind string, p map[string]string, what string) Transmission {
	if p == nil {
		p = map[string]string{}
	}
	p[ParamSelf] = kind
	field = strings.TrimSpace(field)
	text := fmt.Sprintf("%s, %s, %s", TrafficStation(field), cs, what)
	if field != "" {
		text += ", " + field
	}
	return pilotTx(PosTraffic, cs, IntentSelfAnnounce, p, text)
}

// SelfTaxiing: "Henderson Traffic, OKABC, taxiing to runway 24,
// Henderson".
func SelfTaxiing(field, cs, runway string) Transmission {
	return selfTx(field, cs, "taxiing", map[string]string{ParamRunway: runway}, "taxiing to runway "+runway)
}

// SelfDeparting: "…, departing runway 24, to the north, Henderson";
// then "" says nothing more, PatternStay "remaining in the pattern".
func SelfDeparting(field, cs, runway, then string) Transmission {
	what := "departing runway " + runway
	if then != "" {
		what += ", " + then
	}
	return selfTx(field, cs, "departing", map[string]string{ParamRunway: runway}, what)
}

// PatternStay: a departure staying in the pattern (SelfDeparting).
const PatternStay = "remaining in the pattern"

// SelfInbound: "…, 10 miles south, inbound for landing, Henderson";
// from "" leaves the direction out.
func SelfInbound(field, cs string, miles int, from string) Transmission {
	where := fmt.Sprintf("%d miles", miles)
	if miles == 1 {
		where = "1 mile"
	}
	if from != "" {
		where += " " + from
	}
	return selfTx(field, cs, "inbound", nil, where+", inbound for landing")
}

// SelfPattern: a leg of the pattern, as said ("left downwind", "right
// base", "final"): "…, left downwind runway 24, Henderson".
func SelfPattern(field, cs, leg, runway string) Transmission {
	return selfTx(field, cs, "pattern", map[string]string{ParamCircuit: leg, ParamRunway: runway}, leg+" runway "+runway)
}

// SelfClear: "…, clear of runway 24, Henderson".
func SelfClear(field, cs, runway string) Transmission {
	return selfTx(field, cs, "clear", map[string]string{ParamRunway: runway}, "clear of runway "+runway)
}

// ReleasedForDeparture is approach's or center's release of an IFR
// departure at an untowered field: "OKABC, released for departure, report
// airborne".
func ReleasedForDeparture(cs string) Transmission {
	return Transmission{Position: PosApproach, Callsign: cs, Intent: IntentReleased,
		Text: cs + ", released for departure, report airborne"}
}

// CancelIFR is an IFR flight cancelling its flight plan with station
// (approach or center), at or near an untowered field: "Portland Approach,
// OKABC, cancel IFR".
func CancelIFR(station, cs string) Transmission {
	t := pilotTx(PosApproach, cs, IntentCancelIFR, nil, cs+", cancel IFR")
	if station != "" {
		t.Text = station + ", " + t.Text
	}
	return t
}

// IFRCancelled is the answer: "OKABC, IFR cancellation received".
func IFRCancelled(cs string) Transmission {
	return Transmission{Position: PosApproach, Callsign: cs, Intent: IntentCancelIFR,
		Text: cs + ", IFR cancellation received"}
}

// UncontrolledField is a field without a tower as the radio works it
// (RadioOptions.Uncontrolled): its name as called ("Henderson") and its
// common frequency ("" none known).
type UncontrolledField struct {
	Name, Freq string
}

// localPosition: worked at the field itself, by nobody at a field without
// a tower.
func localPosition(p Position) bool {
	return p == PosTower || p == PosGround || p == PosDelivery
}

// AdvisoryFrequency is approach's or center's handoff of a flight to a
// field without a tower: "OKABC, change to advisory frequency approved".
func AdvisoryFrequency(from Position, cs string) Transmission {
	return Transmission{Position: from, Callsign: cs, Intent: IntentContact, Params: map[string]string{ParamPosition: string(PosTraffic), ParamStation: "Change to advisory frequency"},
		Text: cs + ", change to advisory frequency approved"}
}

// SelfAnnounceFor is what is said instead of t at a field without a tower
// (#1296), ok false when nothing is: our controllers still decide who
// goes when, and the crews announce it to the traffic around.
//   - the taxi clearance to a runway: SelfTaxiing;
//   - the take-off clearance: SelfDeparting;
//   - the landing clearance: SelfPattern, final;
//   - runway vacated: SelfClear;
//   - approach's or center's handoff to the tower or ground:
//     AdvisoryFrequency.
//
// Every other call of tower, ground or delivery, and the crews' calls to
// them, nobody makes. The other positions' calls stay as they are.
func SelfAnnounceFor(field string, t Transmission) (Transmission, bool) {
	rwy := t.Params[ParamRunway]
	if !localPosition(t.Position) {
		if t.Intent == IntentContact && !t.Pilot && localPosition(Position(t.Params[ParamPosition])) {
			return AdvisoryFrequency(t.Position, t.Callsign), true
		}
		return t, true
	}
	switch {
	case t.Pilot && t.Intent == IntentVacated:
		return SelfClear(field, t.Callsign, rwy), true
	case t.Pilot && t.Intent == IntentCheckIn && vacatedRunway(t.Params[ParamState]) != "":
		return SelfClear(field, t.Callsign, vacatedRunway(t.Params[ParamState])), true
	case t.Pilot:
		return Transmission{}, false
	case t.Intent == IntentTaxi && rwy != "" && t.Params[ParamStand] == "":
		return SelfTaxiing(field, t.Callsign, rwy), true
	case t.Intent == IntentTakeoff:
		return SelfDeparting(field, t.Callsign, rwy, ""), true
	case t.Intent == IntentLanding:
		return SelfPattern(field, t.Callsign, "final", rwy), true
	}
	return Transmission{}, false
}

// vacatedRunway is the runway of an arrival's first call to ground,
// "runway 24 vacated at D"; "" any other report.
func vacatedRunway(report string) string {
	rest, ok := strings.CutPrefix(report, "runway ")
	if !ok {
		return ""
	}
	rwy, _, ok := strings.Cut(rest, " vacated")
	if !ok {
		return ""
	}
	return rwy
}

// FieldCallName is the name a field's traffic is called by ("Henderson
// Traffic"): its common frequency's name without the service ("HENDERSON
// INFORMATION": Henderson), else the airport's name without "Airport",
// "International" and the like, else its ICAO code.
func FieldCallName(l *airport.Layout) string {
	if l == nil {
		return ""
	}
	strip := func(name string, drop ...string) string {
		var keep []string
		for _, w := range strings.Fields(name) {
			if !slices.Contains(drop, strings.ToLower(strings.Trim(w, ".,()"))) {
				keep = append(keep, strings.ToUpper(w[:1])+strings.ToLower(w[1:]))
			}
		}
		return strings.Join(keep, " ")
	}
	if f, ok := l.SelfAnnounceFrequency(); ok {
		if n := strip(f.Name, "information", "info", "unicom", "ctaf", "multicom", "traffic", "radio", "advisory", "common"); n != "" {
			return n
		}
	}
	if n := strip(l.Name, "airport", "international", "intl", "aerodrome", "airfield", "aeropuerto", "aeroport", "aeroporto", "flughafen", "letiste"); n != "" {
		return n
	}
	return l.ICAO
}
