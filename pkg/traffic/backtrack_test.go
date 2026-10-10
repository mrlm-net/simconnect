package traffic

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

func lowiGraph(t *testing.T) *airport.Graph {
	t.Helper()
	b, err := os.ReadFile("../airport/testdata/LOWI-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l airport.Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(&l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestBacktrackLOWI: at LOWI no taxiway reaches the 08 threshold (A joins
// 260 m in at the nearest): the departure enters, backtracks, turns round
// within the runway's width and lines up at the threshold for the full
// length, never stopping on the runway before it.
func TestBacktrackLOWI(t *testing.T) {
	for _, end := range []string{"08", "26"} {
		t.Run(end, func(t *testing.T) { backtrackLOWI(t, end) })
	}
}

func backtrackLOWI(t *testing.T, endName string) {
	g := lowiGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	stand, err := g.Layout.ParkingIndex("14")
	if err != nil {
		t.Fatal(err)
	}
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: stand, Runway: endName, Model: "FSLTL A320 Air France SL", Tail: "TVS1", HoldForRunway: true}); err != nil {
		t.Fatal(err)
	}
	if !ctl.Backtracks() {
		t.Fatalf("%s at LOWI not backtracked", endName)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1900, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	pos := g.Layout.Parking[stand].Position
	frame := func() { now = now.Add(time.Second / 60); ctl.Handle(positionMsg(mon, 77, pos, 0, 0, true)) }
	for i := 0; i < 60*60*30 && ctl.State() != TaxiHoldingShort; i++ {
		frame()
	}
	if ctl.State() != TaxiHoldingShort {
		t.Fatalf("state %v, not at the holding point", ctl.State())
	}
	start := len(placements(ec))
	ctl.ClearToLineUp()
	for i := 0; i < 60*300 && ctl.State() != TaxiLinedUp; i++ {
		frame()
	}
	if ctl.State() != TaxiLinedUp {
		t.Fatalf("state %v, not lined up", ctl.State())
	}
	_, end, _ := g.Layout.RunwayEnd(endName)
	rwy := ctl.runway
	ps := placements(ec)[start:]
	last := ps[len(ps)-1]
	thr := end.Threshold
	if d := calc.HaversineMeters(thr.Lat, thr.Lon, last.Latitude, last.Longitude); d > 90 {
		t.Errorf("lined up %.0f m from the %s threshold", d, endName)
	}
	if dh := math.Abs(math.Mod(last.Heading-end.Heading+540, 360) - 180); dh > 5 {
		t.Errorf("lined up heading %.0f, runway %.0f", last.Heading, end.Heading)
	}
	for _, p := range ps {
		off := math.Abs(calc.CrossTrackMeters(rwy.Primary.Threshold.Lat, rwy.Primary.Threshold.Lon, rwy.Secondary.Threshold.Lat, rwy.Secondary.Threshold.Lon, p.Latitude, p.Longitude))
		if alongHeading(thr, end.Heading, airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}) > -5 && off > rwy.Width/2 && alongHeading(thr, end.Heading, airport.LatLon{Lat: p.Latitude, Lon: p.Longitude}) < 150 {
			t.Errorf("off the runway in the turn: %.1f m from the centreline", off)
			break
		}
	}
}

// TestBacktrackPhrase: the line-up with the backtrack, ICAO and FAA.
func TestBacktrackPhrase(t *testing.T) {
	if got := Backtracked(ClearedLineUp("CSA1", "08"), true).Text; got != "CSA1, enter runway 08 and backtrack, line up and wait" {
		t.Errorf("ICAO %q", got)
	}
	if got := Backtracked(ClearedLineUp("CSA1", "08"), false).Text; got != "CSA1, runway 08, line up and wait" {
		t.Errorf("without %q", got)
	}
}
