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
	IntentRequestWeather   Intent = "request_weather"   // the crew asks for the wind and QNH
	IntentEstablished      Intent = "established"       // the crew reports established on the localizer
	IntentRequestDirect    Intent = "request_direct"    // the crew asks to fly direct to a fix
	IntentRequestClearance Intent = "request_clearance" // the departure clearance, first call to delivery
	IntentRequestStartUp   Intent = "request_start_up"  // ready for start-up, first call to ground
	IntentRequestPushback  Intent = "request_pushback"  // ready for push
	IntentRequestTaxi      Intent = "request_taxi"      // ready to taxi
	IntentRequestDescent   Intent = "request_descent"   // ready to descend, to the centre or approach (#686)
	IntentReadyDeparture   Intent = "ready_departure"   // holding short of the runway, ready for departure
	IntentHoldingShort     Intent = "holding_short"     // stopped short of a runway to cross
	IntentCheckIn          Intent = "check_in"          // first call on a frequency
	IntentVacated          Intent = "vacated"           // runway vacated
	IntentCorrection       Intent = "correction"        // controller: negative, the clearance again
	IntentSayAgain         Intent = "say_again"         // controller: say again
)

// Pilot parameters.
const (
	ParamInfo  = "information" // the ATIS letter on a first call
	ParamState = "report"      // a first call's report as said: "holding short runway 24"
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

// EstablishedReport is a crew established on the localizer, as asked in
// the approach clearance: "Localizer established runway 24, CSA1" (CAP
// 413 6.27).
func EstablishedReport(cs, runway string) Transmission {
	return pilotTx(PosApproach, cs, IntentEstablished, map[string]string{ParamRunway: runway}, "Localizer established runway "+runway+", "+cs)
}

// RequestWeather is a crew asking for the weather: "Ruzyne Tower, CSA1,
// request weather". The wording is the project's (no source read gives
// one); the answer is WeatherReport.
func RequestWeather(pos Position, cs string) Transmission {
	return pilotTx(pos, cs, IntentRequestWeather, nil, cs+", request weather")
}

// RequestDirect is a crew asking to fly direct to fix: "CSA1, request
// direct GOLOP" (the project's wording); the answer is ClearedDirectTo.
func RequestDirect(pos Position, cs, fix string) Transmission {
	return pilotTx(pos, cs, IntentRequestDirect, map[string]string{ParamFix: fix}, cs+", request direct "+fix)
}

// RequestDescent is a crew ready to descend, to the centre or approach:
// "CSA123, request descent" (#686).
func RequestDescent(pos Position, cs string) Transmission {
	return pilotTx(pos, cs, IntentRequestDescent, nil, cs+", request descent")
}

// RequestTaxiIntersection is a departure ready to taxi asking to take the
// runway from an intersection: "CSA1, request taxi, intersection B"
// (Doc 4444 12.3.4.7 a, its intentions; #621).
func RequestTaxiIntersection(cs, entry string) Transmission {
	return pilotTx(PosGround, cs, IntentRequestTaxi, map[string]string{ParamEntry: entry}, cs+", request taxi, intersection "+entry)
}

// RequestTaxi is a departure pushed back and ready to taxi.
func RequestTaxi(cs string) Transmission {
	return pilotTx(PosGround, cs, IntentRequestTaxi, nil, cs+", request taxi")
}

// HoldingShortReport is a crew stopped short of a runway it is to cross,
// at taxiway at ("" none): "CSA1, holding short of runway 12 at F".
func HoldingShortReport(cs, runway, at string) Transmission {
	text := fmt.Sprintf("%s, holding short of runway %s", cs, runway)
	if at != "" {
		text += " at " + at
	}
	return pilotTx(PosGround, cs, IntentHoldingShort, map[string]string{ParamRunway: runway, ParamEntry: at}, text)
}

// ReadyForDeparture is a departure holding short of its runway, at entry
// for an intersection departure ("" full length): "CSA1, holding short
// runway 24 at Z, ready for departure".
func ReadyForDeparture(cs, runway, entry string) Transmission {
	return pilotTx(PosTower, cs, IntentReadyDeparture, map[string]string{ParamRunway: runway, ParamEntry: entry},
		fmt.Sprintf("%s, %s, ready for departure", cs, HoldingShortSaid(runway, entry)))
}

// HoldingShortSaid is where a departure holds as its crew says it:
// "holding short runway 24", "holding short runway 24 at Z" for an
// intersection.
func HoldingShortSaid(runway, entry string) string {
	s := "holding short runway " + runway
	if entry != "" {
		s += " at " + entry
	}
	return s
}

// TaxiingToSaid is a departure on its way to the runway as its crew says
// it on first calling tower: "taxiing to runway 24", "taxiing to runway 24
// at Z" for an intersection.
func TaxiingToSaid(runway, entry string) string {
	s := "taxiing to runway " + runway
	if entry != "" {
		s += " at " + entry
	}
	return s
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

// VFRForLanding is a VFR arrival's first call to the tower (Doc 4444
// 12.3.4.13 a, d): "Ruzyne Tower, OKABC, Cessna 172, 5 miles north, 2000
// feet, information Alpha, for landing" (typ, position and level as said;
// info "" when no ATIS).
func VFRForLanding(station, cs, typ, position, level, info string) Transmission {
	text := station + ", " + cs
	for _, s := range []string{typ, position, level} {
		if s != "" {
			text += ", " + s
		}
	}
	return pilotTx(PosTower, cs, IntentVFRForLanding, map[string]string{ParamStation: station, ParamType: typ, ParamCircuit: position, ParamLevel: level, ParamInfo: info},
		text+withInfo(info)+", for landing")
}

// CircuitReport is a report of the position in the circuit (12.3.4.14 a):
// "OKABC, downwind".
func CircuitReport(cs, position string) Transmission {
	return pilotTx(PosTower, cs, IntentCircuitReport, map[string]string{ParamCircuit: position}, cs+", "+position)
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
	// An instruction joined to the call (Joined): the call's own readback,
	// if it has one, then the joined one's ("identified" alone has none:
	// live in MyCrew, identified + cleared ILS approach read back nothing).
	if also := p[ParamAlsoReadback]; also != "" {
		own := t
		own.Params = map[string]string{}
		for k, v := range p {
			if k != ParamAlsoSaid && k != ParamAlsoReadback {
				own.Params[k] = v
			}
		}
		s := capital(also)
		if rb, ok := Readback(own); ok {
			s = strings.TrimSuffix(rb.Text, ", "+cs) + ", " + also
		}
		return pilotTx(t.Position, cs, IntentReadback, cloneParams(p, ParamIntent, string(t.Intent)), s+", "+cs), true
	}
	var s string
	// The readbacks as docs/traffic-phraseology.md quotes them (Doc 4444
	// 4.5.7.5, CAP 413 examples): the clearance's items, then the call sign.
	if t.Phraseology == PhraseologyFAA {
		// The FAA's: the clearance as given, without the call sign (AIM
		// 4-4-7), for those worded the FAA's way.
		if said, ok := phraseFAA(cs, t.Intent, p); ok && t.Intent != IntentIdentified {
			s = capital(strings.TrimPrefix(said, cs+", "))
			rb := pilotTx(t.Position, cs, IntentReadback, cloneParams(p, ParamIntent, string(t.Intent)), s+", "+cs)
			rb.Phraseology = PhraseologyFAA
			return rb, true
		}
	}
	switch t.Intent {
	case IntentTrafficInfo:
		s = "Looking out" // traffic information is acknowledged, not read back
	case IntentWeather:
		s = "QNH " + p[ParamQNH] // the pressure setting is read back (4.5.7.5.1)
		if p[ParamQNH] == "" {
			return Transmission{}, false
		}
	case IntentDirectTo:
		s = "Cleared direct to " + p[ParamFix]
	case IntentVector:
		switch {
		case p[ParamFix] != "":
			s = "Resume own navigation direct " + p[ParamFix]
		case p[ParamTurn] != "":
			s = "Turn " + p[ParamTurn] + " heading " + p[ParamHeading]
		default:
			s = "Fly heading " + p[ParamHeading]
		}
	case IntentIdentified:
		if p[ParamLevel] == "" {
			return Transmission{}, false // identification alone needs no readback
		}
		s = "Climb to " + p[ParamLevel]
	case IntentDepartureClearance:
		s = capital(departureClearance(p)) // CAP 413 2.68
	case IntentArrivalClearance:
		s = capital(arrivalClearance(p))
	case IntentApproachClearance:
		s = capital(approachClearance(p))
	case IntentStartUp:
		s = "Start up approved"
	case IntentPushback:
		s = "Pushback approved"
		if p[ParamStartUp] != "" {
			s = "Pushback and start up approved"
		}
		if p[ParamFacing] != "" {
			s += ", facing " + p[ParamFacing]
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
	case IntentGiveWay:
		s = "Giving way to the " + p[ParamGiveWay]
	case IntentRunwayChange:
		s = capital(strings.TrimPrefix(runwayChange(p), "runway change, "))
	case IntentTaxiLimit:
		if p[ParamLimit] == "" {
			s = "Holding at the marked point"
		} else {
			s = "Holding short of " + p[ParamLimit] // 12.3.4.8 note
		}
	case IntentJoinCircuit:
		s = fmt.Sprintf("Join %s runway %s", p[ParamCircuit], p[ParamRunway])
		if p[ParamQNH] != "" {
			s += ", QNH " + p[ParamQNH] // the pressure setting is read back (4.5.7.5.1)
		}
	case IntentStraightIn:
		s = "Straight-in approach runway " + p[ParamRunway]
		if p[ParamQNH] != "" {
			s += ", QNH " + p[ParamQNH]
		}
	case IntentFollow:
		s = "Number " + p[ParamNumber]
	case IntentCircuitInstr, IntentCircuitDelay:
		s = capital(p[ParamInstr])
	case IntentTouchAndGo:
		s = "Cleared touch and go"
		if p[ParamInstr] != "" {
			s = "Cleared " + p[ParamInstr]
		}
	case IntentFullStop:
		s = "Make full stop"
	case IntentCross:
		s = "Cross runway " + p[ParamRunway]
		if p[ParamBehind] != "" {
			s = fmt.Sprintf("Behind the landing %s, cross runway %s, behind", p[ParamBehind], p[ParamRunway])
		}
	case IntentLineUp:
		s = capital(runwayAt(p)) + ", line up and wait"
		if p[ParamBehind] != "" {
			s = fmt.Sprintf("Behind the landing %s, line up and wait runway %s, behind", p[ParamBehind], p[ParamRunway])
		}
	case IntentTakeoff:
		s = capital(runwayAt(p)) + ", cleared for take-off" // CAP 413: runway first
		if p[ParamNoDelay] != "" {
			s += ", no delay"
		}
	case IntentLanding:
		s = fmt.Sprintf("Runway %s, cleared to land", p[ParamRunway])
	case IntentHoldPosition:
		s = "Hold position" // read back as given (the project's choice over Doc 4444 12.3.4.8 note's "Holding")
	case IntentContinueTaxi:
		s = "Continue taxi"
	case IntentCancelTakeoff:
		s = "Holding" // 12.3.4.8 note, 12.3.4.11 c
	case IntentStop:
		s = "Stopping"
	case IntentGoAround:
		s = "Going around"
	case IntentSequence:
		var parts []string
		if p[ParamNumber] != "" {
			parts = append(parts, "number "+p[ParamNumber])
		}
		if p[ParamSpeed] != "" {
			parts = append(parts, "reduce speed to "+p[ParamSpeed]+" knots")
		}
		if p[ParamOrbit] != "" {
			parts = append(parts, "orbit "+p[ParamOrbit])
		}
		if p[ParamFinalSpeed] != "" {
			parts = append(parts, "reduce to final approach speed")
		}
		s = strings.Join(parts, ", ")
		if s != "" {
			s = strings.ToUpper(s[:1]) + s[1:]
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
		switch p[ParamClimb] {
		case "stop":
			s = "Stop descent at " + p[ParamLevel]
		case "stop climb":
			s = "Stop climb at " + p[ParamLevel]
		case "maintain":
			s = "Maintain " + p[ParamLevel]
		case "continue climb", "continue descent":
			s = capital(levelVerb(p[ParamClimb])) + " to " + p[ParamLevel]
		}
	case IntentDescendVia:
		s = capital(descendVia(p))
	case IntentVisual:
		s = "Cleared visual approach runway " + p[ParamRunway]
	case IntentCrossLevel:
		s = "Cross " + p[ParamFix] + " at or " + p[ParamClimb] + " " + p[ParamLevel]
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
	IntentDescendVia:         {ParamLevel},
	IntentVisual:             {ParamRunway},
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

// GoingAround is a crew going around on its own (#621): no landing
// clearance by its decision point, or an approach not stable: "CSA1, going
// around".
func GoingAround(cs string) Transmission {
	return pilotTx(PosTower, cs, IntentPilotGoAround, nil, cs+", going around")
}

// RejectingTakeoff is a crew rejecting its take-off on its own (#621):
// "CSA1, stopping".
func RejectingTakeoff(cs string) Transmission {
	return pilotTx(PosTower, cs, IntentPilotReject, nil, cs+", stopping")
}

// Acknowledge is a controller's "roger" to a crew's report: "CSA1, roger".
func Acknowledge(pos Position, cs string) Transmission {
	return Transmission{Position: pos, Callsign: cs, Intent: IntentAcknowledge, Text: cs + ", roger"}
}

// IntentStandby: the controller has the crew wait for an answer
// ("CSA1, standby"), #739.
const IntentStandby Intent = "standby"

// Service vehicles on the radio (#752): a vehicle's request to proceed
// via taxiways or cross a runway, and ground's "proceed via".
const (
	IntentVehicleRequest Intent = "vehicle_request"
	IntentVehicleProceed Intent = "vehicle_proceed"
	ParamVia                    = "via" // taxiways as said: "A, B"
)

// Visual approach (#766).
const (
	IntentRequestVisual Intent = "request_visual"
	IntentVisual        Intent = "visual_approach"
)

// RequestVisual is a crew asking for a visual approach: ICAO "CSA1,
// request visual approach" (Doc 4444 12.3.3.1 n). FAA crews report the
// airport in sight instead (JO 7110.65 7-4-3); not worded here.
func RequestVisual(pos Position, cs string) Transmission {
	return pilotTx(pos, cs, IntentRequestVisual, nil, cs+", request visual approach")
}

// ClearedVisual clears a visual approach: "CSA1, cleared visual approach
// runway 24" (Doc 4444 12.3.3.1 o; JO 7110.65 7-4-3).
func ClearedVisual(pos Position, cs, runway string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentVisual, Params: map[string]string{ParamRunway: runway}})
}
