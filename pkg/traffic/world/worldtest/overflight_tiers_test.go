package worldtest

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// The overflights' distance tiers on the test bed (#1482): far from the
// viewer an overflight is a virtual track, flown by the World with no
// simulator object; within the materialise range it is created at the
// track's pose, beyond the dematerialise range taken out again. Measured
// second by second: how many are in the simulator at once against the cap,
// where each one appears against its virtual track, whether one ever flips
// at a range's edge, what they cost in SimConnect calls, and whether the
// ones in the simulator follow the viewer when it moves.

// overflightRun sets a run of the test bed apart from today's defaults.
type overflightRun struct {
	perHour                        float64 // overflights in the peak hour (0: the default)
	maxOver, maxVirtual            int     // MaxOverflights, MaxVirtualOverflights (0: the defaults)
	materialiseNM, dematerialiseNM float64 // the tiers (0: off)
	// moveAfter: the user aircraft is put moveNM east of where it is this
	// long after the start (0: never).
	moveAfter time.Duration
	moveNM    float64
	// pairsMeet: conflict-watch firings between two overflights are
	// reported, not failed: at 60 an hour today's level planning lets a
	// pair meet now and then, with the tiers off as well (the same pair,
	// the same seed); the tiers only change which are in the simulator.
	pairsMeet bool
}

// The budget of the overflights' SimConnect calls with the tiers on, per
// minute of traffic time, from the measurements at 60 an hour for 4 h at
// LKPR (the PR states them): the World's own calls (creations, removals,
// waypoint lists, data requests, positions, events) and the data the
// simulator sends for them (their entries in the area's scan).
const (
	tierCallsPerMin = 1.5
	tierDataPerMin  = 300.0
)

// Placement limits: an aircraft created for a virtual track appears this
// near its pose (#1482).
const (
	tierPlaceM   = 50.0
	tierPlaceDeg = 1.0
	// tierTurnDegPerSec: a standard rate turn; flown on virtually from
	// where it was taken out, a track may go on turning at it.
	tierTurnDegPerSec = 3.0
)

// overflightTierHours is the traffic time the tiers' run takes:
// WORLDTEST_OVERFLIGHT_HOURS, else 4.
func overflightTierHours() int {
	if n, err := strconv.Atoi(os.Getenv("WORLDTEST_OVERFLIGHT_HOURS")); err == nil && n > 0 {
		return n
	}
	return 4
}

// TestOverflightTiers runs the test bed with the tiers on at LKPR:
// 60 overflights an hour, in the simulator within 40 NM of the viewer, the
// user aircraft moved 60 NM east halfway through (clear of the hour, when
// the next hour's overflights are planned, so a seed replays); then a
// short run with
// every overflight in range (120 NM) and room for 4 in the simulator, the
// cap held.
func TestOverflightTiers(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World for hours of traffic time")
	}
	root := dataDir(t)
	if _, err := os.Stat(filepath.Join(root, "LKPR", "layout.json")); err != nil {
		t.Skipf("no offline data for LKPR in %s", root)
	}
	t.Run("LKPR", func(t *testing.T) {
		hours := overflightTierHours()
		run := overflightRun{perHour: 60, materialiseNM: 40, moveAfter: time.Duration(hours)*time.Hour/2 + 17*time.Minute, moveNM: 60, pairsMeet: true}
		m := runOverflights(t, root, "LKPR", hours, run)
		m.report(t)
		m.check(t)
		m.tiers.check(t, m, true)
	})
	t.Run("cap", func(t *testing.T) {
		run := overflightRun{perHour: 60, materialiseNM: 120, dematerialiseNM: 130, maxOver: 4, pairsMeet: true}
		m := runOverflights(t, root, "LKPR", 1, run)
		m.report(t)
		m.check(t)
		m.tiers.check(t, m, false)
		if m.peakOver != run.maxOver {
			t.Errorf("cap: peak %d overflights in the simulator, want the cap %d reached (all in range)", m.peakOver, run.maxOver)
		}
	})
}

// tierMetrics are what a run with the tiers on measured.
type tierMetrics struct {
	run         overflightRun
	matNM, deNM float64
	maxOver     int
	flown       map[string]bool // overflights flown, virtually or not (the board)

	// The last two samples of each virtual track, by call sign.
	last, prev map[string]trackSample
	everVirt   map[string]bool
	shared     map[string]bool // overflightMetrics.shared: also an airport's flight
	// Creations and removals looked at so far (Loopback.Creations, .Removals).
	nCreated, nRemoved int
	placed             []placeCheck
	// removedAt: an object removed, waiting for the next sample to say
	// whether its track flew on virtually.
	removedAt []Placed
	deCheck   []placeCheck // removals flown on virtually: the step to the track
	// Per call sign: the samples within the materialise range, entries into
	// it, and how long a breach of the tiers lasted.
	inside       map[string]bool
	entries      map[string]int
	lateIn       map[string]time.Time // virtual within the range, room in the simulator
	lateOut      map[string]time.Time // in the simulator beyond the dematerialise range
	breaches     []string
	peakVirtual  int
	sumVirtual   int
	samples      int
	moved        bool
	movedAt      time.Time
	oldViewer    airport.LatLon
	followed     []string // created after the move farther than deNM from the old viewer
	overflights  map[string]bool
	creates      map[string]int // creations of each overflight
	maxPlaceM    float64
	maxPlaceDeg  float64
	unsampled    int // flew virtually, created with no sample of it a second before
	bornNear     int // created in the second it was planned: near the viewer from the start
	maxDeStepM   float64
	maxDeStepDeg float64
	// apiSeen: the snapshot and the schedule looked at once, with virtual
	// tracks about; apiErr what they got wrong.
	apiSeen bool
	apiErr  []string
}

type trackSample struct {
	at                 time.Time
	lat, lon, alt, hdg float64
	kts                float64
}

// placeCheck is an aircraft created at (or a track flown on from) a pose,
// against where the other said it is.
type placeCheck struct {
	tail       string
	at         time.Time
	m, deg, nm float64 // off by; nm: from the viewer
	sampled    bool
	// flewVirtually: seen virtual before (not only in the second it was
	// planned, near the viewer from the start).
	flewVirtually bool
}

func newTierMetrics(run overflightRun) *tierMetrics {
	t := world.OverflightTiers{MaterialiseNM: run.materialiseNM, DematerialiseNM: run.dematerialiseNM}
	n := run.maxOver
	if n == 0 {
		n = 20
	}
	return &tierMetrics{run: run, matNM: t.MaterialiseNM, deNM: t.Dematerialise(), maxOver: n, flown: map[string]bool{},
		last: map[string]trackSample{}, prev: map[string]trackSample{}, everVirt: map[string]bool{}, inside: map[string]bool{},
		entries: map[string]int{}, lateIn: map[string]time.Time{}, lateOut: map[string]time.Time{}, creates: map[string]int{}}
}

// tierBreachFor: a track on the wrong side of a range longer than this is
// a breach (the tiers look each second; the World learns where the user
// aircraft is each second).
const tierBreachFor = 5 * time.Second

// sample takes the virtual tracks (as the map has them) and the objects
// now: the creations since the last sample against their tracks, the
// removals, the ranges.
func (x *tierMetrics) sample(t *testing.T, h *Harness) {
	now := h.Clock.Now()
	var list []world.Traffic
	if err := h.World.Get("/api/traffic", &list); err != nil {
		t.Fatal(err)
	}
	viewer := airport.LatLon{}
	objects := h.Sim.Objects()
	for _, o := range objects {
		if o.ID == UserObjectID {
			viewer = airport.LatLon{Lat: o.Lat, Lon: o.Lon}
		}
	}
	virt := map[string]world.Traffic{}
	for _, v := range list {
		if v.Virtual {
			virt[v.Tail] = v
			x.everVirt[v.Tail] = true
		}
	}
	// The creations since: where each appeared against its track's last
	// sample, flown on to the creation.
	cr := h.Sim.Creations()
	for _, c := range cr[x.nCreated:] {
		pc := placeCheck{tail: c.Tail, at: c.At, nm: calc.HaversineNM(viewer.Lat, viewer.Lon, c.Lat, c.Lon), flewVirtually: x.everVirt[c.Tail]}
		if s1, ok := x.last[c.Tail]; ok && c.At.Sub(s1.at) >= 0 && c.At.Sub(s1.at) <= 1500*time.Millisecond {
			lat, lon, hdg := extrapolate(x.prev[c.Tail], s1, c.At)
			pc.sampled = true
			pc.m = calc.HaversineMeters(lat, lon, c.Lat, c.Lon)
			pc.deg = math.Abs(math.Remainder(c.Heading-hdg, 360))
		}
		x.placed = append(x.placed, pc)
		if x.moved && calc.HaversineNM(x.oldViewer.Lat, x.oldViewer.Lon, c.Lat, c.Lon) > x.deNM {
			x.followed = append(x.followed, c.Tail)
		}
	}
	x.nCreated = len(cr)
	// The removals before: flown on virtually from where they were?
	for _, r := range x.removedAt {
		v, ok := virt[r.Tail]
		if !ok {
			continue // gone for good (done), or not a track
		}
		pc := placeCheck{tail: r.Tail, at: r.At, nm: calc.HaversineNM(viewer.Lat, viewer.Lon, r.Lat, r.Lon), sampled: true}
		// The object's last pose flown on to the sample at the track's speed.
		lat, lon := calc.DisplaceByHeading(r.Lat, r.Lon, r.Heading, v.GroundKts*1852/3600*now.Sub(r.At).Seconds())
		pc.m = calc.HaversineMeters(lat, lon, v.Latitude, v.Longitude)
		// Turning on as it turned, at no more than a standard rate.
		pc.deg = math.Max(0, math.Abs(math.Remainder(v.Heading-r.Heading, 360))-tierTurnDegPerSec*now.Sub(r.At).Seconds())
		x.deCheck = append(x.deCheck, pc)
	}
	rm := h.Sim.Removals()
	x.removedAt = append(x.removedAt[:0], rm[x.nRemoved:]...)
	x.nRemoved = len(rm)
	// The samples of the tracks, for the next creations.
	for cs, v := range virt {
		if s, ok := x.last[cs]; ok {
			x.prev[cs] = s
		} else {
			delete(x.prev, cs)
		}
		x.last[cs] = trackSample{at: now, lat: v.Latitude, lon: v.Longitude, alt: v.Alt, hdg: v.Heading, kts: v.GroundKts}
	}
	for cs := range x.last {
		if _, ok := virt[cs]; !ok {
			delete(x.last, cs) // in the simulator now, or gone: the next creation needs a fresh sample
			delete(x.prev, cs)
		}
	}
	if !x.apiSeen && len(virt) > 0 {
		x.apiSeen = x.checkAPI(t, h, virt)
	}
	x.samples++
	x.sumVirtual += len(virt)
	x.peakVirtual = max(x.peakVirtual, len(virt))
	// The ranges: who is on which side, and in the simulator or not.
	in := 0
	where := map[string]float64{}
	for _, o := range objects {
		if o.ID == UserObjectID || o.Vehicle || !x.isOverflight(o.ATCID) {
			continue
		}
		in++
		d := calc.HaversineNM(viewer.Lat, viewer.Lon, o.Lat, o.Lon)
		where[o.ATCID] = d
		x.breach(x.lateOut, o.ATCID, d > x.deNM+1, now, "in the simulator %.1f NM from the viewer (dematerialise %.0f NM)", d, x.deNM)
	}
	for cs, v := range virt {
		d := calc.HaversineNM(viewer.Lat, viewer.Lon, v.Latitude, v.Longitude)
		where[cs] = d
		x.breach(x.lateIn, cs, d < x.matNM-1 && in < x.maxOver, now, "virtual %.1f NM from the viewer with room in the simulator (materialise %.0f NM)", d, x.matNM)
	}
	for cs, d := range where {
		inside := d <= x.matNM
		if inside && !x.inside[cs] {
			x.entries[cs]++
		}
		x.inside[cs] = inside
	}
	for cs := range x.lateIn {
		if _, ok := virt[cs]; !ok {
			delete(x.lateIn, cs)
		}
	}
	for cs := range x.lateOut {
		if _, ok := where[cs]; !ok {
			delete(x.lateOut, cs)
		}
	}
}

// checkAPI: the snapshot has the same virtual tracks as the map, marked
// virtual, and the schedule says the tiers and how many are virtual.
// The World runs on meanwhile: what reads differently before and after
// (one materialised between the calls) is looked at again the next second;
// false then.
func (x *tierMetrics) checkAPI(t *testing.T, h *Harness, virt map[string]world.Traffic) bool {
	read := func() map[string]world.Traffic {
		out := map[string]world.Traffic{}
		for _, v := range h.World.Snapshot().Virtual {
			out[v.Tail] = v
		}
		return out
	}
	snap := read()
	var v struct {
		MaterialiseNM float64 `json:"overflightMaterialiseNM"`
		DeNM          float64 `json:"overflightDematerialiseNM"`
		Virtual       int     `json:"virtualOverflights"`
		MaxOver       int     `json:"maxOverflights"`
	}
	if err := h.World.Get("/api/schedule", &v); err != nil {
		t.Fatal(err)
	}
	if after := read(); len(after) != len(snap) || len(snap) != len(virt) {
		return false
	}
	for cs, s := range snap {
		if !s.Virtual || s.ObjectID == 0 {
			x.apiErr = append(x.apiErr, fmt.Sprintf("snapshot: %s not marked virtual or with no ID", cs))
		}
		if _, ok := virt[cs]; !ok {
			x.apiErr = append(x.apiErr, "snapshot: "+cs+", not on the map")
		}
	}
	if v.MaterialiseNM != x.matNM || v.DeNM != x.deNM || v.Virtual != len(virt) || v.MaxOver != x.maxOver {
		x.apiErr = append(x.apiErr, fmt.Sprintf("schedule: tiers %.0f/%.0f NM, %d virtual, max %d; want %.0f/%.0f NM, %d, %d",
			v.MaterialiseNM, v.DeNM, v.Virtual, v.MaxOver, x.matNM, x.deNM, len(virt), x.maxOver))
	}
	return true
}

// breach notes cs on the wrong side of a range (bad) since when, and a
// breach once it lasts tierBreachFor.
func (x *tierMetrics) breach(since map[string]time.Time, cs string, bad bool, now time.Time, format string, args ...any) {
	if !bad {
		delete(since, cs)
		return
	}
	at, ok := since[cs]
	if !ok {
		since[cs] = now
		return
	}
	if !at.IsZero() && now.Sub(at) >= tierBreachFor {
		x.breaches = append(x.breaches, fmt.Sprintf("%s %s: ", now.Format("15:04:05"), cs)+fmt.Sprintf(format, args...))
		since[cs] = time.Time{} // said once, until it is on the right side again
	}
}

// isOverflight: a track seen virtual, or an overflight on the board.
func (x *tierMetrics) isOverflight(cs string) bool {
	return (x.everVirt[cs] || x.flown[cs]) && !x.shared[cs]
}

// moveViewer puts the user aircraft run.moveNM east once run.moveAfter
// has passed.
func (x *tierMetrics) moveViewer(h *Harness, start time.Time) {
	if x.run.moveAfter <= 0 || x.moved || h.Clock.Now().Sub(start) < x.run.moveAfter {
		return
	}
	h.Sim.User(func(o *Object) {
		x.oldViewer = airport.LatLon{Lat: o.Lat, Lon: o.Lon}
		o.Lat, o.Lon = calc.DisplaceByHeading(o.Lat, o.Lon, 90, x.run.moveNM*1852)
	})
	x.moved, x.movedAt = true, h.Clock.Now()
}

// finish keeps the overflights' creations only (over: the board's).
func (x *tierMetrics) finish(over map[string]bool) {
	x.overflights = over
	keep := x.placed[:0]
	for _, p := range x.placed {
		if !over[p.tail] || x.shared[p.tail] {
			continue
		}
		keep = append(keep, p)
		x.creates[p.tail]++
		if !p.sampled {
			if p.flewVirtually {
				x.unsampled++
			} else {
				x.bornNear++
			}
			continue
		}
		x.maxPlaceM, x.maxPlaceDeg = math.Max(x.maxPlaceM, p.m), math.Max(x.maxPlaceDeg, p.deg)
	}
	x.placed = keep
	de := x.deCheck[:0]
	for _, p := range x.deCheck {
		if !x.shared[p.tail] {
			de = append(de, p)
		}
	}
	x.deCheck = de
	for _, p := range x.deCheck {
		x.maxDeStepM, x.maxDeStepDeg = math.Max(x.maxDeStepM, p.m), math.Max(x.maxDeStepDeg, p.deg)
	}
	f := x.followed[:0]
	for _, cs := range x.followed {
		if over[cs] {
			f = append(f, cs)
		}
	}
	x.followed = f
}

// extrapolate is where a track sampled at s0 then s1 is at t: on at its
// speed, turning at the rate it turned between them.
func extrapolate(s0, s1 trackSample, t time.Time) (lat, lon, hdg float64) {
	dt := t.Sub(s1.at).Seconds()
	rate := 0.0
	if gap := s1.at.Sub(s0.at).Seconds(); !s0.at.IsZero() && gap > 0 && gap <= 2 {
		rate = math.Remainder(s1.hdg-s0.hdg, 360) / gap
	}
	hdg = math.Mod(s1.hdg+rate*dt+360, 360)
	lat, lon = calc.DisplaceByHeading(s1.lat, s1.lon, s1.hdg+rate*dt/2, s1.kts*1852/3600*dt)
	return lat, lon, hdg
}

// callsPerMin are the overflights' own calls (the World's) and the data
// the simulator sent for them, per minute.
func (x *tierMetrics) callsPerMin(m *overflightMetrics) (calls, data float64) {
	for _, k := range []string{CallCreate, CallRemove, CallWaypoints, CallPosition, CallSetData, CallRequest, CallEvent} {
		calls += float64(m.callsOver[k])
	}
	return calls / m.minutes, float64(m.callsOver[CallData]) / m.minutes
}

func (x *tierMetrics) report(b *strings.Builder, m *overflightMetrics) {
	calls, data := x.callsPerMin(m)
	flips, most := 0, ""
	for cs, n := range x.creates {
		flips += n
		if most == "" || n > x.creates[most] || n == x.creates[most] && cs < most {
			most = cs
		}
	}
	fmt.Fprintf(b, "  tiers: in the simulator within %.0f NM of the viewer, virtual beyond %.0f NM, at most %d in it\n", x.matNM, x.deNM, x.maxOver)
	fmt.Fprintf(b, "    overflights flown %d; in the simulator at once peak %d mean %.1f; virtual at once peak %d mean %.1f\n",
		len(x.flown), m.peakOver, m.meanOver, x.peakVirtual, float64(x.sumVirtual)/float64(max(1, x.samples)))
	fmt.Fprintf(b, "    created in the simulator %d times (%d overflights, the most %s %d); planned near the viewer %d; flew virtually but no sample a second before %d\n", flips, len(x.creates), most, x.creates[most], x.bornNear, x.unsampled)
	fmt.Fprintf(b, "    placed against the track: at most %.1f m, %.2f°; flown on virtually after removal: at most %.1f m, %.2f° (%d)\n", x.maxPlaceM, x.maxPlaceDeg, x.maxDeStepM, x.maxDeStepDeg, len(x.deCheck))
	if len(x.shared) > 0 {
		fmt.Fprintf(b, "    left out, flown under the same call sign by an airport's flight too: %v\n", tierSorted(x.shared))
	}
	fmt.Fprintf(b, "    calls per minute: the World's %.2f (budget %.1f), data %.1f (budget %.0f)\n", calls, tierCallsPerMin, data, tierDataPerMin)
	if x.moved {
		fmt.Fprintf(b, "    viewer moved %.0f NM east at %s: %d created after it beyond %.0f NM of where it was %v\n", x.run.moveNM, x.movedAt.Format("15:04"), len(x.followed), x.deNM, x.followed)
	}
	fmt.Fprintf(b, "    tier breaches: %v\n", x.breaches)
}

// check: the cap held, every creation where its track is, none flipping at
// an edge, the calls within the budget, the tiers following the viewer.
func (x *tierMetrics) check(t *testing.T, m *overflightMetrics, budget bool) {
	t.Helper()
	if len(x.flown) == 0 {
		t.Fatalf("%s: no overflight flown", m.icao)
	}
	if m.peakOver > x.maxOver {
		t.Errorf("%s: %d overflights in the simulator at once, the cap is %d", m.icao, m.peakOver, x.maxOver)
	}
	if len(x.placed) == 0 {
		t.Errorf("%s: no overflight created in the simulator", m.icao)
	}
	for _, p := range x.placed {
		if p.sampled && (p.m > tierPlaceM || p.deg > tierPlaceDeg) {
			t.Errorf("%s %s at %s: created %.1f m and %.2f° off its track (at most %.0f m, %.0f°)", m.icao, p.tail, p.at.Format("15:04:05"), p.m, p.deg, tierPlaceM, tierPlaceDeg)
		}
		if p.nm > x.matNM+1 {
			t.Errorf("%s %s at %s: created %.1f NM from the viewer, beyond the materialise range %.0f NM", m.icao, p.tail, p.at.Format("15:04:05"), p.nm, x.matNM)
		}
	}
	if x.unsampled > 0 {
		t.Errorf("%s: %d of %d creations of a virtual track with no sample of it a second before", m.icao, x.unsampled, len(x.placed))
	}
	for _, p := range x.deCheck {
		if p.nm < x.deNM-1 {
			t.Errorf("%s %s at %s: taken out %.1f NM from the viewer, within the dematerialise range %.0f NM", m.icao, p.tail, p.at.Format("15:04:05"), p.nm, x.deNM)
		}
		if p.m > tierPlaceM || p.deg > tierPlaceDeg {
			t.Errorf("%s %s at %s: flown on virtually %.1f m and %.2f° (past a standard rate turn) off where it was", m.icao, p.tail, p.at.Format("15:04:05"), p.m, p.deg)
		}
	}
	// Once per crossing of the range at most.
	var cs []string
	for c := range x.creates {
		cs = append(cs, c)
	}
	sort.Strings(cs)
	for _, c := range cs {
		if x.creates[c] > x.entries[c] {
			t.Errorf("%s %s: created %d times, within the materialise range %d times", m.icao, c, x.creates[c], x.entries[c])
		}
	}
	for _, b := range x.breaches {
		t.Errorf("%s: %s", m.icao, b)
	}
	if !x.apiSeen {
		t.Errorf("%s: never a virtual track on the map, the snapshot and the schedule alike", m.icao)
	}
	for _, e := range x.apiErr {
		t.Errorf("%s: %s", m.icao, e)
	}
	if budget {
		calls, data := x.callsPerMin(m)
		if calls > tierCallsPerMin || data > tierDataPerMin {
			t.Errorf("%s: overflights' calls %.2f/min (budget %.1f), data %.1f/min (budget %.0f)", m.icao, calls, tierCallsPerMin, data, tierDataPerMin)
		}
	}
	if x.moved && x.movedAt.Before(m.hourly[len(m.hourly)-1].at) && len(x.followed) == 0 {
		t.Errorf("%s: no overflight created near the viewer after it moved %.0f NM", m.icao, x.run.moveNM)
	}
}

// tierSorted are the keys of m, sorted.
func tierSorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
