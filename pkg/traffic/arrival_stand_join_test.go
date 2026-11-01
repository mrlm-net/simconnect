package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestArrivalKeepsItsRouteToAStandFarBack: a stop far back on its stand leaves
// no route point standAxisMeters behind it. The arrival still rolls out, takes
// its exit at the exit's speed and follows the route, not straight at the stand
// from the runway (live, LKTB 3M: a B738 "vacated" at 137 kt, 4 s after touchdown).
func TestArrivalKeepsItsRouteToAStandFarBack(t *testing.T) {
	for _, c := range []struct {
		icao, end, stand string
		nose             float64
	}{
		{"LKTB", "27", "3M", 20}, {"LKTB", "27", "4M", 20}, {"LKTB", "09", "3M", 20},
		{"LKTB", "27", "12", 15}, {"LKTB", "27", "13", 15}, {"LKTB", "27", "9", 15},
		{"EGLL", "27L", "2", 15}, {"EGLL", "09R", "2", 15},
	} {
		g := airportGraph(t, c.icao)
		tried := 0
		for st, sp := range g.Layout.Parking {
			if sp.Label() != c.stand {
				continue
			}
			ec := &eventClient{}
			inj := NewInjector(ec)
			ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
			if err := ctl.Start(ArrivalRequest{Graph: g, Runway: c.end, Parking: st, Model: "B738", Tail: "OKABC", NoseOffset: c.nose,
				InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
				continue // no way from this runway to this one of the same label
			}
			tried++
			go func() {
				for range ctl.Events() {
				}
			}()
			p := ctl.Plan()
			route := p.Route.Points[:p.VacateIndex+1]
			mon := DefaultArrivalRequestBase + arrReqMonitor
			now := time.Now()
			ctl.now = func() time.Time { return now }
			ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
			vacateKts, off := -1.0, 0.0
			for i := 0; i < 60*1200 && ctl.State() < ArrivalAwaitingTaxi; i++ {
				now = now.Add(time.Second / 60)
				ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
				if ctl.State() == ArrivalVacating {
					if vacateKts < 0 {
						vacateKts = ctl.last.GroundSpeed
					}
					off = math.Max(off, distToPolyline(ctl.last.Position, route))
				}
			}
			t.Logf("%s %s %s: exit %s, vacating at %.0f kt, up to %.0f m off the route", c.icao, c.end, c.stand, p.Exit.Taxiway, vacateKts, off)
			if vacateKts < 0 || vacateKts > 45 {
				t.Errorf("%s %s %s: vacating at %.0f kt", c.icao, c.end, c.stand, vacateKts)
			}
			if off > 30 {
				t.Errorf("%s %s %s: %.0f m off the route on the way off the runway", c.icao, c.end, c.stand, off)
			}
		}
		if tried == 0 {
			t.Errorf("%s %s %s: no arrival planned", c.icao, c.end, c.stand)
		}
	}
}

// TestStandJoinNearTheStandGoesStraightIn: close behind the stand already, with
// the route ahead of the axis left, the aircraft pulls straight in as before.
func TestStandJoinNearTheStandGoesStraightIn(t *testing.T) {
	stop := airport.LatLon{Lat: 50, Lon: 14}
	nose := offsetHeading(stop, 90, -40)
	route := []airport.LatLon{offsetHeading(stop, 90, 30), offsetHeading(stop, 90, 60)}
	if last, axis := standJoin(route, 0, nose, stop, 90); last != -1 || axis != standAxisMeters {
		t.Errorf("last %d, axis %.0f m; want -1, %.0f m", last, axis, standAxisMeters)
	}
}

func distToPolyline(p airport.LatLon, line []airport.LatLon) float64 {
	best := math.Inf(1)
	for i := 1; i < len(line); i++ {
		best = math.Min(best, pointSegDist(p, line[i-1], line[i]))
	}
	return best
}
