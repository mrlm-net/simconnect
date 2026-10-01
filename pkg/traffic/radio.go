//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// Radio (#415): what ATC says, as structured transmissions instead of log
// text — who, to whom, what (the intent) and its parameters, and the text
// in the application's normal tokens ("CSA123, runway 24, cleared for
// take-off"). A log, a UI or a voice library (voice-goio takes the text and
// normalises it for speech) renders it; the frequency is filled in when the
// positions have frequencies (#416).

// Position is an ATC position.
type Position string

const (
	PosDelivery  Position = "delivery"
	PosGround    Position = "ground"
	PosTower     Position = "tower"
	PosApproach  Position = "approach"
	PosDeparture Position = "departure"
	PosCenter    Position = "center"
	PosATIS      Position = "atis"
)

// Intent is what a transmission does.
type Intent string

const (
	IntentDepartureClearance Intent = "departure_clearance" // cleared the SID, runway
	IntentArrivalClearance   Intent = "arrival_clearance"   // cleared the STAR, expect the approach
	IntentApproachClearance  Intent = "approach_clearance"  // cleared the approach to the runway
	IntentStartUp            Intent = "start_up"            // start up approved
	IntentPushback           Intent = "pushback"            // pushback approved
	IntentTaxi               Intent = "taxi"                // taxi to and hold short of the runway, or to the stand
	IntentTaxiLimit          Intent = "taxi_limit"          // taxi and hold short (a limit on the route)
	IntentGiveWay            Intent = "give_way"            // give way to other traffic on the ground
	IntentRunwayChange       Intent = "runway_change"       // a new runway in use: new SID or STAR (#456)
	IntentCross              Intent = "cross"               // cross a runway
	IntentLineUp             Intent = "line_up"             // line up and wait
	IntentTakeoff            Intent = "takeoff"             // cleared for take-off
	IntentLanding            Intent = "landing"             // cleared to land
	IntentHoldPosition       Intent = "hold_position"       // hold position (on the ground)
	IntentStop               Intent = "stop"                // stop immediately (a take-off roll)
	IntentCancelTakeoff      Intent = "cancel_takeoff"      // hold position, cancel take-off clearance
	IntentGoAround           Intent = "go_around"           // go around (ParamReason: why)
	IntentSequence           Intent = "sequence"            // number, delay and how it is lost
	IntentDirect             Intent = "direct"              // direct to the final
	IntentHold               Intent = "hold"                // hold at a fix
	IntentLeaveHold          Intent = "leave_hold"          // leave the hold, continue the arrival
	IntentHoldLevel          Intent = "hold_level"          // descend in the hold
	IntentSpeed              Intent = "speed"               // reduce or increase speed
	IntentLevel              Intent = "level"               // climb or descend
	IntentHeading            Intent = "heading"             // turn left or right heading
	IntentContact            Intent = "contact"             // a handoff: contact the next position (#416)
)

// Parameter keys of a transmission. Values are the text as said (a runway
// "24", taxiways "B2, H, A", a level "FL210" or "9000 ft").
const (
	ParamRunway   = "runway"
	ParamEntry    = "entry"    // where an intersection departure enters its runway ("B")
	ParamStartUp  = "startup"  // "1": the start-up asked for or approved with the pushback
	ParamFacing   = "facing"   // where a push ends facing: "east"
	ParamGiveWay  = "giveway"  // the traffic given way to, as described: "A320 passing left to right"
	ParamTaxiways = "taxiways" // as said: "B2, H, A"
	ParamStand    = "stand"
	ParamLimit    = "limit" // a taxiway to hold short of; "" a marked point
	ParamSID      = "sid"
	ParamSTAR     = "star"
	ParamApproach = "approach" // the approach expected ("ILS")
	ParamReason   = "reason"
	ParamNumber   = "number" // in the landing sequence
	ParamDelay    = "delay"
	ParamLose     = "lose" // how the delay is lost: "210 kt, +3.2 NM"
	ParamFix      = "fix"
	ParamHoldIn   = "entry_type"  // hold entry: direct, teardrop, parallel
	ParamAltitude = "altitude"    // feet
	ParamExpect   = "expect"      // expect further clearance, HH:MM
	ParamSpeed    = "speed"       // knots
	ParamLevel    = "level"       // "flight level 210" or "altitude 9000 feet"
	ParamHeading  = "heading"     // degrees, three digits
	ParamTurn     = "turn"        // left, right
	ParamClimb    = "climb"       // climb, descend
	ParamSlower   = "slower"      // "true": reduce, else increase
	ParamTraffic  = "traffic"     // why a resolution: "traffic DLH2, 0.8 NM in 2m40s"
	ParamPosition = "position"    // a handoff's next position
	ParamStation  = "station"     // … as said: "Praha Tower"
	ParamFreq     = "frequency"   // … its frequency: "118.105"
	ParamWhen     = "when"        // … a condition: "when vacated"
	ParamDest     = "destination" // a clearance limit as said: "Frankfurt"
	ParamSquawk   = "squawk"      // SSR code: "4521"
	ParamWind     = "wind"        // as said: "wind 100 degrees 6 knots"
)

// Transmission is one message on the radio.
type Transmission struct {
	At        time.Time         `json:"at"`
	Airport   string            `json:"airport,omitempty"`
	Frequency string            `json:"frequency,omitempty"` // with #416
	Position  Position          `json:"position"`            // the controller's
	Pilot     bool              `json:"pilot,omitempty"`     // said by the pilot (#417)
	Callsign  string            `json:"callsign"`
	Intent    Intent            `json:"intent"`
	Params    map[string]string `json:"params,omitempty"`
	Text      string            `json:"text"`
}

// Say is t with its text: the ATC phrase (ICAO phraseology, the
// application's normal tokens) for its intent and parameters.
func Say(t Transmission) Transmission {
	t.Text = phrase(t.Callsign, t.Intent, t.Params)
	return t
}

// phrase is the text of a controller's transmission: ICAO phraseology as
// docs/traffic-phraseology.md quotes it (Doc 4444 chapter 12, CAP 413 for
// wording and order Doc 4444 leaves open).
func phrase(cs string, in Intent, p map[string]string) string {
	via := ""
	if p[ParamTaxiways] != "" {
		via = " via " + p[ParamTaxiways]
	}
	wind := ""
	if p[ParamWind] != "" {
		wind = ", " + p[ParamWind] // after the clearance (CAP 413 4.27, 4.51)
	}
	why := ""
	if p[ParamTraffic] != "" {
		why = ", due traffic" // Doc 4444 12.4.1.5: the reason, not the numbers
	}
	switch in {
	case IntentDepartureClearance:
		// Identification, limit, route (the SID), runway, level, SSR code
		// (Doc 4444 6.3.2.3, 11.4.2.6.2.1; CAP 413 2.68).
		return cs + ", " + departureClearance(p)
	case IntentArrivalClearance:
		// Identification, STAR, runway in use, cleared level (6.5.2.3);
		// the approach to expect (CAP 413 6.9).
		return cs + ", " + arrivalClearance(p)
	case IntentApproachClearance:
		return cs + ", " + approachClearance(p)
	case IntentStartUp:
		return cs + ", start up approved" // Doc 4444 12.3.4.3 c
	case IntentPushback:
		s := cs + ", pushback approved" // Doc 4444 12.3.4.4 b
		if p[ParamStartUp] != "" {
			s = cs + ", pushback and start up approved" // both in one
		}
		if p[ParamFacing] != "" {
			s += ", facing " + p[ParamFacing]
		}
		return s
	case IntentTaxi:
		if p[ParamStand] != "" {
			return fmt.Sprintf("%s, taxi to stand %s%s", cs, p[ParamStand], via) // CAP 413 4.68
		}
		// To the runway and hold short of it, as the project uses (the user's
		// choice over Doc 4444's "taxi to holding point"): an intersection is
		// "runway 24 at B".
		entry := ""
		if p[ParamEntry] != "" {
			entry = " at " + p[ParamEntry]
		}
		return fmt.Sprintf("%s, taxi to and hold short of runway %s%s%s", cs, p[ParamRunway], entry, via)
	case IntentGiveWay:
		return fmt.Sprintf("%s, give way to the %s", cs, p[ParamGiveWay])
	case IntentRunwayChange:
		return cs + ", " + runwayChange(p)
	case IntentTaxiLimit:
		if p[ParamLimit] == "" {
			return fmt.Sprintf("%s, taxi%s, hold position at the marked point", cs, via)
		}
		return fmt.Sprintf("%s, taxi%s, hold short of %s", cs, via, p[ParamLimit]) // 12.3.4.8
	case IntentCross:
		return fmt.Sprintf("%s, cross runway %s", cs, p[ParamRunway]) // 12.3.4.9
	case IntentLineUp:
		return fmt.Sprintf("%s, runway %s, line up and wait", cs, p[ParamRunway]) // 12.3.4.10
	case IntentTakeoff:
		// Its own transmission, never with the line-up (CAP 413 4.29).
		return fmt.Sprintf("%s, runway %s, cleared for take-off%s", cs, p[ParamRunway], wind) // 12.3.4.11 a
	case IntentLanding:
		return fmt.Sprintf("%s, runway %s, cleared to land%s", cs, p[ParamRunway], wind) // 12.3.4.16 a
	case IntentHoldPosition:
		return cs + ", hold position" // 12.3.4.8
	case IntentStop:
		return fmt.Sprintf("%s, stop immediately, %s, stop immediately", cs, cs) // 12.3.4.11 e
	case IntentCancelTakeoff:
		return cs + ", hold position, cancel take-off, I say again, cancel take-off" // 12.3.4.11 c
	case IntentGoAround:
		reason := ""
		if p[ParamReason] != "" {
			reason = ", " + p[ParamReason]
		}
		return cs + ", go around, I say again, go around" + reason // 12.3.4.18; CAP 413 4.64
	case IntentSequence:
		// The number in traffic (CAP 413 6.23) and how it is spaced: a speed
		// (Doc 4444 12.4.1.6), or the delay it is to expect.
		s := fmt.Sprintf("%s, number %s", cs, p[ParamNumber])
		if p[ParamSpeed] != "" {
			s += fmt.Sprintf(", for spacing reduce speed to %s knots", p[ParamSpeed])
		}
		if p[ParamDelay] != "" {
			s += fmt.Sprintf(", expect %s minutes delay", p[ParamDelay])
		}
		return s
	case IntentDirect:
		return fmt.Sprintf("%s, proceed direct to final, number %s", cs, p[ParamNumber])
	case IntentHold:
		// Doc 4444 12.3.3.3 b; CAP 413 6.11.
		return fmt.Sprintf("%s, hold at %s as published, maintain %s, expect further clearance at %s", cs, p[ParamFix], p[ParamLevel], p[ParamExpect])
	case IntentLeaveHold:
		return fmt.Sprintf("%s, leave %s, number %s, continue the arrival", cs, p[ParamFix], p[ParamNumber])
	case IntentHoldLevel:
		return fmt.Sprintf("%s, descend to %s", cs, p[ParamLevel]) // 12.3.1.2 a
	case IntentSpeed:
		verb := "increase"
		if p[ParamSlower] == "true" {
			verb = "reduce"
		}
		return fmt.Sprintf("%s, %s speed to %s knots%s", cs, verb, p[ParamSpeed], why) // 12.4.1.6
	case IntentLevel:
		return fmt.Sprintf("%s, %s to %s%s", cs, p[ParamClimb], p[ParamLevel], why) // 12.3.1.2 a
	case IntentHeading:
		return fmt.Sprintf("%s, turn %s heading %s%s", cs, p[ParamTurn], p[ParamHeading], why) // 12.4.1.3
	case IntentContact:
		station := strings.TrimSpace(p[ParamStation] + " " + p[ParamFreq])
		if p[ParamWhen] != "" {
			return fmt.Sprintf("%s, %s contact %s", cs, p[ParamWhen], station) // 12.3.4.20; CAP 413 4.68
		}
		return fmt.Sprintf("%s, contact %s", cs, station) // 12.3.1.4 a
	}
	return cs + ", " + string(in)
}

// entryOf is the holding point said in a taxi clearance: " F", "" none.
func entryOf(p map[string]string) string {
	if p[ParamEntry] == "" {
		return ""
	}
	return " " + p[ParamEntry]
}

// departureClearance is a departure clearance after the call sign, which a
// readback repeats: "cleared to Frankfurt, BALTU 7D departure, flight
// planned route, runway 24, climb via SID to flight level 100, squawk 4521".
func departureClearance(p map[string]string) string {
	s := "cleared"
	if p[ParamDest] != "" {
		s += " to " + p[ParamDest] + ","
	}
	if p[ParamSID] != "" {
		s += " " + p[ParamSID] + " departure,"
	}
	s += " flight planned route" // Doc 4444 12.3.2.2
	if p[ParamRunway] != "" {
		s += ", runway " + p[ParamRunway]
	}
	switch {
	case p[ParamLevel] != "" && p[ParamSID] != "":
		s += ", climb via SID to " + p[ParamLevel] // 12.3.1.2 z
	case p[ParamLevel] != "":
		s += ", climb to " + p[ParamLevel] // no SID: the level alone
	}
	if p[ParamSquawk] != "" {
		s += ", squawk " + p[ParamSquawk]
	}
	return s
}

// arrivalClearance is an arrival clearance after the call sign: "cleared
// VLM 6T arrival, runway 06, descend to flight level 100, expect ILS
// approach".
func arrivalClearance(p map[string]string) string {
	s := "cleared " + p[ParamSTAR] + " arrival"
	if p[ParamRunway] != "" {
		s += ", runway " + p[ParamRunway]
	}
	if p[ParamLevel] != "" {
		s += ", descend to " + p[ParamLevel]
	}
	if p[ParamApproach] != "" {
		s += ", expect " + p[ParamApproach] + " approach"
	}
	return s
}

// ClearedApproach clears an arrival for the approach (kind "ILS", ""
// none named) to runway: "CSA1, cleared ILS approach runway 24".
func ClearedApproach(cs, kind, runway string) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentApproachClearance, Params: map[string]string{ParamApproach: kind, ParamRunway: runway}})
}

// approachClearance is an approach clearance after the call sign.
func approachClearance(p map[string]string) string {
	if p[ParamApproach] == "" {
		return "cleared approach runway " + p[ParamRunway]
	}
	return "cleared " + p[ParamApproach] + " approach runway " + p[ParamRunway]
}

// WindSaid is the surface wind as a tower says it, magnetic degrees:
// "wind 100 degrees 6 knots", "wind 270 degrees 18 knots gusting 28 knots",
// "wind calm" below a knot (Doc 4444 12.3.1.8 a).
func WindSaid(dirMag, kts, gustKts float64) string {
	if kts < 1 {
		return "wind calm"
	}
	dir := int(math.Round(dirMag/10)*10) % 360
	if dir == 0 {
		dir = 360
	}
	s := fmt.Sprintf("wind %03d degrees %.0f knots", dir, kts)
	if gustKts >= kts+10 {
		s += fmt.Sprintf(" gusting %.0f knots", gustKts)
	}
	return s
}

// SaidProcedure is a SID or STAR as said: the designator split before its
// number ("BALT7D" → "BALT 7D"), with the fix it is named after written
// out when fix starts with it ("BALT7D", "BALTU" → "BALTU 7D").
func SaidProcedure(designator, fix string) string {
	i := strings.IndexFunc(designator, func(r rune) bool { return r >= '0' && r <= '9' })
	if i <= 0 {
		return designator
	}
	name := designator[:i]
	if fix != "" && strings.HasPrefix(strings.ToUpper(fix), strings.ToUpper(name)) {
		name = strings.ToUpper(fix)
	}
	return name + " " + designator[i:]
}

// LevelSaid is a level as ATC says it: "flight level 210" above 10000 ft,
// "9000 feet" at or below (Doc 4444 12.3.1.1). LevelSaidAbove takes the
// airport's transition altitude.
func LevelSaid(altFt float64) string { return LevelSaidAbove(altFt, 10000) }

// LevelSaidAbove is a level as said with flight levels above the transition
// altitude transitionFt (LKPR: 5000): "flight level 070", "5000 feet".
func LevelSaidAbove(altFt, transitionFt float64) string {
	if transitionFt <= 0 {
		transitionFt = 10000
	}
	if altFt > transitionFt {
		return fmt.Sprintf("flight level %03d", int(math.Round(altFt/100)))
	}
	return fmt.Sprintf("%.0f feet", math.Round(altFt/100)*100)
}

// Transmission builders: the controller's position, intent and parameters
// of each clearance, with its text (Say).

// DepartureClearance is what a departure clearance gives, as said.
type DepartureClearance struct {
	Destination string // the clearance limit: "Frankfurt"
	SID         string // SaidProcedure: "BALTU 7D"
	Runway      string
	Level       string // the initial climb, LevelSaid: "5000 feet"
	Squawk      string // "4521"
}

// ClearedDeparture is the departure clearance: "CSA123, cleared to
// Frankfurt, BALTU 7D departure, runway 24, climb via SID to 5000 feet,
// squawk 4521".
func ClearedDeparture(cs string, c DepartureClearance) Transmission {
	return Say(Transmission{Position: PosDelivery, Callsign: cs, Intent: IntentDepartureClearance, Params: map[string]string{
		ParamDest: c.Destination, ParamSID: c.SID, ParamRunway: c.Runway, ParamLevel: c.Level, ParamSquawk: c.Squawk}})
}

// ClearedArrival clears an arrival's STAR (SaidProcedure) to runway,
// descending to level (LevelSaid, "" none), expecting approach ("ILS").
func ClearedArrival(cs, star, approach, runway, level string) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentArrivalClearance, Params: map[string]string{ParamSTAR: star, ParamApproach: approach, ParamRunway: runway, ParamLevel: level}})
}

// ClearedStartUp approves the start-up (Doc 4444 12.3.4.3).
func ClearedStartUp(cs string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentStartUp})
}

// ClearedPushbackAndStartUp approves the pushback and the start-up in one.
func ClearedPushbackAndStartUp(cs string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentPushback, Params: map[string]string{ParamStartUp: "1"}})
}

// ClearedPushback approves the pushback (Doc 4444 12.3.4.4).
func ClearedPushback(cs string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentPushback})
}

// WithFacing adds where the push ends facing ("east") to a pushback
// approval: "CSA1, pushback approved, facing east".
func WithFacing(t Transmission, facing string) Transmission {
	if facing == "" || t.Intent != IntentPushback {
		return t
	}
	p := map[string]string{ParamFacing: facing}
	for k, v := range t.Params {
		p[k] = v
	}
	t.Params, t.Text = p, ""
	return Say(t)
}

// ClearedTaxiToRunway clears a departure to the holding point (entry: an
// intersection's, "" full length) of runway via taxiways (as said).
func ClearedTaxiToRunway(cs, runway, entry string, taxiways []string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentTaxi, Params: map[string]string{ParamRunway: runway, ParamEntry: entry, ParamTaxiways: strings.Join(taxiways, ", ")}})
}

// ClearedTaxiToStand clears an arrival to its stand via taxiways.
func ClearedTaxiToStand(cs, stand string, taxiways []string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentTaxi, Params: map[string]string{ParamStand: stand, ParamTaxiways: strings.Join(taxiways, ", ")}})
}

// RunwayChange tells an aircraft of a new runway in use (#456): a
// departure its SID (sid, at pos ground or delivery), an arrival its STAR
// and approach: "CSA1, runway change, runway 06 in use, VOZ 2D departure".
func RunwayChange(pos Position, cs, runway, sid, star, approach string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentRunwayChange, Params: map[string]string{ParamRunway: runway, ParamSID: sid, ParamSTAR: star, ParamApproach: approach}})
}

// runwayChange is a runway change after the call sign.
func runwayChange(p map[string]string) string {
	s := "runway change, runway " + p[ParamRunway] + " in use"
	if p[ParamSID] != "" {
		s += ", " + p[ParamSID] + " departure"
	}
	if p[ParamSTAR] != "" {
		s += ", " + p[ParamSTAR] + " arrival"
	}
	if p[ParamApproach] != "" {
		s += ", expect " + p[ParamApproach] + " approach"
	}
	return s
}

// GiveWay tells a taxiing aircraft to give way to other traffic,
// described by type and how it passes: "CSA1, give way to the A320
// passing left to right".
func GiveWay(cs, traffic string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentGiveWay, Params: map[string]string{ParamGiveWay: traffic}})
}

// ClearedTaxiUpTo clears as far as a limit: hold short of taxiway limit
// ("" a marked point).
func ClearedTaxiUpTo(cs string, taxiways []string, limit string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentTaxiLimit, Params: map[string]string{ParamTaxiways: strings.Join(taxiways, ", "), ParamLimit: limit}})
}

// ClearedCross clears crossing runway (a runway name, "12/30"): on the
// ground frequency, the tower having agreed (the aircraft stays with
// ground across it, as at most airports).
func ClearedCross(cs, runway string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentCross, Params: map[string]string{ParamRunway: runway}})
}

// ClearedLineUp is "line up and wait" on runway.
func ClearedLineUp(cs, runway string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentLineUp, Params: map[string]string{ParamRunway: runway}})
}

// ClearedTakeoff clears the take-off from runway, with the wind (WindSaid,
// "" none). Given at the holding point it means line up and take off.
func ClearedTakeoff(cs, runway, wind string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentTakeoff, Params: map[string]string{ParamRunway: runway, ParamWind: wind}})
}

// ClearedToLand clears the landing on runway, with the wind (WindSaid, ""
// none): "CSA123, runway 06, cleared to land, wind 100 degrees 6 knots".
func ClearedToLand(cs, runway, wind string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentLanding, Params: map[string]string{ParamRunway: runway, ParamWind: wind}})
}

// HoldPosition, Stop and CancelTakeoff stop an aircraft on the ground;
// GoAround sends an arrival around (reason "": none said).
func HoldPosition(cs string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentHoldPosition})
}

func Stop(cs string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentStop})
}

func CancelTakeoff(cs string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentCancelTakeoff})
}

func GoAround(cs, reason string) Transmission {
	p := map[string]string{}
	if reason != "" {
		p[ParamReason] = reason
	}
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentGoAround, Params: p})
}

// Sequenced tells an arrival its number and how it is spaced: the speed it
// is to fly (a.SpeedKts), and the delay to expect, in whole minutes, when
// more than speed (path stretching, a hold) absorbs it.
func Sequenced(cs string, number int, delay time.Duration, a Absorption) Transmission {
	p := map[string]string{ParamNumber: fmt.Sprint(number), ParamLose: a.String()}
	if a.SpeedKts > 0 {
		p[ParamSpeed] = fmt.Sprintf("%.0f", a.SpeedKts)
	}
	if min := int(math.Round(delay.Minutes())); min >= 1 && (a.ExtraNM > 0 || a.Left > 0) {
		p[ParamDelay] = fmt.Sprint(min)
	}
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentSequence, Params: p})
}

// DirectToFinal sends an arrival direct to the final.
func DirectToFinal(cs string, number int) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentDirect, Params: map[string]string{ParamNumber: fmt.Sprint(number)}})
}

// HoldAt holds an arrival at fix with entry at altFt, expecting further
// clearance at efc.
func HoldAt(cs, fix string, entry HoldEntry, altFt float64, efc time.Time) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHold, Params: map[string]string{
		ParamFix: fix, ParamHoldIn: entry.String(), ParamAltitude: fmt.Sprintf("%.0f", altFt), ParamLevel: LevelSaid(altFt), ParamExpect: efc.Format("1504")}})
}

// LeaveHoldAt releases an arrival from the hold at fix as number.
func LeaveHoldAt(cs, fix string, number int) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentLeaveHold, Params: map[string]string{ParamFix: fix, ParamNumber: fmt.Sprint(number)}})
}

// HoldDescend steps a holding arrival down to altFt.
func HoldDescend(cs string, altFt float64) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHoldLevel, Params: map[string]string{ParamAltitude: fmt.Sprintf("%.0f", altFt), ParamLevel: LevelSaid(altFt)}})
}

// Resolved is a conflict resolution for an aircraft now at altFt, heading
// hdg and kts, said by pos (center en route, approach near the airport).
func Resolved(pos Position, r Resolution, altFt, hdg, kts float64) Transmission {
	t := Transmission{Position: pos, Callsign: r.Callsign, Params: map[string]string{ParamTraffic: r.Why}}
	switch r.Kind {
	case ResolveSpeed:
		t.Intent = IntentSpeed
		t.Params[ParamSpeed] = fmt.Sprintf("%.0f", r.Kts)
		if r.Kts < kts {
			t.Params[ParamSlower] = "true"
		}
	case ResolveLevel:
		t.Intent = IntentLevel
		t.Params[ParamLevel] = LevelSaid(r.AltFt)
		t.Params[ParamClimb] = "climb"
		if r.AltFt < altFt {
			t.Params[ParamClimb] = "descend"
		}
	default:
		t.Intent = IntentHeading
		t.Params[ParamHeading] = fmt.Sprintf("%03.0f", r.HeadingDeg)
		t.Params[ParamTurn] = "right"
		if math.Mod(r.HeadingDeg-hdg+540, 360)-180 < 0 {
			t.Params[ParamTurn] = "left"
		}
	}
	return Say(t)
}

// Handoff hands an aircraft from position from to position to: "CSA123,
// contact Praha Tower 118.105", said by from. station is the next
// position as said ("" its name: "Tower"), freq its frequency ("" none said).
func Handoff(cs string, from, to Position, station, freq string) Transmission {
	if station == "" {
		station = PositionName(to)
	}
	return Say(Transmission{Position: from, Callsign: cs, Intent: IntentContact,
		Params: map[string]string{ParamPosition: string(to), ParamStation: station, ParamFreq: freq}})
}

// WhenVacatedContact is the tower's transfer of a landing aircraft: "CSA123,
// when vacated contact Ruzyne Ground 121.91" (Doc 4444 12.3.4.20).
func WhenVacatedContact(cs string, from, to Position, station, freq string) Transmission {
	t := Handoff(cs, from, to, station, freq)
	t.Params[ParamWhen] = "when vacated"
	return Say(t)
}

// PositionName is a position as said: "Tower", "Ground", "Delivery".
func PositionName(p Position) string {
	s := string(p)
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// StationName is a frequency's name as said: "PRAHA TOWER" → "Praha
// Tower"; a name without its position gets it (MSFS names LKPR's tower
// "RUZYNE": "Ruzyne Tower"); "" the position's name.
func StationName(name string, p Position) string {
	if strings.TrimSpace(name) == "" {
		return PositionName(p)
	}
	words := strings.Fields(strings.ToLower(name))
	named := false
	for i, w := range words {
		switch w {
		case "tower", "ground", "approach", "departure", "delivery", "clearance", "center", "centre", "control", "radar", "director", "information", "atis", "radio", "apron":
			named = true
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	if !named {
		words = append(words, PositionName(p))
	}
	return strings.Join(words, " ")
}

// DeparturePosition is the position working a departure in state s:
// delivery while it gets its clearance, ground to its runway's holding
// point (and across other runways: crossings are on the ground
// frequency), tower from its runway's holding point to the take-off, and
// departure once airborne and handed to MSFS AI. atOwnRunway: holding
// short of the runway it departs from.
func DeparturePosition(s TaxiState, atOwnRunway bool) Position {
	switch s {
	case TaxiIdle, TaxiSpawning:
		return PosDelivery
	case TaxiHoldingShort:
		if atOwnRunway {
			return PosTower
		}
		return PosGround
	case TaxiLiningUp, TaxiLinedUp, TaxiDeparting:
		return PosTower
	case TaxiComplete:
		return PosDeparture
	}
	return PosGround
}

// ArrivalPosition is the position working an arrival in state s: approach
// on the STAR and approach, tower once established on the final (onFinal)
// and through the landing roll and the vacating, ground from there to the
// stand.
func ArrivalPosition(s ArrivalState, onFinal bool) Position {
	switch s {
	case ArrivalApproaching:
		if onFinal {
			return PosTower
		}
		return PosApproach
	case ArrivalLanding, ArrivalRollout, ArrivalVacating:
		return PosTower
	}
	if s < ArrivalApproaching {
		return PosApproach
	}
	return PosGround
}

// SpeakingTime is how long text takes to say on the radio: a controller's
// pace, about 160 words a minute, and a breath.
func SpeakingTime(text string) time.Duration {
	return time.Duration(len(strings.Fields(text)))*375*time.Millisecond + 500*time.Millisecond
}

// RadioOptions tune a Radio.
type RadioOptions struct {
	// Keep: how many recent transmissions are kept (default 200).
	Keep int
	// Now stamps transmissions (default time.Now; SimClock.Now for traffic
	// time).
	Now func() time.Time
	// OnTransmission is called with every transmission, in order.
	OnTransmission func(Transmission)
	// FrequencyOf is the frequency of position pos at airport, as set
	// ("118.105"; "" none): filled into transmissions without one (#416).
	FrequencyOf func(airport string, pos Position) string
	// ReadBack: our pilots read back every clearance to them (#417), on
	// the same frequency, after it.
	ReadBack bool
	// SaidCallsign writes a call sign as said ("DLH1675" → "Lufthansa
	// 1675", ScheduleConfig.SaidCallsign): the text of each transmission
	// uses it, so text and voice are the same (#462). Callsign keeps the
	// ICAO form.
	SaidCallsign func(cs string) string
}

// Radio carries the transmissions of our controllers (and, with #417,
// their pilots): each is stamped, kept and handed to OnTransmission.
type Radio struct {
	opts RadioOptions
	mu   sync.Mutex
	kept []Transmission
	busy map[string]time.Time // by frequency: said until
}

// NewRadio creates a radio.
func NewRadio(opts RadioOptions) *Radio {
	if opts.Keep <= 0 {
		opts.Keep = 200
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Radio{opts: opts, busy: map[string]time.Time{}}
}

// Transmit sends t: stamped (when not already) at airport, on its
// position's frequency, kept, and handed on. One transmission at a time on
// a frequency: while one is said, the next is stamped for when it ends
// (SpeakingTime and a second's pause), so a voice plays them in turn. It
// returns t as sent: stamped, on its frequency.
func (r *Radio) Transmit(airport string, t Transmission) Transmission {
	if t.At.IsZero() {
		t.At = r.opts.Now()
	}
	if t.Airport == "" {
		t.Airport = airport
	}
	if t.Text == "" {
		t = Say(t)
	}
	if r.opts.SaidCallsign != nil && t.Callsign != "" {
		if said := r.opts.SaidCallsign(t.Callsign); said != t.Callsign {
			t.Text = strings.ReplaceAll(t.Text, t.Callsign, said)
		}
	}
	if t.Frequency == "" && r.opts.FrequencyOf != nil {
		t.Frequency = r.opts.FrequencyOf(t.Airport, t.Position)
	}
	r.mu.Lock()
	if t.Frequency != "" {
		key := t.Airport + " " + t.Frequency
		if until := r.busy[key]; t.At.Before(until) {
			t.At = until
		}
		r.busy[key] = t.At.Add(SpeakingTime(t.Text) + time.Second)
	}
	r.kept = append(r.kept, t)
	if len(r.kept) > r.opts.Keep {
		r.kept = r.kept[len(r.kept)-r.opts.Keep:]
	}
	on := r.opts.OnTransmission
	r.mu.Unlock()
	if on != nil {
		on(t)
	}
	if r.opts.ReadBack && !t.Pilot && t.Callsign != "" {
		if rb, ok := Readback(t); ok {
			rb.Airport, rb.Frequency = t.Airport, t.Frequency
			r.Transmit(t.Airport, rb)
		}
	}
	return t
}

// ClearAt is when freq at airport is clear again: the end of what is said
// on it, readbacks included, and a breath (#462: a crew acts on a
// clearance once it has read it back).
func (r *Radio) ClearAt(airport, freq string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.busy[airport+" "+freq]
}

// Recent is up to n of the latest transmissions at airport ("" all),
// oldest first.
func (r *Radio) Recent(airport string, n int) []Transmission {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Transmission
	for i := len(r.kept) - 1; i >= 0 && (n <= 0 || len(out) < n); i-- {
		if airport == "" || r.kept[i].Airport == airport {
			out = append(out, r.kept[i])
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
