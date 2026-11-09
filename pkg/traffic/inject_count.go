package traffic

import (
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// CountNear counts the objects taken over (aircraft and ground vehicles
// alike) last placed within radiusM of at: what of ours the simulator
// draws there (ManagerOptions.MaxObjects).
func (i *Injector) CountNear(at airport.LatLon, radiusM float64) int {
	const mPerDeg = 111320.0
	cos := math.Cos(at.Lat * math.Pi / 180)
	i.mu.Lock()
	defer i.mu.Unlock()
	n := 0
	for _, o := range i.objects {
		if !o.taken || !o.rate.haveLast {
			continue
		}
		dn := (o.rate.last.Latitude - at.Lat) * mPerDeg
		de := (o.rate.last.Longitude - at.Lon) * mPerDeg * cos
		if math.Hypot(dn, de) <= radiusM {
			n++
		}
	}
	return n
}
