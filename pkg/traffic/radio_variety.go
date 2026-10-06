package traffic

import (
	"hash/fnv"
	"maps"
	"math/rand/v2"
	"strings"
	"time"
)

// Variety makes the radio a little less predictable, as a real frequency
// is (#721): crews answer after their own pause, word the pleasantries
// their own way, now and then ask for a clearance again or read it back
// wrong (the controller corrects it). Only what is said and when varies:
// every transmission keeps its intent and parameters, so readback checks
// and the API stay exact, and no clearance changes. It is seeded: the same
// seed and the same traffic say the same.
type Variety struct {
	// Seed picks the crews' styles and the variations.
	Seed uint64 `json:"seed"`
	// SayAgain is the share of clearances a crew asks to be said again
	// (default 0.02; below 0 none).
	SayAgain float64 `json:"sayAgain,omitempty"`
	// ReadbackError is the share of clearances read back with an error the
	// controller corrects (default 0.01; below 0 none).
	ReadbackError float64 `json:"readbackError,omitempty"`
	// MissedCall is the share of handoffs a crew does not answer: the
	// controller calls again after a pause (default 0.03; below 0 none).
	MissedCall float64 `json:"missedCall,omitempty"`
}

// IntentPilotSayAgain is a crew asking the controller to say a clearance
// again: "Say again, CSA1" (#721).
const IntentPilotSayAgain Intent = "pilot_say_again"

// ParamRepeat on a controller's transmission: "1", said again after a
// crew's "say again" (#721).
const ParamRepeat = "repeat"

// CrewStyle is how a crew talks on the radio (#721), the same for a call
// sign through its flight: Quick or slow to answer, Chatty with "good day"
// and "bye" or terse.
type CrewStyle struct {
	Quick  bool `json:"quick"`
	Slow   bool `json:"slow"`
	Chatty bool `json:"chatty"`
}

// StyleOf is the crew style of call sign cs under v.
func (v Variety) StyleOf(cs string) CrewStyle {
	h := fnv.New64a()
	h.Write([]byte(cs))
	n := h.Sum64() ^ v.Seed
	n ^= n >> 29
	n *= 0xbf58476d1ce4e5b9
	n ^= n >> 32
	return CrewStyle{Quick: n%4 == 0, Slow: n%4 == 3, Chatty: (n>>8)%3 != 0}
}

// pause is the extra time a crew of style s takes before it answers, on
// top of the radio's breath: about 0.5–3 s in all.
func (s CrewStyle) pause(rng *rand.Rand) time.Duration {
	lo, hi := 0.0, 1.2 // seconds
	switch {
	case s.Quick:
		lo, hi = 0, 0.5
	case s.Slow:
		lo, hi = 0.8, 2.0
	}
	return time.Duration((lo + rng.Float64()*(hi-lo)) * float64(time.Second))
}

func (v Variety) rate(r, def float64) float64 {
	if r == 0 {
		return def
	}
	return max(r, 0)
}

// varied is a controller's t worded with a pleasantry when the frequency
// is quiet (busy: the frequency had to wait): "…, good day" on a handoff.
func (r *Radio) varied(t Transmission, busy bool) Transmission {
	if r.rng == nil {
		return t
	}
	if t.Pilot {
		return r.greeted(t, busy)
	}
	t = r.greetedBack(t, busy)
	if busy || t.Intent != IntentContact {
		return t
	}
	if r.rng.Float64() < 0.4 {
		t.Text += []string{", good day", ", good day", ", bye"}[r.rng.IntN(3)]
	}
	return t
}

// greetingCalls are a crew's first calls on a frequency: "Ruzyne Radar,
// good morning, CSA1, …".
var greetingCalls = map[Intent]bool{
	IntentCheckIn: true, IntentRequestClearance: true, IntentRequestStartUp: true,
	IntentRequestPushback: true, IntentVFRForLanding: true,
}

// greeted is a crew's first call t with a greeting after the station
// (#721): most crews greet, chatty ones nearly always, fewer on a busy
// frequency; the words vary, by the time of day (t.At, the traffic's
// clock) or a plain "hello" / "good day".
func (r *Radio) greeted(t Transmission, busy bool) Transmission {
	station := t.Params[ParamStation]
	if !greetingCalls[t.Intent] || station == "" || !strings.HasPrefix(t.Text, station+", ") {
		return t
	}
	key := t.Airport + " " + t.Frequency + " " + t.Callsign
	r.firstCalls[key] = false // answered next, perhaps greeted back
	share := 0.6
	if r.opts.Variety.StyleOf(t.Callsign).Chatty {
		share = 0.9
	}
	if busy {
		share /= 2
	}
	if r.rng.Float64() >= share {
		return t
	}
	r.firstCalls[key] = true
	words := r.greeting(t.At, true)
	// Where: after the station most, before it, or after the call sign.
	rest := strings.TrimPrefix(t.Text, station+", ")
	switch n := r.rng.IntN(10); {
	case n < 6:
		t.Text = station + ", " + words + ", " + rest
	case n < 8:
		t.Text = capital(words) + ", " + station + ", " + rest
	default:
		cs, after, more := strings.Cut(rest, ", ")
		if !more {
			t.Text = station + ", " + words + ", " + rest
			break
		}
		t.Text = station + ", " + cs + ", " + words + ", " + after
	}
	return t
}

// greeting is a greeting at the time at, picked at random: the time of
// day most, its short form ("morning", a crew's only), "good day", "hello".
func (r *Radio) greeting(at time.Time, crew bool) string {
	daytime := "good evening"
	switch h := at.Hour(); {
	case h >= 4 && h < 12:
		daytime = "good morning"
	case h >= 12 && h < 18:
		daytime = "good afternoon"
	}
	short := daytime
	if crew {
		short = strings.TrimPrefix(daytime, "good ")
	}
	return []string{daytime, daytime, daytime, daytime, short, "good day", "good day", "hello"}[r.rng.IntN(8)]
}

// greetedBack is a controller's t, its first answer to a crew's first call
// on the frequency, with a greeting after the call sign: "CSA1, good
// morning, identified" — mostly when the crew greeted, now and then when
// it did not; less on a busy frequency.
func (r *Radio) greetedBack(t Transmission, busy bool) Transmission {
	key := t.Airport + " " + t.Frequency + " " + t.Callsign
	crewGreeted, ok := r.firstCalls[key]
	if !ok || t.Callsign == "" {
		return t
	}
	delete(r.firstCalls, key)
	share := 0.25
	if crewGreeted {
		share = 0.75
	}
	if busy {
		share /= 2
	}
	if !strings.HasPrefix(t.Text, t.Callsign+", ") || r.rng.Float64() >= share {
		return t
	}
	t.Text = t.Callsign + ", " + r.greeting(t.At, false) + ", " + strings.TrimPrefix(t.Text, t.Callsign+", ")
	return t
}

// variedReadback is rb, the readback of t, with crew cs's style: a
// goodbye on a handoff for a chatty crew, a different "looking out".
func (r *Radio) variedReadback(t, rb Transmission, busy bool) Transmission {
	style := r.opts.Variety.StyleOf(t.Callsign)
	switch {
	case t.Intent == IntentContact && style.Chatty && !busy:
		rb.Text += []string{", good day", ", good day", ", bye", ", bye bye", ", cheers", ", see you"}[r.rng.IntN(6)]
	case t.Intent == IntentTrafficInfo && r.rng.IntN(2) == 0:
		rb.Text = strings.Replace(rb.Text, "Looking out", "Looking", 1)
	}
	return rb
}

// readbackErrorKeys are what a crew may read back wrong (#721): numbers a
// correction fixes, not the runway.
var readbackErrorKeys = []string{ParamFreq, ParamHeading, ParamLevel, ParamSquawk, ParamSpeed, ParamAltitude}

// wrongReadback is rb with one of t's numbers read back wrong, and what
// the controller heard; false when t has none to get wrong.
func wrongReadback(t, rb Transmission, rng *rand.Rand) (Transmission, map[string]string, bool) {
	var keys []string
	for _, k := range readbackErrorKeys {
		if v := t.Params[k]; v != "" && strings.Contains(rb.Text, v) && wrongValue(k, v) != v {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return rb, nil, false
	}
	k := keys[rng.IntN(len(keys))]
	wrong := wrongValue(k, t.Params[k])
	rb.Text = strings.Replace(rb.Text, t.Params[k], wrong, 1)
	heard := maps.Clone(t.Params)
	heard[k] = wrong
	return rb, heard, true
}

// wrongValue is v (the value of param k) as a crew might mishear it: an
// altitude a thousand feet off ("6000" → "7000", not "6010", which nobody
// says), else wrongDigit.
func wrongValue(k, v string) string {
	if k == ParamAltitude {
		return wrongDigitAt(v, 4)
	}
	return wrongDigit(v)
}

// wrongDigit is s with the digit before its last one a step up ("270" →
// "280", "121.910" → "121.920", "4521" → "4531"); s when it has fewer than
// two digits.
func wrongDigit(s string) string { return wrongDigitAt(s, 2) }

// wrongDigitAt is s with its n-th digit from the end a step up; s when it
// has fewer digits.
func wrongDigitAt(s string, n int) string {
	b := []byte(s)
	seen := 0
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < '0' || b[i] > '9' {
			continue
		}
		if seen++; seen == n {
			b[i] = '0' + (b[i]-'0'+1)%10
			return string(b)
		}
	}
	return s
}

// readBack has the crew of t read it back, with the radio's variety: after
// the crew's pause; now and then a "say again" and t again, or a wrong
// readback, the controller's correction and the right one.
func (r *Radio) readBack(airport string, t Transmission, busy bool) {
	rb, ok := Readback(t)
	if !ok {
		return
	}
	rb.Airport, rb.Frequency = t.Airport, t.Frequency
	v := r.opts.Variety
	if r.rng == nil || v == nil {
		r.Transmit(airport, rb)
		return
	}
	r.mu.Lock()
	style := v.StyleOf(t.Callsign)
	pause := style.pause(r.rng)
	roll := r.rng.Float64()
	sayAgain := t.Params[ParamRepeat] == "" && t.Intent != IntentCorrection && roll < v.rate(v.SayAgain, 0.02)
	var wrong Transmission
	var heard map[string]string
	wrongOK := false
	// One slip per exchange: a clearance said again (a missed call, a
	// "say again") is read back right (live, TVS1539: "say again", then
	// "6010 feet" read back and the right readback after it, uncorrected).
	if !sayAgain && t.Params[ParamRepeat] == "" && t.Intent != IntentCorrection && roll < v.rate(v.SayAgain, 0.02)+v.rate(v.ReadbackError, 0.01) {
		wrong, heard, wrongOK = wrongReadback(t, rb, r.rng)
	}
	missed := !sayAgain && !wrongOK && t.Intent == IntentContact && t.Params[ParamRepeat] == "" && r.rng.Float64() < v.rate(v.MissedCall, 0.03)
	silence := time.Duration((6 + 4*r.rng.Float64()) * float64(time.Second))
	if !sayAgain {
		rb = r.variedReadback(t, rb, busy)
	}
	r.mu.Unlock()
	at := func() time.Time { return r.ClearAt(t.Airport, t.Frequency).Add(pause) }
	switch {
	case missed:
		// No answer: after a silence the controller calls again, and the
		// crew reads it back then.
		again := t
		again.At = r.ClearAt(t.Airport, t.Frequency).Add(silence)
		again.Params = cloneParams(t.Params, ParamRepeat, "1")
		r.Transmit(airport, again)
	case sayAgain:
		sa := pilotTx(t.Position, t.Callsign, IntentPilotSayAgain, nil, "Say again, "+t.Callsign)
		sa.Airport, sa.Frequency, sa.At = t.Airport, t.Frequency, at()
		r.transmit(airport, sa, false)
		again := t
		again.At = time.Time{}
		again.Params = cloneParams(t.Params, ParamRepeat, "1")
		r.Transmit(airport, again) // read back in its turn
	case wrongOK:
		wrong.At = at()
		r.transmit(airport, wrong, false)
		if c, ok := CheckReadback(t, heard); !ok {
			c.At = time.Time{}
			r.transmit(airport, c, false)
		}
		rb.At = at()
		r.transmit(airport, rb, false)
	default:
		rb.At = at()
		r.transmit(airport, rb, false)
	}
}

// SetVariety switches the radio's variety on (v) or off (nil), #721.
func (r *Radio) SetVariety(v *Variety) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opts.Variety = v
	r.rng = nil
	if v != nil {
		r.rng = rand.New(rand.NewPCG(v.Seed, 0x721))
	}
}

// Variety is the radio's variety, nil when off.
func (r *Radio) Variety() *Variety {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.opts.Variety
}
