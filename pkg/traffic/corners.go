//go:build windows
// +build windows

package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Turns in the air: MSFS AI flies a waypoint chain by turning at each
// point, late and hard (live, up to 6.5°/s — some 45° of bank — on a
// go-around's crosswind turn at 180 kt). Rounding the chain's corners into
// fly-by arcs of the aircraft's standard turn — rate one (3°/s) up to its
// type's most bank — makes the turns as wide as its speed asks.

// The most bank of the standard turns by airframe: TurnBankDeg for jets,
// TurnBankTurbopropDeg for turboprops (and pistons), which fly slower and
// turn tighter.
var (
	TurnBankDeg          = 25.0
	TurnBankTurbopropDeg = 30.0
)

// MaxBankDeg is the most bank of p's standard turns.
func MaxBankDeg(p AircraftProfile) float64 {
	if p.Category == CategoryTurboprop || p.Category == CategoryPiston {
		return TurnBankTurbopropDeg
	}
	return TurnBankDeg
}

// StandardBankDeg is the bank of a standard turn at kts: rate one (3°/s,
// about a tenth of the speed plus 7°), at most maxBank.
func StandardBankDeg(kts, maxBank float64) float64 {
	return math.Min(maxBank, kts/10+7)
}

// turnRadiusMeters is the radius of a turn at kts and bankDeg: v²/(g·tanφ).
func turnRadiusMeters(kts, bankDeg float64) float64 {
	v := kts * 1852 / 3600
	return v * v / (9.81 * math.Tan(bankDeg*math.Pi/180))
}

// roundCorners replaces each corner of wps (not the first or the last
// point) turning more than 15° with a fly-by arc of the radius a standard
// turn (StandardBankDeg, at most maxBank) has at the corner's speed, as few points as keep the
// arc (one per 45° and its ends: MSFS AI circles points set too close).
// An arc never takes more than 45 % of either leg; on short legs it is
// tighter. Altitudes and speeds are the corner's.
func roundCorners(wps []types.SIMCONNECT_DATA_WAYPOINT, maxBank float64) []types.SIMCONNECT_DATA_WAYPOINT {
	if len(wps) < 3 {
		return wps
	}
	out := []types.SIMCONNECT_DATA_WAYPOINT{wps[0]}
	for i := 1; i < len(wps)-1; i++ {
		a, c, b := out[len(out)-1], wps[i], wps[i+1]
		in := calc.BearingDegrees(a.Latitude, a.Longitude, c.Latitude, c.Longitude)
		outHdg := calc.BearingDegrees(c.Latitude, c.Longitude, b.Latitude, b.Longitude)
		turn := math.Mod(outHdg-in+540, 360) - 180 // + right
		legIn := calc.HaversineMeters(a.Latitude, a.Longitude, c.Latitude, c.Longitude)
		legOut := calc.HaversineMeters(c.Latitude, c.Longitude, b.Latitude, b.Longitude)
		th := math.Abs(turn) * math.Pi / 180
		if math.Abs(turn) < 15 || math.Abs(turn) > 170 || legIn < 200 || legOut < 200 {
			out = append(out, c)
			continue
		}
		kts := math.Max(c.KtsSpeed, 140)
		r := turnRadiusMeters(kts, StandardBankDeg(kts, maxBank))
		d := r * math.Tan(th/2)
		if lim := 0.45 * math.Min(legIn, legOut); d > lim {
			d, r = lim, lim/math.Tan(th/2)
		}
		// The arc from where it leaves the inbound leg to where it joins the
		// outbound one, about its centre off to the side of the turn.
		t1lat, t1lon := calc.DisplaceByHeading(c.Latitude, c.Longitude, in+180, d)
		side := 90.0
		if turn < 0 {
			side = -90
		}
		clat, clon := calc.DisplaceByHeading(t1lat, t1lon, in+side, r)
		n := int(math.Ceil(math.Abs(turn)/45 - 0.05)) // (a right angle on the sphere is a hair over 90°)
		for k := 0; k <= n; k++ {
			brg := in - side + turn*float64(k)/float64(n) // centre → point
			lat, lon := calc.DisplaceByHeading(clat, clon, brg, r)
			p := c
			p.Latitude, p.Longitude = lat, lon
			out = append(out, p)
		}
	}
	return append(out, wps[len(wps)-1])
}

// roundedChain rounds the corners of an arrival's chain flown from here:
// up to the align point, not at it — the align and join points, the last
// two, stay as they are (AbsorbDelay, the holds and DirectToJoin keep to
// them).
func roundedChain(here airport.LatLon, wps []types.SIMCONNECT_DATA_WAYPOINT, maxBank float64) []types.SIMCONNECT_DATA_WAYPOINT {
	if len(wps) < 3 {
		return wps
	}
	head := append([]types.SIMCONNECT_DATA_WAYPOINT{{Latitude: here.Lat, Longitude: here.Lon, KtsSpeed: wps[0].KtsSpeed}}, wps[:len(wps)-1]...)
	return append(roundCorners(head, maxBank)[1:], wps[len(wps)-1])
}
