package traffic

// A VFR flight through a controlled aerodrome's zone (the MyCrew app's
// ATC, the counterpart of VFRForLanding and JoinCircuit): the tower asks it
// to report a visual reporting point or leaving the zone, the crew reports
// it, and the tower lets it go: "frequency change approved", or a contact
// with flight information. CAP 413 6.7 and Doc 4444 12.3.4 word them;
// "wilco" answers a request to report (Doc 4444 12.2.4).
const (
	IntentReportAt          Intent = "report_at"           // tower: report at a point (ParamFix)
	IntentReportLeavingZone Intent = "report_leaving_zone" // tower: report leaving the zone [via a point]
	IntentPilotAtPoint      Intent = "pilot_at_point"      // pilot: at a point (ParamFix)
	IntentPilotLeavingZone  Intent = "pilot_leaving_zone"  // pilot: leaving the zone [via a point]
	IntentFrequencyChange   Intent = "frequency_change"    // tower: frequency change approved
)

// ReportAt asks cs to report at a visual reporting point: "OKABC, report at
// NOVEMBER" — a VFR arrival before it joins the circuit, or a departure on
// its way out.
func ReportAt(pos Position, cs, point string) Transmission {
	return Transmission{Position: pos, Callsign: cs, Intent: IntentReportAt, Params: map[string]string{ParamFix: point},
		Text: cs + ", report at " + point}
}

// ReportLeavingZone asks a VFR departure to report leaving the control
// zone: "OKABC, report leaving the zone via NOVEMBER" (via "" none).
func ReportLeavingZone(pos Position, cs, via string) Transmission {
	text := cs + ", report leaving the zone"
	if via != "" {
		text += " via " + via
	}
	return Transmission{Position: pos, Callsign: cs, Intent: IntentReportLeavingZone, Params: map[string]string{ParamFix: via}, Text: text}
}

// AtPoint is a crew's report at a visual reporting point: "OKABC,
// NOVEMBER", with its altitude when given ("OKABC, NOVEMBER, 2500 feet").
func AtPoint(pos Position, cs, point, level string) Transmission {
	text := cs + ", " + point
	if level != "" {
		text += ", " + level
	}
	return pilotTx(pos, cs, IntentPilotAtPoint, map[string]string{ParamFix: point, ParamLevel: level}, text)
}

// LeavingZone is a VFR departure's report at the zone boundary: "OKABC,
// leaving the zone via NOVEMBER" (via "" none).
func LeavingZone(pos Position, cs, via string) Transmission {
	text := cs + ", leaving the zone"
	if via != "" {
		text += " via " + via
	}
	return pilotTx(pos, cs, IntentPilotLeavingZone, map[string]string{ParamFix: via}, text)
}

// FrequencyChangeApproved lets a VFR flight leave the tower's frequency:
// "OKABC, frequency change approved"; squawk, when given, the code to set
// on leaving ("squawk 7000"). To hand it to flight information use
// ContactFIS instead.
func FrequencyChangeApproved(pos Position, cs, squawk string) Transmission {
	text := cs + ", frequency change approved"
	if squawk != "" {
		text += ", squawk " + squawk
	}
	return Transmission{Position: pos, Callsign: cs, Intent: IntentFrequencyChange, Params: map[string]string{ParamSquawk: squawk}, Text: text}
}

// ContactFIS hands a VFR flight leaving the zone to flight information:
// "OKABC, contact Praha Information 126.1" (station as said, freq as
// written), read back as any contact.
func ContactFIS(from Position, cs, station, freq string) Transmission {
	return Handoff(cs, from, PosInformation, station, freq)
}
