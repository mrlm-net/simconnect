package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The attitude an arrival has where the injected approach takes it over
// (#1636: round a circuit's base turn, level in one write). It is wanted
// at that moment only, so it is no part of the monitor, which can be read
// every frame: the injector's, where it placed the aircraft, else read by
// itself (its own definition, ONCE): each monitor answer nearing the
// hand-over, at once a second, and at the hand-over, whose answer comes
// before the approach first places the aircraft.

const (
	arrDefAttitude = 7
	arrReqAttitude = 4
)

// attitudeAskMeters: this far out beyond the join it starts asking.
const attitudeAskMeters = 3 * 1852.0

// arrivalAttitude is the attitude definition's answer, SimConnect's signs.
type arrivalAttitude struct {
	PitchDown, BankLeft float64
}

func defineAttitude(client engine.Client, def uint32) error {
	for i, name := range []string{"PLANE PITCH DEGREES", "PLANE BANK DEGREES"} {
		if err := client.AddToDataDefinition(def, name, "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
			return err
		}
	}
	return nil
}

// askAttitude reads the aircraft's attitude once.
func (c *ArrivalController) askAttitude() {
	if client := c.fleet.clientOrNil(); client != nil {
		c.note("attitude", client.RequestDataOnSimObject(c.reqBase+arrReqAttitude, c.defBase+arrDefAttitude, c.objectID,
			types.SIMCONNECT_PERIOD_ONCE, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0))
	}
}

// nearHandOver: flown by MSFS AI and near enough its join to want the
// attitude.
func (c *ArrivalController) nearHandOver(pos, threshold airport.LatLon) bool {
	return localDist(pos, threshold) < c.proc.JoinMeters+attitudeAskMeters
}

// handleAttitude takes askAttitude's answer: kept for the hand-over, and
// blended from at once when the approach has taken over but not moved the
// aircraft yet.
func (c *ArrivalController) handleAttitude(m *types.SIMCONNECT_RECV_SIMOBJECT_DATA) {
	if uint32(m.DwDefineID) != c.defBase+arrDefAttitude {
		return
	}
	c.attitude, c.hasAttitude = *engine.CastDataAs[arrivalAttitude](&m.DwData), true
	if c.approach != nil && !c.flyingProc && c.blend.attitudeFresh {
		c.blendAttitude()
	}
}

// blendAttitude fades the injected approach in from the attitude shown.
func (c *ArrivalController) blendAttitude() {
	a, ok := c.attitude, c.hasAttitude
	if c.inj != nil {
		if p, _, shown := c.inj.Shown(c.objectID); shown {
			a, ok = arrivalAttitude{PitchDown: p.Pitch, BankLeft: p.Bank}, true
		}
	}
	if !ok || c.approach == nil || math.IsNaN(a.PitchDown+a.BankLeft) {
		return
	}
	p := c.approach.Pose()
	c.blend.dPitch, c.blend.dBank = -a.PitchDown-p.PitchDeg, -a.BankLeft-p.BankDeg
	c.blend.attitudeLeft = attitudeBlendSeconds
}
