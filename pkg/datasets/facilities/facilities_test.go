//go:build windows

package facilities

import (
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/datasets"
)

// Every facility set: each OPEN closed by its own CLOSE in order, as
// SimConnect's AddToFacilityDefinition wants, and fields only inside one.
func TestFacilitySetsNest(t *testing.T) {
	for name, make := range map[string]func() *datasets.FacilityDataSet{
		"airport": NewAirportFacilityDataset, "approach": NewApproachFacilityDataset,
		"approach transition": NewApproachTransitionFacilityDataset, "approach leg": NewApproachLegFacilityDataset,
		"final approach leg": NewFinalApproachLegFacilityDataset, "missed approach leg": NewMissedApproachLegFacilityDataset,
		"departure": NewDepartureFacilityDataset, "arrival": NewArrivalFacilityDataset,
		"runway transition": NewRunwayTransitionFacilityDataset, "enroute transition": NewEnrouteTransitionFacilityDataset,
		"frequency": NewFrequencyFacilityDataset, "helipad": NewHelipadFacilityDataset,
		"jetway": NewJetwayFacilityDataset, "ndb": NewNDBFacilityDataset,
		"parking": NewParkingFacilityDataset, "pavement": NewPavementFacilityDataset,
		"approach lights": NewApproachLightsFacilityDataset, "vasi": NewVASIFacilityDataset,
		"start": NewStartFacilityDataset, "runway": NewRunwayFacilityDataset,
		"taxi point": NewTaxiPointFacilityDataset, "taxi path": NewTaxiPathFacilityDataset,
		"taxi name": NewTaxiNameFacilityDataset, "vor": NewVORFacilityDataset,
		"waypoint": NewWaypointFacilityDataset, "route": NewRouteFacilityDataset,
	} {
		defs := make().Definitions
		if len(defs) < 3 {
			t.Errorf("%s: %d definitions", name, len(defs))
			continue
		}
		var open []string
		for _, d := range defs {
			s := string(d)
			switch {
			case strings.HasPrefix(s, "OPEN "):
				open = append(open, strings.TrimPrefix(s, "OPEN "))
			case strings.HasPrefix(s, "CLOSE "):
				if len(open) == 0 || open[len(open)-1] != strings.TrimPrefix(s, "CLOSE ") {
					t.Errorf("%s: %q with %v open", name, s, open)
					continue
				}
				open = open[:len(open)-1]
			case len(open) == 0:
				t.Errorf("%s: field %q outside any OPEN", name, s)
			}
		}
		if len(open) != 0 {
			t.Errorf("%s: never closed: %v", name, open)
		}
	}
}
