package traffic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Every table exports its shipped copy and takes it back unchanged (#768).
func TestDictsRoundTrip(t *testing.T) {
	names := dict.Names()
	for _, want := range []string{"traffic.telephony", "traffic.aircraftTypes", "traffic.wake", "traffic.airlines", "nav.performance", "airport.limits"} {
		if !strings.Contains(strings.Join(names, " "), want) {
			t.Errorf("no table %s in %v", want, names)
		}
	}
	before := ProfileFor("B738")
	for _, n := range names {
		b, err := dict.Export(n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		var env dict.Envelope
		if err := json.Unmarshal(b, &env); err != nil || env.Name != n || env.ID == "" || len(env.Items) < 3 {
			t.Fatalf("%s: envelope %v %+v", n, err, env.Name)
		}
		if err := dict.Use(n, b); err != nil {
			t.Fatalf("%s: use its own export: %v", n, err)
		}
	}
	if after := ProfileFor("B738"); after.Type != before.Type || after.Motion.SpanMeters != before.Motion.SpanMeters {
		t.Errorf("B738 changed by its own export: %+v", after)
	}
	for _, n := range names {
		_ = dict.Reset(n)
	}
}

// A host's data is merged by id over the shipped copy; Reset brings the
// shipped one back; telephony carries its source and licence (#768).
func TestDictsUse(t *testing.T) {
	defer dict.Reset("traffic.telephony")
	shippedCSA, _, _ := Telephony("CSA")
	defer dict.Reset("nav.performance")
	b, _ := dict.Export("traffic.telephony")
	var env dict.Envelope
	_ = json.Unmarshal(b, &env)
	if !strings.Contains(env.Licence, "CC BY-SA") || !strings.Contains(env.Source, "wikipedia") {
		t.Errorf("telephony source %q licence %q", env.Source, env.Licence)
	}
	if err := dict.Use("traffic.telephony", []byte(`[{"icao":"CSA","telephony":"CZECH","name":"Czech Airlines"},{"icao":"ZZZ","telephony":"ZULU","name":"Test"}]`)); err != nil {
		t.Fatal(err)
	}
	if tel, _, ok := Telephony("CSA"); !ok || tel != "CZECH" {
		t.Errorf("CSA %q", tel)
	}
	if tel, _, ok := Telephony("ZZZ"); !ok || tel != "ZULU" {
		t.Errorf("ZZZ %q", tel)
	}
	if _, _, ok := Telephony("DLH"); !ok {
		t.Error("DLH lost: not merged over the shipped copy")
	}
	_ = dict.Reset("traffic.telephony")
	if tel, _, _ := Telephony("CSA"); tel != shippedCSA {
		t.Errorf("reset: CSA %q", tel)
	}
	if err := dict.Use("nav.performance", []byte(`{"name":"nav.performance","id":"type","items":[{"type":"B738","cruiseTASKts":470}]}`)); err != nil {
		t.Fatal(err)
	}
	if p := nav.PerformanceFor("B738"); p.CruiseTASKts != 470 {
		t.Errorf("B738 cruise %v", p.CruiseTASKts)
	}
	if err := dict.Use("nav.performance", []byte(`{"name":"traffic.wake","items":[]}`)); err == nil {
		t.Error("another table's data taken")
	}
	if err := dict.Use("no.such", []byte(`[]`)); err == nil {
		t.Error("a table that is not there")
	}
}
