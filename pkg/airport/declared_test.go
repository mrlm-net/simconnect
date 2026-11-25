package airport

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The JSON names the data repos fill (#1660, #1661), read into RawRunway.
func TestDeclaredJSONNames(t *testing.T) {
	var r RawRunway
	in := `{"primaryTakeoffStartM":1,"primaryToraM":2,"primaryAsdaM":3,"primaryLdaM":4,"primaryUse":1,
		"secondaryTakeoffStartM":5,"secondaryToraM":6,"secondaryAsdaM":7,"secondaryLdaM":8,"secondaryUse":2}`
	if err := json.Unmarshal([]byte(in), &r); err != nil {
		t.Fatal(err)
	}
	want := RawRunway{PrimaryTakeoffStartM: 1, PrimaryTORAM: 2, PrimaryASDAM: 3, PrimaryLDAM: 4, PrimaryUse: 1,
		SecondaryTakeoffStartM: 5, SecondaryTORAM: 6, SecondaryASDAM: 7, SecondaryLDAM: 8, SecondaryUse: 2}
	if r != want {
		t.Fatalf("read %+v, want %+v", r, want)
	}
}

// A layout's JSON with a use out of 0-3 reads as take-offs and landings,
// never as an end used for nothing (#1661).
func TestRunwayEndUseJSONClamped(t *testing.T) {
	for in, want := range map[string]RunwayEndUse{"{\"use\":7}": RunwayUseBoth, "{\"use\":-1}": RunwayUseBoth, "{\"use\":2}": RunwayUseLanding, "{}": RunwayUseBoth} {
		var e RunwayEnd
		if err := json.Unmarshal([]byte(in), &e); err != nil {
			t.Fatal(err)
		}
		if e.Use != want {
			t.Errorf("%s: use %s, want %s", in, e.Use, want)
		}
	}
}

// Every declared value written as 0 builds the same layout as none written,
// and nothing is refused.
func TestDeclaredZeroSameLayout(t *testing.T) {
	raw := rawLKPR(t)
	plain, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"takeoffStartM", "toraM", "asdaM", "ldaM", `"use"`} {
		if bytes.Contains(b, []byte(f)) {
			t.Errorf("layout JSON has %s without one set", f)
		}
	}
	var zeros RawAirport
	rb, _ := json.Marshal(raw)
	_ = json.Unmarshal(rb, &zeros)
	for i := range zeros.Runways {
		r := &zeros.Runways[i]
		r.PrimaryTakeoffStartM, r.PrimaryTORAM, r.PrimaryASDAM, r.PrimaryLDAM, r.PrimaryUse = 0, 0, 0, 0, 0
		r.SecondaryTakeoffStartM, r.SecondaryTORAM, r.SecondaryASDAM, r.SecondaryLDAM, r.SecondaryUse = 0, 0, 0, 0, 0
	}
	if errs := CheckDeclared(zeros); errs != nil {
		t.Errorf("refused: %v", errs)
	}
	got, err := BuildLayout(zeros)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain.Runways, got.Runways) {
		t.Errorf("runways differ with zero declared values")
	}
	g0, err := BuildGraph(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range plain.Runways {
		for _, e := range []RunwayEnd{r.Primary, r.Secondary} {
			entries, err := g0.RunwayEntries(e.Name)
			if err != nil {
				t.Fatal(err)
			}
			for _, x := range entries {
				if want := r.Length - x.FromThreshold; x.FromThreshold > 0 && math.Abs(x.Remaining-want) > 1e-6 {
					t.Errorf("%s at %s: remaining %.1f, want %.1f", e.Name, x.Taxiway, x.Remaining, want)
				}
			}
		}
	}
}

// Synthetic declared distances on LKPR 24: the take-off start, TORA, ASDA
// and LDA as given, the landing end LDA past the displaced threshold, and
// entries' run left to the end of the TORA.
func TestDeclaredDistances(t *testing.T) {
	raw := rawLKPR(t)
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	ri, primary := -1, false
	for i, r := range l.Runways {
		if r.Primary.Name == "24" || r.Secondary.Name == "24" {
			ri, primary = i, r.Primary.Name == "24"
		}
	}
	if ri < 0 {
		t.Fatal("no 24")
	}
	r := &raw.Runways[ri]
	if primary {
		r.PrimaryTakeoffStartM, r.PrimaryTORAM, r.PrimaryASDAM, r.PrimaryLDAM, r.PrimaryDisplacedM, r.PrimaryUse = 200, 3000, 3300, 2900, 300, int32(RunwayUseTakeoff)
	} else {
		r.SecondaryTakeoffStartM, r.SecondaryTORAM, r.SecondaryASDAM, r.SecondaryLDAM, r.SecondaryDisplacedM, r.SecondaryUse = 200, 3000, 3300, 2900, 300, int32(RunwayUseTakeoff)
	}
	if errs := CheckDeclared(raw); errs != nil {
		t.Fatalf("refused: %v", errs)
	}
	if l, err = BuildLayout(raw); err != nil {
		t.Fatal(err)
	}
	rwy, e, _ := l.RunwayEnd("24")
	if e.TORA(rwy.Length) != 3000 || e.ASDA(rwy.Length) != 3300 || e.LandingDistance(rwy.Length) != 2900 {
		t.Errorf("TORA %.0f ASDA %.0f LDA %.0f", e.TORA(rwy.Length), e.ASDA(rwy.Length), e.LandingDistance(rwy.Length))
	}
	if !e.ForTakeoff() || e.ForLanding() {
		t.Errorf("use %s: take-off %v landing %v", e.Use, e.ForTakeoff(), e.ForLanding())
	}
	// Each point measured off from the pavement end as the layout places
	// points (calc.DisplaceByHeading, on the WGS84 ellipsoid): a sphere's
	// great circle reads 0.25% short at LKPR.
	off := func(p LatLon, along float64) float64 {
		lat, lon := calc.DisplaceByHeading(e.Threshold.Lat, e.Threshold.Lon, e.Heading, along)
		return calc.HaversineMeters(lat, lon, p.Lat, p.Lon)
	}
	if d := off(e.TakeoffStart(), 200); d > 0.5 {
		t.Errorf("take-off start %.1f m off 200 m from the pavement end", d)
	}
	if d := off(e.LandingEnd(rwy.Length), 3200); d > 1 {
		t.Errorf("landing end %.1f m off 3200 m from the pavement end", d)
	}
	for along, want := range map[float64]float64{0: 3000, 200: 3000, 1000: 2200, 3300: 0} {
		if got := e.TakeoffRunAhead(along, rwy.Length); got != want {
			t.Errorf("run ahead of %.0f m: %.0f, want %.0f", along, got, want)
		}
	}
	g, err := BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := g.RunwayEntries("24")
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range entries {
		if want := e.TakeoffRunAhead(x.FromThreshold, rwy.Length); x.Remaining != want || x.Remaining > 3000 {
			t.Errorf("24 at %s (%.0f m in): remaining %.0f, want %.0f", x.Taxiway, x.FromThreshold, x.Remaining, want)
		}
	}
}

// Values the layout does not take are reported and left out: a negative
// take-off start (a paved start before the runway's end is not supported),
// a TORA or LDA past the runway's end, an ASDA under the TORA, a bad use.
func TestDeclaredRefused(t *testing.T) {
	raw := rawLKPR(t)
	r := &raw.Runways[0]
	L := r.Length
	r.PrimaryTakeoffStartM, r.PrimaryTORAM, r.PrimaryUse = -150, L+50, 7
	r.SecondaryASDAM, r.SecondaryTORAM, r.SecondaryLDAM = 1000, 2000, L+10
	errs := CheckDeclared(raw)
	if len(errs) != 5 {
		t.Fatalf("%d refused, want 5: %v", len(errs), errs)
	}
	for _, err := range errs {
		if !errors.Is(err, ErrBadDeclared) {
			t.Errorf("%v not ErrBadDeclared", err)
		}
		t.Log(err)
	}
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	p, s := l.Runways[0].Primary, l.Runways[0].Secondary
	if p.TakeoffStartM != 0 || p.TORAM != 0 || p.Use != RunwayUseBoth {
		t.Errorf("primary kept %+v", p)
	}
	if s.ASDAM != 0 || s.TORAM != 2000 || s.LDAM != 0 {
		t.Errorf("secondary ASDA %.0f TORA %.0f LDA %.0f, want 0, 2000, 0", s.ASDAM, s.TORAM, s.LDAM)
	}
}
