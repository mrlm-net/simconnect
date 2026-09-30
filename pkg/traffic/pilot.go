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
	IntentReadback        Intent = "readback"         // a clearance read back
	IntentRequestPushback Intent = "request_pushback" // ready for push and start-up
	IntentRequestTaxi     Intent = "request_taxi"     // ready to taxi
	IntentReadyDeparture  Intent = "ready_departure"  // ready for departure at the holding point
	IntentCheckIn         Intent = "check_in"         // first call on a frequency
	IntentVacated         Intent = "vacated"          // runway vacated
	IntentCorrection      Intent = "correction"       // controller: negative, the clearance again
	IntentSayAgain        Intent = "say_again"        // controller: say again
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

// RequestPushback is a departure ready on its stand.
func RequestPushback(cs, stand, info string) Transmission {
	p := map[string]string{ParamStand: stand, ParamInfo: info}
	return pilotTx(PosGround, cs, IntentRequestPushback, p, fmt.Sprintf("%s, stand %s, request push and start-up%s", cs, stand, withInfo(info)))
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

// Vacated reports the runway vacated.
func Vacated(cs, runway string) Transmission {
	return pilotTx(PosGround, cs, IntentVacated, map[string]string{ParamRunway: runway},
		fmt.Sprintf("%s, runway %s vacated", cs, runway))
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
	switch t.Intent {
	case IntentDepartureClearance:
		s = fmt.Sprintf("Cleared %s departure, runway %s", p[ParamSID], p[ParamRunway])
	case IntentArrivalClearance:
		s = fmt.Sprintf("%s arrival, %s runway %s", p[ParamSTAR], p[ParamApproach], p[ParamRunway])
	case IntentPushback:
		s = "Push and start approved"
	case IntentTaxi:
		if p[ParamStand] != "" {
			s = "Taxi to stand " + p[ParamStand]
		} else {
			hp := "Holding point"
			if p[ParamEntry] != "" {
				hp += " " + p[ParamEntry]
			}
			s = hp + " runway " + p[ParamRunway]
		}
		if p[ParamTaxiways] != "" {
			s += " via " + p[ParamTaxiways]
		}
	case IntentTaxiLimit:
		if p[ParamLimit] == "" {
			s = "Holding at the marked point"
		} else {
			s = "Holding short of " + p[ParamLimit]
		}
	case IntentCross:
		s = "Crossing runway " + p[ParamRunway]
	case IntentLineUp:
		s = fmt.Sprintf("Lining up and waiting runway %s", p[ParamRunway])
	case IntentTakeoff:
		s = fmt.Sprintf("Cleared for take-off runway %s", p[ParamRunway])
	case IntentHoldPosition, IntentCancelTakeoff:
		s = "Holding position"
	case IntentStop:
		s = "Stopping"
	case IntentGoAround:
		s = "Going around"
	case IntentSequence:
		if p[ParamLose] != "" {
			s = "Number " + p[ParamNumber] + ", " + p[ParamLose]
		} else {
			s = "Number " + p[ParamNumber]
		}
	case IntentDirect:
		s = "Direct to the final"
	case IntentHold:
		s = fmt.Sprintf("Hold at %s, maintain %s ft", p[ParamFix], p[ParamAltitude])
	case IntentLeaveHold:
		s = "Leaving the hold at " + p[ParamFix]
	case IntentHoldLevel:
		s = "Descend " + p[ParamAltitude] + " ft"
	case IntentSpeed:
		s = "Speed " + p[ParamSpeed] + " knots"
	case IntentLevel:
		s = strings.ToUpper(p[ParamClimb][:1]) + p[ParamClimb][1:] + " " + p[ParamLevel]
	case IntentHeading:
		s = "Turn " + p[ParamTurn] + " heading " + p[ParamHeading]
	case IntentContact:
		s = strings.TrimSpace(p[ParamStation] + " " + p[ParamFreq])
	default:
		return Transmission{}, false
	}
	return pilotTx(t.Position, cs, IntentReadback, cloneParams(p, ParamIntent, string(t.Intent)), s+", "+cs), true
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
	IntentDepartureClearance: {ParamSID, ParamRunway},
	IntentTaxi:               {ParamRunway, ParamStand},
	IntentTaxiLimit:          {ParamLimit},
	IntentCross:              {ParamRunway},
	IntentLineUp:             {ParamRunway},
	IntentTakeoff:            {ParamRunway},
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

// SayAgain is the controller asking cs (or whoever called, cs "") to say
// again.
func SayAgain(pos Position, cs string) Transmission {
	text := "Station calling, say again your call sign"
	if cs != "" {
		text = cs + ", say again"
	}
	return Transmission{Position: pos, Callsign: cs, Intent: IntentSayAgain, Text: text}
}
