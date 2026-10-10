package flight

import (
	"fmt"
	"math"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Judging a recorded flight: its profile (the figures pilots look at) and
// findings against common airline practice, each with advice, and a score
// to rank flights by. From the track alone; a runway given, the touchdown
// is placed on it too.

// Severities of a finding: Info costs nothing, Minor MinorPoints, Major
// MajorPoints of the score.
const (
	Info  = "info"
	Minor = "minor"
	Major = "major"

	MinorPoints = 5
	MajorPoints = 15
)

// Finding is one thing seen in the flight, with what to do about it.
type Finding struct {
	Phase    string  `json:"phase"` // taxi, takeoff, climb, cruise, approach, landing
	Code     string  `json:"code"`  // e.g. "hard-landing"
	Severity string  `json:"severity"`
	T        float64 `json:"t"`     // the sample's simulation time
	Value    float64 `json:"value"` // what was measured (fpm, kt, °, ft, m)
	Text     string  `json:"text"`  // what happened and the advice
}

// Profile is a flight's key figures (0 not seen).
type Profile struct {
	MaxTaxiKts     float64 `json:"maxTaxiKts,omitempty"`
	RotateKts      float64 `json:"rotateKts,omitempty"`
	LiftoffKts     float64 `json:"liftoffKts,omitempty"`
	LiftoffPitch   float64 `json:"liftoffPitch,omitempty"`
	MaxPitchRate   float64 `json:"maxPitchRate,omitempty"` // °/s in the rotation
	MaxAltFt       float64 `json:"maxAltFt,omitempty"`
	MaxBank        float64 `json:"maxBank,omitempty"` // airborne
	Gate1000Kts    float64 `json:"gate1000Kts,omitempty"`
	Gate1000Fpm    float64 `json:"gate1000Fpm,omitempty"`
	Gate500Kts     float64 `json:"gate500Kts,omitempty"`
	Gate500Fpm     float64 `json:"gate500Fpm,omitempty"`
	ApproachKts    float64 `json:"approachKts,omitempty"`
	TouchdownFpm   float64 `json:"touchdownFpm,omitempty"`
	TouchdownKts   float64 `json:"touchdownKts,omitempty"`
	TouchdownPitch float64 `json:"touchdownPitch,omitempty"`
	TouchdownBank  float64 `json:"touchdownBank,omitempty"`
	// TouchdownM is how far past the threshold it touched down, and
	// CentrelineM how far off the centreline (right positive): with a
	// runway given.
	TouchdownM  float64 `json:"touchdownM,omitempty"`
	CentrelineM float64 `json:"centrelineM,omitempty"`
	Bounces     int     `json:"bounces,omitempty"`
}

// Assessment is a flight judged: Score 0…100 (100 less the findings'
// points), the profile and the findings in time order.
type Assessment struct {
	Score    int       `json:"score"`
	Profile  Profile   `json:"profile"`
	Findings []Finding `json:"findings"`
}

// AssessRunway is the landing runway: threshold, true heading.
type AssessRunway struct {
	Lat, Lon float64
	Heading  float64
}

// AssessOptions tune Assess; zero values take the defaults.
type AssessOptions struct {
	// ApproachKts is the approach speed (Vapp); 0: the median IAS between
	// 1000 and 200 ft on the final.
	ApproachKts float64
	// Runway is the landing runway, for where it touched down.
	Runway *AssessRunway
	// TailstrikePitch: the pitch at lift-off or touchdown that risks the
	// tail (11.5° the A320's on the ground); 0: 11.
	TailstrikePitch float64
	// Checklists: when the take-off and landing checklists were complete
	// (checklist.ForAssess); given, they are judged in place of the gear
	// at 1000 ft.
	Checklists []ChecklistDone
}

// Limits Assess judges by (common airline practice).
const (
	assessTaxiKts         = 30.0  // on the ground before the take-off roll and after the rollout
	assessRotateRate      = 4.0   // °/s: faster risks the tail
	assessSpeedLimitKts   = 250.0 // below 10,000 ft
	assessSpeedLimitFt    = 10000.0
	assessBank            = 30.0 // airborne; Major from 35
	assessBankLow         = 10.0 // below 100 ft
	assessStableSinkFpm   = 1000.0
	assessStableBelowKts  = 5.0  // Vapp − 5
	assessStableAboveKts  = 10.0 // Vapp + 10
	assessHardFpm         = 600.0
	assessFirmFpm         = 360.0
	assessTouchBank       = 3.0
	assessTouchFarM       = 900.0 // past the threshold: long
	assessTouchShortM     = 150.0 // before: short of the aiming point
	assessCentrelineM     = 5.0
	assessBounceS         = 5.0 // airborne again within this after touching
	assessRunwayRollKts   = 40.0
	assessLightsEngineN1  = 15.0
	assessDefaultTailPith = 11.0
)

// Assess judges t's flight.
func Assess(t *Track, o AssessOptions) Assessment {
	a := Assessment{Score: 100, Findings: []Finding{}}
	ss := t.Samples
	if len(ss) < 2 {
		return a
	}
	tail := o.TailstrikePitch
	if tail == 0 {
		tail = assessDefaultTailPith
	}
	add := func(phase, code, sev string, s Sample, v float64, format string, args ...any) {
		a.Findings = append(a.Findings, Finding{Phase: phase, Code: code, Severity: sev, T: s.T, Value: math.Round(v*10) / 10, Text: fmt.Sprintf(format, args...)})
	}
	p := &a.Profile

	// The take-off roll starts (40 kt), lift-off, touchdown, the rollout ends.
	roll, lift, touch, stop := -1, -1, -1, -1
	for i := 1; i < len(ss); i++ {
		if roll < 0 && ss[i].OnGround && ss[i].IAS >= assessRunwayRollKts {
			roll = i
		}
		if lift < 0 && ss[i-1].OnGround && !ss[i].OnGround && ss[i].IAS > assessRunwayRollKts {
			lift = i
		}
		if lift >= 0 && !ss[i-1].OnGround && ss[i].OnGround {
			touch = i // the last one wins
		}
	}
	if touch > 0 {
		for i := touch; i < len(ss); i++ {
			if ss[i].OnGround && ss[i].IAS < assessRunwayRollKts {
				stop = i
				break
			}
		}
	}

	// Taxi: before the roll, after the rollout.
	taxiFast := false
	for i, s := range ss {
		if !s.OnGround || (roll >= 0 && i >= roll && (stop < 0 || i < stop)) {
			continue
		}
		p.MaxTaxiKts = math.Max(p.MaxTaxiKts, s.GS)
		if s.GS > assessTaxiKts && !taxiFast {
			taxiFast = true
			add("taxi", "taxi-fast", Minor, s, s.GS, "Taxied at %.0f kt: keep to %.0f kt or less, and about 10 kt in turns.", s.GS, assessTaxiKts)
		}
	}
	// Engines running without the beacon.
	for _, s := range ss {
		if s.EngineCount > 0 && s.N1[0] > assessLightsEngineN1 && s.Lights&LightBeacon == 0 {
			add("taxi", "beacon-off", Minor, s, s.N1[0], "Engines running without the beacon: switch it on before start-up.")
			break
		}
	}

	if lift > 0 {
		s := ss[lift]
		p.LiftoffKts, p.LiftoffPitch = s.IAS, s.Pitch
		if cl, ok := checklistDone(o.Checklists, "before-takeoff"); ok && (!cl.Done || cl.T > s.T) {
			add("takeoff", "before-takeoff-checklist", Minor, s, 0, "Took off before the before take-off checklist was complete: run it at the holding point, down to the line-up items.")
		}
		if s.Lights&LightLanding == 0 {
			add("takeoff", "landing-lights-off", Minor, s, 0, "Took off without the landing lights: on for the take-off, off above 10,000 ft.")
		}
		if s.Pitch > tail {
			add("takeoff", "tailstrike-risk", Major, s, s.Pitch, "Lifted off at %.1f° of pitch: past %.0f° the tail can strike. Rotate smoothly to about 7–8° and let it fly off.", s.Pitch, tail)
		}
		// The rotation: from where the nose left the roll's pitch (as
		// Learn finds it) to 5 s after lift-off, the fastest pitch rate.
		var roll []float64
		from := lift - 1
		for from > 0 && ss[from].OnGround && s.T-ss[from].T < 20 {
			roll = append(roll, ss[from].Pitch)
			from--
		}
		rot := lift
		for i := lift - 1; i > from; i-- {
			if ss[i].Pitch <= median(roll)+1 {
				rot = min(i+1, lift)
				break
			}
		}
		p.RotateKts = ss[rot].IAS
		for i := max(rot, 1); i < len(ss) && ss[i].T-s.T < 5; i++ {
			if dt := ss[i].T - ss[i-1].T; dt > 0 {
				p.MaxPitchRate = math.Max(p.MaxPitchRate, (ss[i].Pitch-ss[i-1].Pitch)/dt)
			}
		}
		p.MaxPitchRate = math.Round(p.MaxPitchRate*10) / 10
		if p.MaxPitchRate > assessRotateRate {
			add("takeoff", "fast-rotation", Minor, s, p.MaxPitchRate, "Rotated at %.1f°/s: about 3°/s keeps the tail clear.", p.MaxPitchRate)
		}
	}

	// Airborne: bank, the speed limit, the highest altitude.
	bankSeen, speedSeen, bankLowSeen := false, false, false
	for i, s := range ss {
		if s.OnGround || (lift >= 0 && i < lift) {
			continue
		}
		h := agl(s)
		p.MaxAltFt = math.Max(p.MaxAltFt, s.AltFt)
		bank := math.Abs(s.Bank)
		p.MaxBank = math.Max(p.MaxBank, bank)
		if bank > assessBank && !bankSeen {
			bankSeen = true
			sev := Minor
			if bank > assessBank+5 {
				sev = Major
			}
			add(phaseOf(i, lift, touch), "steep-bank", sev, s, bank, "Banked %.0f°: keep within %.0f° (25° is the norm).", bank, assessBank)
		}
		if h < 100 && bank > assessBankLow && !bankLowSeen {
			bankLowSeen = true
			add(phaseOf(i, lift, touch), "bank-near-ground", Major, s, bank, "Banked %.0f° below 100 ft: wings level near the ground, a wingtip or engine can touch.", bank)
		}
		if s.AltFt < assessSpeedLimitFt-100 && s.IAS > assessSpeedLimitKts+10 && !speedSeen {
			speedSeen = true
			add(phaseOf(i, lift, touch), "speed-limit", Minor, s, s.IAS, "Flew %.0f kt below 10,000 ft: 250 kt at most there, unless ATC says otherwise.", s.IAS)
		}
	}

	// The approach: stable at 1000 and 500 ft above the ground.
	if touch > 0 {
		vapp := o.ApproachKts
		var final []float64
		maxFlaps := 0
		for _, s := range ss {
			maxFlaps = max(maxFlaps, s.FlapsIndex)
		}
		for i := touch - 1; i > 0 && agl(ss[i]) < 1000; i-- {
			if agl(ss[i]) >= 200 {
				final = append(final, ss[i].IAS)
			}
		}
		if vapp == 0 {
			vapp = median(final)
		}
		p.ApproachKts = math.Round(vapp)
		gate := func(ft float64) (Sample, bool) {
			for i := touch - 1; i > 0; i-- {
				if agl(ss[i]) >= ft {
					return ss[i], !ss[i].OnGround
				}
			}
			return Sample{}, false
		}
		if s, ok := gate(1000); ok {
			p.Gate1000Kts, p.Gate1000Fpm = s.IAS, s.VS
		}
		if s, ok := gate(500); ok {
			p.Gate500Kts, p.Gate500Fpm = s.IAS, s.VS
			var why []string
			if !s.GearHandle {
				why = append(why, "the gear up")
			}
			if s.FlapsIndex < maxFlaps {
				why = append(why, "not in the landing flaps")
			}
			if vapp > 0 && (s.IAS < vapp-assessStableBelowKts || s.IAS > vapp+assessStableAboveKts) {
				why = append(why, fmt.Sprintf("%.0f kt against %.0f", s.IAS, vapp))
			}
			if -s.VS > assessStableSinkFpm {
				why = append(why, fmt.Sprintf("sinking %.0f fpm", -s.VS))
			}
			if len(why) > 0 {
				add("approach", "unstable-500", Major, s, -s.VS, "Not stable at 500 ft (%s) and landed: an unstable approach at 500 ft is a go-around.", joinAnd(why))
			}
		}
		if cl, ok := checklistDone(o.Checklists, "landing"); ok {
			if s, at := gate(1000); at && (!cl.Done || cl.T > s.T) {
				add("approach", "landing-checklist-late", Minor, s, agl(s), "The landing checklist was not done by 1000 ft: have it done, the gear down and the landing flaps set by then.")
			}
		} else if s, ok := gate(1000); ok && !s.GearHandle {
			add("approach", "gear-late", Minor, s, agl(s), "The gear was still up at 1000 ft: have it down and the landing checklist done by then.")
		}

		// The touchdown: the sink rate just before it.
		s, before := ss[touch], ss[touch-1]
		fpm := -before.VS
		p.TouchdownFpm, p.TouchdownKts, p.TouchdownPitch, p.TouchdownBank = math.Round(fpm), s.IAS, s.Pitch, s.Bank
		switch {
		case fpm > assessHardFpm:
			add("landing", "hard-landing", Major, s, fpm, "Touched down at %.0f fpm, a hard landing (over %.0f): have it checked. Flare at 20–30 ft and keep the thrust to the retard call.", fpm, assessHardFpm)
		case fpm > assessFirmFpm:
			add("landing", "firm-landing", Minor, s, fpm, "Touched down at %.0f fpm, firm: a flare a little earlier makes it 100–300 fpm.", fpm)
		case fpm < 60:
			add("landing", "floated", Info, s, fpm, "Touched down at %.0f fpm: very soft, but floating eats runway; on a short runway land it firmly.", fpm)
		}
		if s.Pitch < 0 {
			add("landing", "nose-first", Major, s, s.Pitch, "Touched down at %.1f° of pitch, nose gear first: flare to about 3–5° nose up.", s.Pitch)
		} else if s.Pitch > tail {
			add("landing", "tailstrike-risk", Major, s, s.Pitch, "Touched down at %.1f° of pitch: past %.0f° the tail can strike. Hold the flare attitude, do not keep pulling.", s.Pitch, tail)
		}
		if b := math.Abs(s.Bank); b > assessTouchBank {
			add("landing", "bank-at-touchdown", Minor, s, b, "Touched down banked %.1f°: wings level, a little wing into wind only in a crosswind.", b)
		}
		// Bounces: airborne again within assessBounceS of touching down.
		first := touch
		for i := touch - 1; i > lift && i > 0; i-- {
			if ss[i].OnGround && !ss[i-1].OnGround {
				if ss[touch].T-ss[i].T < assessBounceS*float64(p.Bounces+1) {
					p.Bounces++
					first = i
					continue
				}
				break
			}
		}
		if p.Bounces > 0 {
			add("landing", "bounced", Minor, ss[first], float64(p.Bounces), "Bounced %d time(s): hold the attitude after a bounce and let it settle, or go around if it is high.", p.Bounces)
		}
		if r := o.Runway; r != nil {
			t := ss[first]
			along := calc.AlongTrackMeters(r.Lat, r.Lon, displaced(r, 10000), displaceLon(r, 10000), t.Lat, t.Lon)
			off := calc.CrossTrackMeters(r.Lat, r.Lon, displaced(r, 10000), displaceLon(r, 10000), t.Lat, t.Lon)
			p.TouchdownM, p.CentrelineM = math.Round(along), math.Round(off*10)/10
			switch {
			case along < assessTouchShortM:
				add("landing", "short-landing", Major, t, along, "Touched down %.0f m past the threshold: aim for the markers 300 m in.", along)
			case along > assessTouchFarM:
				add("landing", "long-landing", Minor, t, along, "Touched down %.0f m past the threshold: past the touchdown zone. Aim for 300–450 m and do not float.", along)
			}
			if math.Abs(off) > assessCentrelineM {
				add("landing", "off-centreline", Minor, t, off, "Touched down %.1f m off the centreline: keep the nose on it with the rudder in the flare.", math.Abs(off))
			}
		}
	}

	for _, f := range a.Findings {
		switch f.Severity {
		case Minor:
			a.Score -= MinorPoints
		case Major:
			a.Score -= MajorPoints
		}
	}
	a.Score = max(a.Score, 0)
	return a
}

// phaseOf names sample i's phase by the lift-off and touchdown.
func phaseOf(i, lift, touch int) string {
	switch {
	case touch > 0 && i >= touch:
		return "landing"
	case touch > 0 && i > (lift+touch)/2:
		return "approach"
	}
	return "climb"
}

func displaced(r *AssessRunway, m float64) float64 {
	lat, _ := calc.DisplaceByHeading(r.Lat, r.Lon, r.Heading, m)
	return lat
}

func displaceLon(r *AssessRunway, m float64) float64 {
	_, lon := calc.DisplaceByHeading(r.Lat, r.Lon, r.Heading, m)
	return lon
}

// joinAnd joins items as "a, b and c".
func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	out := items[0]
	for _, s := range items[1 : len(items)-1] {
		out += ", " + s
	}
	return out + " and " + items[len(items)-1]
}

// ChecklistDone is when a checklist was complete on the flight (Done, at
// simulation time T), for Assess: by name, "before-takeoff" and "landing".
type ChecklistDone struct {
	Name string  `json:"name"`
	Done bool    `json:"done"`
	T    float64 `json:"t"`
}

func checklistDone(cs []ChecklistDone, name string) (ChecklistDone, bool) {
	for _, c := range cs {
		if c.Name == name {
			return c, true
		}
	}
	return ChecklistDone{}, false
}
