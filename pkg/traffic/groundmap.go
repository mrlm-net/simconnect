package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// groundMap is the ground elevation the simulator gave under our aircraft,
// by position, in cells of about a metre: an aircraft is placed on the
// ground where it goes, not where the last sample was taken. Placed on the
// sample a frame old, it rode a step in the runway mesh a frame late
// (live, every take-off on LKPR 24 at 76 kt: +0.31 ft for a frame).
type groundMap struct {
	cells map[groundCell]float32
}

type groundCell struct{ lat, lon int32 }

// groundCellDeg is a cell's size: about 1.1 m north–south.
const groundCellDeg = 1e-5

// groundMapMax: the cells kept at most; beyond it the map starts again (a
// long session over many airports).
const groundMapMax = 1 << 20

func cellOf(p airport.LatLon) groundCell {
	return groundCell{int32(math.Round(p.Lat / groundCellDeg)), int32(math.Round(p.Lon / groundCellDeg))}
}

// put records the ground elevation ft at p.
func (m *groundMap) put(p airport.LatLon, ft float64) {
	if p == (airport.LatLon{}) {
		return
	}
	if m.cells == nil || len(m.cells) >= groundMapMax {
		m.cells = make(map[groundCell]float32, 4096)
	}
	m.cells[cellOf(p)] = float32(ft)
}

// at is the ground elevation at p, when seen there.
func (m *groundMap) at(p airport.LatLon) (float64, bool) {
	v, ok := m.cells[cellOf(p)]
	return float64(v), ok
}

// groundUnder is the ground under o placed at p: as seen there before,
// else its last sample. i.mu held.
func (i *Injector) groundUnder(o *injected, p airport.LatLon) float64 {
	if ft, ok := i.ground.at(p); ok && math.Abs(ft-o.groundFt) < groundMapTrustFt {
		return ft
	}
	return o.groundFt
}

// groundMapTrustFt: a cell further than this from the aircraft's own
// sample is not taken (a bridge, a stale cell).
const groundMapTrustFt = 3.0
