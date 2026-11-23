package nav

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A waypoint keeps its procedure point's DME arc marks (#1646).
func TestNavWaypointKeepsDMEArc(t *testing.T) {
	straight := airport.LatLon{Lat: 50.1, Lon: 14.1}
	w := navWaypoint(airport.NavPoint{DMEArc: true, Straight: straight}, nil, "", PhaseApproach)
	if !w.DMEArc || w.Straight != straight {
		t.Fatalf("waypoint %+v: the arc's marks are lost", w)
	}
}

// DMEArc and Straight are opt-in (#1646): zero, they aren't in the JSON and
// JSON without them reads as zero; set, they go round.
func TestDMEArcFieldsOptIn(t *testing.T) {
	at := airport.LatLon{Lat: 50, Lon: 14}
	straight := airport.LatLon{Lat: 50.1, Lon: 14.1}
	for name, v := range map[string]any{
		"NavPoint": airport.NavPoint{Ident: "FIX", Position: at},
		"Waypoint": Waypoint{Ident: "FIX", Position: at},
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("dmeArc")) || bytes.Contains(b, []byte("straight")) {
			t.Errorf("%s: zero fields in the JSON: %s", name, b)
		}
	}
	old := []byte(`{"ident":"FIX","position":{"lat":50,"lon":14}}`)
	var n airport.NavPoint
	var w Waypoint
	if err := json.Unmarshal(old, &n); err != nil || n.DMEArc || n.Straight != (airport.LatLon{}) {
		t.Errorf("NavPoint from JSON without the fields: %+v, %v", n, err)
	}
	if err := json.Unmarshal(old, &w); err != nil || w.DMEArc || w.Straight != (airport.LatLon{}) {
		t.Errorf("Waypoint from JSON without the fields: %+v, %v", w, err)
	}
	n, w = airport.NavPoint{}, Waypoint{}
	b, err := json.Marshal(airport.NavPoint{DMEArc: true, Straight: straight})
	if err == nil {
		err = json.Unmarshal(b, &n)
	}
	if err != nil || !n.DMEArc || n.Straight != straight {
		t.Errorf("NavPoint set, round JSON: %+v, %v", n, err)
	}
	b, err = json.Marshal(Waypoint{DMEArc: true, Straight: straight})
	if err == nil {
		err = json.Unmarshal(b, &w)
	}
	if err != nil || !w.DMEArc || w.Straight != straight {
		t.Errorf("Waypoint set, round JSON: %+v, %v", w, err)
	}
}
