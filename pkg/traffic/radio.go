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
	IntentPushback           Intent = "pushback"            // push back and start-up approved
	IntentTaxi               Intent = "taxi"                // taxi to the holding point or the stand
	IntentTaxiLimit          Intent = "taxi_limit"          // taxi and hold short (a limit on the route)
	IntentCross              Intent = "cross"               // cross a runway
	IntentLineUp             Intent = "line_up"             // line up and wait
	IntentTakeoff            Intent = "takeoff"             // cleared for take-off (ParamLineUp: line up and go)
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
	ParamEntry    = "entry"    // the holding point of an intersection departure ("B")
	ParamTaxiways = "taxiways" // as said: "B2, H, A"
	ParamStand    = "stand"
	ParamLimit    = "limit" // a taxiway to hold short of; "" a marked point
	ParamSID      = "sid"
	ParamSTAR     = "star"
	ParamApproach = "approach" // the approach expected ("ILS")
	ParamLineUp   = "line_up"  // "true": line up and take off in one
	ParamReason   = "reason"
	ParamNumber   = "number" // in the landing sequence
	ParamDelay    = "delay"
	ParamLose     = "lose" // how the delay is lost: "210 kt, +3.2 NM"
	ParamFix      = "fix"
	ParamHoldIn   = "entry_type" // hold entry: direct, teardrop, parallel
	ParamAltitude = "altitude"   // feet
	ParamExpect   = "expect"     // expect further clearance, HH:MM
	ParamSpeed    = "speed"      // knots
	ParamLevel    = "level"      // "flight level 210" or "altitude 9000 feet"
	ParamHeading  = "heading"    // degrees, three digits
	ParamTurn     = "turn"       // left, right
	ParamClimb    = "climb"      // climb, descend
	ParamSlower   = "slower"     // "true": reduce, else increase
	ParamTraffic  = "traffic"    // why a resolution: "traffic DLH2, 0.8 NM in 2m40s"
	ParamPosition = "position"   // a handoff's next position
	ParamStation  = "station"    // … as said: "Praha Tower"
	ParamFreq     = "frequency"  // … its frequency: "118.105"
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

// phrase is the text of a controller's transmission.
func phrase(cs string, in Intent, p map[string]string) string {
	via := ""
	if p[ParamTaxiways] != "" {
		via = " via " + p[ParamTaxiways]
	}
	reason := ""
	if p[ParamReason] != "" {
		reason = " — " + p[ParamReason]
	}
	switch in {
	case IntentDepartureClearance:
		return fmt.Sprintf("%s, cleared %s departure, runway %s", cs, p[ParamSID], p[ParamRunway])
	case IntentArrivalClearance:
		return fmt.Sprintf("%s, cleared %s arrival, expect %s approach runway %s", cs, p[ParamSTAR], p[ParamApproach], p[ParamRunway])
	case IntentPushback:
		return cs + ", push back and start-up approved"
	case IntentTaxi:
		if p[ParamStand] != "" {
			return fmt.Sprintf("%s, taxi to stand %s%s", cs, p[ParamStand], via)
		}
		entry := ""
		if p[ParamEntry] != "" {
			entry = " " + p[ParamEntry]
		}
		return fmt.Sprintf("%s, taxi to holding point%s runway %s%s", cs, entry, p[ParamRunway], via)
	case IntentTaxiLimit:
		if p[ParamLimit] == "" {
			return fmt.Sprintf("%s, taxi%s, hold position at the marked point", cs, via)
		}
		return fmt.Sprintf("%s, taxi%s, hold short of %s", cs, via, p[ParamLimit])
	case IntentCross:
		return fmt.Sprintf("%s, cross runway %s", cs, p[ParamRunway])
	case IntentLineUp:
		return fmt.Sprintf("%s, runway %s, line up and wait", cs, p[ParamRunway])
	case IntentTakeoff:
		if p[ParamLineUp] == "true" {
			return fmt.Sprintf("%s, runway %s, line up, cleared for take-off", cs, p[ParamRunway])
		}
		return fmt.Sprintf("%s, runway %s, cleared for take-off", cs, p[ParamRunway])
	case IntentHoldPosition:
		return cs + ", hold position"
	case IntentStop:
		return cs + ", stop immediately, I say again, stop immediately"
	case IntentCancelTakeoff:
		return cs + ", hold position, cancel take-off clearance, I say again, cancel take-off clearance"
	case IntentGoAround:
		return cs + ", go around, I say again, go around" + reason
	case IntentSequence:
		if p[ParamDelay] == "" {
			return fmt.Sprintf("%s, number %s, lose a minute: %s", cs, p[ParamNumber], p[ParamLose])
		}
		return fmt.Sprintf("%s, number %s, delay %s: %s", cs, p[ParamNumber], p[ParamDelay], p[ParamLose])
	case IntentDirect:
		return fmt.Sprintf("%s, proceed direct to the final, number %s", cs, p[ParamNumber])
	case IntentHold:
		return fmt.Sprintf("%s, hold at %s, %s entry, maintain %s ft, expect further clearance %s", cs, p[ParamFix], p[ParamHoldIn], p[ParamAltitude], p[ParamExpect])
	case IntentLeaveHold:
		return fmt.Sprintf("%s, leave the hold at %s, number %s, continue the arrival", cs, p[ParamFix], p[ParamNumber])
	case IntentHoldLevel:
		return fmt.Sprintf("%s, descend %s ft, hold as published", cs, p[ParamAltitude])
	case IntentSpeed:
		verb := "increase"
		if p[ParamSlower] == "true" {
			verb = "reduce"
		}
		return fmt.Sprintf("%s, %s speed %s knots, %s", cs, verb, p[ParamSpeed], p[ParamTraffic])
	case IntentLevel:
		return fmt.Sprintf("%s, %s %s, %s", cs, p[ParamClimb], p[ParamLevel], p[ParamTraffic])
	case IntentHeading:
		return fmt.Sprintf("%s, turn %s heading %s, %s", cs, p[ParamTurn], p[ParamHeading], p[ParamTraffic])
	case IntentContact:
		if p[ParamFreq] == "" {
			return fmt.Sprintf("%s, contact %s", cs, p[ParamStation])
		}
		return fmt.Sprintf("%s, contact %s %s", cs, p[ParamStation], p[ParamFreq])
	}
	return cs + ", " + string(in)
}

// LevelSaid is a level as ATC says it: "flight level 210" at 10000 ft and
// above, "altitude 9000 feet" below.
func LevelSaid(altFt float64) string {
	if altFt >= 10000 {
		return fmt.Sprintf("flight level %03d", int(math.Round(altFt/100)))
	}
	return fmt.Sprintf("altitude %.0f feet", altFt)
}

// Transmission builders: the controller's position, intent and parameters
// of each clearance, with its text (Say).

// ClearedDeparture clears a departure's SID from runway.
func ClearedDeparture(cs, sid, runway string) Transmission {
	return Say(Transmission{Position: PosDelivery, Callsign: cs, Intent: IntentDepartureClearance, Params: map[string]string{ParamSID: sid, ParamRunway: runway}})
}

// ClearedArrival clears an arrival's STAR, expecting approach to runway.
func ClearedArrival(cs, star, approach, runway string) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentArrivalClearance, Params: map[string]string{ParamSTAR: star, ParamApproach: approach, ParamRunway: runway}})
}

// ClearedPushback approves the pushback and start-up.
func ClearedPushback(cs string) Transmission {
	return Say(Transmission{Position: PosGround, Callsign: cs, Intent: IntentPushback})
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

// ClearedTakeoff clears the take-off from runway; lineUp: line up and go
// in one.
func ClearedTakeoff(cs, runway string, lineUp bool) Transmission {
	p := map[string]string{ParamRunway: runway}
	if lineUp {
		p[ParamLineUp] = "true"
	}
	return Say(Transmission{Position: PosTower, Callsign: cs, Intent: IntentTakeoff, Params: p})
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

// Sequenced tells an arrival its number and how it loses its delay (delay
// 0: a minute asked by hand).
func Sequenced(cs string, number int, delay time.Duration, a Absorption) Transmission {
	p := map[string]string{ParamNumber: fmt.Sprint(number), ParamLose: a.String()}
	if delay > 0 {
		p[ParamDelay] = delay.Round(time.Second).String()
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
		ParamFix: fix, ParamHoldIn: entry.String(), ParamAltitude: fmt.Sprintf("%.0f", altFt), ParamExpect: efc.Format("15:04")}})
}

// LeaveHoldAt releases an arrival from the hold at fix as number.
func LeaveHoldAt(cs, fix string, number int) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentLeaveHold, Params: map[string]string{ParamFix: fix, ParamNumber: fmt.Sprint(number)}})
}

// HoldDescend steps a holding arrival down to altFt.
func HoldDescend(cs string, altFt float64) Transmission {
	return Say(Transmission{Position: PosApproach, Callsign: cs, Intent: IntentHoldLevel, Params: map[string]string{ParamAltitude: fmt.Sprintf("%.0f", altFt)}})
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
