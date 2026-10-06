package traffic

import "strings"

// A revised departure clearance (the runway in use changed after the
// clearance was read back, so the SID changes with it), its readback, and
// a crew unable to fly it.
//
// Sources: CAP 413 (ed. 24) 2.73, an amended clearance is read in full and
// cancels the previous one; 4.38, revised clearances to an aircraft on the
// runway or at the holding position are prefixed "hold position", read
// back "holding"; 2.72, "unable" with the reason. ICAO wording here repeats
// the whole departure clearance after the prefix (Doc 4444 12.3.2.1 c's
// "RECLEARED" is not used: CAP 413 says "re-cleared" should not be). FAA,
// JO 7110.65 4-2-5: "CHANGE (portion of route) TO READ (new portion of
// route)".

// Intents of a revised departure clearance.
const (
	IntentRevisedDeparture Intent = "revised_departure" // the departure clearance again, a new SID
	IntentPilotUnable      Intent = "pilot_unable"      // crew: unable (what), (reason)
	IntentUnableAck        Intent = "unable_ack"        // controller: roger, (what next)
)

// Params of a revised departure clearance.
const (
	ParamOldSID       = "oldSid"       // the SID cleared before (SaidProcedure)
	ParamHoldPosition = "holdPosition" // "1": prefixed "hold position"
)

// RevisedAt is where a departure is when its clearance is revised: before
// taxi (the usual case: the runway changed between the clearance and the
// taxi), taxiing, or at the holding position or on the runway.
type RevisedAt int

const (
	RevisedBeforeTaxi RevisedAt = iota // delivery, at the stand
	RevisedTaxiing                     // ground
	RevisedHolding                     // tower: "hold position" first (CAP 413 4.38)
)

// RevisedDepartureClearance is the departure clearance again for a runway
// change: c with its new SID and runway, oldSID the SID cleared before
// (SaidProcedure; "" unknown), said by the unit the aircraft is with at
// (delivery, ground, tower). ICAO: "CSA123, cleared to Frankfurt, VOZ 5D
// departure, flight planned route, runway 06, climb via SID to flight
// level 100, squawk 4521", at the holding position after "hold position".
// FAA: "CSA123, change VOZ 5M departure to read VOZ 5D departure".
func RevisedDepartureClearance(cs string, c DepartureClearance, oldSID string, at RevisedAt, ph Phraseology) Transmission {
	p := map[string]string{ParamDest: c.Destination, ParamSID: c.SID, ParamRunway: c.Runway, ParamLevel: c.Level, ParamSquawk: c.Squawk, ParamOldSID: oldSID}
	pos := PosDelivery
	switch at {
	case RevisedTaxiing:
		pos = PosGround
	case RevisedHolding:
		pos = PosTower
		p[ParamHoldPosition] = "1"
	}
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentRevisedDeparture, Params: p, Phraseology: ph})
}

// revisedDeparture is the clearance after the call sign.
func revisedDeparture(p map[string]string, faa bool) string {
	s := ""
	if p[ParamHoldPosition] != "" {
		s = "hold position, "
	}
	if faa && p[ParamOldSID] != "" && p[ParamSID] != "" {
		return s + "change " + p[ParamOldSID] + " departure to read " + p[ParamSID] + " departure" // 4-2-5
	}
	if faa {
		t, _ := phraseFAA("", IntentDepartureClearance, p)
		return s + strings.TrimPrefix(t, ", ")
	}
	return s + departureClearance(p)
}

// readbackRevised is the crew's readback: "Holding, cleared to …" after
// the prefix (CAP 413 4.38), else the clearance. Built from the params,
// not the text: a transmitted text carries the spoken call sign ("CSA
// Lines 123"), not t.Callsign (found by the MyCrew app).
func readbackRevised(t Transmission) string {
	body := revisedDeparture(t.Params, false)
	if rest, ok := strings.CutPrefix(body, "hold position, "); ok {
		return "Holding, " + rest
	}
	return capital(body)
}

// Unable is a crew unable to fly what it was cleared (CAP 413 2.72):
// "Unable VOZ 5M departure, due performance, CSA123". what is what it
// cannot do ("VOZ 5M departure"), reason why ("due performance"; "" none).
func Unable(pos Position, cs, what, reason string) Transmission {
	s := "Unable " + what
	if reason != "" {
		s += ", " + reason
	}
	return pilotTx(pos, cs, IntentPilotUnable, map[string]string{ParamSID: what}, s+", "+cs)
}

// UnableAcknowledged is the controller's answer to an unable: "CSA123,
// roger, hold position", then (next, "" none) what follows, e.g. "expect
// radar vectors" or "advise when ready". Project wording: no source read
// gives one.
func UnableAcknowledged(pos Position, cs string, holdPosition bool, next string) Transmission {
	s := cs + ", roger"
	if holdPosition {
		s += ", hold position"
	}
	if next != "" {
		s += ", " + next
	}
	return Transmission{Position: pos, Callsign: cs, Intent: IntentUnableAck, Text: s}
}
