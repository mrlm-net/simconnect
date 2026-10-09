package world

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestBalancedDeparture: at LROP with 26L and 26R in use together, a
// departure from stand 214 takes 26L (the shorter taxi) while it is quiet,
// and 26R once 26L has three departures queued (live, every stand is
// nearest 26L and 26R stood empty).
func TestBalancedDeparture(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LROP-layout.json")
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
	stand, err := l.ParkingIndex("214")
	if err != nil {
		t.Skip("no stand 214:", err)
	}
	var ends []airport.RunwayEnd
	for _, n := range []string{"26L", "26R"} {
		_, e, ok := l.RunwayEnd(n)
		if !ok {
			t.Fatalf("no %s", n)
		}
		ends = append(ends, e)
	}
	cc := &controlCenter{items: map[int]*controlled{}}
	if got := cc.balancedDeparture(g, ends, stand); got != "26L" {
		t.Fatalf("quiet: %s, want 26L", got)
	}
	for i := range 3 {
		it := &controlled{}
		it.view = ControlView{Kind: "departure", ICAO: "LROP", Runway: "26L", OnGround: true}
		cc.items[i] = it
	}
	if got := cc.balancedDeparture(g, ends, stand); got != "26R" {
		t.Errorf("26L with 3 queued: %s, want 26R", got)
	}
}
