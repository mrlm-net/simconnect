package traffic

import (
	"fmt"
	"strings"
	"time"
)

// Emergencies (the emergency add-on, part 2): an aircraft declares a
// distress (MAYDAY) or an urgency (PAN PAN), is acknowledged, squawks 7700
// and lands with priority, the others fitted around it (Doc 4444 15.1,
// CAP 413 chapter 8).

// EmergencyKind is what happened, as a scenario names it.
type EmergencyKind string

const (
	EmergencyEngineFailure  EmergencyKind = "engine-failure"
	EmergencyMedical        EmergencyKind = "medical"
	EmergencyFuel           EmergencyKind = "fuel"
	EmergencyPressurisation EmergencyKind = "pressurisation"
	EmergencyFire           EmergencyKind = "fire"
	EmergencyHydraulic      EmergencyKind = "hydraulic"
)

// Emergency levels: a distress call or an urgency call.
const (
	LevelMayday = "mayday"
	LevelPanPan = "panpan"
)

// EmergencyLevel is the call for kind: MAYDAY for an engine failure, a
// fire or a pressurisation failure (grave and imminent danger), PAN PAN
// for the rest.
func EmergencyLevel(kind EmergencyKind) string {
	switch kind {
	case EmergencyEngineFailure, EmergencyFire, EmergencyPressurisation:
		return LevelMayday
	}
	return LevelPanPan
}

// emergencySaid is kind as a crew says it.
func emergencySaid(kind EmergencyKind) string {
	switch kind {
	case EmergencyEngineFailure:
		return "engine failure"
	case EmergencyMedical:
		return "medical emergency on board"
	case EmergencyFuel:
		return "fuel shortage" // "fuel emergency" and "fuel priority" are not recognised terms (ICAO Phraseology Reference Guide, ALL CLEAR)
	case EmergencyPressurisation:
		return "loss of cabin pressure, descending"
	case EmergencyFire:
		return "engine fire"
	case EmergencyHydraulic:
		return "hydraulic failure"
	}
	return strings.ReplaceAll(string(kind), "-", " ")
}

// EmergencySquawk is the SSR code of an emergency.
const EmergencySquawk = "7700"

// DeclareEmergency is the crew's call: "MAYDAY MAYDAY MAYDAY, Ruzyne
// Approach, CSA123, engine failure, request priority landing runway 24"
// (PAN PAN three times for an urgency). level "" is kind's.
func DeclareEmergency(pos Position, station, cs string, kind EmergencyKind, level, runway string) Transmission {
	if level == "" {
		level = EmergencyLevel(kind)
	}
	call := "MAYDAY MAYDAY MAYDAY"
	if level == LevelPanPan {
		call = "PAN PAN, PAN PAN, PAN PAN"
	}
	text := call + ", "
	if station != "" {
		text += station + ", "
	}
	text += cs + ", " + emergencySaid(kind) + ", request priority landing"
	if runway != "" {
		text += " runway " + runway
	}
	return pilotTx(pos, cs, IntentEmergency, map[string]string{ParamLevel: level, ParamRunway: runway, ParamStation: station}, text)
}

// AcknowledgeEmergency is ATC's answer: "CSA123, roger the MAYDAY, squawk
// 7700, you are number one, runway 24" ("Roger the MAYDAY": ICAO
// Phraseology Reference Guide, ALL CLEAR; roger the PAN PAN for an
// urgency).
func AcknowledgeEmergency(pos Position, cs, level, runway string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentEmergencyAck,
		Params: map[string]string{ParamLevel: level, ParamRunway: runway, ParamSquawk: EmergencySquawk}})
}

// emergencyAck is AcknowledgeEmergency's words after the call sign.
func emergencyAck(p map[string]string) string {
	said := "MAYDAY"
	if p[ParamLevel] == LevelPanPan {
		said = "PAN PAN"
	}
	s := "roger the " + said + ", squawk " + p[ParamSquawk] + ", you are number one"
	if p[ParamRunway] != "" {
		s += ", runway " + p[ParamRunway]
	}
	return s
}

// CustomCall is a controller's call as a scenario writes it, said on
// pos's frequency as written: no phrasebook wording, no call sign added.
func CustomCall(pos Position, cs, text string) Transmission {
	return Transmission{Position: pos, Callsign: cs, Intent: IntentCustom, Text: text}
}

// Roger is a crew's acknowledgement of a call with nothing to read back:
// "Roger, CSA4KM".
func Roger(pos Position, cs string) Transmission {
	return pilotTx(pos, cs, IntentReadback, map[string]string{ParamIntent: string(IntentCustom)}, "Roger, "+cs)
}

// CancelEmergency is the crew cancelling its emergency: "Praha Approach,
// CSA123, cancel MAYDAY" (cancel PAN PAN). The project's wording: the
// ICAO Phraseology Reference Guide says an emergency "can always be
// cancelled" but quotes no words.
func CancelEmergency(pos Position, station, cs, level string) Transmission {
	said := "MAYDAY"
	if level == LevelPanPan {
		said = "PAN PAN"
	}
	text := cs + ", cancel " + said
	if station != "" {
		text = station + ", " + text
	}
	return pilotTx(pos, cs, IntentEmergencyCancel, map[string]string{ParamLevel: level, ParamStation: station}, text)
}

// AcknowledgeCancelEmergency is ATC's answer: "CSA123, roger, MAYDAY
// cancelled", with "squawk 4521" when a code goes back (squawk "" none).
// The project's wording, as CancelEmergency.
func AcknowledgeCancelEmergency(pos Position, cs, level, squawk string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentEmergencyCancelAck,
		Params: map[string]string{ParamLevel: level, ParamSquawk: squawk}})
}

// cancelAck is AcknowledgeCancelEmergency's words after the call sign.
func cancelAck(p map[string]string) string {
	said := "MAYDAY"
	if p[ParamLevel] == LevelPanPan {
		said = "PAN PAN"
	}
	s := "roger, " + said + " cancelled"
	if p[ParamSquawk] != "" {
		s += ", squawk " + p[ParamSquawk]
	}
	return s
}

// EmergencyQuestions is ATC asking for what a distress or urgency message
// should carry when the first call did not (ICAO Phraseology Reference
// Guide, ALL CLEAR: "nature of the emergency, fuel endurance and persons
// on board … and pilot's intentions"): "CSA123, report persons on board,
// fuel endurance and intentions". The question is the project's wording:
// the guide lists the items, not the words.
func EmergencyQuestions(pos Position, cs string) Transmission {
	return Transmission{Position: pos, Callsign: cs, Intent: IntentEmergencyQuestions,
		Text: cs + ", report persons on board, fuel endurance and intentions"}
}

// EmergencyDetails is the crew's answer, as the guide's example has it
// ("…, 150 persons on board, endurance three hours"): "CSA123, 156
// persons on board, endurance 2 hours 10 minutes, request immediate
// landing runway 24". persons 0, endurance 0 or intentions "": left out.
func EmergencyDetails(pos Position, cs string, persons int, endurance time.Duration, intentions string) Transmission {
	text := cs
	if persons > 0 {
		text += fmt.Sprintf(", %d persons on board", persons)
	}
	if endurance > 0 {
		text += ", endurance " + enduranceSaid(endurance)
	}
	if intentions != "" {
		text += ", " + intentions
	}
	return pilotTx(pos, cs, IntentEmergencyDetails, map[string]string{}, text)
}

// enduranceSaid is an endurance as said: "2 hours 10 minutes", "45 minutes".
func enduranceSaid(d time.Duration) string {
	m := int(d.Round(time.Minute).Minutes())
	h, m := m/60, m%60
	switch {
	case h == 0:
		return fmt.Sprintf("%d minutes", m)
	case m == 0 && h == 1:
		return "1 hour"
	case m == 0:
		return fmt.Sprintf("%d hours", h)
	case h == 1:
		return fmt.Sprintf("1 hour %d minutes", m)
	}
	return fmt.Sprintf("%d hours %d minutes", h, m)
}

// Intentions as a crew gives them in a distress or urgency message (the
// guide's example: "request immediate landing at Metro").
func IntentionsLand(runway string) string {
	if runway == "" {
		return "request immediate landing"
	}
	return "request immediate landing runway " + runway
}

// IntentionsReturn is going back to the departure airport (its name as
// said, "Prague").
func IntentionsReturn(airport string) string { return "request return to " + airport }

// IntentionsDivert is diverting to another airport (its name as said).
func IntentionsDivert(airport string) string { return "request diversion to " + airport }
