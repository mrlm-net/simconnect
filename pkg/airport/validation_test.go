package airport

import (
	"encoding/json"

	"fmt"
	"math"
	"os"

	"testing"
)

// loadAirport builds the Layout of a test airport from the facility data
// captured in MSFS 2024 (testdata/<ICAO>.json).
func loadAirport(t testing.TB, icao string) *Layout {
	t.Helper()
	b, err := os.ReadFile("testdata/" + icao + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var raw RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// runwayEnds are the names of every runway end of l.
func runwayEnds(l *Layout) []string {
	var out []string
	for _, r := range l.Runways {
		out = append(out, r.Primary.Name, r.Secondary.Name)
	}
	return out
}

// unheldCrossings lists the runway crossings of r that are not between two
// hold-shorts of the runway crossed, as the injected ground drive needs them
// (traffic's crossingZones): before the route enters a runway surface its
// last hold-short must protect that runway, and after it leaves, its next
// one. A stretch on a runway at the start of the route (an exit off the
// runway vacated) or at its end (the departure runway) is not a crossing.
// recrossed lists the stretches back across the runway vacated before the
// route has passed one of its hold-shorts: a taxi-in that leaves the runway
// on one side and crosses it again to the other.
func unheldCrossings(g *Graph, r *Route) (unheld, recrossed []string) {
	type hold struct {
		rwy int
		at  int // route node index
	}
	var holds []hold
	for i, id := range r.Nodes {
		if h := g.Nodes[id].HoldShort; h != nil {
			holds = append(holds, hold{h.Runway, i})
		}
	}
	// Surface stretches: the segments from..to (inclusive; segment i runs
	// from node i-1 to node i) along which the route is on runway rwy.
	type stretch struct{ rwy, from, to int }
	var stretches []stretch
	inside := g.onRunway(r.Points[0])
	if inside >= 0 {
		stretches = append(stretches, stretch{inside, 0, 0})
	}
	for i := 1; i < len(r.Points); i++ {
		a, b := r.Points[i-1], r.Points[i]
		steps := int(g.distance(a, b)/5) + 1
		for s := 1; s <= steps; s++ {
			f := float64(s) / float64(steps)
			on := g.onRunway(LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f})
			switch {
			case on >= 0 && on == inside:
				stretches[len(stretches)-1].to = i
			case on >= 0:
				stretches = append(stretches, stretch{on, i, i})
			}
			inside = on
		}
	}
	vacated := g.onRunway(r.Points[0])
	for _, s := range stretches {
		if s.from == 0 {
			continue // vacating
		}
		before, after, heldVacated := -1, -1, false
		for _, h := range holds {
			if h.at <= s.from-1 {
				before = h.rwy
				heldVacated = heldVacated || h.rwy == vacated
			}
			if h.at >= s.to && after == -1 {
				after = h.rwy
			}
		}
		last := s.to == len(r.Points)-1 && g.onRunway(r.Points[len(r.Points)-1]) == s.rwy
		if before == s.rwy && (last || after == s.rwy) {
			continue
		}
		desc := fmt.Sprintf("%s (nodes %d–%d, hold before %v after %v)", g.Layout.Runways[s.rwy].Name(), r.Nodes[s.from-1], r.Nodes[s.to], before == s.rwy, after == s.rwy)
		if s.rwy == vacated && !heldVacated {
			recrossed = append(recrossed, desc)
		} else {
			unheld = append(unheld, desc)
		}
	}
	return unheld, recrossed
}

// validatedAirports are the airports validated against their facility data
// (#376): LKPR, tuned live, is the reference the others are held to.
var validatedAirports = []string{"LKPR", "EDDM", "LOWW", "EGLL"}

// TestValidateAirportLayouts (#376): at every validated airport the layout
// and graph build; every runway end has hold-shorts, entries and exits, all
// named; every stand an aircraft can use reaches every runway end on
// taxiways, at a hold-short near the threshold (within 600 m along the
// runway: a hold beside a diagonal entry lies further along than the
// entry, LKPR 12 at D 556 m), with every runway crossing between
// hold-shorts; and from every exit of every runway end every stand
// is reached the same way. Taxi-ins that turn back across the runway just
// vacated (exit on the far side of the stand) are counted, not failed: the
// exit chosen for an arrival (traffic's bestExit) avoids them.
func TestValidateAirportLayouts(t *testing.T) {
	for _, icao := range validatedAirports {
		t.Run(icao, func(t *testing.T) {
			l := loadAirport(t, icao)
			g, err := BuildGraph(l)
			if err != nil {
				t.Fatal(err)
			}
			stands := l.SuitableStands(0)
			t.Logf("%s: %d runways, %d parking spots (%d usable), %d taxi nodes", icao, len(l.Runways), len(l.Parking), len(stands), len(g.Nodes))
			ends := runwayEnds(l)
			exits := map[string][]RunwayExit{}
			for _, end := range ends {
				rwy, _, _ := l.RunwayEnd(end)
				entries, err := g.RunwayEntries(end)
				if err != nil || len(entries) == 0 {
					t.Errorf("%s: %d entries (%v)", end, len(entries), err)
				}
				x, err := g.RunwayExits(end)
				if err != nil || len(x) == 0 {
					t.Errorf("%s: %d exits (%v)", end, len(x), err)
				}
				exits[end] = x
				if n := len(g.HoldShortNodes(rwy.Index)); n == 0 {
					t.Errorf("%s: no hold-short", end)
				}
				var en, ex []string
				for _, e := range entries {
					en = append(en, fmt.Sprintf("%s@%.0f", e.Taxiway, e.FromThreshold))
					if e.Taxiway == "" {
						t.Errorf("%s: entry %.0f m from the threshold has no name", end, e.FromThreshold)
					}
				}
				for _, e := range x {
					ex = append(ex, fmt.Sprintf("%s@%.0f", e.Taxiway, e.Along))
					if e.Taxiway == "" {
						t.Errorf("%s: exit %.0f m from the threshold has no name", end, e.Along)
					}
				}
				t.Logf("%s: entries %v", end, en)
				t.Logf("%s: exits %v", end, ex)
			}

			out, tight, unheld := 0, 0, 0
			for _, i := range stands {
				st := l.Parking[i]
				for _, end := range ends {
					r, err := g.RouteToRunway(i, end, RouteOptions{HalfSpan: math.Min(st.Radius, 18)})
					if err != nil {
						t.Errorf("%s → %s: %v", st.Label(), end, err)
						continue
					}
					out++
					checkRoute(t, g, r)
					if r.Tight {
						tight++
						t.Logf("%s → %s: no route fits a %.0f m span, via %v", st.Label(), end, 2*math.Min(st.Radius, 18), r.Taxiways)
					}
					if r.HoldShort == nil {
						t.Errorf("%s → %s: route ends off a hold-short", st.Label(), end)
						continue
					}
					rwy, e, _ := l.RunwayEnd(end)
					along := r.HoldShort.FromPrimary
					if e.Name == rwy.Secondary.Name {
						along = rwy.Length - along
					}
					if along > 600 {
						t.Errorf("%s → %s: holds %.0f m from the threshold", st.Label(), end, along)
					}
					bad, back := unheldCrossings(g, r)
					for _, c := range append(bad, back...) {
						unheld++
						t.Errorf("%s → %s via %v: crosses %s without holding", st.Label(), end, r.Taxiways, c)
					}
				}
			}
			in, back := 0, 0
			for _, end := range ends {
				for _, x := range exits[end] {
					for _, i := range stands {
						st := l.Parking[i]
						r, err := g.RouteFromRunway(x, i, RouteOptions{HalfSpan: math.Min(st.Radius, 18)})
						if err != nil {
							t.Errorf("%s exit %s (%.0f m) → %s: %v", end, x.Taxiway, x.Along, st.Label(), err)
							continue
						}
						in++
						bad, re := unheldCrossings(g, r)
						for _, c := range bad {
							unheld++
							t.Errorf("%s exit %s → %s via %v: crosses %s without holding", end, x.Taxiway, st.Label(), r.Taxiways, c)
						}
						if len(re) > 0 {
							back++
						}
					}
				}
			}
			t.Logf("%s: %d taxi-outs (%d tight), %d taxi-ins (%d back across the runway vacated), %d crossings without holds", icao, out, tight, in, back, unheld)
		})
	}
}
