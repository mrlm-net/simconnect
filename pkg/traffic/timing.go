//go:build windows
// +build windows

package traffic

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Natural timing (#343): every wait and duration with a real-world
// counterpart varies a little from aircraft to aircraft, so traffic never
// looks scripted. Each aircraft draws one factor per spread when its
// controller starts (from the controller's RNG; TaxiWithSeed and
// ArrivalWithSeed make it reproducible) and keeps it for the whole flight:
// one crew is a little quicker than the next, not erratic. A spread of 0
// gives exactly the tunable. Gate waits and the after-landing dwell vary
// by DwellJitter on each wait.
var (
	// BeaconLeadSpread varies BeaconLeadTime (beacon on to push).
	BeaconLeadSpread = 0.3
	// TaxiLightSpread varies TaxiLightDelay (taxi light to moving; after
	// landing, landing lights off to taxi light on).
	TaxiLightSpread = 0.3
	// TugDisconnectSpread varies TugDisconnectSeconds (push done to the
	// tug backing off), for tugs that support it (SetDisconnectDelay).
	TugDisconnectSpread = 0.3
	// FlapsSpread varies how long flaps take: FlapsSetSeconds,
	// FlapsRetractClimbSeconds, FlapsRetractSeconds, FlapsFullSeconds.
	FlapsSpread = 0.2
	// GearUpSpread varies GearUpDelaySeconds after lift-off.
	GearUpSpread = 0.2
	// TaxiSpeedSpread varies the taxi speed (MotionProfile.CruiseKts) and
	// PushbackSpeedSpread the pushback pace.
	TaxiSpeedSpread     = 0.08
	PushbackSpeedSpread = 0.1
)

// timing is one aircraft's draw of the spreads: factors around 1.
type timing struct {
	beacon, taxiLight, tug, flaps, gearUp, taxiSpeed, pushSpeed float64
}

// spread draws a factor within 1±s.
func spread(rng *rand.Rand, s float64) float64 {
	if s <= 0 || rng == nil {
		return 1
	}
	return 1 + s*(2*rng.Float64()-1)
}

// drawTiming draws an aircraft's factors.
func drawTiming(rng *rand.Rand) timing {
	return timing{
		beacon: spread(rng, BeaconLeadSpread), taxiLight: spread(rng, TaxiLightSpread), tug: spread(rng, TugDisconnectSpread),
		flaps: spread(rng, FlapsSpread), gearUp: spread(rng, GearUpSpread),
		taxiSpeed: spread(rng, TaxiSpeedSpread), pushSpeed: spread(rng, PushbackSpeedSpread),
	}
}

// f returns factor x, or 1 before the draw.
func f(x float64) float64 {
	if x == 0 {
		return 1
	}
	return x
}

// taxiSpeed scales a profile's taxi speed by the aircraft's factor, never
// above the airport's taxi limit.
func taxiSpeed(p MotionProfile, factor, limitKts float64) MotionProfile {
	p.CruiseKts *= f(factor)
	if limitKts > 0 {
		p.CruiseKts = math.Min(p.CruiseKts, limitKts)
	}
	return p
}

// disconnectDelayer is a tug whose wait before it drives off can be set.
type disconnectDelayer interface {
	SetDisconnectDelay(seconds float64)
}

// SetDisconnectDelay sets how long the tug waits after the push before it
// backs off the aircraft (default TugDisconnectSeconds).
func (t *SimObjectTug) SetDisconnectDelay(seconds float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.waitLeft = seconds
}

// De-icing (#323): the treatment takes DefaultDeicingDwell (varied by
// DwellJitter); a pad must lie within DeicingPadReachMeters of a taxi node.
var (
	DefaultDeicingDwell   = 6 * 60 * time.Second
	DeicingPadReachMeters = 60.0
)

// Deicing asks for a de-icing before departure (#323).
type Deicing struct {
	// Pad is where: a remote pad the route passes, where the aircraft
	// stops with engines running; nil de-ices on the stand after the
	// pushback clearance, before the push.
	Pad *airport.DeicingPad
	// Dwell is how long the treatment takes; 0 means DefaultDeicingDwell.
	Dwell time.Duration
}
