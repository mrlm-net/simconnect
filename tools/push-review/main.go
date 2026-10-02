//go:build windows
// +build windows

// push-review writes review overlays of pushbacks planned without and with
// the wide-turn rule (traffic.PushWideRadiusCost at -weight, kept only where
// it is the same push, smoother), for judging a change by eye on the
// airport map (?overlay=, served from <dump-dir>/review).
//
// For each airport (a facility dump, as in pkg/airport/testdata) it plans
// the push from every third stand to both ends of the longest runway both
// ways, and writes the stands whose push
// changes most (-top), and those named in -stands, as
// <ICAO>-<stand>-<runway>-w<weight×10>.geojson: the stand, the push, a tow,
// the start of the taxi and the target pose.
//
//	go run ./tools/push-review -weight 3 -stands "LKPR:C28,A3" -out C:/msfs-development/dumps/review/w
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	data := flag.String("data", "pkg/airport/testdata", "folder of facility dumps (<ICAO>.json)")
	airports := flag.String("airports", "LKPR,EDDF,EDDM,EGLL,LOWW,EHAM,LFPG,LROP,KJFK,LKTB", "airports to review")
	out := flag.String("out", "review", "folder for the overlays")
	weight := flag.Float64("weight", traffic.PushWideRadiusCost, "PushWideRadiusCost for the rule, against the rule off (PushTurnRadiusCost)")
	top := flag.Int("top", 3, "stands per airport whose push changes most")
	stands := flag.String("stands", "", `stands always shown: "LKPR:C28,A3;EDDF:A16"`)
	model := flag.String("model", "FSLTL_B738_RYR", "aircraft model")
	flag.Parse()
	named := map[string][]string{}
	for _, part := range strings.Split(*stands, ";") {
		if icao, list, ok := strings.Cut(part, ":"); ok {
			named[icao] = strings.Split(list, ",")
		}
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	before := traffic.PushTurnRadiusCost // the rule off: wide = tight
	for _, icao := range strings.Split(*airports, ",") {
		g, err := load(filepath.Join(*data, icao+".json"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", icao, err)
			continue
		}
		review(g, *model, before, *weight, *top, named[icao], *out)
	}
}

func load(file string) (*airport.Graph, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		return nil, err
	}
	return airport.BuildGraph(l)
}

type cand struct {
	stand int
	rwy   string
	d     float64
	a, b  traffic.PlannedPush
}

func review(g *airport.Graph, model string, before, after float64, top int, named []string, out string) {
	l := g.Layout
	var rw *airport.Runway
	for i := range l.Runways {
		if rw == nil || l.Runways[i].Length > rw.Length {
			rw = &l.Runways[i]
		}
	}
	wide := traffic.PushWideRadiusCost
	plan := func(stand int, rwy string, w float64) (traffic.PlannedPush, bool) {
		traffic.PushWideRadiusCost = w
		defer func() { traffic.PushWideRadiusCost = wide }()
		p, err := traffic.PlanPush(traffic.TaxiRequest{Graph: g, Parking: stand, Runway: rwy, Model: model})
		return p, err == nil
	}
	var cands []cand
	for i, st := range l.Parking {
		if ga(st.Type) {
			continue
		}
		want := slices.Contains(named, st.Label())
		if i%3 != 0 && !want {
			continue
		}
		for _, rwy := range []string{rw.Primary.Name, rw.Secondary.Name} {
			a, ok1 := plan(i, rwy, before)
			b, ok2 := plan(i, rwy, after)
			if !ok1 || !ok2 {
				continue
			}
			d := math.Max(differ(a.Push, b.Push), differ(b.Push, a.Push))
			if want {
				d += 1e6 // always shown
			}
			cands = append(cands, cand{i, rwy, d, a, b})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].d > cands[j].d })
	tag := func(w float64) string { return fmt.Sprintf("w%02.0f", w*10) }
	shown := 0
	for _, c := range cands {
		if shown >= top && c.d < 1e6 {
			break
		}
		st := l.Parking[c.stand]
		base := fmt.Sprintf("%s-%s-%s-", l.ICAO, st.Label(), c.rwy)
		write(filepath.Join(out, base+tag(before)+".geojson"), st, c.a)
		write(filepath.Join(out, base+tag(after)+".geojson"), st, c.b)
		fmt.Printf("%s%s/%s: pushes differ by %.0f m; push %.0f → %.0f m\n", base, tag(before), tag(after), math.Mod(c.d, 1e6), length(c.a.Push), length(c.b.Push))
		shown++
	}
}

func ga(t types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE) bool {
	switch t {
	case types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_SMALL,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_MEDIUM, types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_LARGE,
		types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_RAMP_GA_EXTRA:
		return true
	}
	return false
}

// differ is the farthest a point of a lies from the line b (meters).
func differ(a, b []airport.LatLon) float64 {
	worst := 0.0
	for _, p := range a {
		best := math.Inf(1)
		for i := 1; i < len(b); i++ {
			best = math.Min(best, segDist(p, b[i-1], b[i]))
		}
		worst = math.Max(worst, best)
	}
	return worst
}

func segDist(p, a, b airport.LatLon) float64 {
	ab := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
	if ab < 0.01 {
		return calc.HaversineMeters(a.Lat, a.Lon, p.Lat, p.Lon)
	}
	along := calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon)
	switch {
	case along <= 0:
		return calc.HaversineMeters(a.Lat, a.Lon, p.Lat, p.Lon)
	case along >= ab:
		return calc.HaversineMeters(b.Lat, b.Lon, p.Lat, p.Lon)
	}
	return math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon))
}

func length(pts []airport.LatLon) float64 {
	m := 0.0
	for i := 1; i < len(pts); i++ {
		m += calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
	}
	return m
}

func write(file string, st airport.Parking, p traffic.PlannedPush) {
	line := func(pts []airport.LatLon) [][2]float64 {
		out := make([][2]float64, len(pts))
		for i, q := range pts {
			out[i] = [2]float64{q.Lon, q.Lat}
		}
		return out
	}
	feature := func(geom map[string]any, props map[string]any) map[string]any {
		return map[string]any{"type": "Feature", "geometry": geom, "properties": props}
	}
	fs := []map[string]any{
		feature(map[string]any{"type": "Point", "coordinates": [2]float64{st.Position.Lon, st.Position.Lat}}, map[string]any{"kind": "stand", "label": st.Label(), "heading": st.Heading}),
		feature(map[string]any{"type": "LineString", "coordinates": line(p.Push)}, map[string]any{"kind": "push"}),
	}
	if len(p.Tow) > 1 {
		fs = append(fs, feature(map[string]any{"type": "LineString", "coordinates": line(p.Tow)}, map[string]any{"kind": "tow"}))
	}
	// The first 150 m of the taxi-out.
	var taxi []airport.LatLon
	run := 0.0
	for i, q := range p.Taxi {
		if i > 0 {
			run += calc.HaversineMeters(p.Taxi[i-1].Lat, p.Taxi[i-1].Lon, q.Lat, q.Lon)
		}
		taxi = append(taxi, q)
		if run > 150 {
			break
		}
	}
	if len(taxi) > 1 {
		fs = append(fs, feature(map[string]any{"type": "LineString", "coordinates": line(taxi)}, map[string]any{"kind": "taxi"}))
	}
	fs = append(fs, feature(map[string]any{"type": "Point", "coordinates": [2]float64{p.Pose.Lon, p.Pose.Lat}}, map[string]any{"kind": "pose", "heading": p.Heading}))
	b, _ := json.Marshal(map[string]any{"type": "FeatureCollection", "features": fs})
	if err := os.WriteFile(file, b, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
