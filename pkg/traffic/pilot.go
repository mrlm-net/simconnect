//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"strings"
)

// The pilot side of the radio (#417): requests and reports our pilots
// make, their first call on each frequency, the readbacks of clearances,
// and the check of a readback against the clearance with the correction a
// controller gives.

// Pilot intents.
const (
	IntentReadback         Intent = "readback"          // a clearance read back
	IntentRequestClearance Intent = "request_clearance" // the departure clearance, first call to delivery
	IntentRequestStartUp   Intent = "request_start_up"  // ready for start-up, first call to ground
	IntentRequestPushback  Intent = "request_pushback"  // ready for push
	IntentRequestTaxi      Intent = "request_taxi"      // ready to taxi
	IntentReadyDeparture   Intent = "ready_departure"   // ready for departure at the holding point
	IntentCheckIn          Intent = "check_in"          // first call on a frequency
	IntentVacated          Intent = "vacated"           // runway vacated
	IntentCorrection       Intent = "correction"        // controller: negative, the clearance again
	IntentSayAgain         Intent = "say_again"         // controller: say again
)

// Pilot parameters.
const (
	ParamInfo  = "information" // the ATIS letter on a first call
	ParamState = "report"      // a first call's report as said: "holding point runway 24"
)

// pilotTx is a transmission said by the pilot of cs to position pos.
func pilotTx(pos Position, cs string, in Intent, p map[string]string, text string) Transmission {
	return Transmission{Position: pos, Pilot: true, Callsign: cs, Intent: in, Params: p, Text: text}
}

// RequestClearance is a departure asking delivery for its clearance, its
// first call: "Ruzyne Delivery, CSA123, stand A4, information Bravo,
// request clearance to Frankfurt" (the order of CAP 413 4.9; the request
// wording itself is not in Doc 4444: docs/traffic-phraseology.md).
func RequestClearance(station, cs, stand, info, destination string) Transmission {
	p := map[string]string{ParamStation: station, ParamStand: stand, ParamInfo: info, ParamDest: destination}
	text := cs
	if station != "" {
		text = station + ", " + cs
	}
	req := "request clearance"
	if destination != "" {
		req += " to " + destination
	}
	return pilotTx(PosDelivery, cs, IntentRequestClearance, p, fmt.Sprintf("%s, stand %s%s, %s", text, stand, withInfo(info), req))
}

// RequestPushback is a departure ready on its stand, its first call to
// ground: "Ruzyne Ground, CSA123, stand A4, information Bravo, request
// pushback" (Doc 4444 12.3.4.4 a, the order of CAP 413 4.9); station ""
// when already in contact.
func RequestPushback(station, cs, stand, info string) Transmission {
	return firstCall(IntentRequestPushback, station, cs, stand, info, "request pushback")
}

// RequestPushbackAndStartUp asks for both in one call: "Ruzyne Ground,
// CSA123, stand C22, request pushback and start up".
func RequestPushbackAndStartUp(station, cs, stand, info string) Transmission {
	t := firstCall(IntentRequestPushback, station, cs, stand, info, "request pushback and start up")
	t.Params[ParamStartUp] = "1"
	return t
}

// RequestStartUp asks for the start-up once the tug has gone: "CSA123,
// request start up" (Doc 4444 12.3.4.3 a); with station and stand as a
// first call.
func RequestStartUp(station, cs, stand, info string) Transmission {
	return firstCall(IntentRequestStartUp, station, cs, stand, info, "request start up")
}

// firstCall is "[station, ]cs[, stand (stand)][, information (info)], req".
func firstCall(in Intent, station, cs, stand, info, req string) Transmission {
	p := map[string]string{ParamStation: station, ParamStand: stand, ParamInfo: info}
	text := cs
	if station != "" {
		text = station + ", " + cs
	}
	if stand != "" {
		text += ", stand " + stand
	}
	return pilotTx(PosGround, cs, in, p, text+withInfo(info)+", "+req)
}

// RequestTaxi is a departure pushed back and ready to taxi.
func RequestTaxi(cs string) Transmission {
	return pilotTx(PosGround, cs, IntentRequestTaxi, nil, cs+", request taxi")
}

// ReadyForDeparture is a departure at its runway's holding point.
func ReadyForDeparture(cs, runway string) Transmission {
	return pilotTx(PosTower, cs, IntentReadyDeparture, map[string]string{ParamRunway: runway},
		fmt.Sprintf("%s, holding point runway %s, ready for departure", cs, runway))
}

// Vacated reports the runway vacated: "CSA123, runway vacated" (Doc 4444
// 12.3.4.7 z).
func Vacated(cs, runway string) Transmission {
	return pilotTx(PosGround, cs, IntentVacated, map[string]string{ParamRunway: runway}, cs+", runway vacated")
}

// CheckIn is the first call on a new frequency: "Ruzyne Tower, CSA123,
// established ILS runway 24" (report: what it is doing; info: the ATIS
// letter, on the first call of all).
func CheckIn(pos Position, station, cs, report, info string) Transmission {
	text := station + ", " + cs
	if report != "" {
		text += ", " + report
	}
	return pilotTx(pos, cs, IntentCheckIn, map[string]string{ParamStation: station, ParamState: report, ParamInfo: info}, text+withInfo(info))
}

func withInfo(info string) string {
	if info == "" {
		return ""
	}
	return ", information " + info
}

// Readback is the pilot's readback of controller transmission t, the ICAO
// way: what must be read back (runway, holding point, limits, levels,
// headings, speeds, frequencies), then the call sign. ok is false for
// transmissions not read back.
func Readback(t Transmission) (Transmission, bool) {
	p := t.Params
	cs := t.Callsign
	var s string
	// The readbacks as docs/traffic-phraseology.md quotes them (Doc 4444
	// 4.5.7.5, CAP 413 examples): the clearance's items, then the call sign.
	switch t.Intent {
	case IntentDepartureClearance:
		s = capital(departureClearance(p)) // CAP 413 2.68
	case IntentArrivalClearance:
		s = capital(arrivalClearance(p))
	case IntentStartUp:
		s = "Start up approved"
	case IntentPushback:
		s = "Pushback approved"
		if p[ParamStartUp] != "" {
			s = "Pushback and start up approved"
		}
	case IntentTaxi:
		if p[ParamStand] != "" {
			s = "Taxi to stand " + p[ParamStand]
		} else {
			s = fmt.Sprintf("Taxi to and hold short of runway %s%s", p[ParamRunway], strings.Replace(entryOf(p), " ", " at ", 1))
		}
		if p[ParamTaxiways] != "" {
			s += " via " + p[ParamTaxiways]
		}
	case IntentTaxiLimit:
		if p[ParamLimit] == "" {
			s = "Holding at the marked point"
		} else {
			s = "Holding short of " + p[ParamLimit] // 12.3.4.8 note
		}
	case IntentCross:
		s = "Cross runway " + p[ParamRunway]
	case IntentLineUp:
		s = fmt.Sprintf("Runway %s, line up and wait", p[ParamRunway])
	case IntentTakeoff:
		s = fmt.Sprintf("Runway %s, cleared for take-off", p[ParamRunway]) // CAP 413: runway first
	case IntentLanding:
		s = fmt.Sprintf("Runway %s, cleared to land", p[ParamRunway])
	case IntentHoldPosition, IntentCancelTakeoff:
		s = "Holding" // 12.3.4.8 note, 12.3.4.11 c
	case IntentStop:
		s = "Stopping"
	case IntentGoAround:
		s = "Going around"
	case IntentSequence:
		s = "Number " + p[ParamNumber]
		if p[ParamSpeed] != "" {
			s += ", reduce speed to " + p[ParamSpeed] + " knots"
		}
	case IntentDirect:
		s = "Direct to final"
	case IntentHold:
		s = fmt.Sprintf("Hold at %s as published, maintain %s", p[ParamFix], p[ParamLevel])
	case IntentLeaveHold:
		s = "Leaving " + p[ParamFix]
	case IntentHoldLevel:
		s = "Descend to " + p[ParamLevel]
	case IntentSpeed:
		verb := "Increase"
		if p[ParamSlower] == "true" {
			verb = "Reduce"
		}
		s = verb + " speed to " + p[ParamSpeed] + " knots"
	case IntentLevel:
		s = capital(p[ParamClimb]) + " to " + p[ParamLevel]
	case IntentHeading:
		s = "Turn " + p[ParamTurn] + " heading " + p[ParamHeading]
	case IntentContact:
		s = strings.TrimSpace(p[ParamStation] + " " + p[ParamFreq])
		if p[ParamWhen] != "" {
			s = capital(p[ParamWhen]) + " " + s // CAP 413 4.68
		}
	default:
		return Transmission{}, false
	}
	return pilotTx(t.Position, cs, IntentReadback, cloneParams(p, ParamIntent, string(t.Intent)), s+", "+cs), true
}

// capital is s with its first letter a capital.
func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ParamIntent on a readback: the intent of the clearance read back.
const ParamIntent = "of"

func cloneParams(p map[string]string, k, v string) map[string]string {
	out := map[string]string{k: v}
	for a, b := range p {
		out[a] = b
	}
	return out
}

// readbackKeys are what must be read back right, by clearance.
var readbackKeys = map[Intent][]string{
	IntentDepartureClearance: {ParamSID, ParamRunway, ParamSquawk},
	IntentTaxi:               {ParamRunway, ParamStand},
	IntentTaxiLimit:          {ParamLimit},
	IntentCross:              {ParamRunway},
	IntentLineUp:             {ParamRunway},
	IntentTakeoff:            {ParamRunway},
	IntentLanding:            {ParamRunway},
	IntentHold:               {ParamFix, ParamAltitude},
	IntentHoldLevel:          {ParamAltitude},
	IntentSpeed:              {ParamSpeed},
	IntentLevel:              {ParamLevel},
	IntentHeading:            {ParamHeading},
	IntentContact:            {ParamFreq},
}

// CheckReadback compares what a pilot read back (heard: parameters as
// recognised, e.g. a voice recogniser's tags) with clearance t. A wrong or
// missing item gets the controller's correction: "CSA123, negative, hold
// short of A". ok is true when the readback is right.
func CheckReadback(t Transmission, heard map[string]string) (Transmission, bool) {
	for _, k := range readbackKeys[t.Intent] {
		want := t.Params[k]
		if want == "" {
			continue
		}
		if !sameSaid(heard[k], want) {
			c := t
			c.Pilot = false
			c.Intent = IntentCorrection
			c.Params = cloneParams(t.Params, ParamIntent, string(t.Intent))
			c.Text = strings.Replace(t.Text, t.Callsign+", ", t.Callsign+", negative, ", 1)
			return c, false
		}
	}
	return Transmission{}, true
}

// sameSaid compares two values as said: case, spaces and leading zeros
// ("06" and "6", "FL210" and "flight level 210") aside.
func sameSaid(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.NewReplacer("flight level ", "fl", " ", "", "feet", "", "ft", "", "altitude", "").Replace(s)
		return strings.TrimLeft(s, "0")
	}
	return norm(a) == norm(b)
}

// IntentATIS is an ATIS broadcast (#418).
const IntentATIS Intent = "atis"

// ATISInformation is an airport's ATIS broadcast of information letter
// (phonetic, "Bravo"), text as broadcast: said on the ATIS frequency when a
// new information is out (a voice loops the latest).
func ATISInformation(letter, text string) Transmission {
	return Transmission{Position: PosATIS, Intent: IntentATIS, Params: map[string]string{ParamInfo: letter}, Text: text}
}

// IntentReadbackCorrect: the controller confirms a readback.
const IntentReadbackCorrect Intent = "readback_correct"

// ReadbackCorrect is the controller at pos confirming a readback: "CSA123,
// readback correct" (CAP 413 2.68 shows "BIGJET 347, correct").
func ReadbackCorrect(pos Position, cs string) Transmission {
	return Transmission{Position: pos, Callsign: cs, Intent: IntentReadbackCorrect, Text: cs + ", readback correct"}
}

// SayAgain is the controller asking cs (or whoever called, cs "") to say
// again.
func SayAgain(pos Position, cs string) Transmission {
	text := "Station calling, say again your call sign"
	if cs != "" {
		text = cs + ", say again"
	}
	return Transmission{Position: pos, Callsign: cs, Intent: IntentSayAgain, Text: text}
}
