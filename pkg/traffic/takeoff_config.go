package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/convert"
)

// Runway-aware take-off configuration (#1566). A type's TakeoffProfile is
// its normal take-off: a reduced (derated, assumed-temperature) thrust and
// its usual take-off flaps, at a long-haul weight. Where the runway's take-off
// run available (TORA, from the pavement end) is shorter than that needs
// (RequiredTakeoffRun at the airport's elevation and temperature), the crew
// takes off short-field: full rated thrust (TOGA) and the higher take-off
// flap setting (lower Vr and climb speed), and if that still does not fit, at
// a lower weight. ChooseTakeoff picks the first that fits.
//
// The figures are engineering estimates, not taken from any manufacturer's
// performance data (none was read for this): kept conservative and in
// proportion to the published take-off distances in knownTypes.
const (
	// ShortFieldThrustFactor: full rated thrust accelerates this much
	// harder than the derated normal take-off (a derate of about 13 %).
	ShortFieldThrustFactor = 1.15
	// ShortFieldSpeedFactor: the short-field flap setting lowers the stall
	// speed, so Vr and the climb speed, by about 5 % (a tenth more
	// maximum lift coefficient: speeds go with its inverse square root).
	ShortFieldSpeedFactor = 0.95
	// TakeoffMinWeightShare is the lightest a weight-limited take-off goes,
	// as a share of the normal take-off weight: a short sector, less fuel
	// and payload. Below it the type does not depart (RequiredTakeoffRun is
	// longer than the runway).
	TakeoffMinWeightShare = 0.85
	// takeoffWeightStep: the weight comes down in these steps until the
	// take-off fits.
	takeoffWeightStep = 0.01
)

// TakeoffThrust is the thrust a take-off is flown with.
type TakeoffThrust string

const (
	TakeoffThrustDerate TakeoffThrust = "derate" // reduced (flex, assumed temperature)
	TakeoffThrustFull   TakeoffThrust = "full"   // full rated thrust (TOGA)
)

// TakeoffConfig is the configuration a departure takes off in, chosen for
// its runway (ChooseTakeoff): the flaps (detent name and percent of the
// handle travel), the thrust, short-field or not, and the weight when
// limited by the runway.
type TakeoffConfig struct {
	Flaps      string        `json:"flaps,omitempty"`
	FlapsPct   float64       `json:"flapsPct"`
	Thrust     TakeoffThrust `json:"thrust"`
	ShortField bool          `json:"shortField,omitempty"`
	// WeightPct is the take-off weight as percent of the normal one when the
	// runway limits it; 0 (omitted) not limited.
	WeightPct float64 `json:"weightPct,omitempty"`
	// RequiredM is the run this configuration needs (RequiredTakeoffRun's
	// rule), AvailableM the take-off run available it was chosen for.
	RequiredM  float64 `json:"requiredM,omitempty"`
	AvailableM float64 `json:"availableM,omitempty"`
}

// ShortFieldTakeoff is p flown short-field at weightShare (1: the normal
// weight) of the normal take-off weight: full thrust (ShortFieldThrustFactor)
// and the short-field flaps (ShortFieldSpeedFactor) on Vr and the climb
// speeds; a lighter aircraft rotates and climbs out slower (speeds with
// the square root of the weight) and accelerates harder (with its inverse).
func ShortFieldTakeoff(p TakeoffProfile, weightShare float64) TakeoffProfile {
	if p == (TakeoffProfile{}) {
		p = DefaultTakeoffProfile()
	}
	w := math.Max(0.1, math.Min(1, weightShare))
	speed := ShortFieldSpeedFactor * math.Sqrt(w)
	p.RollAccel *= ShortFieldThrustFactor / w
	p.RotateKts *= speed
	p.ClimbKts *= speed
	return p
}

// takeoffRunNormal is RequiredTakeoffRun's rule for p as it is: the
// distance to 35 ft flown by the TakeoffMover, lengthened for the
// conditions and by TakeoffRunMargin.
func takeoffRunNormal(p TakeoffProfile, c TakeoffConditions) float64 {
	m := NewTakeoffMover(airport.LatLon{}, 0, 0, p)
	pose := m.Pose()
	for i := 0; i < 20*300 && pose.HeightFt < 35; i++ {
		pose = m.Step(0.05)
	}
	factor := 1 + 0.10*math.Max(0, c.ElevationFt)/1000 + 0.01*math.Max(0, c.ISADeviationC)
	return pose.Distance * factor * TakeoffRunMargin
}

// liftoffRun is where p's wheels leave the runway, meters from the start
// of the roll (sea level, ISA: as the TakeoffMover flies it).
func liftoffRun(p TakeoffProfile) float64 {
	m := NewTakeoffMover(airport.LatLon{}, 0, 0, p)
	pose := m.Pose()
	for i := 0; i < 20*300 && pose.Phase != TakeoffAirborne; i++ {
		pose = m.Step(0.05)
	}
	return pose.LiftoffDistance
}

// ChooseTakeoff picks p's configuration for a take-off run available of
// availableM (0: not known, the normal take-off) under c: the normal
// take-off (derated, flaps fl.TakeoffPct) where it fits, else short-field
// (full thrust, fl.ShortTakeoffPct), else short-field at the heaviest weight
// down to TakeoffMinWeightShare that fits. Where not even that fits (a
// runway the dispatch check should have refused) the weight comes down
// further, to half at most, until the wheels leave the runway a margin
// (TakeoffRunMargin) before its end.
func ChooseTakeoff(p TakeoffProfile, fl FlapSchedule, c TakeoffConditions, availableM float64) (TakeoffProfile, TakeoffConfig) {
	if p == (TakeoffProfile{}) {
		p = DefaultTakeoffProfile()
	}
	cfg := TakeoffConfig{Flaps: fl.TakeoffDetent, FlapsPct: fl.TakeoffPct, Thrust: TakeoffThrustDerate, AvailableM: availableM}
	need := takeoffRunNormal(p, c)
	if availableM <= 0 || need <= availableM {
		cfg.RequiredM = need
		return p, cfg
	}
	cfg.Thrust, cfg.ShortField = TakeoffThrustFull, true
	if fl.ShortTakeoffPct > 0 {
		cfg.Flaps, cfg.FlapsPct = fl.ShortTakeoffDetent, fl.ShortTakeoffPct
	}
	w := 1.0
	for {
		sp := ShortFieldTakeoff(p, w)
		need = takeoffRunNormal(sp, c)
		if need <= availableM || w <= TakeoffMinWeightShare+1e-9 {
			if need > availableM {
				// Last resort: lift off before the runway's end, with the
				// margin TakeoffRunMargin keeps.
				for w > 0.5 && liftoffRun(sp)*TakeoffRunMargin > availableM {
					w -= takeoffWeightStep
					sp = ShortFieldTakeoff(p, w)
				}
				need = takeoffRunNormal(sp, c)
			}
			if w < 1-1e-9 {
				cfg.WeightPct = math.Round(w * 100)
			}
			cfg.RequiredM = need
			return sp, cfg
		}
		w = math.Max(TakeoffMinWeightShare, w-takeoffWeightStep)
	}
}

// chooseTakeoff sets the departure's take-off configuration for a take-off
// run available of availableM (ChooseTakeoff) at its airport's elevation
// and temperature, and tells it with the next event.
func (c *TaxiController) chooseTakeoff(availableM float64) {
	a := c.aircraft()
	cond := TakeoffConditions{ISADeviationC: c.req.ISADeviationC}
	if c.req.Graph != nil {
		cond.ElevationFt = convert.MetersToFeet(c.req.Graph.Layout.Altitude)
	}
	p, cfg := ChooseTakeoff(withDefaults(c.req.Takeoff, DefaultTakeoffProfile()), a.Flaps, cond, availableM)
	if a.Category == CategoryPiston {
		cfg.Thrust = TakeoffThrustFull // a light single takes off at full power
	}
	c.toProfile, c.toConfig = p, &cfg
	c.last.Takeoff = c.toConfig
}

// plannedTakeoffRun is the take-off run available from where the taxi
// route meets the runway: its entry's Remaining, the full length when
// backtracking or the entry is not known.
func (c *TaxiController) plannedTakeoffRun() float64 {
	g := c.req.Graph
	if g == nil || c.route == nil || len(c.route.Nodes) == 0 || c.Backtracks() {
		return c.runwayLength
	}
	last := c.route.Nodes[len(c.route.Nodes)-1]
	if entries, err := g.RunwayEntries(c.end.Name); err == nil {
		for _, e := range entries {
			if e.HeldAt(last) || e.Node == last {
				return e.Remaining
			}
		}
	}
	return c.runwayLength
}

// chooseTakeoffAt sets the configuration again for the roll starting at
// start: the runway ahead of it to the pavement end. Flaps set for another
// configuration move to this one's.
func (c *TaxiController) chooseTakeoffAt(start airport.LatLon) {
	if c.runwayLength <= 0 {
		if c.toConfig == nil {
			c.chooseTakeoff(0)
		}
		return
	}
	set := c.flaps.target
	c.chooseTakeoff(c.runwayLength - math.Max(0, alongHeading(c.end.Threshold, c.end.Heading, start)))
	if to := c.toConfig.FlapsPct; set > 0 && set != to {
		c.flaps.target, c.flaps.rate = to, math.Max(to, c.flaps.pct)/(FlapsSetSeconds*f(c.timing.flaps))
	}
}

// takeoffFlapsPct is the take-off flap setting: the chosen configuration's,
// the type's normal one before.
func (c *TaxiController) takeoffFlapsPct() float64 {
	if c.toConfig != nil {
		return c.toConfig.FlapsPct
	}
	return c.aircraft().Flaps.TakeoffPct
}

// takeoffThrottlePct is the throttle set for the roll: TakeoffThrottlePct,
// all of it at full thrust.
func (c *TaxiController) takeoffThrottlePct() float64 {
	if c.toConfig != nil && c.toConfig.Thrust == TakeoffThrustFull {
		return 100
	}
	return TakeoffThrottlePct
}
