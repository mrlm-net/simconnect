package traffic

import (
	"fmt"
	"strings"
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
