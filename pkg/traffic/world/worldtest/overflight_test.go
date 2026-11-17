package worldtest

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// The overflights' test bed (#1481): the schedule on at an airport, its
// overflights crossing the 100 NM around it flown by SimlessAI on their
// waypoint chains for hours of traffic time, measured: how many enter per
// hour against the rate and waves, at which levels each way, how close
// they come to each other and to our arrivals and departures, the conflict
// watch's firings, jumps, and what they cost in SimConnect calls.
//
// overflightAirports: the test bed, a busy hub elsewhere, a quiet one far
// from both. An airport not in the offline data is skipped (CI has LKPR
// only, from the repository's own test data).
var overflightAirports = []string{"LKPR", "EGLL", "LZKZ"}

// overflightStart: 06:00 UTC, the morning wave at all three.
var overflightStart = time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)

// overflightTitles are the loopback's models and one of every type the
// schedule's airlines fly (long haul crosses the area too): no overflight
// cancelled for want of a model, as none is with traffic models installed.
func overflightTitles() []string {
	out := slices.Clone(DefaultAircraftTitles)
	seen := map[string]bool{}
	for _, a := range traffic.DefaultScheduleConfig().Airlines {
		for typ := range a.Fleet {
			title := "FSLTL_FAIB_" + typ + "_TEST"
			if a.Cargo {
				title += "_CARGO" // a cargo airline flies freighters only
			}
			if !seen[title] {
				seen[title] = true
				out = append(out, title)
			}
		}
	}
	sort.Strings(out[len(DefaultAircraftTitles):])
	return out
}

// overflightHours is the traffic time each airport runs:
// WORLDTEST_OVERFLIGHT_HOURS, else 2.
func overflightHours() int {
	if n, err := strconv.Atoi(os.Getenv("WORLDTEST_OVERFLIGHT_HOURS")); err == nil && n > 0 {
		return n
	}
	return 2
}

// TestOverflights runs the test bed at each airport and pins today's
// behaviour: the overflights planned each hour as the generator's rate
// and waves have them, no conflict-watch firing between two overflights,
// no jump of an overflight.
func TestOverflights(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World for hours of traffic time")
	}
	root := dataDir(t)
	for _, icao := range overflightAirports {
		t.Run(icao, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(root, icao, "layout.json")); err != nil {
				t.Skipf("no offline data for %s in %s", icao, root)
			}
			m := runOverflights(t, root, icao, overflightHours(), overflightRun{})
			m.report(t)
			m.check(t)
		})
	}
}

// overflightMetrics are what a run measured.
type overflightMetrics struct {
	// shared: call signs flown as an overflight and as a flight of an
	// airport too (hours apart); their objects and calls are not told
	// apart by tail, so they are left out of the costs and the tiers.
	shared     map[string]bool
	icao       string
	seed       uint64
	hours      int
	wall       time.Duration
	minutes    float64
	perHour    float64 // the rate asked, at density 1
	planned    map[string]boardFlight
	enter0     map[string]time.Time // when each was first planned to enter the area
	delayed    int                  // entered later than first planned (spawn retried)
	hourly     []overflightHour
	created    []string       // the overflights created in the simulator
	failed     int            // planned, due, never created
	levels     map[string]int // "FL370 E" → overflights
	wrongSide  []string       // flew a level of the other direction (semicircular rule)
	closestAll pairApproach   // any two airborne, within 1000 ft
	closestOO  pairApproach   // two overflights
	closestOX  pairApproach   // an overflight and another of ours
	lossesOO   []string       // pairs of overflights within 5 NM and 1000 ft (sampled)
	conflicts  map[string]int // conflict-watch firings involving an overflight, by pair "A|B"
	confOO     int            // of them between two overflights
	resolved   map[string]int // resolutions given to overflights, by kind
	sepLosses  []string       // the separation monitor's losses involving an overflight
	jumps      []Jump         // of overflights
	allJumps   map[string]int // of any aircraft, by placer: "world after ai"
	calls      map[string]int // all calls by kind
	callsOver  map[string]int // the overflights' calls by kind, summed
	tiers      *tierMetrics   // with the distance tiers on (#1482)
	peakAC     int
	meanAC     float64
	peakOver   int
	meanOver   float64
	fingerprnt string
}

// boardFlight is a flight on the schedule's board (GET /api/schedule).
type boardFlight struct {
	Callsign, Kind, Origin, Destination, Status, Note string
	Err                                               string `json:"error"`
	Enter, Exit                                       time.Time
}

type overflightHour struct {
	at                         time.Time
	mean                       float64 // the generator's mean: rate × density × wave
	planned, entered, complete int
}

type pairApproach struct {
	nm, ft float64
	a, b   string
	at     time.Time
}

func (p *pairApproach) offer(nm, ft float64, a, b string, at time.Time) {
	if p.a == "" || nm < p.nm {
		*p = pairApproach{nm, ft, a, b, at}
	}
}

func (p pairApproach) String() string {
	if p.a == "" {
		return "none"
	}
	return fmt.Sprintf("%.1f NM %.0f ft %s/%s at %s", p.nm, p.ft, p.a, p.b, p.at.Format("15:04:05"))
}

// overflightAirportRefs gives the World the airports of the schedule's
// table, for the overflights' ends, when the offline data has no
// airports.json (CI's).
func overflightAirportRefs(t *testing.T, h *Harness, root string) {
	if fileExists(filepath.Join(root, "airports.json")) {
		return
	}
	var refs []traffic.AirportRef
	for _, a := range traffic.DefaultScheduleConfig().Airports {
		refs = append(refs, traffic.AirportRef{ICAO: a.ICAO, Position: a.Position})
	}
	b, err := json.Marshal(refs)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "airports.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.World.LoadAirportsFile(p); err != nil {
		t.Fatal(err)
	}
}

func runOverflights(t *testing.T, root, icao string, hours int, run overflightRun) *overflightMetrics {
	wall := time.Now()
	o := Options{DataDir: root, ICAO: icao, Manual: true, FrameRate: 5, Start: overflightStart, AircraftTitles: overflightTitles()}
	if testing.Verbose() && os.Getenv("WORLDTEST_LOG") != "" {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	overflightAirportRefs(t, h, root)
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{icao}, Density: 1, MaxAircraft: 12, VFR: &no, Seed: h.Seed,
		OverflightsPerHour: run.perHour, MaxOverflights: run.maxOver, OverflightMaterialiseNM: run.materialiseNM, OverflightDematerialiseNM: run.dematerialiseNM,
		MaxVirtualOverflights: run.maxVirtual}); err != nil {
		t.Fatal(err)
	}
	m := &overflightMetrics{icao: icao, seed: h.Seed, hours: hours, planned: map[string]boardFlight{}, enter0: map[string]time.Time{},
		levels: map[string]int{}, conflicts: map[string]int{}, resolved: map[string]int{}, callsOver: map[string]int{}, allJumps: map[string]int{}, shared: map[string]bool{}}
	if run.materialiseNM > 0 {
		m.tiers = newTierMetrics(run)
		m.tiers.shared = m.shared
	}
	start := h.Clock.Now()
	end := start.Truncate(time.Hour).Add(time.Duration(hours) * time.Hour) // whole hours
	centre := airport.LatLon{Lat: h.Layout.Latitude, Lon: h.Layout.Longitude}

	over := map[string]bool{}  // the overflights' call signs
	wrong := map[string]bool{} // flew a level of the other direction
	flown := map[string]map[string]bool{}
	lastAlt := map[string]float64{} // at the sample before
	seenConf := map[string]bool{}
	seenRes := map[string]bool{}
	seenLoss := map[string]bool{}
	lossOO := map[string]bool{}
	samples, sumAC, sumOver := 0, 0, 0
	board := func() {
		var v struct {
			OverflightsPerHour float64       `json:"overflightsPerHour"`
			Flights            []boardFlight `json:"flights"`
		}
		if err := h.World.Get("/api/schedule", &v); err != nil {
			t.Fatal(err)
		}
		m.perHour = v.OverflightsPerHour
		for _, f := range v.Flights {
			if f.Kind == "overflight" {
				over[f.Callsign] = true
				if m.tiers != nil && (f.Status == "enroute" || f.Status == "done" && f.Err == "") {
					m.tiers.flown[f.Callsign] = true // flown, virtually or in the simulator
				}
				m.planned[f.Callsign] = f
				if _, ok := m.enter0[f.Callsign]; !ok {
					m.enter0[f.Callsign] = f.Enter
				}
			} else {
				m.shared[f.Callsign] = true // an overflight's as well, maybe (sorted out below)
			}
		}
	}
	for now := start; now.Before(end); now = h.Clock.Now() {
		if m.tiers != nil {
			// Second by second: where each virtual track is when it is
			// created in the simulator (tiers.go).
			for range 5 {
				h.Advance(time.Second)
				m.tiers.sample(t, h)
			}
			m.tiers.moveViewer(h, start)
		} else {
			h.Advance(5 * time.Second)
		}
		at := h.Clock.Now()
		if samples%12 == 0 {
			board()
		}
		samples++
		// Who is in the simulator, and how close.
		var air []Object
		nAC, nOver := 0, 0
		for _, ob := range h.Sim.Objects() {
			if ob.Vehicle || ob.ID == UserObjectID {
				continue
			}
			nAC++
			if over[ob.ATCID] && !m.shared[ob.ATCID] {
				nOver++
				// Its level, each way: level flight (within 20 ft of where it
				// was 5 s before) at a whole flight level, from FL100.
				prev, ok := lastAlt[ob.ATCID]
				lastAlt[ob.ATCID] = ob.AltFt
				if ok && math.Abs(ob.AltFt-prev) < 20 && ob.AltFt >= 9950 && math.Abs(ob.AltFt-math.Round(ob.AltFt/1000)*1000) < 50 {
					fl := int(math.Round(ob.AltFt/1000) * 10)
					east := math.Mod(ob.Heading+360, 360) < 180
					side := map[bool]string{true: "E", false: "W"}[east]
					if flown[ob.ATCID] == nil {
						flown[ob.ATCID] = map[string]bool{}
					}
					flown[ob.ATCID][fmt.Sprintf("FL%03d %s", fl, side)] = true
					if !semicircular(fl, east) {
						wrong[ob.ATCID] = true
					}
				}
			}
			if !ob.OnGround {
				air = append(air, ob)
			}
		}
		sumAC += nAC
		sumOver += nOver
		m.peakAC, m.peakOver = max(m.peakAC, nAC), max(m.peakOver, nOver)
		for i := range air {
			for j := i + 1; j < len(air); j++ {
				a, b := air[i], air[j]
				ft := math.Abs(a.AltFt - b.AltFt)
				if ft >= 1000 {
					continue
				}
				nm := calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
				m.closestAll.offer(nm, ft, a.ATCID, b.ATCID, at)
				switch oa, ob := over[a.ATCID], over[b.ATCID]; {
				case oa && ob:
					m.closestOO.offer(nm, ft, a.ATCID, b.ATCID, at)
					if nm < 5 {
						lossOO[pairKey(a.ATCID, b.ATCID)] = true
					}
				case oa || ob:
					m.closestOX.offer(nm, ft, a.ATCID, b.ATCID, at)
				}
			}
		}
		// The conflict watch and the separation monitor.
		var sep struct {
			Conflicts   []traffic.Conflict `json:"conflicts"`
			Resolutions []struct {
				At             time.Time
				Callsign, Kind string
			} `json:"resolutions"`
			Losses []struct {
				A, B  string
				Since time.Time
			} `json:"losses"`
		}
		if err := h.World.Get("/api/separation", &sep); err != nil {
			t.Fatal(err)
		}
		for _, c := range sep.Conflicts {
			if !over[c.A] && !over[c.B] {
				continue
			}
			k := pairKey(c.A, c.B)
			if !seenConf[k] {
				seenConf[k] = true
				m.conflicts[k]++
				if over[c.A] && over[c.B] {
					m.confOO++
				}
			}
		}
		for _, r := range sep.Resolutions {
			k := r.At.String() + r.Callsign
			if over[r.Callsign] && !seenRes[k] {
				seenRes[k] = true
				m.resolved[r.Kind]++
			}
		}
		for _, l := range sep.Losses {
			k := pairKey(l.A, l.B) + l.Since.String()
			if (over[l.A] || over[l.B]) && !seenLoss[k] {
				seenLoss[k] = true
				m.sepLosses = append(m.sepLosses, fmt.Sprintf("%s/%s at %s", l.A, l.B, l.Since.Format("15:04:05")))
			}
		}
	}
	board()
	m.minutes = h.Clock.Now().Sub(start).Minutes()
	m.meanAC, m.meanOver = float64(sumAC)/float64(samples), float64(sumOver)/float64(samples)
	for cs := range flown {
		for k := range flown[cs] {
			m.levels[k]++
		}
	}
	for cs := range wrong {
		m.wrongSide = append(m.wrongSide, cs)
	}
	sort.Strings(m.wrongSide)
	for k := range lossOO {
		m.lossesOO = append(m.lossesOO, k)
	}
	sort.Strings(m.lossesOO)

	// What they cost, and whether they jumped.
	m.calls = h.Sim.Calls()
	if m.tiers != nil {
		m.tiers.finish(over)
	}
	for cs := range m.shared {
		if !over[cs] {
			delete(m.shared, cs)
		}
	}
	for cs := range over {
		if m.shared[cs] {
			continue
		}
		c := h.Sim.CallsOf(cs)
		if c[CallCreate] > 0 || m.tiers != nil && m.tiers.flown[cs] {
			m.created = append(m.created, cs)
		}
		for k, n := range c {
			m.callsOver[k] += n
		}
	}
	sort.Strings(m.created)
	for _, j := range h.Sim.Jumps() {
		m.allJumps[j.By+" after "+j.From]++
		if over[j.Tail] && !m.shared[j.Tail] {
			m.jumps = append(m.jumps, j)
		}
	}

	// Per hour: planned (by the time each enters the area), entered (created).
	waves := traffic.DefaultScheduleConfig().Waves
	hf := fnv.New64a()
	var names []string
	for cs := range m.planned {
		names = append(names, cs)
	}
	sort.Strings(names)
	for _, cs := range names {
		f := m.planned[cs]
		fmt.Fprintf(hf, "%s %s %s %s;", cs, f.Origin, f.Destination, m.enter0[cs].Format(time.RFC3339))
		if f.Enter.After(m.enter0[cs]) {
			m.delayed++
		}
	}
	m.fingerprnt = strconv.FormatUint(hf.Sum64(), 16)
	for hr := start.Truncate(time.Hour); hr.Before(end); hr = hr.Add(time.Hour) {
		x := overflightHour{at: hr, mean: m.perHour * waves[traffic.LocalHour(icao, centre, hr)]}
		for _, cs := range names {
			f, enter := m.planned[cs], m.enter0[cs]
			if enter.Before(hr) || !enter.Before(hr.Add(time.Hour)) {
				continue
			}
			x.planned++
			if slices.Contains(m.created, cs) {
				x.entered++
			}
			if !f.Exit.After(end) {
				x.complete++
			}
		}
		m.hourly = append(m.hourly, x)
	}
	// Due (entered the area before the end) and never created.
	for _, cs := range names {
		if f := m.planned[cs]; f.Enter.Before(end.Add(-time.Minute)) && !slices.Contains(m.created, cs) {
			m.failed++
			t.Logf("%s %s → %s due %s, not created: %s %s %s", cs, f.Origin, f.Destination, f.Enter.Format("15:04"), f.Status, f.Err, f.Note)
		}
	}
	m.wall = time.Since(wall)
	return m
}

// semicircular reports whether flight level fl is one of the direction's
// (cruiseLevelFt's rule): odd for 000–179°, even for 180–359°, up to
// FL410; above it FL450 and FL490 east, FL430, FL470, FL510 west.
func semicircular(fl int, east bool) bool {
	n := fl / 10
	if fl <= 410 {
		return (n%2 == 1) == east
	}
	if east {
		return n == 45 || n == 49
	}
	return n == 43 || n == 47 || n == 51
}

func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

func (m *overflightMetrics) report(t *testing.T) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "overflights at %s: WORLDTEST_SEED=%d, %d h of traffic time in %v of wall time, planned %d (fingerprint %s)\n",
		m.icao, m.seed, m.hours, m.wall.Round(time.Second), len(m.planned), m.fingerprnt)
	fmt.Fprintf(&b, "  per hour (rate %.0f/h × wave): hour, mean, planned, entered\n", m.perHour)
	for _, x := range m.hourly {
		fmt.Fprintf(&b, "    %s  %.1f  %d  %d\n", x.at.Format("15:04"), x.mean, x.planned, x.entered)
	}
	fmt.Fprintf(&b, "  created %d, due and not created %d, entering later than first planned %d\n", len(m.created), m.failed, m.delayed)
	var names []string
	for cs := range m.planned {
		names = append(names, cs)
	}
	sort.Slice(names, func(i, j int) bool {
		x, y := m.enter0[names[i]], m.enter0[names[j]]
		return x.Before(y) || x.Equal(y) && names[i] < names[j]
	})
	for _, cs := range names {
		f := m.planned[cs]
		fmt.Fprintf(&b, "    %-8s %s → %s enters %s (planned %s) exits %s, %s\n", cs, f.Origin, f.Destination, f.Enter.Format("15:04"), m.enter0[cs].Format("15:04"), f.Exit.Format("15:04"), f.Status)
	}
	var lv []string
	for k, n := range m.levels {
		lv = append(lv, fmt.Sprintf("%s:%d", k, n))
	}
	sort.Strings(lv)
	fmt.Fprintf(&b, "  levels (overflights per level, E 000–179° / W 180–359°): %s\n", strings.Join(lv, " "))
	fmt.Fprintf(&b, "  semicircular rule broken by: %v\n", m.wrongSide)
	fmt.Fprintf(&b, "  closest within 1000 ft (sampled every 5 s): any two %v; two overflights %v; overflight and ours %v\n", m.closestAll, m.closestOO, m.closestOX)
	fmt.Fprintf(&b, "  overflight pairs within 5 NM and 1000 ft (sampled): %v\n", m.lossesOO)
	fmt.Fprintf(&b, "  conflict watch: %d pairs with an overflight (%d both overflights) %v; resolutions to overflights %v\n", len(m.conflicts), m.confOO, m.conflicts, m.resolved)
	fmt.Fprintf(&b, "  separation monitor losses with an overflight: %v\n", m.sepLosses)
	fmt.Fprintf(&b, "  jumps: %d of overflights; of any aircraft (by placer) %v\n", len(m.jumps), m.allJumps)
	for i, j := range m.jumps {
		if i == 10 {
			break
		}
		fmt.Fprintf(&b, "    %s %s %.1f m %.1f° in %v at %.0f kt, by %s after %s\n", j.At.Format("15:04:05.000"), j.Tail, j.Metres, j.TurnDeg, j.Dt, j.Kts, j.By, j.From)
	}
	fmt.Fprintf(&b, "  aircraft in the simulator: peak %d, mean %.1f; overflights peak %d, mean %.1f\n", m.peakAC, m.meanAC, m.peakOver, m.meanOver)
	kinds := []string{CallCreate, CallRemove, CallWaypoints, CallPosition, CallSetData, CallRequest, CallScan, CallEvent, CallData, CallAIMove}
	n := float64(max(1, len(m.created)))
	fmt.Fprintf(&b, "  calls: kind, all, all/min, overflights, per overflight, overflights/min\n")
	for _, k := range kinds {
		fmt.Fprintf(&b, "    %-9s %7d %8.1f %7d %8.1f %8.2f\n", k, m.calls[k], float64(m.calls[k])/m.minutes, m.callsOver[k], float64(m.callsOver[k])/n, float64(m.callsOver[k])/m.minutes)
	}
	if m.tiers != nil {
		m.tiers.report(&b, m)
	}
	t.Log(b.String())
}

// check pins today's behaviour (#1481).
func (m *overflightMetrics) check(t *testing.T) {
	t.Helper()
	if len(m.created) == 0 {
		t.Errorf("%s: no overflight created in %d h", m.icao, m.hours)
	}
	// The generator draws the floor or the ceiling of the mean each hour
	// (rate × density × the hour's wave), never more; a draw that finds no
	// pair of airports whose route crosses the area is dropped, more often
	// at an airport far from the busy pairs: half the mean at least.
	var planned int
	var mean float64
	for _, x := range m.hourly {
		if hi := int(math.Ceil(x.mean)); x.planned > hi {
			t.Errorf("%s %s: %d overflights planned, want %d at most (mean %.1f)", m.icao, x.at.Format("15:04"), x.planned, hi, x.mean)
		}
		planned += x.planned
		mean += x.mean
	}
	if float64(planned) < mean/2 {
		t.Errorf("%s: %d overflights planned in %d h, want %.0f at least (half the mean)", m.icao, planned, m.hours, mean/2)
	}
	if m.confOO > 0 && (m.tiers == nil || !m.tiers.run.pairsMeet) {
		t.Errorf("%s: %d conflict-watch firings between two overflights: %v", m.icao, m.confOO, m.conflicts)
	}
	if len(m.jumps) > 0 {
		t.Errorf("%s: %d jumps of overflights, the first %s %.1f m in %v at %.0f kt by %s", m.icao, len(m.jumps), m.jumps[0].Tail, m.jumps[0].Metres, m.jumps[0].Dt, m.jumps[0].Kts, m.jumps[0].By)
	}
}
