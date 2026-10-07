//go:build windows
// +build windows

package datasets_test

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/datasets/aircraft"
	"github.com/mrlm-net/simconnect/pkg/datasets/environment"
	"github.com/mrlm-net/simconnect/pkg/datasets/navigation"
	"github.com/mrlm-net/simconnect/pkg/datasets/objects"
	"github.com/mrlm-net/simconnect/pkg/datasets/simulator"
	"github.com/mrlm-net/simconnect/pkg/datasets/traffic"
	"github.com/mrlm-net/simconnect/pkg/registry"
)

// Every SimVar of the SimVar datasets is a name the registry knows, in a
// unit it accepts: a wrong name is refused by the simulator
// (NAME_UNRECOGNIZED) and leaves the whole definition unread.
func TestDatasetNamesInRegistry(t *testing.T) {
	sets := map[string]func() *datasets.DataSet{
		"aircraft/position":        aircraft.NewPositionDataset,
		"aircraft/airspeed":        aircraft.NewAirspeedDataset,
		"aircraft/engine":          aircraft.NewEngineDataset,
		"aircraft/controlSurfaces": aircraft.NewControlSurfacesDataset,
		"environment/weather":      environment.NewWeatherDataset,
		"environment/time":         environment.NewTimeDataset,
		"navigation/radio":         navigation.NewRadioDataset,
		"navigation/gps":           navigation.NewGPSDataset,
		"objects/position":         objects.NewSimObjectPositionDataset,
		"simulator/state":          simulator.NewSimStateDataset,
		"simulator/camera":         simulator.NewCameraDataset,
		"traffic/aircraft":         traffic.NewAircraftDataset,
	}
	for set, f := range sets {
		for _, d := range f().Definitions {
			if err := registry.Validate(d.Name, d.Unit); err != nil {
				t.Errorf("%s: %v", set, err)
			}
		}
	}
}
