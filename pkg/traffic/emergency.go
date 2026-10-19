package traffic

import "strings"

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
		return "minimum fuel"
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

// AcknowledgeEmergency is ATC's answer: "CSA123, roger MAYDAY, squawk
// 7700, you are number one, runway 24" (roger PAN PAN for an urgency).
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
	s := "roger " + said + ", squawk " + p[ParamSquawk] + ", you are number one"
	if p[ParamRunway] != "" {
		s += ", runway " + p[ParamRunway]
	}
	return s
}
