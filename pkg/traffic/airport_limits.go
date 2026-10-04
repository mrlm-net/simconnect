package traffic

import "github.com/mrlm-net/simconnect/pkg/airport"

// capTaxiSpeed lowers the motion profile's taxi speed to the airport's
// TaxiMaxKts where the profile is faster (#335).
func capTaxiSpeed(p *MotionProfile, lim *airport.Limits) {
	if lim != nil && lim.TaxiMaxKts > 0 && p.CruiseKts > lim.TaxiMaxKts {
		p.CruiseKts = lim.TaxiMaxKts
	}
}

// apronSpans collects the stretches of a ground path along route edges that
// touch an apron node (airport.Graph.Apron: a stand's junction with the
// taxilane), to cap them at the airport's ApronMaxKts.
type apronSpans struct {
	g      *airport.Graph
	spans  [][2]float64
	prev   airport.NodeID
	prevAt float64
	has    bool
}

// add records route node id at distance at along the path.
func (a *apronSpans) add(id airport.NodeID, at float64) {
	if a.has && (a.g.Apron(a.prev) || a.g.Apron(id)) {
		a.spans = append(a.spans, [2]float64{a.prevAt, at})
	}
	a.prev, a.prevAt, a.has = id, at, true
}

// limit caps path over the recorded spans at lim.ApronMaxKts, braking at
// decel before them; nothing without limits.
func (a *apronSpans) limit(path *GroundPath, lim *airport.Limits, decel float64) {
	if lim == nil || lim.ApronMaxKts <= 0 {
		return
	}
	for _, s := range a.spans {
		path.LimitRange(s[0], s[1], lim.ApronMaxKts, decel)
	}
}

// handoverFt is the height above the runway at which the injected take-off
// hands over to MSFS AI: the airport's ClimbHandoverFt, else
// ClimbHandoverFt.
func (c *TaxiController) handoverFt() float64 {
	if c.req.VFR {
		return VFRHandoverFt
	}
	if a := c.req.Airport; a != nil && a.ClimbHandoverFt > 0 {
		return a.ClimbHandoverFt
	}
	return ClimbHandoverFt
}
