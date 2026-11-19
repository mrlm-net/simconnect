package worldtest

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// What the overflights' tracks look like on the test bed (#1484): each
// one's positions inside the area once a minute, and how many fly within
// 1 NM of the busiest line.

// spreadNM: two tracks this near are one line.
const spreadNM = 1.0

// spreadMinPoints: a track is on another's line with this many of its
// minutes within spreadNM of it (some 25 NM and more).
const spreadMinPoints = 4

type crossingPoint struct {
	at       time.Time
	lat, lon float64
	altFt    float64
}

type crossingMetrics struct {
	centre airport.LatLon
	tracks map[string][]crossingPoint
	// busiest: the most tracks on one line, and whose line it is.
	busiest      int
	busiestOf    string
	tracksInside int
}

func newCrossingMetrics(h *Harness, centre airport.LatLon) *crossingMetrics {
	return &crossingMetrics{centre: centre, tracks: map[string][]crossingPoint{}}
}

// sample takes an overflight in the simulator (each 5 s sample; its track
// once a minute, inside the area).
func (c *crossingMetrics) sample(ob Object, at time.Time, n int) {
	if n%12 != 0 || calc.HaversineNM(c.centre.Lat, c.centre.Lon, ob.Lat, ob.Lon) > 100 {
		return
	}
	c.tracks[ob.ATCID] = append(c.tracks[ob.ATCID], crossingPoint{at: at, lat: ob.Lat, lon: ob.Lon, altFt: ob.AltFt})
}

// nearTrack is how far p is from the polyline of tr (NM), the nearest of
// its legs (within each leg, else its ends).
func nearTrack(tr []crossingPoint, p crossingPoint) float64 {
	best := math.Inf(1)
	for i := 1; i < len(tr); i++ {
		a, b := tr[i-1], tr[i]
		leg := calc.HaversineNM(a.lat, a.lon, b.lat, b.lon)
		along := calc.AlongTrackMeters(a.lat, a.lon, b.lat, b.lon, p.lat, p.lon) / 1852
		var d float64
		switch {
		case leg < 0.1 || along < 0:
			d = calc.HaversineNM(a.lat, a.lon, p.lat, p.lon)
		case along > leg:
			d = calc.HaversineNM(b.lat, b.lon, p.lat, p.lon)
		default:
			d = math.Abs(calc.CrossTrackMeters(a.lat, a.lon, b.lat, b.lon, p.lat, p.lon)) / 1852
		}
		best = math.Min(best, d)
	}
	return best
}

func (c *crossingMetrics) finish(m *overflightMetrics) {
	var names []string
	for cs, tr := range c.tracks {
		if len(tr) >= spreadMinPoints && !m.shared[cs] {
			names = append(names, cs)
		}
	}
	sort.Strings(names)
	c.tracksInside = len(names)
	for _, a := range names {
		on := 1
		for _, b := range names {
			if a == b {
				continue
			}
			near := 0
			for _, p := range c.tracks[b] {
				if nearTrack(c.tracks[a], p) <= spreadNM {
					near++
				}
			}
			if near >= spreadMinPoints {
				on++
			}
		}
		if on > c.busiest {
			c.busiest, c.busiestOf = on, a
		}
	}
}

// busiestShare is the share of the tracks on the busiest line.
func (c *crossingMetrics) busiestShare() float64 {
	if c.tracksInside == 0 {
		return 0
	}
	return float64(c.busiest) / float64(c.tracksInside)
}

func (c *crossingMetrics) report(b *strings.Builder) {
	fmt.Fprintf(b, "  tracks: %d crossing the area; the busiest line (%s's) has %d of them within %.0f NM (%.0f%%)\n",
		c.tracksInside, c.busiestOf, c.busiest, spreadNM, 100*c.busiestShare())
}
