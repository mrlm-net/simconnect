package traffic

import (
	"fmt"
	"strings"
	"testing"
)

// fakeVehicleATC clears a request after asks calls and records them.
type fakeVehicleATC struct {
	asks    int
	got     map[string]int
	vacated []string
}

func (f *fakeVehicleATC) Cleared(r VehicleRequest) bool {
	k := fmt.Sprintf("%s %s%s", r.Gate, r.Runway, strings.Join(r.Taxiways, ","))
	f.got[k]++
	return f.got[k] > f.asks
}
func (f *fakeVehicleATC) Vacated(r VehicleRequest) { f.vacated = append(f.vacated, r.Runway) }

// The vehicle routes of real airports: the ones across a runway hold
// short of it and ask to cross; stretches along taxiways are asked for;
// LKPR's depot routes cross no runway (#752).
func TestVehicleGatesOnRealRoutes(t *testing.T) {
	runways, taxiways := 0, 0
	for _, icao := range []string{"LKPR", "EDDF", "EGLL"} {
		l := airportGraph(t, icao).Layout
		for i, p := range l.Parking {
			if i%3 != 0 {
				continue
			}
			d, ok := nearestDepot(l, p.Position)
			if !ok {
				continue
			}
			r, err := l.VehicleRoute(d, p.Position)
			if err != nil {
				continue
			}
			path, err := NewGroundPath(r, tugRoadProfile())
			if err != nil {
				continue
			}
			for _, g := range vehicleGates(l, path) {
				switch g.Gate {
				case GateRunway:
					runways++
					if icao == "LKPR" {
						t.Errorf("LKPR %s: crosses %s", p.Label(), g.Runway)
					}
					// Held outside the runway: the hold point is not on it.
					if rw, on := l.RunwayAt(path.PointAt(g.at), 0); on {
						t.Errorf("%s %s: holds on runway %s", icao, p.Label(), rw.Name())
					}
				case GateTaxiway:
					taxiways++
					if g.end-g.at < VehicleTaxiwayAlongM {
						t.Errorf("%s: a %0.f m taxiway stretch asked for", icao, g.end-g.at)
					}
				}
			}
		}
	}
	if runways < 1 || taxiways < 1 {
		t.Errorf("%d runway and %d taxiway gates on the routes", runways, taxiways)
	}
}

// A vehicle holds at its first gate until ATC clears it, then drives on;
// off the runway it reports vacated.
func TestVehicleHoldsUntilCleared(t *testing.T) {
	l := airportGraph(t, "EGLL").Layout
	var path *GroundPath
	var gates []vehicleGate
	for _, p := range l.Parking {
		d, ok := nearestDepot(l, p.Position)
		if !ok {
			continue
		}
		r, err := l.VehicleRoute(d, p.Position)
		if err != nil {
			continue
		}
		if gp, err := NewGroundPath(r, tugRoadProfile()); err == nil {
			for _, g := range vehicleGates(l, gp) {
				if g.Gate == GateRunway {
					path, gates = gp, vehicleGates(l, gp)
				}
			}
		}
		if path != nil {
			break
		}
	}
	if path == nil {
		t.Skip("no EGLL vehicle route across a runway")
	}
	atc := &fakeVehicleATC{asks: 5, got: map[string]int{}}
	y := &vehicleYield{}
	y.SetATC(atc, l, "tug")
	a, b := path.PointAt(0), path.PointAt(5)
	m := NewGroundMoverFrom(path, tugRoadProfile(), localBearing(a, b), 0)
	first := gates[0]
	held := false
	for i := 0; i < 20000 && !m.Pose().Arrived; i++ {
		y.check(m)
		pose := m.Step(0.1)
		if !first.cleared && pose.Distance > first.at+1.5 && atc.got[fmt.Sprintf("%s %s%s", first.Gate, first.Runway, strings.Join(first.Taxiways, ","))] <= atc.asks {
			t.Fatalf("passed its first gate (%s at %.0f m) at %.0f m before cleared", first.Gate, first.at, pose.Distance)
		}
		if pose.GroundSpeedKts < 0.2 && pose.Distance > 0 && pose.Distance <= first.at+1 {
			held = true
		}
	}
	if !held || !m.Pose().Arrived {
		t.Errorf("held %v, arrived %v", held, m.Pose().Arrived)
	}
	if len(atc.vacated) == 0 {
		t.Error("crossed a runway, reported nothing")
	}
}
