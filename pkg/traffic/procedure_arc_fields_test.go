package traffic

import (
	"reflect"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// A route without DMEArc marks is planned as before (#1646): no point
// moved or dropped, even where a marked arc would be flown straight.
func TestPlanArrivalWithoutArcMarksUnchanged(t *testing.T) {
	marked := arcTestRoute(t, 70, 9.2, 9.2, true)
	route := slices.Clone(marked)
	for i := range route {
		route[i].DMEArc, route[i].Straight = false, airport.LatLon{}
	}
	if got := soundArcRoute(route, arcTestEnd, ProcedureJoinNm*1852, 2500); !reflect.DeepEqual(got, route) {
		t.Errorf("route changed: %+v", got)
	}
	if got := withoutArcs(route, nil); !reflect.DeepEqual(got, route) {
		t.Errorf("withoutArcs changed it: %+v", got)
	}
	got, err := PlanArrivalProcedure(route, arcTestEnd, ProcedureJoinNm*1852, 2500)
	if err != nil {
		t.Fatal(err)
	}
	want, err := planArrival(route, arcTestEnd, ProcedureJoinNm*1852, 2500)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plan changed: %+v, want %+v", got, want)
	}
	onArc := func(n airport.NavPoint) bool {
		return n.Computed() && n.LegType == types.SIMCONNECT_FACILITY_LEG_TYPE_AF
	}
	if !slices.ContainsFunc(route, onArc) {
		t.Fatal("setup: no arc points in the route")
	}
	if len(soundArcRoute(marked, arcTestEnd, ProcedureJoinNm*1852, 2500)) >= len(marked) {
		t.Fatal("setup: the marked arc isn't flown straight")
	}
}
