package worldtest

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/airspace"
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

	// Transits (#1485).
	h        *Harness
	transits map[string]bool
	moving   int            // tracks climbing or descending 2000 ft or more in the area
	bands    map[string]int // tracks by their level nearest the centre
	below250 int
	bumpy    []string // transits not monotone in the area, or steeper than the AI flies
	inView   []string // transits created or removed within 40 NM of the centre (tiers off)
	nTransit int

	// The focus airport's terminal airspace (#1486).
	stack      []*airspace.Shape
	stackNames []string
	groundM    float64
	inTMA      map[string]string
}

func newCrossingMetrics(h *Harness, centre airport.LatLon) *crossingMetrics {
	return &crossingMetrics{centre: centre, tracks: map[string][]crossingPoint{}, h: h, transits: map[string]bool{}, bands: map[string]int{}}
}

// sample takes an overflight in the simulator (each 5 s sample; its track
// once a minute, inside the area).
func (c *crossingMetrics) sample(ob Object, at time.Time, n int) {
	c.sampleTerminal(ob, at)
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
	c.finishTransits(m, names)
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

func (c *crossingMetrics) report(b *strings.Builder, m *overflightMetrics) {
	c.reportTransits(b)
	c.reportTerminal(b, m)
	fmt.Fprintf(b, "  tracks: %d crossing the area; the busiest line (%s's) has %d of them within %.0f NM (%.0f%%)\n",
		c.tracksInside, c.busiestOf, c.busiest, spreadNM, 100*c.busiestShare())
}

// transitBands are the level bands of the histogram.
var transitBands = []struct {
	name   string
	belowF float64
}{{"<FL100", 10000}, {"FL100-195", 20000}, {"FL200-245", 25000}, {"FL250-345", 35000}, {">=FL350", math.Inf(1)}}

// steepFpm: a track climbing or descending faster than this (a minute's
// samples) is steeper than the AI flies.
const steepFpm = 4500.0

func (c *crossingMetrics) finishTransits(m *overflightMetrics, names []string) {
	var v struct {
		Transits []string `json:"transitFlights"`
	}
	if err := c.h.World.Get("/api/schedule", &v); err == nil {
		for _, cs := range v.Transits {
			c.transits[cs] = true
		}
	}
	for _, cs := range names {
		tr := c.tracks[cs]
		lo, hi := math.Inf(1), math.Inf(-1)
		near, nearNM := tr[0], math.Inf(1)
		for _, p := range tr {
			lo, hi = math.Min(lo, p.altFt), math.Max(hi, p.altFt)
			if d := calc.HaversineNM(c.centre.Lat, c.centre.Lon, p.lat, p.lon); d < nearNM {
				near, nearNM = p, d
			}
		}
		if hi-lo >= 2000 {
			c.moving++
		}
		for _, b := range transitBands {
			if near.altFt < b.belowF {
				c.bands[b.name]++
				break
			}
		}
		if near.altFt < 25000 {
			c.below250++
		}
		if !c.transits[cs] {
			continue
		}
		c.nTransit++
		// Its plan's profile: a climb, its cruise, a descent (a short one
		// does all three across the area: LOT868 EDDB → LOWW, FL206 up to
		// FL370, down to FL273); no climb after a descent of 300 ft or
		// more, no minute steeper than steepFpm.
		down, dip := false, false
		for i := 1; i < len(tr); i++ {
			d := tr[i].altFt - tr[i-1].altFt
			if mins := tr[i].at.Sub(tr[i-1].at).Minutes(); mins > 0 && math.Abs(d)/mins > steepFpm {
				c.bumpy = append(c.bumpy, fmt.Sprintf("%s %.0f fpm at %s", cs, d/mins, tr[i].at.Format("15:04")))
				break
			}
			switch {
			case d > 300 && down:
				dip = true
			case d < -300:
				down = true
			}
			if dip {
				c.bumpy = append(c.bumpy, cs+" down and up again")
				break
			}
		}
	}
	if m.tiers != nil {
		return // created and removed by the tiers near the viewer
	}
	for _, p := range append(c.h.Sim.Creations(), c.h.Sim.Removals()...) {
		if c.transits[p.Tail] && calc.HaversineNM(c.centre.Lat, c.centre.Lon, p.Lat, p.Lon) < 40 {
			c.inView = append(c.inView, fmt.Sprintf("%s at %s %.0f NM", p.Tail, p.At.Format("15:04:05"), calc.HaversineNM(c.centre.Lat, c.centre.Lon, p.Lat, p.Lon)))
		}
	}
}

func (c *crossingMetrics) reportTransits(b *strings.Builder) {
	var bands []string
	for _, x := range transitBands {
		bands = append(bands, fmt.Sprintf("%s:%d", x.name, c.bands[x.name]))
	}
	share := 0.0
	if c.tracksInside > 0 {
		share = float64(c.moving) / float64(c.tracksInside)
	}
	fmt.Fprintf(b, "  transits: %d planned, %d crossing in the simulator; of all crossing tracks %d climbing or descending (%.0f%%), %d below FL250; levels nearest the centre %s\n",
		len(c.transits), c.nTransit, c.moving, 100*share, c.below250, strings.Join(bands, " "))
	fmt.Fprintf(b, "  transits not monotone or too steep: %v; created or removed in view: %v\n", c.bumpy, c.inView)
}

// The focus airport's terminal airspace (#1486): crossing traffic inside
// it below its ceiling, sampled every 5 s.

// bedStores: the airspaces opened, by data root.
var bedStores sync.Map

// bedAirspaces is the airspaces of the data at root (nil: none there).
func bedAirspaces(root string) *airspace.Store {
	if s, ok := bedStores.Load(root); ok {
		return s.(*airspace.Store)
	}
	dir := filepath.Join(root, "airspaces")
	if _, err := os.Stat(filepath.Join(dir, "index.json")); err != nil {
		return nil
	}
	s, err := airspace.Open(os.DirFS(dir), airspace.StoreSkipBroken(), airspace.StorePreferNewerSet())
	if err != nil {
		return nil
	}
	got, _ := bedStores.LoadOrStore(root, s)
	return got.(*airspace.Store)
}

// terminal takes the focus airport's terminal airspace from the data at
// root (none there: not looked at).
func (c *crossingMetrics) terminal(root string, fieldFt float64) {
	s := bedAirspaces(root)
	if s == nil {
		return
	}
	c.groundM = fieldFt * 0.3048
	c.stack, _ = s.TerminalStack(c.centre.Lat, c.centre.Lon, c.groundM, 24500*0.3048)
	c.inTMA = map[string]string{}
	for _, sh := range c.stack {
		c.stackNames = append(c.stackNames, fmt.Sprintf("%s to %.0f ft", sh.Airspace.Name, sh.Airspace.Ceiling(c.groundM)/0.3048))
	}
}

// sampleTerminal notes a crossing aircraft inside the terminal airspace.
func (c *crossingMetrics) sampleTerminal(ob Object, at time.Time) {
	if len(c.stack) == 0 || c.inTMA[ob.ATCID] != "" || calc.HaversineNM(c.centre.Lat, c.centre.Lon, ob.Lat, ob.Lon) > 60 {
		return
	}
	if airspace.InStack(c.stack, ob.Lat, ob.Lon, ob.AltFt*0.3048, c.groundM, 0) {
		c.inTMA[ob.ATCID] = fmt.Sprintf("%s at %s, %.0f ft %.0f NM out", ob.ATCID, at.Format("15:04:05"), ob.AltFt, calc.HaversineNM(c.centre.Lat, c.centre.Lon, ob.Lat, ob.Lon))
	}
}

// tmaList is who was inside, sorted.
func (c *crossingMetrics) tmaList(m *overflightMetrics) []string {
	var out []string
	for cs, s := range c.inTMA {
		if !m.shared[cs] {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func (c *crossingMetrics) reportTerminal(b *strings.Builder, m *overflightMetrics) {
	if len(c.stack) == 0 {
		return
	}
	fmt.Fprintf(b, "  terminal airspace %s: crossing traffic inside below its ceiling: %d %v\n", strings.Join(c.stackNames, ", "), len(c.tmaList(m)), c.tmaList(m))
}

// transitSet is the transits' call signs (GET /api/schedule).
func (c *crossingMetrics) transitSet() map[string]bool {
	var v struct {
		Transits []string `json:"transitFlights"`
	}
	out := map[string]bool{}
	if err := c.h.World.Get("/api/schedule", &v); err == nil {
		for _, cs := range v.Transits {
			out[cs] = true
		}
	}
	return out
}
