// Command smoothness-report turns the traffic smoothness benchmarks
// (pkg/traffic/smoothness_bench_test.go, `go test -bench` output) and the
// recorded history of earlier runs into SVG bar charts for
// docs/traffic-smoothness.md. Usage:
//
//	smoothness-report -bench bench.txt -history docs/images/smoothness/history.csv -out docs/images/smoothness
//
// The history CSV has one row per measured step: chart,label,value,unit.
// Each chart gets the history's bars and a last "now" bar from -bench
// (where the chart names a benchmark). No third-party packages.
package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"html"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// charts: the charts drawn, their title, the benchmark measured now (""
// none: history only) and its metric (ns/op shown as ms, B/op as MB).
var charts = []struct {
	name, title, bench, metric, unit string
	scale                           float64
}{
	{"ground-checks", "Traffic checks per aircraft per frame", "BenchmarkGroundChecksFrame", "ns/op", "ms", 1e-6},
	{"push-plan-time", "Pushback plan (C22, one departure)", "BenchmarkPushPlan", "ns/op", "ms", 1e-6},
	{"push-plan-alloc", "Pushback plan garbage", "BenchmarkPushPlan", "B/op", "MB", 1e-6},
	{"spawn", "Arrival controller start (on the connection's goroutine)", "BenchmarkSpawnArrivalStart", "ns/op", "ms", 1e-6},
	{"stand-pick", "Arrival stand pick (on the connection's goroutine)", "BenchmarkAssignStandArrival", "ns/op", "ms", 1e-6},
	{"live-jumps", "Frame jumps of moving traffic, per 10 min (live, LKPR)", "", "", "jumps", 1},
}

type bar struct {
	label string
	value float64
}

func main() {
	benchFile := flag.String("bench", "", "go test -bench output")
	history := flag.String("history", "docs/images/smoothness/history.csv", "measured history CSV")
	out := flag.String("out", "docs/images/smoothness", "where the SVGs go")
	flag.Parse()

	now := map[string]map[string]float64{} // benchmark → metric → value
	if *benchFile != "" {
		var err error
		if now, err = readBench(*benchFile); err != nil {
			log.Fatal(err)
		}
	}
	hist, err := readHistory(*history)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, c := range charts {
		bars := hist[c.name]
		if v, ok := now[c.bench][c.metric]; ok {
			bars = append(bars, bar{"now", v * c.scale})
		}
		if len(bars) == 0 {
			continue
		}
		path := filepath.Join(*out, c.name+".svg")
		if err := os.WriteFile(path, []byte(svg(c.title, c.unit, bars)), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("wrote", path)
	}
}

// readBench reads `go test -bench -benchmem` lines: name, runs, then value
// unit pairs.
func readBench(path string) (map[string]map[string]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]map[string]float64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 4 || !strings.HasPrefix(fs[0], "Benchmark") {
			continue
		}
		name := fs[0]
		if i := strings.LastIndex(name, "-"); i > 0 {
			name = name[:i] // the GOMAXPROCS suffix
		}
		m := map[string]float64{}
		for i := 2; i+1 < len(fs); i += 2 {
			if v, err := strconv.ParseFloat(fs[i], 64); err == nil {
				m[fs[i+1]] = v
			}
		}
		out[name] = m
	}
	return out, sc.Err()
}

func readHistory(path string) (map[string][]bar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	out := map[string][]bar{}
	for i, r := range rows {
		if i == 0 || len(r) < 3 {
			continue // the header
		}
		v, err := strconv.ParseFloat(r[2], 64)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		out[r[0]] = append(out[r[0]], bar{r[1], v})
	}
	return out, nil
}

// svg is a horizontal bar chart: one bar per step, its value at its end;
// light and dark backgrounds both (no fill behind the chart).
func svg(title, unit string, bars []bar) string {
	const (
		w, left, right, top, rowH, barH = 720.0, 230.0, 90.0, 44.0, 34.0, 22.0
	)
	h := top + rowH*float64(len(bars)) + 16
	max := 0.0
	for _, b := range bars {
		max = math.Max(max, b.value)
	}
	if max == 0 {
		max = 1
	}
	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="system-ui, sans-serif" font-size="13">`+"\n", w, h, w, h)
	fmt.Fprintf(&s, `<text x="0" y="20" font-size="15" font-weight="600" fill="#6b7280">%s</text>`+"\n", html.EscapeString(title))
	for i, b := range bars {
		y := top + rowH*float64(i)
		bw := (w - left - right) * b.value / max
		color := "#9ca3af"
		if i == len(bars)-1 {
			color = "#2563eb" // the latest
		}
		fmt.Fprintf(&s, `<text x="%.0f" y="%.1f" text-anchor="end" fill="#6b7280">%s</text>`+"\n", left-10, y+barH/2+4, html.EscapeString(b.label))
		fmt.Fprintf(&s, `<rect x="%.0f" y="%.1f" width="%.1f" height="%.0f" rx="3" fill="%s"/>`+"\n", left, y, math.Max(bw, 1), barH, color)
		fmt.Fprintf(&s, `<text x="%.1f" y="%.1f" fill="#6b7280">%s %s</text>`+"\n", left+math.Max(bw, 1)+8, y+barH/2+4, num(b.value), unit)
	}
	s.WriteString("</svg>\n")
	return s.String()
}

func num(v float64) string {
	switch {
	case v >= 100:
		return strconv.FormatFloat(v, 'f', 0, 64)
	case v >= 10:
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}
