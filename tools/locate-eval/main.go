//go:build windows
// +build windows

// Command locate-eval measures airport.Locate on facility dumps: for each
// group of airports (a line of idents, the first the centre, the others
// around it) it samples positions at the airports near the centre — every
// stand and taxi node, runway centrelines and edges, the same jittered, and
// approach and departure corridors — and checks the airport Locate names,
// next to the plain nearest-reference-point method.
//
//	go run ./tools/locate-eval -data ../dumps/locate -groups ../dumps/locate-groups.txt
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

type sample struct {
	p      airport.LatLon
	q      airport.LocateQuery
	kind   string
	truth  []string // any of these is right
	source string
}

func main() {
	data := flag.String("data", "../dumps/locate", "folder of facility dumps (<ICAO>.json)")
	groupsFile := flag.String("groups", "../dumps/locate-groups.txt", "groups of idents, one per line, the centre first")
	near := flag.Float64("near", 5000, "sample the airports with geometry within this of the centre (m)")
	show := flag.Int("show", 40, "failures to list")
	flag.Parse()
	b, err := os.ReadFile(*groupsFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cache := map[string]*airport.Layout{}
	load := func(id string) *airport.Layout {
		if l, ok := cache[id]; ok {
			return l
		}
		var raw airport.RawAirport
		var l *airport.Layout
		if js, err := os.ReadFile(filepath.Join(*data, id+".json")); err == nil && json.Unmarshal(js, &raw) == nil {
			l, _ = airport.BuildLayout(raw)
		}
		cache[id] = l
		return l
	}
	type tally struct{ n, ok, base int }
	byKind := map[string]*tally{}
	var fails []string
	rng := rand.New(rand.NewPCG(1, 2))
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		ids := strings.Fields(line)
		var group []*airport.Layout
		for _, id := range ids {
			if l := load(id); l != nil {
				group = append(group, l)
			}
		}
		if len(group) == 0 || group[0].ICAO != ids[0] {
			continue
		}
		centre := group[0]
		for _, a := range group {
			if len(a.Runways) == 0 || dist(centre.Latitude, centre.Longitude, a.Latitude, a.Longitude) > *near {
				continue
			}
			record := func(s sample, got airport.Location, ok bool) {
				t := byKind[s.kind]
				if t == nil {
					t = &tally{}
					byKind[s.kind] = t
				}
				t.n++
				if ok && slices.Contains(s.truth, got.ICAO) {
					t.ok++
				} else if len(fails) < *show {
					fails = append(fails, fmt.Sprintf("%-14s at %s (%.6f,%.6f) want %v, got %s %s %q %.0f m (alt %v)",
						s.kind, s.source, s.p.Lat, s.p.Lon, s.truth, got.ICAO, got.Feature, got.Name, got.Meters, got.Alternatives))
				}
				if slices.Contains(s.truth, nearestRef(s.p, group)) {
					t.base++
				}
			}
			for _, s := range samples(a, group, rng) {
				got, ok := airport.Locate(s.q, group)
				record(s, got, ok)
			}
			// Whole flights through a Tracker: every point after the first
			// (the take-off point on the runway) counts.
			for _, fl := range flights(a, group, rng) {
				tr := airport.NewTracker()
				tr.SetDestination(fl.dest)
				for i, s := range fl.points {
					got, ok := tr.Update(s.q, group)
					if i > 0 || !s.q.OnGround {
						record(s, got, ok)
					}
				}
			}
		}
	}
	var kinds []string
	for k := range byKind {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	var all tally
	fmt.Printf("%-16s %8s %9s %9s\n", "kind", "samples", "Locate", "nearest")
	for _, k := range kinds {
		t := byKind[k]
		all.n, all.ok, all.base = all.n+t.n, all.ok+t.ok, all.base+t.base
		fmt.Printf("%-16s %8d %8.3f%% %8.3f%%\n", k, t.n, 100*float64(t.ok)/float64(t.n), 100*float64(t.base)/float64(t.n))
	}
	fmt.Printf("%-16s %8d %8.3f%% %8.3f%%\n", "all", all.n, 100*float64(all.ok)/float64(all.n), 100*float64(all.base)/float64(all.n))
	for _, f := range fails {
		fmt.Println("  ✗", f)
	}
}

// samples are the positions to check at airport a among group, with what
// counts as right: on a's surface, a (or, where other airports share that
// very surface, any of them); jittered off it, a unless another airport's
// surface is within 100 m (then contested, not sampled).
func samples(a *airport.Layout, group []*airport.Layout, rng *rand.Rand) []sample {
	var out []sample
	onSurface := func(p airport.LatLon) []string {
		var ids []string
		for _, l := range group {
			if loc, ok := airport.Locate(airport.LocateQuery{Position: p, OnGround: true}, []*airport.Layout{l}); ok && loc.Meters <= 0.5 && loc.Feature != airport.NearAirport {
				ids = append(ids, l.ICAO)
			}
		}
		return ids
	}
	clearOfOthers := func(p airport.LatLon, m float64) bool {
		for _, l := range group {
			if l == a {
				continue
			}
			if loc, ok := airport.Locate(airport.LocateQuery{Position: p, OnGround: true}, []*airport.Layout{l}); ok && loc.Meters < m {
				return false
			}
		}
		return true
	}
	ground := func(kind string, p airport.LatLon) {
		q := airport.LocateQuery{Position: p, OnGround: true}
		if t := onSurface(p); slices.Contains(t, a.ICAO) {
			out = append(out, sample{p, q, kind, t, a.ICAO})
		}
		j := offset(p, rng.Float64()*360, rng.Float64()*25)
		if clearOfOthers(j, 100) {
			out = append(out, sample{j, airport.LocateQuery{Position: j, OnGround: true}, kind + "~25m", []string{a.ICAO}, a.ICAO})
		}
	}
	for _, pk := range a.Parking {
		ground("stand", pk.Position)
	}
	for _, tp := range a.TaxiPoints {
		ground("taxi node", tp.Position)
	}
	for _, r := range a.Runways {
		length := dist(r.Primary.Threshold.Lat, r.Primary.Threshold.Lon, r.Secondary.Threshold.Lat, r.Secondary.Threshold.Lon)
		h := bearing(r.Primary.Threshold, r.Secondary.Threshold)
		for s := 0.0; s <= length; s += 100 {
			c := offset(r.Primary.Threshold, h, s)
			ground("runway", c)
			ground("runway edge", offset(c, h+90*sign(rng), math.Max(0, r.Width/2-1)))
		}
		aliases := sameRunway(a, r, group)
		for _, e := range [2]struct{ from, to airport.RunwayEnd }{{r.Primary, r.Secondary}, {r.Secondary, r.Primary}} {
			hd := bearing(e.from.Threshold, e.to.Threshold)
			for nm := 0.2; nm <= 10; nm += 0.4 {
				before := nm * 1852
				w := 150 + before*math.Tan(10*math.Pi/180)
				p := offset(offset(e.from.Threshold, hd+180, before), hd+90, (rng.Float64()*2-1)*0.25*w)
				ft := before / 0.3048 * math.Tan(3*math.Pi/180)
				out = append(out, sample{p, airport.LocateQuery{Position: p, HeightFt: ft, Track: hd + (rng.Float64()*2-1)*10, HasTrack: true, VerticalFpm: -700, RunwayMeters: 0.7 * r.Length}, "approach", aliases, a.ICAO + " " + e.from.Name})
				out = append(out, sample{p, airport.LocateQuery{Position: p, HeightFt: ft}, "approach no trk", aliases, a.ICAO + " " + e.from.Name})
			}
			for nm := 0.2; nm <= 5; nm += 0.4 {
				beyond := nm * 1852
				w := 150 + beyond*math.Tan(15*math.Pi/180)
				p := offset(offset(e.to.Threshold, hd, beyond), hd+90, (rng.Float64()*2-1)*0.25*w)
				ft := 500 + beyond/0.3048*math.Tan(8*math.Pi/180)
				out = append(out, sample{p, airport.LocateQuery{Position: p, HeightFt: ft, Track: hd + (rng.Float64()*2-1)*10, HasTrack: true, VerticalFpm: 1800, RunwayMeters: 0.7 * r.Length}, "departure", aliases, a.ICAO + " " + e.from.Name})
				out = append(out, sample{p, airport.LocateQuery{Position: p, HeightFt: ft}, "departure no trk", aliases, a.ICAO + " " + e.from.Name})
			}
		}
	}
	return out
}

func sign(rng *rand.Rand) float64 {
	if rng.IntN(2) == 0 {
		return -1
	}
	return 1
}

// nearestRef is the plain method: the airport whose reference point is nearest.
func nearestRef(p airport.LatLon, group []*airport.Layout) string {
	best, id := math.Inf(1), ""
	for _, l := range group {
		if d := dist(p.Lat, p.Lon, l.Latitude, l.Longitude); d < best {
			best, id = d, l.ICAO
		}
	}
	return id
}

func dist(lat1, lon1, lat2, lon2 float64) float64 {
	k := math.Cos((lat1+lat2)/2*math.Pi/180) * 111320
	return math.Hypot((lon2-lon1)*k, (lat2-lat1)*111320)
}

func bearing(a, b airport.LatLon) float64 {
	k := math.Cos((a.Lat + b.Lat) / 2 * math.Pi / 180)
	return math.Mod(math.Atan2((b.Lon-a.Lon)*k, b.Lat-a.Lat)*180/math.Pi+360, 360)
}

func offset(p airport.LatLon, hdg, m float64) airport.LatLon {
	r := hdg * math.Pi / 180
	return airport.LatLon{Lat: p.Lat + m*math.Cos(r)/111320, Lon: p.Lon + m*math.Sin(r)/(111320*math.Cos(p.Lat*math.Pi/180))}
}

// sameRunway is a and every airport of group with runway r too: the same
// runway line (within 3° and 250 m of it, overlapping it: scenery and stock data differ by up to 200 m), aliases of one
// field whose data differ a little, any of them right.
func sameRunway(a *airport.Layout, r airport.Runway, group []*airport.Layout) []string {
	h := bearing(r.Primary.Threshold, r.Secondary.Threshold)
	length := dist(r.Primary.Threshold.Lat, r.Primary.Threshold.Lon, r.Secondary.Threshold.Lat, r.Secondary.Threshold.Lon)
	// along and across r's line from its primary threshold, metres.
	coords := func(p airport.LatLon) (float64, float64) {
		d := dist(r.Primary.Threshold.Lat, r.Primary.Threshold.Lon, p.Lat, p.Lon)
		b := (bearing(r.Primary.Threshold, p) - h) * math.Pi / 180
		return d * math.Cos(b), math.Abs(d * math.Sin(b))
	}
	out := []string{a.ICAO}
	for _, l := range group {
		if l == a {
			continue
		}
		for _, o := range l.Runways {
			oh := bearing(o.Primary.Threshold, o.Secondary.Threshold)
			diff := math.Abs(math.Mod(oh-h+540, 180) - 0) // the line, either direction
			diff = math.Min(diff, 180-diff)
			a0, x0 := coords(o.Primary.Threshold)
			a1, x1 := coords(o.Secondary.Threshold)
			if diff <= 3 && x0 <= 250 && x1 <= 250 && math.Max(a0, a1) > 0 && math.Min(a0, a1) < length {
				out = append(out, l.ICAO)
				break
			}
		}
	}
	return out
}

// flight is a sequence of positions through a Tracker, with the
// destination it is given ("" none).
type flight struct {
	dest   string
	points []sample
}

// flights at a: for each runway end a departure (a point on the runway,
// then the climb-out by position only), an approach to a with a as the
// destination (by position only) and one with no destination (with track
// and vertical speed), each wandering up to a quarter of the corridor
// either side.
func flights(a *airport.Layout, group []*airport.Layout, rng *rand.Rand) []flight {
	var out []flight
	for _, r := range a.Runways {
		aliases := sameRunway(a, r, group)
		for _, e := range [2]struct{ from, to airport.RunwayEnd }{{r.Primary, r.Secondary}, {r.Secondary, r.Primary}} {
			hd := bearing(e.from.Threshold, e.to.Threshold)
			name := a.ICAO + " " + e.from.Name
			dep := flight{points: []sample{{e.from.Threshold, airport.LocateQuery{Position: e.from.Threshold, OnGround: true}, "flight dep", aliases, name}}}
			for nm := 0.2; nm <= 5; nm += 0.4 {
				beyond := nm * 1852
				w := 150 + beyond*math.Tan(15*math.Pi/180)
				p := offset(offset(e.to.Threshold, hd, beyond), hd+90, (rng.Float64()*2-1)*0.25*w)
				ft := 500 + beyond/0.3048*math.Tan(8*math.Pi/180)
				dep.points = append(dep.points, sample{p, airport.LocateQuery{Position: p, HeightFt: ft}, "flight dep", aliases, name})
			}
			toDest := flight{dest: a.ICAO}
			blind := flight{}
			for nm := 10.0; nm >= 0.2; nm -= 0.4 {
				before := nm * 1852
				w := 150 + before*math.Tan(10*math.Pi/180)
				p := offset(offset(e.from.Threshold, hd+180, before), hd+90, (rng.Float64()*2-1)*0.25*w)
				ft := before / 0.3048 * math.Tan(3*math.Pi/180)
				toDest.points = append(toDest.points, sample{p, airport.LocateQuery{Position: p, HeightFt: ft}, "flight arr dest", aliases, name})
				blind.points = append(blind.points, sample{p, airport.LocateQuery{Position: p, HeightFt: ft, Track: hd, HasTrack: true, VerticalFpm: -700}, "flight arr", aliases, name})
			}
			out = append(out, dep, toDest, blind)
		}
	}
	return out
}
