package worldtest

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// enrouteToldCS is the arrival of the en route tests: due at LKPR 50
// minutes on, it appears en route about 5 minutes on and flies some 20
// minutes to its STAR entry; enrouteBehindCS follows it on the same way,
// due 6 minutes later.
const (
	enrouteToldCS   = "RYR1419"
	enrouteBehindCS = "RYR1420"
)

// startEnrouteTold starts LKPR with approach staffed or not, and the two
// arrivals; it returns once the first flies en route, its object.
func startEnrouteTold(t *testing.T, staffed bool) (*Harness, *lockedBuf, Object) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the World")
	}
	log := &lockedBuf{}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	var out io.Writer = log
	if testing.Verbose() {
		out = io.MultiWriter(log, os.Stderr)
	}
	o.World.Output = out
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	if staffed {
		h.World.SetStaffed([]world.StaffedStation{{ICAO: "LKPR", Position: "approach"}})
	}
	off := false
	if err := h.World.SetSchedule(world.ScheduleSettings{Enabled: true, Airports: []string{"LKPR"}, Density: 3, MaxAircraft: 10, Generator: &off}); err != nil {
		t.Fatal(err)
	}
	sta := h.Clock.Now().Add(50 * time.Minute)
	if err := h.World.AddFlights([]traffic.Flight{
		{Callsign: enrouteToldCS, Airline: "RYR", Type: "B738", Origin: "EPWA", Destination: "LKPR", STD: sta.Add(-70 * time.Minute), STA: sta, DistanceNM: 339},
		{Callsign: enrouteBehindCS, Airline: "RYR", Type: "B738", Origin: "EPWA", Destination: "LKPR", STD: sta.Add(-64 * time.Minute), STA: sta.Add(6 * time.Minute), DistanceNM: 339},
	}); err != nil {
		t.Fatal(err)
	}
	var obj Object
	if !h.Until(15*time.Minute, func() bool {
		if enrouteStatus(t, h) != "enroute" {
			return false
		}
		var ok bool
		obj, ok = enrouteObject(h)
		return ok && !obj.OnGround
	}) {
		t.Fatalf("%s never en route (status %q)", enrouteToldCS, enrouteStatus(t, h))
	}
	if _, ok := h.Aircraft(enrouteToldCS); ok {
		t.Fatalf("%s en route has an arrival controller already", enrouteToldCS)
	}
	return h, log, obj
}

// enrouteStatus is the arrival's status on the schedule's board.
func enrouteStatus(t *testing.T, h *Harness) string {
	var v struct {
		Flights []boardFlight `json:"flights"`
	}
	if err := h.World.Get("/api/schedule", &v); err != nil {
		t.Fatal(err)
	}
	for _, f := range v.Flights {
		if f.Callsign == enrouteToldCS {
			return f.Status
		}
	}
	return ""
}

// enrouteObject is the arrival's aircraft in the loopback.
func enrouteObject(h *Harness) (Object, bool) {
	for _, o := range h.Sim.Objects() {
		if o.ATCID == enrouteToldCS && !o.Vehicle {
			return o, true
		}
	}
	return Object{}, false
}

// sequenceNumber is cs's number in LKPR's landing sequence, 0 none.
func sequenceNumber(h *Harness, cs string) int {
	var v []struct {
		Sequence []traffic.SequenceEntry `json:"sequence"`
	}
	if err := h.World.Get("/api/sequence?icao=LKPR", &v); err != nil {
		return 0
	}
	for _, r := range v {
		for _, e := range r.Sequence {
			if e.Callsign == cs {
				return e.Number
			}
		}
	}
	return 0
}

// enrouteJumps flies d second by second and reports each step of the
// arrival's aircraft no aircraft flies in a second, and each 10 s out of
// the landing sequence.
func enrouteJumps(h *Harness, d time.Duration, jumps *[]string) {
	prev, had := enrouteObject(h)
	for gone := time.Duration(0); gone < d; gone += time.Second {
		h.Advance(time.Second)
		o, ok := enrouteObject(h)
		if !ok {
			return
		}
		if gone%(10*time.Second) == 0 && sequenceNumber(h, enrouteToldCS) == 0 {
			*jumps = append(*jumps, h.Clock.Now().Format("15:04:05")+": out of the landing sequence")
		}
		if had {
			nm := calc.HaversineNM(prev.Lat, prev.Lon, o.Lat, o.Lon)
			if nm > 0.25 || math.Abs(o.AltFt-prev.AltFt) > 120 {
				*jumps = append(*jumps, h.Clock.Now().Format("15:04:05")+": "+
					strings.Join([]string{ftoa(nm, 2) + " NM", ftoa(o.AltFt-prev.AltFt, 0) + " ft"}, ", "))
			}
		}
		prev, had = o, true
	}
}

func ftoa(f float64, prec int) string { return strconv.FormatFloat(f, 'f', prec, 64) }

// heardSince are the transmissions to or by cs from n on.
func heardSince(h *Harness, n int, cs string) []traffic.Transmission {
	var out []traffic.Transmission
	for _, tx := range h.Heard()[n:] {
		if tx.Callsign == cs {
			out = append(out, tx)
		}
	}
	return out
}

// toldAndReadBack reports whether the transmissions have approach's call
// containing sub and the crew's readback after it.
func toldAndReadBack(txs []traffic.Transmission, sub string) (bool, bool) {
	told, back := false, false
	for _, tx := range txs {
		l := strings.ToLower(tx.Text)
		if !tx.Pilot && strings.Contains(l, sub) {
			told = true
		} else if told && tx.Pilot && strings.Contains(l, sub) {
			back = true
		}
	}
	return told, back
}

// TestEnrouteToldStaffed (#1419): on a staffed approach the player gives an
// arrival still en route, before its STAR entry, a speed, a heading and a
// level: each said by approach and read back, flown with no jump, and it is
// handed over at its entry as before and lands, the conflict watch quiet.
func TestEnrouteToldStaffed(t *testing.T) {
	h, log, obj := startEnrouteTold(t, true)
	do := func(action string, body map[string]any) error {
		_, err := h.World.Do(http.MethodPost, "/api/approach/LKPR/"+enrouteToldCS+"/"+action, body)
		return err
	}
	var jumps []string
	h.Advance(30 * time.Second)

	// Speed.
	n := len(h.Heard())
	if err := do("speed", map[string]any{"kts": 260}); err != nil {
		t.Fatalf("speed en route: %v", err)
	}
	enrouteJumps(h, 3*time.Minute, &jumps)
	if told, back := toldAndReadBack(heardSince(h, n, enrouteToldCS), "speed to 260 knots"); !told || !back {
		t.Errorf("speed 260: said %v, read back %v", told, back)
	}
	if o, _ := enrouteObject(h); o.GroundKts > 275 {
		t.Errorf("flying %.0f kt 3 minutes after speed 260", o.GroundKts)
	}

	// Heading: 40° off its track, magnetic (LKPR's variation is a few
	// degrees east).
	obj, _ = enrouteObject(h)
	mag := math.Mod(math.Round((obj.Heading+40)/5)*5+360, 360)
	if mag == 0 {
		mag = 360
	}
	n = len(h.Heard())
	if err := do("heading", map[string]any{"heading": mag}); err != nil {
		t.Fatalf("heading en route: %v", err)
	}
	enrouteJumps(h, 2*time.Minute, &jumps)
	said := ftoaHeading(mag)
	if told, back := toldAndReadBack(heardSince(h, n, enrouteToldCS), "heading "+said); !told || !back {
		t.Errorf("heading %s: said %v, read back %v", said, told, back)
	}
	if o, _ := enrouteObject(h); math.Abs(math.Mod(o.Heading-mag+540, 360)-180) > 15 {
		t.Errorf("flying %.0f° two minutes after heading %s", o.Heading, said)
	}

	// Level: 5000 ft down, at least FL110.
	obj, _ = enrouteObject(h)
	ft := math.Max(11000, math.Floor((obj.AltFt-5000)/1000)*1000)
	n = len(h.Heard())
	if err := do("descend", map[string]any{"ft": ft}); err != nil {
		t.Fatalf("descend en route: %v", err)
	}
	enrouteJumps(h, 30*time.Second, &jumps)
	if told, back := toldAndReadBack(heardSince(h, n, enrouteToldCS), "descend"); !told || !back {
		t.Errorf("descend to %.0f ft: said %v, read back %v", ft, told, back)
	}
	low := math.Inf(1)
	for range 8 * 60 / 10 {
		enrouteJumps(h, 10*time.Second, &jumps)
		o, ok := enrouteObject(h)
		if !ok {
			break
		}
		low = math.Min(low, o.AltFt)
		if math.Abs(o.AltFt-ft) < 200 {
			break
		}
	}
	if math.Abs(low-ft) > 300 {
		t.Errorf("told %.0f ft, down to %.0f ft in 8 minutes", ft, low)
	}
	// Own navigation again at the end of the heading's leg.
	if !h.Until(10*time.Minute, func() bool { return h.Said(enrouteToldCS, "resume own navigation") }) {
		t.Error("never told to resume own navigation after the heading")
	}
	if _, ok := h.Aircraft(enrouteToldCS); ok {
		t.Log("handed over before the end of the heading's leg")
	}

	// Handed over at its STAR entry as before, the other too, each cleared
	// and handed to the tower by the player; they land in the order of the
	// landing sequence (slowed to 260 kt at cruise, it lands after the
	// other, due later).
	cleared := map[string]bool{}
	var order []string
	first := "" // number 1 of the sequence, both still in it
	h.Until(60*time.Minute, func() bool {
		if len(order) == 0 && sequenceNumber(h, enrouteToldCS) > 0 && sequenceNumber(h, enrouteBehindCS) > 0 {
			first = enrouteToldCS
			if sequenceNumber(h, enrouteBehindCS) < sequenceNumber(h, enrouteToldCS) {
				first = enrouteBehindCS
			}
		}
		for _, cs := range []string{enrouteToldCS, enrouteBehindCS} {
			v, ok := h.Aircraft(cs)
			if !ok {
				continue
			}
			for i, act := range []string{"clearance", "approach", "handoff"} {
				if cleared[cs+act] || !slices.Contains(v.Actions, act) || i > 0 && !cleared[cs+"clearance"] || act == "handoff" && !cleared[cs+"approach"] {
					continue
				}
				if err := h.World.Clear(v.ID, act); err == nil {
					cleared[cs+act] = true
				}
			}
			if v.OnGround && !slices.Contains(order, cs) {
				order = append(order, cs)
			}
		}
		return len(order) == 2
	})
	if len(order) < 2 {
		t.Fatalf("landed %v in an hour, want both", order)
	}
	if order[0] != first {
		t.Errorf("landed %v, the landing sequence had %s first", order, first)
	}
	if len(jumps) > 0 {
		t.Errorf("en route: %s", strings.Join(jumps, "; "))
	}
	lines := conflictLines(log.String(), enrouteToldCS)
	lines = append(lines, conflictLines(log.String(), enrouteBehindCS)...)
	if len(lines) > 0 {
		t.Errorf("conflict watch:\n%s", strings.Join(lines, "\n"))
	}
}

// TestEnrouteToldUnstaffed (#1419): approach not staffed, an arrival en
// route takes no instruction, as before, and nothing is said to it.
func TestEnrouteToldUnstaffed(t *testing.T) {
	h, _, _ := startEnrouteTold(t, false)
	h.Advance(30 * time.Second) // in the landing sequence
	n := len(h.Heard())
	for action, body := range map[string]map[string]any{"speed": {"kts": 260}, "heading": {"heading": 90}, "descend": {"ft": 11000}} {
		_, err := h.World.Do(http.MethodPost, "/api/approach/LKPR/"+enrouteToldCS+"/"+action, body)
		if err == nil || !strings.Contains(err.Error(), "not one of our arrivals") {
			t.Errorf("%s en route, approach not staffed: %v, want not one of our arrivals", action, err)
		}
	}
	h.Advance(10 * time.Second)
	if txs := heardSince(h, n, enrouteToldCS); len(txs) > 0 {
		t.Errorf("said to it: %q", txs[0].Text)
	}
	// The AI's: both land.
	landed := map[string]bool{}
	if !h.Until(60*time.Minute, func() bool {
		for _, cs := range []string{enrouteToldCS, enrouteBehindCS} {
			if v, ok := h.Aircraft(cs); ok && v.OnGround {
				landed[cs] = true
			}
		}
		return len(landed) == 2
	}) {
		t.Errorf("landed %v in an hour, want both", landed)
	}
}

// ftoaHeading is a magnetic heading as said, three digits.
func ftoaHeading(mag float64) string {
	return fmt.Sprintf("%03.0f", mag)
}

// conflictLines are the conflict watch's lines naming cs.
func conflictLines(log, cs string) []string {
	var out []string
	for _, l := range strings.Split(log, "\n") {
		if (strings.Contains(l, "conflict:") || strings.Contains(l, "separation:")) && strings.Contains(l, cs) {
			out = append(out, l)
		}
	}
	return out
}
