package traffic

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// The MyCrew API's aircraft-types set feeds the wake table: closed and
// deprecated items left out, a field it leaves out keeps the shipped value.
func TestWakeFromAPISet(t *testing.T) {
	defer dict.Reset("traffic.wake")
	data := []byte(`{"items":[
		{"key":"A319","set":"aircraft-types","version":"cf250b","closed":false,"deprecated":false,
		 "payload":{"code":"A319","engineType":"jet","engines":2,"manufacturer":"Airbus","model":"A319","recatEU":"E","wtc":"M","typicalSeats":144}},
		{"key":"A388","closed":false,"deprecated":false,"payload":{"code":"A388","wtc":"H"}},
		{"key":"ZZZ1","closed":false,"deprecated":false,"payload":{"code":"ZZZ1","wtc":"L","recatEU":"F"}},
		{"key":"ZZZ2","closed":true,"deprecated":false,"payload":{"code":"ZZZ2","wtc":"H","recatEU":"B"}}]}`)
	fed, err := dict.UseSet("aircraft-types", data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(fed, "traffic.wake") || !slices.Contains(dict.Sets(), "aircraft-types") {
		t.Fatalf("fed %v, sets %v", fed, dict.Sets())
	}
	w := wakeNow.Load()
	if got := w["A319"]; got != (Wake{WakeMedium, RecatE}) {
		t.Errorf("A319 %c/%c, want M/E from the API", got.ICAO, got.Recat)
	}
	if got := w["A388"]; got != (Wake{WakeHeavy, RecatA}) {
		t.Errorf("A388 %c/%c, want H from the API and the shipped RECAT A", got.ICAO, got.Recat)
	}
	if got := w["ZZZ1"]; got != (Wake{WakeLight, RecatF}) {
		t.Errorf("new type ZZZ1 %c/%c", got.ICAO, got.Recat)
	}
	if _, ok := w["ZZZ2"]; ok {
		t.Error("a closed item taken")
	}
	if got := w["B738"]; got != wakeTypes["B738"] {
		t.Errorf("B738, not in the set, %c/%c: not the shipped value", got.ICAO, got.Recat)
	}
}
