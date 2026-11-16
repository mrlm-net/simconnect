package world

import (
	"encoding/json"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A tower simconnect-data filled in from OpenStreetMap keeps its source
// and its tagged height; the sim's stays "simulator" at the default cab.
func TestTowerAtOSM(t *testing.T) {
	for _, c := range []struct {
		tower, source string
		cab           float64
	}{
		{`{"lat":50.1,"lon":14.27,"alt":364,"source":"osm","osm":"way/794617342","heightM":25}`, "osm", 25},
		{`{"lat":50.1,"lon":14.27,"alt":364,"source":"osm","osm":"way/1"}`, "osm", towerHeightM},
		{`{"lat":50.1,"lon":14.27,"alt":359}`, "simulator", towerHeightM},
	} {
		var raw airport.RawAirport
		if err := json.Unmarshal([]byte(`{"icao":"ZZOS","lat":50.1,"lon":14.26,"tower":`+c.tower+`}`), &raw); err != nil {
			t.Fatal(err)
		}
		l, err := airport.BuildLayout(raw)
		if err != nil {
			t.Fatal(err)
		}
		v := (&controlCenter{}).towerAt(&airport.Graph{Layout: l})
		if v.Source != c.source || v.CabM != c.cab || v.Lat != 50.1 || v.Lon != 14.27 {
			t.Errorf("%s: got %+v, want source %s cab %v", c.tower, v, c.source, c.cab)
		}
	}
}
