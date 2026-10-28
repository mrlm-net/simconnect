package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestBaseFlownStraight: vectored onto a base, an arrival flies it
// straight and is turned to intercept only at the end of it, not 15–30 s
// after "for base" (the user, live LKPR: KAL606, RYR1865, RYR1754 turned
// in early onto the final, the ones behind them close). The clearance
// with the heading comes within interceptOnBaseNM of the turn.
func TestBaseFlownStraight(t *testing.T) {
	g := lkprGraph(t)
	for _, star := range []string{"VLM", "GOLOP"} {
		route, err := lkprProcedures(t).Arrival("24", star)
		if err != nil {
			t.Fatal(err)
		}
		ec := &eventClient{}
		ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(NewInjector(ec)))
		c22, _ := g.Layout.ParkingIndex("C22")
		if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
			InjectApproach: true, Procedure: route}); err != nil {
			t.Fatal(err)
		}
		go func() {
			for range ctl.Events() {
			}
		}()
		ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
		ctl.Handle(arrivalPositionMsg(DefaultArrivalRequestBase+arrReqMonitor, 77, route[0].Position, 9000, 0, 250, false))
		ctl.AbsorbDelay(3 * time.Minute)
		if a, err := ctl.AbsorbDelay(3 * time.Minute); err != nil || !a.Downwind {
			t.Logf("%s: no downwind extended", star)
			continue
		}
		leave, base := ctl.vectors[0], ctl.vectors[1]
		flyTo(t, ctl, leave.At)
		flyTo(t, ctl, base.At)
		ctl.mu.Lock()
		turn := ctl.corners[ctl.finalTurn()]
		ctl.mu.Unlock()
		b, end := base.At, airport.LatLon{Lat: turn.Latitude, Lon: turn.Longitude}
		length := calc.HaversineNM(b.Lat, b.Lon, end.Lat, end.Lon)
		hdg := calc.BearingDegrees(b.Lat, b.Lon, end.Lat, end.Lon)
		due := -1.0
		for d := 0.25; d < length; d += 0.25 {
			lat, lon := calc.DisplaceByHeading(b.Lat, b.Lon, hdg, d*1852)
			moveTo(ctl, airport.LatLon{Lat: lat, Lon: lon}, hdg)
			if ctl.TurningFinal() {
				due = d
				break
			}
		}
		t.Logf("%s: base %.1f NM, the intercept due %.2f NM along it", star, length, due)
		if due < 0 || length-due > interceptOnBaseNM+0.25 {
			t.Errorf("%s: the intercept due %.2f NM along a %.1f NM base, want within %.1f NM of its end", star, due, length, interceptOnBaseNM)
		}
	}
}
