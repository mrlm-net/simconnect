package world

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/dict"
)

// TestOffersIntersection: a queue of IntersectionQueue for the full length
// gives a departure an intersection, never a widebody.
func TestOffersIntersection(t *testing.T) {
	for _, c := range []struct {
		model string
		queue int
		want  bool
	}{
		{"A320", 2, false}, {"A320", 3, true}, {"C172", 3, true}, {"B77W", 5, false}, {"A388", 5, false},
	} {
		if got := offersIntersection(c.model, c.queue); got != c.want {
			t.Errorf("%s with %d queuing: %v, want %v", c.model, c.queue, got, c.want)
		}
	}
}

// TestFullLengthQueue: departures for the runway's full length on the
// ground count; one from an intersection, airborne, for another runway or
// the asking one do not.
func TestFullLengthQueue(t *testing.T) {
	cc := &controlCenter{items: map[int]*controlled{}}
	add := func(id int, v ControlView) *controlled {
		it := &controlled{dep: &remoteDep{}}
		it.view = v
		cc.items[id] = it
		return it
	}
	me := add(1, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "taxiing"})
	add(2, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "holding short"})
	add(3, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "taxiing"})
	add(4, ControlView{ICAO: "LKPR", Runway: "24", OnGround: true, State: "holding short", Entry: "B"})
	add(5, ControlView{ICAO: "LKPR", Runway: "24", OnGround: false, State: "climbing"})
	add(6, ControlView{ICAO: "LKPR", Runway: "30", OnGround: true, State: "taxiing"})
	if n := cc.fullLengthQueue("lkpr", "24", me); n != 2 {
		t.Errorf("queue %d, want 2", n)
	}
	if n := cc.entryQueue("LKPR", "24", me, func(e string) bool { return e == "B" }); n != 1 {
		t.Errorf("queue at B %d, want 1", n)
	}
}

func eddmGraph(t *testing.T) *airport.Graph {
	t.Helper()
	b, err := os.ReadFile("../../airport/testdata/EDDM.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestEntryChoicesEDDM (#1030, live: AFR1910 at B10, the user: departures
// at A8): a jet gets the first intersection after the full length, with
// 2,500 m or more left (B12, A12: 2,808 m), never B10 or A8; a regional
// jet or turboprop may go to B10 and A10 (1,800 m), never A8 (1,743 m); a
// widebody none.
func TestEntryChoicesEDDM(t *testing.T) {
	g := eddmGraph(t)
	for _, c := range []struct {
		model, rwy string
		want       []string // the places in order, the first entry of each
	}{
		{"A320", "26L", []string{"B12"}},
		{"A320", "26R", []string{"A12"}},
		{"CRJ9", "26R", []string{"A12", "A10"}},
		{"AT76", "26L", []string{"B12", "B10"}},
		{"B77W", "26R", nil},
	} {
		var got []string
		for _, grp := range entryChoices(g, "EDDM", c.rwy, c.model, g.Nodes[0].Position) {
			got = append(got, grp[0].Taxiway)
		}
		if len(got) != len(c.want) {
			t.Errorf("%s %s: %v, want %v", c.model, c.rwy, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s %s: %v, want %v", c.model, c.rwy, got, c.want)
				break
			}
		}
	}
	if e := firstEntry(g, "EDDM", "26R", "A320", g.Nodes[0].Position); e != "A12" {
		t.Errorf("crew's first entry %q, want A12", e)
	}
}

// An airport's own entries (traffic.intersectionAirports) restrict the
// choice: listed for a class, only those.
func TestEntryChoicesAirportTable(t *testing.T) {
	if err := dict.Use("traffic.intersectionAirports", []byte(`[{"icao":"EDDM","runways":{"26R":{"regional":["A10"]}}}]`)); err != nil {
		t.Fatal(err)
	}
	defer dict.Reset("traffic.intersectionAirports")
	g := eddmGraph(t)
	groups := entryChoices(g, "EDDM", "26R", "CRJ9", g.Nodes[0].Position)
	if len(groups) != 1 || groups[0][0].Taxiway != "A10" {
		t.Errorf("CRJ9 26R with A10 listed: %v", groups)
	}
	if groups := entryChoices(g, "EDDM", "26R", "A320", g.Nodes[0].Position); len(groups) != 1 || groups[0][0].Taxiway != "A12" {
		t.Errorf("A320 26R, its class not listed: %v", groups)
	}
}
