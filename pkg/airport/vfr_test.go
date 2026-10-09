package airport

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// TestVFRFor: LKPR's NOVEMBER (Velvary silo, 50°16'06"N 014°14'21"E) and
// its Praha Information; the entry nearest a point north of the field is
// NOVEMBER; a host's table replaces an airport's data.
func TestVFRFor(t *testing.T) {
	v, ok := VFRFor("lkpr")
	if !ok || v.FIS != "Praha Information" || v.FISFreq != "126.100" {
		t.Fatalf("LKPR: %+v %v", v, ok)
	}
	n, ok := v.Point("november")
	if !ok || math.Abs(n.Position.Lat-50.2683) > 0.001 || math.Abs(n.Position.Lon-14.2392) > 0.001 || !n.Entry {
		t.Fatalf("NOVEMBER %+v", n)
	}
	if e, ok := v.EntryNearest(LatLon{Lat: 50.3, Lon: 14.25}); !ok || e.Name != "NOVEMBER" {
		t.Errorf("nearest entry north: %+v", e)
	}
	if _, ok := VFRFor("LKPD"); !ok {
		t.Error("no LKPD")
	}
	defer dict.Reset("airport.vfr")
	if err := dict.Use("airport.vfr", []byte(`[{"icao":"LKPR","points":[{"name":"ZULU","position":{"lat":50,"lon":14},"entry":true}],"fis":"Praha Information","fisFreq":"126.100"}]`)); err != nil {
		t.Fatal(err)
	}
	if v, _ := VFRFor("LKPR"); len(v.Points) != 1 || v.Points[0].Name != "ZULU" {
		t.Errorf("overridden LKPR: %+v", v.Points)
	}
	if _, ok := VFRFor("LKPD"); !ok {
		t.Error("LKPD lost by overriding LKPR")
	}
}

// TestFrequencyForFIS: an airport with none of the frequencies asked for
// falls back to its flight information (Praha Information 126.100) where
// its VFR data gives one; elsewhere none.
func TestFrequencyForFIS(t *testing.T) {
	l := &Layout{ICAO: "LKPD"}
	f, ok := l.FrequencyFor(FreqTower)
	if !ok || f.Kind != FreqFIS || f.MHz != 126.1 || f.Name != "Praha Information" {
		t.Errorf("LKPD tower: %+v %v", f, ok)
	}
	if _, ok := (&Layout{ICAO: "LPMA"}).FrequencyFor(FreqTower); ok {
		t.Error("LPMA: a frequency without any data")
	}
	if _, ok := l.FrequencyFor(FreqApproach); ok {
		t.Error("an approach (IFR) frequency from flight information")
	}
}

// TestVFRPointsNearAirport: every shipped point lies within the airport's
// control zone reach (a typo in the coordinates puts it far away).
func TestVFRPointsNearAirport(t *testing.T) {
	arp := map[string]LatLon{
		"LKPR": {Lat: 50.1008, Lon: 14.26},
		"LKPD": {Lat: 50.0134, Lon: 15.7386},
		"LKTB": {Lat: 49.1513, Lon: 16.6944},
		"LKKV": {Lat: 50.203, Lon: 12.915},
		"LKMT": {Lat: 49.6963, Lon: 18.1111},
	}
	for icao, v := range KnownVFR {
		c, ok := arp[icao]
		if !ok {
			t.Errorf("%s: no reference point in the test", icao)
			continue
		}
		for _, p := range v.Points {
			dLat := (p.Position.Lat - c.Lat) * 60
			dLon := (p.Position.Lon - c.Lon) * 60 * math.Cos(c.Lat*math.Pi/180)
			if nm := math.Hypot(dLat, dLon); nm > 20 {
				t.Errorf("%s %s: %.1f NM from the airport", icao, p.Name, nm)
			}
		}
	}
}
