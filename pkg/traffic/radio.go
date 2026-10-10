package traffic

import (
	"fmt"
	"math"
	"math/rand/v2"
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
	// PosInformation: flight information (FIS), a VFR flight's station
	// outside the zone (ContactFIS).
	PosInformation Position = "information"
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
	IntentFollowTaxi         Intent = "follow_taxi"         // follow another aircraft on the ground ("follow the company Airbus")
	IntentRunwayChange       Intent = "runway_change"       // a new runway in use: new SID or STAR (#456)
	IntentCross              Intent = "cross"               // cross a runway
	IntentLineUp             Intent = "line_up"             // line up and wait
	IntentTakeoff            Intent = "takeoff"             // cleared for take-off
	IntentLanding            Intent = "landing"             // cleared to land
	IntentHoldPosition       Intent = "hold_position"       // hold position (on the ground)
	IntentContinueTaxi       Intent = "continue_taxi"       // continue taxi after holding position
	IntentStop               Intent = "stop"                // stop immediately (a take-off roll)
	IntentCancelTakeoff      Intent = "cancel_takeoff"      // hold position, cancel take-off clearance
	IntentGoAround           Intent = "go_around"           // go around (ParamReason: why)
	IntentSequence           Intent = "sequence"            // number, delay and how it is lost
	IntentDirect             Intent = "direct"              // direct to the final
	IntentHold               Intent = "hold"                // hold at a fix
	IntentLeaveHold          Intent = "leave_hold"          // leave the hold, continue the arrival
	IntentHoldLevel          Intent = "hold_level"          // descend in the hold
	IntentAirportClosed      Intent = "airport_closed"      // every runway closed: expect holding
	IntentDivert             Intent = "divert"              // the airport stays closed: to an alternate
	IntentSpeed              Intent = "speed"               // reduce or increase speed
	IntentLevel              Intent = "level"               // climb or descend
	IntentCrossLevel         Intent = "cross_level"         // cross a fix at or above (below) a level (#662)
	IntentDescendVia         Intent = "descend_via"         // descend via the STAR to a level (#754)
	IntentHeading            Intent = "heading"             // turn left or right heading
	IntentContact            Intent = "contact"             // a handoff: contact the next position (#416)
	IntentIdentified         Intent = "identified"          // radar identification after the departure's check-in, with its climb
	IntentWeather            Intent = "weather"             // the wind and QNH, asked for by the crew
	IntentDirectTo           Intent = "direct_to"           // cleared direct to a fix, asked for by the crew
	IntentVector             Intent = "vector"              // a radar vector off the STAR, or back onto it (#661)
	IntentUnableDirect       Intent = "unable_direct"       // a crew's direct refused for traffic (#621)
	IntentVFRDeparture       Intent = "vfr_departure"       // VFR departure instructions (CAP 413 Figure 24)
	// VFR in the aerodrome traffic circuit (#569; Doc 4444 12.3.4.13–17).
	IntentJoinCircuit   Intent = "join_circuit"    // join (left/right) (position in circuit) runway, QNH
	IntentStraightIn    Intent = "straight_in"     // make straight-in approach, runway
	IntentFollow        Intent = "follow"          // number (n), follow (traffic)
	IntentCircuitInstr  Intent = "circuit_instr"   // make short/long approach, extend downwind, report base/final, continue approach
	IntentTouchAndGo    Intent = "touch_and_go"    // cleared touch and go
	IntentFullStop      Intent = "full_stop"       // make full stop
	IntentCircuitDelay  Intent = "circuit_delay"   // circle the aerodrome, orbit, make another circuit
	IntentVFRForLanding Intent = "vfr_for_landing" // pilot: (type) (position) (level) [information] for landing
	IntentCircuitReport Intent = "circuit_report"  // pilot: (position in circuit), e.g. downwind
	// The crew decides on its own (#621): pilot "going around"; ATC
	// acknowledges, "roger".
	IntentPilotGoAround Intent = "pilot_go_around"
	IntentPilotReject   Intent = "pilot_reject" // the crew rejects the take-off: "stopping"
	IntentAcknowledge   Intent = "acknowledge"
	IntentTrafficInfo   Intent = "traffic_info" // traffic, (o'clock), (distance), (direction), (type), (level) (#570)
	// The tower's answer to a departure checking in while taxiing: report
	// ready for departure, or hold short with the number to depart before
	// it (CAP 413 4.19, 4.20).
	IntentDepartureOrder Intent = "departure_order"
)

// Phraseology is the wording a transmission is said in: ICAO (Doc 4444,
// "" also means ICAO) or the FAA's (JO 7110.65), by the airport's region
// (PhraseologyFor). docs/traffic-phraseology.md has both side by side.
type Phraseology string

const (
	PhraseologyICAO Phraseology = "icao"
	PhraseologyFAA  Phraseology = "faa"
)

// PhraseologyFor is the phraseology at airport icao: FAA in the United
// States and its territories (K, PA Alaska, PH Hawaii, PG Guam and the
// Northern Marianas, TJ Puerto Rico, TI the Virgin Islands), ICAO
// elsewhere.
func PhraseologyFor(icao string) Phraseology {
	icao = strings.ToUpper(icao)
	if strings.HasPrefix(icao, "K") && len(icao) == 4 {
		return PhraseologyFAA
	}
	for _, p := range []string{"PA", "PH", "PG", "TJ", "TI"} {
		if strings.HasPrefix(icao, p) {
			return PhraseologyFAA
		}
	}
	return PhraseologyICAO
}

// Parameter keys of a transmission. Values are the text as said (a runway
// "24", taxiways "B2, H, A", a level "FL210" or "9000 ft").
const (
	ParamRunway    = "runway"
	ParamEntry     = "entry"     // where an intersection departure enters its runway ("B")
	ParamBacktrack = "backtrack" // "1": enter and taxi back along the runway to its threshold first
	ParamStartUp   = "startup"   // "1": the start-up asked for or approved with the pushback
	ParamFacing    = "facing"    // where a push ends facing: "east"
	ParamBehind    = "behind"    // a conditional line-up: the landing traffic as said ("A320")
	// ParamBehindHow: what the traffic of a conditional line-up does,
	// "landing" ("" too) or "departing".
	ParamBehindHow  = "behindHow"
	ParamGiveWay    = "giveway"   // the traffic given way to, as described: "A320 passing left to right"
	ParamTaxiways   = "taxiways"  // as said: "B2, H, A"
	ParamHoldShort  = "holdShort" // runways to hold short of on the way: "12", "12, 31"
	ParamStand      = "stand"
	ParamLimit      = "limit" // a taxiway to hold short of; "" a marked point
	ParamSID        = "sid"
	ParamSTAR       = "star"
	ParamApproach   = "approach" // the approach expected ("ILS")
	ParamReason     = "reason"
	ParamNumber     = "number" // in the landing sequence
	ParamDelay      = "delay"
	ParamLose       = "lose" // how the delay is lost: "210 kt, +3.2 NM"
	ParamFix        = "fix"
	ParamAirport    = "airport"     // an airport's ICAO (closed: its runways)
	ParamAlternate  = "alternate"   // the airport diverted to
	ParamHoldIn     = "entry_type"  // hold entry: direct, teardrop, parallel
	ParamAltitude   = "altitude"    // feet
	ParamExpect     = "expect"      // expect further clearance, HH:MM
	ParamSpeed      = "speed"       // knots
	ParamFinalSpeed = "final_speed" // "1": reduce to final approach speed
	ParamOrbit      = "orbit"       // a 360 for spacing: "left" or "right"
	ParamLevel      = "level"       // "flight level 210" or "altitude 9000 feet"
	ParamHeading    = "heading"     // degrees, three digits
	ParamTurn       = "turn"        // left, right
	ParamClimb      = "climb"       // climb, descend
	ParamSlower     = "slower"      // "true": reduce, else increase
	ParamResume     = "resume"      // "true": resume normal speed (a speed given before ends)
	ParamTraffic    = "traffic"     // why a resolution: "traffic DLH2, 0.8 NM in 2m40s"
	ParamPosition   = "position"    // a handoff's next position
	ParamStation    = "station"     // … as said: "Praha Tower"
	ParamFreq       = "frequency"   // … its frequency: "118.105"
	ParamWhen       = "when"        // … a condition: "when vacated"
	ParamDest       = "destination" // a clearance limit as said: "Frankfurt"
	ParamSquawk     = "squawk"      // SSR code: "4521"
	ParamWind       = "wind"        // as said: "wind 100 degrees 6 knots"
	ParamQNH        = "qnh"         // hPa: "1013" (FAA: inches, ParamAltimeter)
	ParamAltimeter  = "altimeter"   // inches of mercury ×100: "2992"
	ParamReport     = "report"      // what to report: "established"
	ParamFor        = "for"         // a vector's reason: "spacing", "base"
	// A vector said with another call (WithVector): its words and their
	// readback, after the call's own.
	ParamAlsoSaid     = "alsoSaid"
	ParamAlsoReadback = "alsoReadback"
	ParamIntercept    = "intercept" // the heading to intercept the final, three digits
	ParamRush         = "rush"      // "1": expedite (immediate take-off, expedite crossing, vacating, climb)
	ParamNoDelay      = "no_delay"  // a take-off with traffic on final: its distance in whole NM, "5"
	ParamCircuit      = "circuit"   // a position in the circuit as said: "left downwind", "base", "final"
	ParamInstr        = "instr"     // an approach instruction or delay as said: "extend downwind", "orbit right"
	ParamType         = "type"      // an aircraft type as said: "Cessna 172"
)

// Transmission is one message on the radio.
type Transmission struct {
	At        time.Time `json:"at"`
	Airport   string    `json:"airport,omitempty"`
	Frequency string    `json:"frequency,omitempty"` // with #416
	Position  Position  `json:"position"`            // the controller's
	// Controller is who works the frequency (#722): the same on every
	// frequency one person works, so one voice; "" unknown.
	Controller string            `json:"controller,omitempty"`
	Pilot      bool              `json:"pilot,omitempty"` // said by the pilot (#417)
	Callsign   string            `json:"callsign"`
	Intent     Intent            `json:"intent"`
	Params     map[string]string `json:"params,omitempty"`
	Text       string            `json:"text"`
	// Phraseology is the wording of Text ("" ICAO); the Radio sets FAA at
	// US airports (RadioOptions.Phraseology).
	Phraseology Phraseology `json:"phraseology,omitempty"`
	// Tempo is how fast it is said against a normal pace (0 or 1 normal,
	// above 1 faster): a busy frequency, an urgent call (RadioOptions.TempoOf).
	// SpeakingTime shrinks with it.
	Tempo float64 `json:"tempo,omitempty"`
}

// Say is t with its text: the ATC phrase (ICAO phraseology, the
// application's normal tokens) for its intent and parameters.
func Say(t Transmission) Transmission {
	if t.Phraseology == PhraseologyFAA {
		if s, ok := phraseFAA(t.Callsign, t.Intent, t.Params); ok {
			t.Text = s
			return t
		}
	}
	t.Text = phrase(t.Callsign, t.Intent, t.Params)
	return t
}

// phraseFAA is the FAA wording (JO 7110.65, as quoted in
// docs/traffic-phraseology.md) where it differs from ICAO's; false: the
// ICAO text stands.
func phraseFAA(cs string, in Intent, p map[string]string) (string, bool) {
	if in == IntentDescendVia {
		// 4-5-7 h: "Descend via the Eagul Five arrival.", the STAR's
		// published altitudes; no level.
		return cs + ", descend via the " + p[ParamSTAR] + " arrival", true
	}
	if in == IntentRevisedDeparture {
		return cs + ", " + revisedDeparture(p, true), true
	}
	rwy := "runway " + p[ParamRunway]
	if p[ParamEntry] != "" {
		rwy += " at " + p[ParamEntry] // intersection (3-9-4, 3-9-10)
	}
	switch in {
	case IntentDepartureClearance:
		// 4-3-3: "Cleared to (airport); (SID) departure; then, as filed.
		// Maintain (altitude)."; 4-3-2: "Climb via SID except maintain".
		s := cs + ", cleared"
		if p[ParamDest] != "" {
			s += " to " + p[ParamDest] + " airport"
		}
		if p[ParamSID] != "" {
			s += ", " + p[ParamSID] + " departure"
		}
		s += ", then as filed"
		switch lvl := faaLevel(p[ParamLevel]); {
		case lvl != "" && p[ParamSID] != "":
			s += ", climb via SID except maintain " + lvl
		case lvl != "":
			s += ", maintain " + lvl
		}
		if p[ParamSquawk] != "" {
			s += ", squawk " + p[ParamSquawk]
		}
		return s, true
	case IntentTaxi:
		if p[ParamStand] != "" {
			return "", false
		}
		via := ""
		if p[ParamTaxiways] != "" {
			via = " via " + p[ParamTaxiways]
		}
		return fmt.Sprintf("%s, %s, taxi%s%s", cs, rwy, via, holdShortSaid(p)), true // 3-7-2: runway first, then hold short
	case IntentLineUp:
		// No conditional clearances on the runway in the FAA's rules.
		if p[ParamBacktrack] != "" {
			return fmt.Sprintf("%s, %s, back-taxi, line up and wait", cs, rwy), true
		}
		return fmt.Sprintf("%s, %s, line up and wait", cs, rwy), true // 3-9-4
	case IntentTakeoff:
		return fmt.Sprintf("%s, %s, cleared for takeoff", cs, rwy), true // 3-9-10; civil: no wind
	case IntentLanding:
		return fmt.Sprintf("%s, runway %s, cleared to land", cs, p[ParamRunway]), true // 3-10-5
	case IntentApproachClearance:
		kind, turn := p[ParamApproach], ""
		if p[ParamIntercept] != "" { // on vectors: the turn first (5-9-4)
			turn = "turn " + p[ParamTurn] + " heading " + p[ParamIntercept] + ", "
		}
		if kind == "" {
			return fmt.Sprintf("%s, %scleared approach runway %s", cs, turn, p[ParamRunway]), true
		}
		return fmt.Sprintf("%s, %scleared %s runway %s approach", cs, turn, kind, p[ParamRunway]), true // 4-8-1
	case IntentArrivalClearance:
		s := cs + ", cleared " + p[ParamSTAR] + " arrival"
		if lvl := faaLevel(p[ParamLevel]); lvl != "" {
			s += ", descend and maintain " + lvl // 4-5-7
		}
		if p[ParamAltimeter] != "" {
			s += ", altimeter " + p[ParamAltimeter] // 2-7-2
		}
		return s, true
	case IntentWeather:
		s := cs + ", " + p[ParamWind]
		if p[ParamAltimeter] != "" {
			s += ", altimeter " + p[ParamAltimeter] // 2-7-2
		}
		return s, true
	case IntentIdentified:
		s := cs + ", radar contact" // 5-3-7
		if lvl := faaLevel(p[ParamLevel]); lvl != "" {
			s += ", climb and maintain " + lvl // 4-5-7
		}
		return s, true
	}
	return "", false
}

// faaLevel is a level said the FAA's way: "5000 feet" is "5000" (no
// "feet"), flight levels as they are.
func faaLevel(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "altitude "), " feet")
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
	case IntentRevisedDeparture:
		return cs + ", " + revisedDeparture(p, false)
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
			return fmt.Sprintf("%s, taxi to stand %s%s%s", cs, p[ParamStand], via, holdShortSaid(p)) // CAP 413 4.68
		}
		// To the runway and hold short of it, as the project uses (the user's
		// choice over Doc 4444's "taxi to holding point"): an intersection is
		// "runway 24 at B".
		entry := ""
		if p[ParamEntry] != "" {
			entry = " at " + p[ParamEntry]
		}
		return fmt.Sprintf("%s, taxi to and hold short of runway %s%s%s%s", cs, p[ParamRunway], entry, via, holdShortSaid(p))
	case IntentGiveWay:
		return fmt.Sprintf("%s, give way to the %s", cs, p[ParamGiveWay])
	case IntentFollowTaxi:
		return fmt.Sprintf("%s, follow the %s", cs, p[ParamGiveWay])
	case IntentRunwayChange:
		return cs + ", " + runwayChange(p)
	case IntentTaxiLimit:
		if p[ParamLimit] == "" {
			return fmt.Sprintf("%s, taxi%s, hold position at the marked point", cs, via)
		}
		return fmt.Sprintf("%s, taxi%s, hold short of %s", cs, via, p[ParamLimit]) // 12.3.4.8
	case IntentCross:
		if p[ParamBehind] != "" {
			// Conditional, as a line-up: the condition first, "behind" again
			// at the end.
			return fmt.Sprintf("%s, behind the landing %s, cross runway %s, behind", cs, p[ParamBehind], p[ParamRunway])
		}
		if p[ParamRush] != "" {
			return fmt.Sprintf("%s, expedite crossing runway %s", cs, p[ParamRunway]) // 12.3.4.9
		}
		return fmt.Sprintf("%s, cross runway %s", cs, p[ParamRunway]) // 12.3.4.9
	case IntentLineUp:
		if p[ParamBehind] != "" {
			// Conditional: the condition first, "behind" again at the end.
			return fmt.Sprintf("%s, behind the %s %s, line up and wait runway %s, behind", cs, behindHow(p), p[ParamBehind], p[ParamRunway])
		}
		if p[ParamBacktrack] != "" {
			return fmt.Sprintf("%s, enter %s and backtrack, line up and wait", cs, runwayAt(p)) // no entry at the threshold (12.3.4.10, Doc 4444 BACKTRACK)
		}
		if p[ParamRush] != "" {
			return fmt.Sprintf("%s, %s, line up, be ready for immediate departure", cs, runwayAt(p)) // 12.3.4.10 h
		}
		return fmt.Sprintf("%s, %s, line up and wait", cs, runwayAt(p)) // 12.3.4.10; at an intersection, JO 7110.65 3-9-4
	case IntentTakeoff:
		// Its own transmission, never with the line-up (CAP 413 4.29).
		if p[ParamRush] != "" {
			return fmt.Sprintf("%s, %s, cleared for immediate take-off%s", cs, runwayAt(p), wind) // CAP 413 4.30
		}
		if p[ParamNoDelay] != "" {
			return fmt.Sprintf("%s, %s, cleared for take-off%s, no delay, traffic on %s mile final", cs, runwayAt(p), wind, p[ParamNoDelay])
		}
		return fmt.Sprintf("%s, %s, cleared for take-off%s", cs, runwayAt(p), wind) // 12.3.4.11 a; at an intersection, JO 7110.65 3-9-10
	case IntentLanding:
		return fmt.Sprintf("%s, runway %s, cleared to land%s", cs, p[ParamRunway], wind) // 12.3.4.16 a
	case IntentContinueTaxi:
		return cs + ", continue taxi"
	case IntentHoldPosition:
		return cs + ", hold position" // 12.3.4.8
	case IntentDepartureOrder:
		if p[ParamNumber] == "" || p[ParamNumber] == "0" {
			return cs + ", report ready for departure" // CAP 413 4.20
		}
		// CAP 413 4.19 ("hold at Bravo 1, 2 aircraft to depart before you
		// from runway 20"), the limit as the taxi clearance names it.
		return fmt.Sprintf("%s, hold short of runway %s, %s aircraft to depart before you", cs, p[ParamRunway], p[ParamNumber])
	case IntentStop:
		return fmt.Sprintf("%s, stop immediately, %s, stop immediately", cs, cs) // 12.3.4.11 e
	case IntentCancelTakeoff:
		return cs + ", hold position, cancel take-off, I say again, cancel take-off" // 12.3.4.11 c
	case IntentJoinCircuit:
		s := fmt.Sprintf("%s, join %s runway %s", cs, p[ParamCircuit], p[ParamRunway]) // 12.3.4.13 b, e
		if p[ParamWind] != "" {
			s += ", " + p[ParamWind]
		}
		if p[ParamQNH] != "" {
			s += ", QNH " + p[ParamQNH]
		}
		if p[ParamTraffic] != "" {
			s += ", traffic " + p[ParamTraffic]
		}
		return s
	case IntentStraightIn:
		s := fmt.Sprintf("%s, make straight-in approach, runway %s", cs, p[ParamRunway]) // 12.3.4.13 c
		if p[ParamWind] != "" {
			s += ", " + p[ParamWind]
		}
		if p[ParamQNH] != "" {
			s += ", QNH " + p[ParamQNH]
		}
		return s
	case IntentFollow:
		if p[ParamTraffic] == "" {
			return fmt.Sprintf("%s, number %s", cs, p[ParamNumber]) // number 1: no one to follow
		}
		return fmt.Sprintf("%s, number %s, follow %s", cs, p[ParamNumber], p[ParamTraffic]) // 12.3.4.14 b
	case IntentCircuitInstr, IntentCircuitDelay:
		return cs + ", " + p[ParamInstr] // 12.3.4.15 a–d, 12.3.4.17 a–c
	case IntentTouchAndGo:
		if p[ParamInstr] != "" {
			return cs + ", cleared " + p[ParamInstr] // stop and go (ClearedStopAndGo)
		}
		return cs + ", cleared touch and go" // 12.3.4.16 c
	case IntentFullStop:
		return cs + ", make full stop" // 12.3.4.16 d
	case IntentGoAround:
		reason := ""
		if p[ParamReason] != "" {
			reason = ", " + p[ParamReason]
		}
		return cs + ", go around, I say again, go around" + reason + goAroundInstr(p) // 12.3.4.18; CAP 413 4.64
	case IntentSequence:
		// The number in traffic (CAP 413 6.23) and how it is spaced: a speed
		// (Doc 4444 12.4.1.6), or the delay it is to expect.
		// The number is said once an approach (and when it changes): repeated
		// with every speed it only annoys; without it the call is the speed.
		s := cs
		if p[ParamNumber] != "" {
			s += ", number " + p[ParamNumber]
		}
		if p[ParamSpeed] != "" {
			s += fmt.Sprintf(", for spacing reduce speed to %s knots", p[ParamSpeed])
		}
		if p[ParamOrbit] != "" {
			s += fmt.Sprintf(", orbit %s for spacing", p[ParamOrbit])
		}
		if p[ParamExtendDownwind] != "" {
			s += ", extend downwind, expect vectors"
		}
		if p[ParamFinalSpeed] != "" {
			s += ", for spacing reduce to final approach speed"
		}
		if p[ParamDelay] != "" {
			s += fmt.Sprintf(", expect %s minutes delay", p[ParamDelay])
		}
		return s
	case IntentDirect:
		if p[ParamNumber] == "" {
			return cs + ", proceed direct to final"
		}
		return fmt.Sprintf("%s, proceed direct to final, number %s", cs, p[ParamNumber])
	case IntentHold:
		// Doc 4444 12.3.3.3 b; CAP 413 6.11.
		return fmt.Sprintf("%s, hold at %s as published, maintain %s, expect further clearance at %s", cs, p[ParamFix], p[ParamLevel], p[ParamExpect])
	case IntentAirportClosed:
		return fmt.Sprintf("%s, all runways at %s are closed, expect holding", cs, p[ParamAirport])
	case IntentDivert:
		return fmt.Sprintf("%s, %s remains closed, cleared to %s, proceed direct", cs, p[ParamAirport], p[ParamAlternate])
	case IntentLeaveHold:
		if p[ParamNumber] == "" {
			return fmt.Sprintf("%s, leave %s, continue the arrival", cs, p[ParamFix])
		}
		return fmt.Sprintf("%s, leave %s, number %s, continue the arrival", cs, p[ParamFix], p[ParamNumber])
	case IntentHoldLevel:
		return fmt.Sprintf("%s, descend to %s", cs, p[ParamLevel]) // 12.3.1.2 a
	case IntentSpeed:
		if p[ParamResume] == "true" {
			return cs + ", resume normal speed"
		}
		verb := "increase"
		if p[ParamSlower] == "true" {
			verb = "reduce"
		}
		return fmt.Sprintf("%s, %s speed to %s%s", cs, verb, spokenSpeed(p[ParamSpeed]), why) // 12.4.1.6
	case IntentLevel:
		switch p[ParamClimb] {
		case "stop":
			return fmt.Sprintf("%s, stop descent at %s%s", cs, p[ParamLevel], why)
		case "stop climb":
			return fmt.Sprintf("%s, stop climb at %s%s", cs, p[ParamLevel], why)
		case "maintain":
			return fmt.Sprintf("%s, maintain %s%s", cs, p[ParamLevel], why) // 12.3.2.3 a
		case "continue climb", "continue descent":
			// Plain "climb (or descend) to (level)" (12.3.1.2 a): the traffic it
			// was stopped for is known. Doc 4444's "clear of traffic [appropriate
			// instructions]" is for passing unknown traffic (12.4.1.8 d).
			return fmt.Sprintf("%s, %s to %s", cs, levelVerb(p[ParamClimb]), p[ParamLevel])
		}
		return fmt.Sprintf("%s, %s to %s%s", cs, p[ParamClimb], p[ParamLevel], why) // 12.3.1.2 a
	case IntentCrossLevel:
		return fmt.Sprintf("%s, cross %s at or %s %s%s", cs, p[ParamFix], p[ParamClimb], p[ParamLevel], why) // 12.3.2.4 a
	case IntentDescendVia:
		return cs + ", " + descendVia(p) // 6.5.2.4.1
	case IntentVisual:
		return cs + ", cleared visual approach runway " + p[ParamRunway] // 12.3.3.1 o
	case IntentHeading:
		return fmt.Sprintf("%s, turn %s heading %s%s", cs, p[ParamTurn], p[ParamHeading], why) // 12.4.1.3
	case IntentWeather:
		s := cs + ", " + p[ParamWind] // 12.3.1.8 a
		if p[ParamQNH] != "" {
			s += ", QNH " + p[ParamQNH] // 12.3.1.8 l
		}
		return s
	case IntentDirectTo:
		return fmt.Sprintf("%s, cleared direct to %s", cs, p[ParamFix]) // CAP 413 6.8
	case IntentVector:
		if p[ParamFix] != "" {
			return fmt.Sprintf("%s, resume own navigation direct %s", cs, p[ParamFix]) // 12.4.1.4 b
		}
		s := fmt.Sprintf("%s, fly heading %s", cs, p[ParamHeading]) // 12.4.1.3 d
		if p[ParamTurn] != "" {
			s = fmt.Sprintf("%s, turn %s heading %s", cs, p[ParamTurn], p[ParamHeading]) // 12.4.1.3 e
		}
		if p[ParamFor] != "" {
			s += ", for " + p[ParamFor] // 12.4.1.5 note b, d
		}
		return s
	case IntentUnableDirect:
		return cs + ", unable direct due traffic, continue on the departure"
	case IntentIdentified:
		s := cs + ", identified" // 12.4.1.1 e
		if p[ParamLevel] != "" {
			s += ", climb to " + p[ParamLevel] // 12.3.1.2 a
		}
		return s
	case IntentContact:
		station := strings.TrimSpace(p[ParamStation] + " " + p[ParamFreq])
		if p[ParamRush] != "" && p[ParamWhen] != "" {
			return fmt.Sprintf("%s, expedite vacating, %s contact %s", cs, p[ParamWhen], station) // 12.3.4.7 y
		}
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
	if p[ParamQNH] != "" {
		s += ", QNH " + p[ParamQNH] // with the level (CAP 413 3.9)
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
	s := "cleared approach runway " + p[ParamRunway]
	if p[ParamApproach] != "" {
		s = "cleared " + p[ParamApproach] + " approach runway " + p[ParamRunway] // 12.3.3.2 f
	}
	if p[ParamIntercept] != "" { // on vectors: the intercept first (12.4.2.2 g)
		s = "turn " + p[ParamTurn] + " heading " + p[ParamIntercept] + " to intercept, " + s
	}
	if p[ParamQNH] != "" {
		s += ", QNH " + p[ParamQNH] // CAP 413 6.28
	}
	if p[ParamReport] == "established" {
		s += ", report established" // 12.4.2.2 e
	}
	return s
}

// WindSaid is the surface wind as a tower says it, magnetic degrees, ICAO:
// "wind 100 degrees 6 knots", "wind 270 degrees 18 knots gusting 28 knots",
// "wind calm" below a knot (Doc 4444 12.3.1.8 a). A NaN direction is a
// variable wind: "wind variable 2 knots" (#753).
func WindSaid(dirMag, kts, gustKts float64) string {
	return WindSaidAs(PhraseologyICAO, dirMag, kts, gustKts)
}

// WindSaidAs is the surface wind in phraseology ph (#753); a NaN direction
// is a variable wind.
//
// ICAO: "wind 270 degrees 12 knots", "gusting 28 knots" (Doc 4444
// 12.3.1.8 a; CAP 413 "280 degrees 37 knots gusting 50"), "wind calm"
// under a knot; variable "wind variable 2 knots" — the Doc 4444 pattern
// with "variable" for the direction (neither Doc 4444 nor CAP 413 words
// VRB: unverified).
//
// FAA: "wind 270 at 12", "gusts 20" (JO 7110.65 2-4-17; JO 7110.10 TBL
// 12-1-2), "wind calm" under 3 knots (JO 7110.65 2-6-5), "wind variable at
// 4" (JO 7110.10 TBL 12-1-2).
func WindSaidAs(ph Phraseology, dirMag, kts, gustKts float64) string {
	calm := 1.0
	if ph == PhraseologyFAA {
		calm = 3
	}
	if math.Round(kts) < calm {
		return "wind calm"
	}
	if math.IsNaN(dirMag) {
		if ph == PhraseologyFAA {
			return fmt.Sprintf("wind variable at %.0f", kts)
		}
		return fmt.Sprintf("wind variable %.0f knots", kts)
	}
	if ph == PhraseologyFAA {
		dir := int(math.Round(dirMag/10)*10) % 360
		if dir == 0 {
			dir = 360
		}
		s := fmt.Sprintf("wind %03d at %.0f", dir, kts)
		if gustKts >= kts+10 {
			s += fmt.Sprintf(" gusts %.0f", gustKts)
		}
		return s
	}
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

// WeatherReport answers a crew's weather request: the wind (WindSaid) and
// the QNH: "CSA1, wind 240 degrees 8 knots, QNH 1013". FAA: the altimeter.
func WeatherReport(pos Position, cs, wind, qnh, altimeter string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentWeather, Params: map[string]string{ParamWind: wind, ParamQNH: qnh, ParamAltimeter: altimeter}})
}

// ClearedDirectTo clears an aircraft direct to fix, as its crew asked:
// "CSA1, cleared direct to GOLOP".
func ClearedDirectTo(pos Position, cs, fix string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentDirectTo, Params: map[string]string{ParamFix: fix}})
}

// UnableDirect refuses a crew's request for direct, for traffic (#621):
// "CSA1, unable direct due traffic, continue on the departure".
func UnableDirect(pos Position, cs string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentUnableDirect})
}

// Identified answers a departure's check-in: identified (FAA: radar
// contact), climb to level ("" none): "CSA1, identified, climb to flight
// level 240".
func Identified(pos Position, cs, level string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentIdentified, Params: map[string]string{ParamLevel: level}})
}

// ApproachClearance is an approach clearance's content.
type ApproachClearance struct {
	Kind, Runway string // "ILS", "24"
	QNH          string // hPa ("" none)
	// ReportEstablished asks the crew to report established on the
	// localizer before the tower (Doc 4444 12.4.2.2 e).
	ReportEstablished bool
	// Intercept and Turn: on vectors, the heading to intercept the final
	// (HeadingSaid) and the way to turn to it, said first (#661).
	Intercept, Turn string
}

// ClearedApproachTo clears an approach with its QNH, asking for the
// established report: "CSA1, cleared ILS approach runway 24, QNH 1013,
// report established".
func ClearedApproachTo(cs string, a ApproachClearance) Transmission {
	p := map[string]string{ParamApproach: a.Kind, ParamRunway: a.Runway, ParamQNH: a.QNH}
	if a.Intercept != "" {
		p[ParamIntercept], p[ParamTurn] = a.Intercept, a.Turn
	}
	if a.ReportEstablished {
		p[ParamReport] = "established"
	}
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentApproachClearance, Params: p})
}

// Rushed is clearance t expedited (#510): an immediate take-off, "be ready
// for immediate departure", "expedite crossing", "expedite vacating".
// Clearances without an expedited form are returned as they are.
func Rushed(t Transmission) Transmission {
	switch t.Intent {
	case IntentTakeoff, IntentLineUp, IntentCross, IntentContact:
	default:
		return t
	}
	p := map[string]string{ParamRush: "1"}
	for k, v := range t.Params {
		p[k] = v
	}
	t.Params = p
	return Say(t)
}

// FollowTaxi tells an aircraft on the ground to follow another, described
// by its operator and type, "the company" for its own airline's: "CSA1,
// follow the company Airbus A320", "CSA1, follow the Lufthansa Boeing
// 737". Joined to a taxi clearance it is read back with it.
func FollowTaxi(cs, traffic string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentFollowTaxi, Params: map[string]string{ParamGiveWay: traffic}})
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

// ClearedCrossBehind is a conditional crossing behind the next landing
// aircraft (traffic: its type as said): "CSA1, behind the landing A320,
// cross runway 12, behind". The crew crosses once that aircraft has passed.
func ClearedCrossBehind(cs, traffic, runway string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentCross, Params: map[string]string{ParamRunway: runway, ParamBehind: traffic}})
}

// JoinCircuit tells a VFR arrival to join the circuit (Doc 4444 12.3.4.13
// b, e): position as said ("left downwind", "right base"), with the wind
// (WindSaid, "" none), QNH and traffic ("" none).
func JoinCircuit(cs, position, runway, wind, qnh, traffic string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentJoinCircuit,
		Params: map[string]string{ParamCircuit: position, ParamRunway: runway, ParamWind: wind, ParamQNH: qnh, ParamTraffic: traffic}})
}

// StraightIn is "make straight-in approach, runway (n)" (12.3.4.13 c).
func StraightIn(cs, runway, wind, qnh string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentStraightIn,
		Params: map[string]string{ParamRunway: runway, ParamWind: wind, ParamQNH: qnh}})
}

// FollowTraffic is the place in the circuit: "number 2, follow the Airbus
// A320 on final" (12.3.4.14 b); traffic as said, with its position.
func FollowTraffic(cs string, number int, traffic string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentFollow,
		Params: map[string]string{ParamNumber: fmt.Sprint(number), ParamTraffic: traffic}})
}

// Circuit approach instructions (12.3.4.15) and delays (12.3.4.17), as said.
const (
	InstrShortApproach  = "make short approach"
	InstrLongApproach   = "make long approach"
	InstrExtendDownwind = "extend downwind"
	// InstrExtendCallBase: extended with the base turn left to the
	// controller, who calls it (InstrTurnBase).
	InstrExtendCallBase = "extend downwind, I'll call your base"
	InstrTurnBase       = "turn base now"
	InstrReportBase     = "report base"
	InstrReportFinal    = "report final"
	InstrContinue       = "continue approach"
	DelayCircle         = "circle the aerodrome"
	DelayOrbitRight     = "orbit right"
	DelayOrbitLeft      = "orbit left"
	DelayAnotherCircuit = "make another circuit"
)

// CircuitInstruction is one of the Instr* approach instructions.
func CircuitInstruction(cs, instr string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentCircuitInstr, Params: map[string]string{ParamInstr: instr}})
}

// CircuitDelay is one of the Delay* instructions.
func CircuitDelay(cs, instr string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentCircuitDelay, Params: map[string]string{ParamInstr: instr}})
}

// ClearedTouchAndGo is "cleared touch and go" (12.3.4.16 c).
func ClearedTouchAndGo(cs, runway string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentTouchAndGo, Params: map[string]string{ParamRunway: runway}})
}

// ClearedStopAndGo is "cleared stop and go": a touch-and-go that stops on
// the runway before the take-off (ArrivalRequest.StopAndGo). The wording
// follows ClearedTouchAndGo; Doc 4444 12.3.4.16 does not list it.
func ClearedStopAndGo(cs, runway string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentTouchAndGo, Params: map[string]string{ParamRunway: runway, ParamInstr: "stop and go"}})
}

// MakeFullStop is "make full stop" (12.3.4.16 d).
func MakeFullStop(cs string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentFullStop})
}

// ClearedLineUp is "line up and wait" on runway.
func ClearedLineUp(cs, runway string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentLineUp, Params: map[string]string{ParamRunway: runway}})
}

// ClearedLineUpBehind is a conditional line-up behind the next landing
// aircraft (traffic: its type as said, "A320"): "CSA1, behind the landing
// A320, line up and wait runway 24, behind". The crew lines up once that
// aircraft has passed.
func ClearedLineUpBehind(cs, traffic, runway string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentLineUp, Params: map[string]string{ParamRunway: runway, ParamBehind: traffic}})
}

// AtEntry is a line-up or take-off clearance t from the intersection entry
// ("" the full length): "CSA1, runway 24 at B, cleared for take-off"
// (JO 7110.65 3-9-4, 3-9-10: the intersection said with the runway).
func AtEntry(t Transmission, entry string) Transmission {
	if entry == "" {
		return t
	}
	p := map[string]string{}
	for k, v := range t.Params {
		p[k] = v
	}
	p[ParamEntry] = entry
	t.Params, t.Text = p, ""
	return Say(t)
}

// runwayAt is "runway 24", or "runway 24 at B" from an intersection.
func runwayAt(p map[string]string) string {
	if p[ParamEntry] != "" {
		return "runway " + p[ParamRunway] + " at " + p[ParamEntry]
	}
	return "runway " + p[ParamRunway]
}

// ClearedTakeoff clears the take-off from runway, with the wind (WindSaid,
// "" none). Given at the holding point it means line up and take off.
func ClearedTakeoff(cs, runway, wind string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentTakeoff, Params: map[string]string{ParamRunway: runway, ParamWind: wind}})
}

// ClearedTakeoffNoDelay is ClearedTakeoff with the next arrival finalNM
// out (RunwayClearances.NoDelay): "CSA1, runway 24, cleared for take-off,
// wind 360 degrees 2 knots, no delay, traffic on 5 mile final".
func ClearedTakeoffNoDelay(cs, runway, wind string, finalNM float64) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentTakeoff, Params: map[string]string{
		ParamRunway: runway, ParamWind: wind, ParamNoDelay: fmt.Sprint(int(math.Max(1, math.Round(finalNM))))}})
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

// DepartureOrder is the tower's answer to a departure checking in while
// taxiing to runway, ahead the departures to go before it: "CSA1, report
// ready for departure", or "CSA1, hold short of runway 24, 2 aircraft to
// depart before you".
func DepartureOrder(cs, runway string, ahead int) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentDepartureOrder,
		Params: map[string]string{ParamRunway: runway, ParamNumber: fmt.Sprint(ahead)}})
}

// ContinueTaxi resumes the taxi of an aircraft told to hold position, on
// the route it was cleared: "CSA1, continue taxi".
func ContinueTaxi(cs string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentContinueTaxi})
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

// SequencedDelaySaidFrom: "expect N minutes delay" is said only from this
// many minutes; five or less is not worth telling, speed and vectors take it.
const SequencedDelaySaidFrom = 6

// Sequenced tells an arrival its number and how it is spaced: the speed it
// is to fly (a.SpeedKts), and the delay to expect, in whole minutes, when
// more than speed (path stretching, a hold) absorbs it and it is longer
// than five minutes (SequencedDelaySaidFrom).
func Sequenced(cs string, number int, delay time.Duration, a Absorption) Transmission {
	p := map[string]string{ParamLose: a.String()}
	if number > 0 {
		p[ParamNumber] = fmt.Sprint(number) // 0: told already, not said again
	}
	if a.Orbit != "" {
		p[ParamOrbit] = a.Orbit
	}
	if a.Downwind {
		p[ParamExtendDownwind] = "1"
	}
	if a.SpeedKts > 0 {
		p[ParamSpeed] = fmt.Sprintf("%.0f", a.SpeedKts)
	}
	// The speed is the instruction; the delay is told only when it is long.
	if min := int(math.Round(delay.Minutes())); min >= 1 && (a.ExtraNM > 0 || a.Left > 0) && min >= SequencedDelaySaidFrom {
		p[ParamDelay] = fmt.Sprint(min)
	}
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentSequence, Params: p})
}

// SequencedFinalSpeed has an arrival on the final slow to its final
// approach speed for spacing: "CSA1, number 2, for spacing reduce to final
// approach speed". From pos: the position working it.
func SequencedFinalSpeed(pos Position, cs string, number int) Transmission {
	p := map[string]string{ParamFinalSpeed: "1"}
	if number > 0 {
		p[ParamNumber] = fmt.Sprint(number) // 0: told already
	}
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentSequence, Params: p})
}

// SpeedAssigned gives an arrival flying nowKts a speed of kts by
// approach (#443): "reduce" or "increase speed to"; kts 0 resumes normal
// speed.
func SpeedAssigned(cs string, kts, nowKts float64) Transmission {
	p := map[string]string{}
	if kts <= 0 {
		p[ParamResume] = "true"
	} else {
		p[ParamSpeed] = fmt.Sprintf("%.0f", kts)
		if kts < nowKts {
			p[ParamSlower] = "true"
		}
	}
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentSpeed, Params: p})
}

// DirectToFinal sends an arrival direct to the final.
func DirectToFinal(cs string, number int) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentDirect, Params: numberParam(map[string]string{}, number)})
}

// HoldAt holds an arrival at fix with entry at altFt, expecting further
// clearance at efc (said in UTC, #99: it was local time). ta, when given,
// is the airport's transition altitude: altitude or flight level by it
// (#101: "FL070" and then "7000 feet" at LKPR); none: 10000 ft.
func HoldAt(cs, fix string, entry HoldEntry, altFt float64, efc time.Time, ta ...float64) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHold, Params: map[string]string{
		ParamFix: fix, ParamHoldIn: entry.String(), ParamAltitude: fmt.Sprintf("%.0f", altFt), ParamLevel: levelSaidTA(altFt, ta), ParamExpect: efc.UTC().Format("1504")}})
}

// levelSaidTA is LevelSaidAbove the transition altitude in ta, else
// LevelSaid.
func levelSaidTA(altFt float64, ta []float64) string {
	if len(ta) > 0 && ta[0] > 0 {
		return LevelSaidAbove(altFt, ta[0])
	}
	return LevelSaid(altFt)
}

// LeaveHoldAt releases an arrival from the hold at fix as number.
func LeaveHoldAt(cs, fix string, number int) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentLeaveHold, Params: numberParam(map[string]string{ParamFix: fix}, number)})
}

// HoldDescend steps a holding arrival down to altFt (ta: as HoldAt).
func HoldDescend(cs string, altFt float64, ta ...float64) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHoldLevel, Params: map[string]string{ParamAltitude: fmt.Sprintf("%.0f", altFt), ParamLevel: levelSaidTA(altFt, ta)}})
}

// SaidWhere is what a resolution is said by: the transition altitude for
// levels (0: 10000 ft) and the magnetic variation for headings.
type SaidWhere struct {
	TAFt, MagVar float64
}

// Resolved is a conflict resolution for an aircraft now at altFt, heading
// hdg and kts, said by pos (center en route, approach near the airport).
// where, when given, says levels by the transition altitude and headings
// magnetic (#100: true, and "000" for north; #101).
func Resolved(pos Position, r Resolution, altFt, hdg, kts float64, where ...SaidWhere) Transmission {
	var at SaidWhere
	if len(where) > 0 {
		at = where[0]
	}
	ta := []float64{at.TAFt}
	t := Transmission{Position: pos, Callsign: r.Callsign, Params: map[string]string{ParamTraffic: r.Why}}
	switch r.Kind {
	case ResolveSpeed:
		t.Intent = IntentSpeed
		t.Params[ParamSpeed] = fmt.Sprintf("%.0f", r.Kts)
		switch {
		case r.Said.Mach > 0:
			t.Params[ParamSpeed] = fmt.Sprintf("Mach %.2f", r.Said.Mach) // Doc 4444 4.6.1.6
		case r.Said.IASKts > 0:
			t.Params[ParamSpeed] = fmt.Sprintf("%.0f", r.Said.IASKts)
		}
		if r.Kts < kts {
			t.Params[ParamSlower] = "true"
		}
	case ResolveLevel:
		t.Intent = IntentLevel
		t.Params[ParamLevel] = levelSaidTA(r.AltFt, ta)
		t.Params[ParamClimb] = "climb"
		if r.AltFt < altFt {
			t.Params[ParamClimb] = "descend"
		}
		if r.Stop && !r.FromLevel { // level now: "climb to" / "descend to" it, as set above
			t.Params[ParamClimb] = "stop climb"
			if r.AltFt < altFt {
				t.Params[ParamClimb] = "stop" // stop descent
			}
		}
		if r.Maintain {
			t.Params[ParamClimb] = "maintain" // 12.3.2.3 a
		}
	case ResolveDirect:
		t.Intent = IntentDirectTo
		t.Params[ParamFix] = r.Fix
	case ResolveCross:
		t.Intent = IntentCrossLevel
		t.Params[ParamFix], t.Params[ParamLevel], t.Params[ParamClimb] = r.Fix, levelSaidTA(r.AltFt, ta), "above"
		if r.AltFt < altFt {
			t.Params[ParamClimb] = "below"
		}
	default:
		t.Intent = IntentHeading
		t.Params[ParamHeading] = HeadingSaid(r.HeadingDeg, at.MagVar)
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
	// ControllerOf is who works freq at airport (#722, "" none): filled
	// into transmissions without one. One controller says one thing at a
	// time, whichever of its frequencies.
	ControllerOf func(airport, freq string) string
	// TempoOf is how fast transmissions on freq at airport are said now
	// (1 normal, nil always 1): filled into transmissions without one,
	// readbacks too, so a busy frequency speeds up as a controller does.
	TempoOf func(airport, freq string) float64
	// ReadBack: our pilots read back every clearance to them (#417), on
	// the same frequency, after it.
	ReadBack bool
	// SaidCallsign writes a call sign as said ("DLH1675" → "Lufthansa
	// 1675", ScheduleConfig.SaidCallsign): the text of each transmission
	// uses it, so text and voice are the same (#462). Callsign keeps the
	// ICAO form.
	SaidCallsign func(cs string) string
	// Phraseology is the wording at an airport; nil: PhraseologyFor (FAA in
	// the United States, ICAO elsewhere, #463).
	Phraseology func(airport string) Phraseology
	// Variety varies what crews and controllers say and when (#721); nil:
	// off. Radio.SetVariety changes it.
	Variety *Variety
}

// Radio carries the transmissions of our controllers (and, with #417,
// their pilots): each is stamped, kept and handed to OnTransmission.
type Radio struct {
	opts RadioOptions
	mu   sync.Mutex
	kept []Transmission
	busy map[string]time.Time // by frequency: said until
	rng  *rand.Rand           // the variety's; nil: off
	// firstCalls are crews' first calls on a frequency not yet answered:
	// whether the crew greeted (#721).
	firstCalls map[string]bool
}

// NewRadio creates a radio.
func NewRadio(opts RadioOptions) *Radio {
	if opts.Keep <= 0 {
		opts.Keep = 200
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	r := &Radio{opts: opts, busy: map[string]time.Time{}, firstCalls: map[string]bool{}}
	r.SetVariety(opts.Variety)
	return r
}

// Transmit sends t: stamped (when not already) at airport, on its
// position's frequency, kept, and handed on. One transmission at a time on
// a frequency: while one is said, the next is stamped for when it ends
// (SpeakingTime and a second's pause), so a voice plays them in turn. It
// returns t as sent: stamped, on its frequency.
func (r *Radio) Transmit(airport string, t Transmission) Transmission {
	return r.transmit(airport, t, r.opts.ReadBack)
}

// transmit sends t, read back by its crew when readBack.
func (r *Radio) transmit(airport string, t Transmission, readBack bool) Transmission {
	if t.At.IsZero() {
		t.At = r.opts.Now()
	}
	if t.Airport == "" {
		t.Airport = airport
	}
	if t.Text == "" {
		t = Say(t)
	}
	// The airport's phraseology: a controller's phrase built by Say is said
	// again the FAA's way at a US airport (custom texts stay as they are).
	if !t.Pilot && t.Phraseology == "" {
		ph := PhraseologyFor(t.Airport)
		if r.opts.Phraseology != nil {
			ph = r.opts.Phraseology(t.Airport)
		}
		if ph == PhraseologyFAA && Say(t).Text == t.Text {
			t.Phraseology = PhraseologyFAA
			t = Say(t)
		}
	}
	if t.Frequency == "" && r.opts.FrequencyOf != nil {
		t.Frequency = r.opts.FrequencyOf(t.Airport, t.Position)
	}
	if t.Controller == "" && t.Frequency != "" && r.opts.ControllerOf != nil {
		t.Controller = r.opts.ControllerOf(t.Airport, t.Frequency)
	}
	if t.Tempo == 0 && t.Frequency != "" && r.opts.TempoOf != nil {
		t.Tempo = r.opts.TempoOf(t.Airport, t.Frequency)
	}
	plain := t // as worded, for the crew's readback and a correction
	r.mu.Lock()
	busy := false
	if t.Frequency != "" {
		if until := r.busy[t.Airport+" "+t.Frequency]; t.At.Before(until) {
			t.At, busy = until, true
		}
	}
	mouth := ""
	if !t.Pilot && t.Controller != "" {
		mouth = "controller " + t.Airport + " " + t.Controller
		if until := r.busy[mouth]; t.At.Before(until) {
			t.At = until
		}
	}
	t = r.varied(t, busy)
	if r.opts.SaidCallsign != nil && t.Callsign != "" {
		if said := r.opts.SaidCallsign(t.Callsign); said != t.Callsign {
			t.Text = strings.ReplaceAll(t.Text, t.Callsign, said)
		}
	}
	if t.Frequency != "" {
		r.busy[t.Airport+" "+t.Frequency] = t.At.Add(t.SpeakingTime() + time.Second)
	}
	if mouth != "" {
		r.busy[mouth] = t.At.Add(t.SpeakingTime())
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
	if readBack && !t.Pilot && t.Callsign != "" {
		plain.At = t.At
		r.readBack(t.Airport, plain, busy)
	}
	return t
}

// Occupy marks freq at airport busy until until, for something said on it
// outside this radio (a host's own ATC, #710): what this radio says next
// waits for it. Nothing is kept or heard.
func (r *Radio) Occupy(airport, freq string, until time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := airport + " " + freq
	if until.After(r.busy[key]) {
		r.busy[key] = until
	}
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

// ContinueLevel lets an aircraft stopped for traffic climb or descend on
// to altFt: "RYR1527, climb to flight level 240" (Doc 4444 12.3.1.2 a).
func ContinueLevel(pos Position, cs string, altFt float64, climb bool) Transmission {
	return ContinueLevelAbove(pos, cs, altFt, climb, 10000)
}

// ContinueLevelAbove is ContinueLevel with the airport's transition
// altitude (airport.Limits.TransitionAltitudeFt): "descend to flight level
// 100" at LKPR (TA 5000), not "10000 feet" (#686).
func ContinueLevelAbove(pos Position, cs string, altFt float64, climb bool, transitionFt float64) Transmission {
	verb := "continue descent"
	if climb {
		verb = "continue climb"
	}
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentLevel,
		Params: map[string]string{ParamLevel: LevelSaidAbove(altFt, transitionFt), ParamClimb: verb}})
}

// Climb and Descend clear an aircraft to a level, said against the
// airport's transition altitude: "CSA1, descend to flight level 100",
// "CSA1, climb to 5000 feet" (Doc 4444 12.3.1.2 a; #686).
func Climb(pos Position, cs string, altFt, transitionFt float64) Transmission {
	return levelTo(pos, cs, altFt, transitionFt, "climb")
}

func Descend(pos Position, cs string, altFt, transitionFt float64) Transmission {
	return levelTo(pos, cs, altFt, transitionFt, "descend")
}

func levelTo(pos Position, cs string, altFt, transitionFt float64, verb string) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentLevel,
		Params: map[string]string{ParamLevel: LevelSaidAbove(altFt, transitionFt), ParamClimb: verb}})
}

// levelVerb is the verb of a resumed climb or descent: "continue descent"
// is said "descend", not the noun (#686).
func levelVerb(climb string) string {
	if climb == "continue descent" {
		return "descend"
	}
	return "climb"
}

// StopDescent has a descending arrival level off at altFt for traffic
// below it: "AUA529, stop descent at 7000 feet, due traffic".
func StopDescent(pos Position, cs string, altFt float64, traffic string, ta ...float64) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentLevel,
		Params: map[string]string{ParamLevel: levelSaidTA(altFt, ta), ParamClimb: "stop", ParamTraffic: traffic}})
}

// numberParam adds the number in traffic to p, unless it is 0 (told
// already: not said again).
func numberParam(p map[string]string, number int) map[string]string {
	if number > 0 {
		p[ParamNumber] = fmt.Sprint(number)
	}
	return p
}

// ParamCancel on a descent via the STAR: "level" or "speed" restrictions
// cancelled (#754).
const ParamCancel = "cancel"

// DescendVia clears an arrival on its STAR down to a level, keeping the
// STAR's published level and speed restrictions (#754): ICAO "CSA1,
// descend via STAR to flight level 100" (Doc 4444 6.5.2.4.1 a); FAA "CSA1,
// descend via the VOZ 5A arrival" (JO 7110.65 4-5-7 h, the published
// altitudes, no level). star is the STAR as said (SaidProcedure).
func DescendVia(pos Position, cs, star string, levelFt, transitionFt float64) Transmission {
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentDescendVia,
		Params: map[string]string{ParamSTAR: star, ParamLevel: LevelSaidAbove(levelFt, transitionFt)}})
}

// WithCancelled is a descent via the STAR with its level or speed
// restrictions cancelled ("level", "speed"): "…, cancel level
// restrictions" (Doc 4444 6.5.2.4.1 b, d). The FAA wording has none.
func WithCancelled(t Transmission, what string) Transmission {
	t.Params = cloneParams(t.Params, ParamCancel, what)
	t.Text = ""
	return Say(t)
}

func descendVia(p map[string]string) string {
	s := "descend via STAR to " + p[ParamLevel]
	if c := p[ParamCancel]; c != "" {
		s += ", cancel " + c + " restrictions"
	}
	return s
}

// CrossAt has an aircraft cross fix at or above (above) or at or below a
// level: "CSA1, cross VOZ at or above flight level 120" (Doc 4444
// 12.3.2.4 a; JO 7110.65 4-5-7 "Cross Gramm at or above flight level one
// eight zero"), #754.
func CrossAt(pos Position, cs, fix string, altFt, transitionFt float64, above bool) Transmission {
	way := "below"
	if above {
		way = "above"
	}
	return Say(Transmission{Position: pos, Callsign: cs, Intent: IntentCrossLevel,
		Params: map[string]string{ParamFix: fix, ParamLevel: LevelSaidAbove(altFt, transitionFt), ParamClimb: way}})
}

// spokenSpeed is ParamSpeed as said: "Mach 0.78" as it is, a bare number
// in knots ("250 knots").
func spokenSpeed(s string) string {
	if strings.HasPrefix(s, "Mach") {
		return s
	}
	return s + " knots"
}

// WithHoldShort is taxi clearance t with the runways the route crosses to
// hold short of: "CSA1, taxi to and hold short of runway 24 via A, B, hold
// short of runway 12" (Doc 4444 12.3.4.7 e; JO 7110.65 3-7-2), read back
// with them (4.5.7.5.1 b). None: t as it is.
func WithHoldShort(t Transmission, runways ...string) Transmission {
	if len(runways) == 0 {
		return t
	}
	t.Params = cloneParams(t.Params, ParamHoldShort, strings.Join(runways, ", "))
	t.Text = ""
	return Say(t)
}

// holdShortSaid is the hold short part of a taxi clearance (WithHoldShort):
// ", hold short of runway 12", ", hold short of runways 12 and 31".
func holdShortSaid(p map[string]string) string {
	r := p[ParamHoldShort]
	if r == "" {
		return ""
	}
	if i := strings.LastIndex(r, ", "); i >= 0 {
		return ", hold short of runways " + r[:i] + " and " + r[i+2:]
	}
	return ", hold short of runway " + r
}

// SpeakingTime is how long t takes to say: SpeakingTime of its text at its
// Tempo.
func (t Transmission) SpeakingTime() time.Duration {
	d := SpeakingTime(t.Text)
	if t.Tempo > 0 {
		d = time.Duration(float64(d) / t.Tempo)
	}
	return d
}

// ClearedLineUpBehindDeparting is a conditional line-up behind the
// departure ahead on its take-off roll (traffic: its type as said): "CSA1,
// behind the departing A320, line up and wait runway 24, behind" — given
// when the runway is busy, so the next is lined up as the first rolls.
func ClearedLineUpBehindDeparting(cs, traffic, runway string) Transmission {
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentLineUp, Params: map[string]string{ParamRunway: runway, ParamBehind: traffic, ParamBehindHow: "departing"}})
}

// behindHow is what the traffic of a conditional clearance does: "landing"
// unless said otherwise (ParamBehindHow).
func behindHow(p map[string]string) string {
	if h := p[ParamBehindHow]; h != "" {
		return h
	}
	return "landing"
}

// ParamGoAroundHeading: a go-around's heading as said: "runway heading" or
// a three-digit heading.
const ParamGoAroundHeading = "goAroundHeading"

// goAroundInstr is a go-around's climb and heading as said after it (Doc
// 4444 12.3.4.18 with the missed approach instructions): ", climb to 4200
// feet, fly runway heading"; "" without them.
func goAroundInstr(p map[string]string) string {
	s := ""
	if p[ParamLevel] != "" {
		s += ", climb to " + p[ParamLevel]
	}
	if h := p[ParamGoAroundHeading]; h != "" {
		if h != "runway heading" {
			h = "heading " + h
		}
		s += ", fly " + h
	}
	return s
}

// GoAroundWith is GoAround with the climb and heading the tower gives:
// "CSA1, go around, I say again, go around, traffic on the runway, climb
// to 4200 feet, fly runway heading". level is as said ("4200 feet"),
// heading "runway heading" or three digits; either may be "".
func GoAroundWith(cs, reason, level, heading string) Transmission {
	p := map[string]string{}
	if reason != "" {
		p[ParamReason] = reason
	}
	if level != "" {
		p[ParamLevel] = level
	}
	if heading != "" {
		p[ParamGoAroundHeading] = heading
	}
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentGoAround, Params: p})
}

// GoAroundAcknowledged is the tower's answer to a crew going around on its
// own: "CSA1, roger, climb to 4200 feet, fly runway heading" (#621).
func GoAroundAcknowledged(cs, level, heading string) Transmission {
	p := map[string]string{}
	if level != "" {
		p[ParamLevel] = level
	}
	if heading != "" {
		p[ParamGoAroundHeading] = heading
	}
	return Transmission{Position: PosTower, Callsign: cs, Intent: IntentAcknowledge, Params: p, Text: cs + ", roger" + goAroundInstr(p)}
}

// RadarContactAfterGoAround is approach's answer to a go-around's check-in:
// "CSA1, radar contact, maintain 4200 feet, expect ILS approach runway 24".
func RadarContactAfterGoAround(cs, level, approach, runway string) Transmission {
	text := cs + ", radar contact"
	if level != "" {
		text += ", maintain " + level
	}
	if approach != "" {
		text += ", expect " + approach + " approach runway " + runway
	}
	return Transmission{Position: PosApproach, Callsign: cs, Intent: IntentAcknowledge, Text: text}
}

// ParamExtendDownwind: a sequence call extending the STAR's downwind
// ("extend downwind, expect vectors"; Absorption.Downwind).
const ParamExtendDownwind = "extendDownwind"

// AirportClosed tells an arrival every runway at icao is closed: holding
// to follow.
func AirportClosed(cs, icao string) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentAirportClosed, Params: map[string]string{ParamAirport: icao}})
}

// Divert clears an arrival to alternate, icao still closed.
func Divert(cs, icao, alternate string) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentDivert, Params: map[string]string{ParamAirport: icao, ParamAlternate: alternate}})
}

// Backtracked is t (a line-up) with the backtrack along the runway first
// (no entry at its take-off threshold): "CSA1, enter runway 08 and
// backtrack, line up and wait".
func Backtracked(t Transmission, on bool) Transmission {
	if !on {
		return t
	}
	p := map[string]string{}
	for k, v := range t.Params {
		p[k] = v
	}
	p[ParamBacktrack] = "1"
	t.Params, t.Text = p, ""
	return Say(t)
}
