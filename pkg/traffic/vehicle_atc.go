package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Service vehicles under ATC (#752): on a controlled airport a tug or a
// fuel truck drives onto the manoeuvring area as any other traffic does.
// Before a taxiway it holds and asks ground to proceed; before a runway
// it holds short and asks to cross, and reports vacated once off it. The
// vehicle roads and the aprons need no clearance. Each point is asked of
// a VehicleATC (the World's controllers), again each frame until cleared.

// VehicleGate is what a vehicle asks for.
type VehicleGate string

const (
	GateTaxiway VehicleGate = "taxiway" // proceed via the taxiways ahead
	GateRunway  VehicleGate = "runway"  // cross the runway
)

// VehicleRequest is a vehicle's request at a gate on its way.
type VehicleRequest struct {
	Vehicle  uint32 // its object ID
	Kind     string // "tug", "fuel truck"
	Gate     VehicleGate
	Taxiways []string // GateTaxiway: the taxiways of the stretch, in order
	Runway   string   // GateRunway: the runway ("06/24")
	At       airport.LatLon
}

// VehicleATC clears vehicles (#752): Cleared is asked at each gate until
// it is true (the first ask makes the call); Vacated is told once the
// vehicle is off a runway it crossed.
type VehicleATC interface {
	Cleared(r VehicleRequest) bool
	Vacated(r VehicleRequest)
}

// ATCAware is a vehicle that asks ATC (SimObjectTug, SimObjectFuelTruck).
type ATCAware interface {
	SetATC(atc VehicleATC, l *airport.Layout, kind string)
}

// Where a vehicle holds: VehicleHoldTaxiwayM before a taxiway's edge,
// VehicleHoldRunwayM outside a runway's (its holding point).
const (
	// VehicleTaxiwayAlongM: a vehicle driving this far along taxiways
	// asks for them; a shorter stretch is a road crossing one.
	VehicleTaxiwayAlongM = 30.0
	VehicleHoldTaxiwayM  = 5.0
	VehicleHoldRunwayM   = 40.0
	vehicleGateStep      = 2.0
)

// vehicleGate is a gate on a path: from at to end (meters along it).
type vehicleGate struct {
	VehicleRequest
	at, end          float64
	cleared, vacated bool
}

// vehicleGates are the gates on path through l: each taxiway stretch from
// where it starts (a runway crossing inside it a gate of its own), each
// runway VehicleHoldRunwayM before its edge.
func vehicleGates(l *airport.Layout, path *GroundPath) []vehicleGate {
	if l == nil || path == nil {
		return nil
	}
	var out []vehicleGate
	var cur *vehicleGate
	closeCur := func(s float64) {
		if cur != nil {
			cur.end = s
			// A road across a taxiway: given way as on any crossing, no call.
			if cur.Gate == GateRunway || cur.end-cur.at >= VehicleTaxiwayAlongM {
				out = append(out, *cur)
			}
			cur = nil
		}
	}
	for s := 0.0; s <= path.Length(); s += vehicleGateStep {
		p := path.PointAt(s)
		if r, ok := l.RunwayAt(p, VehicleHoldRunwayM); ok {
			if cur == nil || cur.Gate != GateRunway || cur.Runway != r.Name() {
				closeCur(s)
				cur = &vehicleGate{VehicleRequest: VehicleRequest{Gate: GateRunway, Runway: r.Name(), At: p}, at: s}
			}
			continue
		}
		name, on := l.TaxiwayAt(p)
		if !on {
			closeCur(s)
			continue
		}
		if cur == nil || cur.Gate != GateTaxiway {
			closeCur(s)
			cur = &vehicleGate{VehicleRequest: VehicleRequest{Gate: GateTaxiway, At: p}, at: math.Max(0, s-VehicleHoldTaxiwayM)}
		}
		if name != "" && (len(cur.Taxiways) == 0 || cur.Taxiways[len(cur.Taxiways)-1] != name) {
			cur.Taxiways = append(cur.Taxiways, name)
		}
	}
	closeCur(path.Length())
	return out
}

// gateStop is where on m's path the vehicle must stop for the first gate
// not cleared (false: none ahead), asking ATC as it comes; a runway
// crossed behind it is reported vacated.
func (y *vehicleYield) gateStop(m *GroundMover) (float64, bool) {
	if y.atc == nil || m == nil {
		return 0, false
	}
	if y.gatesFor != m {
		y.gates, y.gatesFor = vehicleGates(y.layout, m.Path()), m
	}
	s := m.Pose().Distance
	for i := range y.gates {
		g := &y.gates[i]
		g.Vehicle, g.Kind = y.self, y.kind
		if g.Gate == GateRunway && g.cleared && !g.vacated && s > g.end {
			g.vacated = true
			y.atc.Vacated(g.VehicleRequest)
		}
		if g.cleared || s > g.at+1 && s > g.end {
			continue
		}
		if s > g.at+1 {
			g.cleared = true // already on it (it started there): on its way
			continue
		}
		if y.atc.Cleared(g.VehicleRequest) {
			g.cleared = true
			continue
		}
		return g.at, true
	}
	return 0, false
}

// SetATC makes the vehicle ask atc at the gates of its ways through l
// (#752); kind is how it is called ("tug", "fuel truck").
func (y *vehicleYield) SetATC(atc VehicleATC, l *airport.Layout, kind string) {
	y.atc, y.layout, y.kind = atc, l, kind
}
