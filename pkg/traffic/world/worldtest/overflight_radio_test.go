package worldtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// Two centre sectors of the bed (#1487), split at the airport's longitude:
// local stations as PUT /api/stations sets them (test names and
// frequencies, not real ones).
const (
	bedWest, bedWestFreq = "Bed Centre West", "127.000"
	bedEast, bedEastFreq = "Bed Centre East", "128.000"
)

func bedSectors(c airport.LatLon) []traffic.Station {
	box := func(w, e float64) []airport.LatLon {
		return []airport.LatLon{{Lat: c.Lat - 4, Lon: w}, {Lat: c.Lat + 4, Lon: w}, {Lat: c.Lat + 4, Lon: e}, {Lat: c.Lat - 4, Lon: e}}
	}
	return []traffic.Station{
		{Position: traffic.PosCenter, Name: bedWest, Freq: bedWestFreq, Area: box(c.Lon-6, c.Lon)},
		{Position: traffic.PosCenter, Name: bedEast, Freq: bedEastFreq, Area: box(c.Lon, c.Lon+6)},
	}
}

// TestOverflightRadio runs LKPR with 60 overflights an hour, the tiers on
// (virtual tracks talk too) and two centre sectors, COM1 on the west one
// (#1487): with the radio off the overflights say nothing; on, they check
// in on the west station entering the area there and are handed off
// leaving it, nothing on the east frequency (not tuned) nor on tower or
// approach, and no more than the cap a minute. Then a player staffs the
// west station: a descent, a heading and a direct-to given to an
// overflight in the simulator are read back and flown at once, with no
// jump.
func TestOverflightRadio(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World for an hour of traffic time")
	}
	root := dataDir(t)
	if _, err := os.Stat(filepath.Join(root, "LKPR", "layout.json")); err != nil {
		t.Skipf("no offline data for LKPR in %s", root)
	}
	o := Options{DataDir: root, ICAO: "LKPR", Manual: true, FrameRate: 5, Start: overflightStart, AircraftTitles: overflightTitles()}
	if testing.Verbose() && os.Getenv("WORLDTEST_LOG") != "" {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	overflightAirportRefs(t, h, root)
	centre := airport.LatLon{Lat: h.Layout.Latitude, Lon: h.Layout.Longitude}
	if _, err := h.World.Do("PUT", "/api/stations?icao=LKPR", bedSectors(centre)); err != nil {
		t.Fatal(err)
	}
	h.Sim.User(func(u *Object) { u.COM1 = 127.0 })
	no := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 1, MaxAircraft: 12, VFR: &no, Seed: h.Seed,
		OverflightsPerHour: 60, OverflightMaterialiseNM: 40}); err != nil {
		t.Fatal(err)
	}
	over, shared := map[string]bool{}, map[string]bool{}
	board := func() {
		var v struct {
			Flights []boardFlight `json:"flights"`
		}
		if err := h.World.Get("/api/schedule", &v); err != nil {
			t.Fatal(err)
		}
		for _, f := range v.Flights {
			if f.Kind == "overflight" {
				over[f.Callsign] = true
			} else {
				shared[f.Callsign] = true // an arrival or departure as well, maybe
			}
		}
		for cs := range shared {
			delete(over, cs)
		}
	}
	heard := map[string]traffic.Transmission{}
	listen := func() {
		var list []traffic.Transmission
		if err := h.World.Get("/api/radio?icao=LKPR&n=400", &list); err != nil {
			t.Fatal(err)
		}
		for _, x := range list {
			heard[x.At.String()+x.Callsign+x.Text] = x
		}
	}
	run := func(d time.Duration) {
		for end := h.Clock.Now().Add(d); h.Clock.Now().Before(end); {
			h.Advance(5 * time.Second)
			if h.Clock.Now().Second() < 5 {
				board()
				listen()
			}
		}
	}

	// Off: silent.
	run(15 * time.Minute)
	for _, x := range heard {
		if over[x.Callsign] {
			t.Errorf("radio off: %s said %q", x.Callsign, x.Text)
		}
	}
	quiet := len(over)

	// On.
	if _, err := h.World.Do("POST", "/api/schedule", map[string]any{"overflightRadio": true}); err != nil {
		t.Fatal(err)
	}
	onAt := h.Clock.Now()
	run(45 * time.Minute)
	var checkIns, handoffs, onEast, wrongFreq int
	perMin := map[string][]time.Time{}
	var sample []string
	for _, x := range heard {
		if !over[x.Callsign] || x.At.Before(onAt) {
			continue
		}
		switch x.Frequency {
		case bedWestFreq:
			perMin[x.Frequency] = append(perMin[x.Frequency], x.At)
		case bedEastFreq:
			onEast++
		default:
			wrongFreq++
			t.Errorf("%s on %s (%s): %q", x.Callsign, x.Frequency, x.Position, x.Text)
		}
		switch {
		case x.Pilot && x.Intent == traffic.IntentCheckIn:
			checkIns++
		case !x.Pilot && x.Intent == traffic.IntentContact:
			handoffs++
		}
		if len(sample) < 6 {
			sample = append(sample, x.Text)
		}
	}
	peak := 0
	for _, at := range perMin[bedWestFreq] {
		n := 0
		for _, b := range perMin[bedWestFreq] {
			if !b.Before(at) && b.Sub(at) < time.Minute {
				n++
			}
		}
		peak = max(peak, n)
	}
	t.Logf("overflights: %d planned (%d by the end of the quiet part); on the west frequency %d transmissions, %d check-ins, %d hand-offs, peak %d a minute; east %d, elsewhere %d; e.g. %q",
		len(over), quiet, len(perMin[bedWestFreq]), checkIns, handoffs, peak, onEast, wrongFreq, sample)
	if checkIns == 0 || handoffs == 0 {
		t.Errorf("%d check-ins and %d hand-offs on the west frequency: want both", checkIns, handoffs)
	}
	// The cap counts at the call; a readback the radio delays behind
	// another transmission may fall a moment later.
	if peak > centreCap+1 {
		t.Errorf("peak %d transmissions a minute on the west frequency, cap %d", peak, centreCap)
	}
	if onEast > 0 {
		t.Errorf("%d transmissions on the east frequency, not tuned", onEast)
	}

	// Staffed: the player's instructions, the centre's two stations
	// staffed once one is in the simulator (none before: refused).
	staffedInstructions(t, h, centre, over)
}

// centreCap is the default cap (OverflightTraffic.CentreCallsPerMinute).
const centreCap = 6

// staffedInstructions gives an overflight in the simulator over the west
// sector a descent, then a heading, then a direct-to, each read back and
// flown at once, with no jump.
func staffedInstructions(t *testing.T, h *Harness, centre airport.LatLon, over map[string]bool) {
	t.Helper()
	pick := func() (Object, bool) {
		for _, ob := range h.Sim.Objects() {
			if !over[ob.ATCID] || ob.OnGround || ob.Lon > centre.Lon || ob.AltFt < 24000 || math.Abs(ob.VSFpm) > 200 {
				continue
			}
			if calc.HaversineNM(centre.Lat, centre.Lon, ob.Lat, ob.Lon) < 35 {
				return ob, true
			}
		}
		return Object{}, false
	}
	var ob Object
	if !h.Until(30*time.Minute, func() bool {
		var ok bool
		ob, ok = pick()
		return ok
	}) {
		t.Fatal("no overflight in the simulator over the west sector in 30 minutes")
	}
	cs := ob.ATCID
	if _, err := h.World.InstructEnroute(world.EnrouteInstruction{Callsign: cs, AltFt: 30000}); !errors.Is(err, world.ErrNotStaffed) {
		t.Errorf("%s: an instruction with no station staffed: %v, want %v", cs, err, world.ErrNotStaffed)
	}
	h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "center", Mode: world.ModeSilent}})
	find := func() Object {
		for _, x := range h.Sim.Objects() {
			if x.ATCID == cs {
				return x
			}
		}
		return Object{}
	}
	// The viewer goes with it: it stays in the simulator (the tiers).
	fly := func(d time.Duration) {
		for end := h.Clock.Now().Add(d); h.Clock.Now().Before(end); {
			if x := find(); x.ATCID != "" {
				h.Sim.User(func(u *Object) { u.Lat, u.Lon = x.Lat, x.Lon })
			}
			h.Advance(5 * time.Second)
		}
	}
	give := func(in world.EnrouteInstruction) traffic.Transmission {
		t.Helper()
		in.Callsign = cs
		wps := h.Sim.CallsOf(cs)[CallWaypoints]
		var tx traffic.Transmission
		b, err := h.World.Do("POST", "/api/enroute/instruct", in)
		if err != nil {
			t.Fatalf("%s: %v %s", cs, err, b)
		}
		if err := json.Unmarshal(b, &tx); err != nil {
			t.Fatal(err)
		}
		fly(5 * time.Second)
		if got := h.Sim.CallsOf(cs)[CallWaypoints]; got != wps+1 {
			t.Errorf("%s: %q: %d waypoint lists set, want 1", cs, tx.Text, got-wps)
		}
		readback := false
		h.Until(30*time.Second, func() bool {
			var list []traffic.Transmission
			_ = h.World.Get("/api/radio?icao=LKPR&n=40", &list)
			for _, x := range list {
				if x.Pilot && x.Callsign == cs && !x.At.Before(tx.At) && x.Frequency == tx.Frequency {
					readback = true
				}
			}
			return readback
		})
		if !readback {
			t.Errorf("%s: %q not read back on its frequency", cs, tx.Text)
		}
		t.Logf("%s: %q (on %s)", cs, tx.Text, tx.Frequency)
		return tx
	}
	// A descent of 2000 ft.
	to := math.Round(ob.AltFt/1000)*1000 - 2000
	tx := give(world.EnrouteInstruction{AltFt: to})
	if !strings.Contains(tx.Text, "descend") {
		t.Errorf("%s: %q: no descent said", cs, tx.Text)
	}
	fly(3 * time.Minute)
	if now := find(); math.Abs(now.AltFt-to) > 300 {
		t.Errorf("%s: at %.0f ft 3 minutes after the descent to %.0f ft", cs, now.AltFt, to)
	}
	// A heading 30° right.
	hdg := math.Mod(find().Heading+30, 360)
	give(world.EnrouteInstruction{Heading: true, HeadingDeg: hdg})
	fly(90 * time.Second)
	if now := find(); math.Abs(math.Mod(now.Heading-hdg+540, 360)-180) > 5 {
		t.Errorf("%s: heading %.0f 90 s after heading %.0f", cs, now.Heading, hdg)
	}
	// Direct to a point 30 NM ahead, 10 NM left of its way.
	now := find()
	lat, lon := calc.DisplaceByHeading(now.Lat, now.Lon, now.Heading-50, 32*1852)
	give(world.EnrouteInstruction{Direct: &airport.LatLon{Lat: lat, Lon: lon}})
	fly(2 * time.Minute)
	if x := find(); x.ATCID != "" {
		want := calc.BearingDegrees(x.Lat, x.Lon, lat, lon)
		if d := math.Abs(math.Mod(x.Heading-want+540, 360) - 180); d > 10 {
			t.Errorf("%s: heading %.0f, the point given bears %.0f", cs, x.Heading, want)
		}
	}
	var jumps []string
	for _, j := range h.Sim.Jumps() {
		if j.Tail == cs {
			jumps = append(jumps, fmt.Sprint(j))
		}
	}
	if len(jumps) > 0 {
		t.Errorf("%s: jumps: %v", cs, jumps)
	}
}
