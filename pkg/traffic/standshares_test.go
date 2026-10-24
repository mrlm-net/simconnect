package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestStandSharesFractional: a stand shared by a pool is a share of one stand.
//
// Munich's stand 0 names 23 airlines and stand 100 names 28 — a common pool, not a
// gate each. Counting a whole stand per name made a pool member look as established
// as the airline with a gate to itself, and the pools are big enough to outnumber the
// hub carrier many times over.
func TestStandSharesFractional(t *testing.T) {
	pool := []string{"AAA", "BBB", "CCC", "DDD"}
	l := &airport.Layout{Parking: []airport.Parking{
		{Type: 9, Radius: 26, Airlines: []string{"DLH"}},
		{Type: 9, Radius: 26, Airlines: []string{"DLH"}},
		{Type: 9, Radius: 26, Airlines: pool},
		{Type: 9, Radius: 26, Airlines: pool},
		// Not a stand: a fuel pump's airline does not fly from it.
		{Type: 12, Radius: 40, Airlines: []string{"XXX"}},
	}}
	s := standShares(l)
	if _, ok := s["XXX"]; ok {
		t.Error("an airline named on a fuel pump got a share of the airport")
	}
	// DLH holds two stands outright; each pool member holds two quarters.
	if math.Abs(s["DLH"]-1) > 1e-9 {
		t.Errorf("the biggest holder is %.3f, want 1", s["DLH"])
	}
	if math.Abs(s["AAA"]-0.25) > 1e-9 {
		t.Errorf("a quarter of two stands against two is %.3f, want 0.25", s["AAA"])
	}
	// And that is what presence multiplies by.
	if homeWeight(s["DLH"]) <= homeWeight(s["AAA"]) {
		t.Errorf("pool member weighted %.2f against the gate holder's %.2f",
			homeWeight(s["AAA"]), homeWeight(s["DLH"]))
	}
}

// TestHubCarrierLeadsItsHub: the airline holding the field flies the most from it,
// even where the config declares no base.
//
// THE DEFECT THIS PINS (#1193). Presence used to be a flag — "named on any stand" —
// earning the full home-carrier eightfold, so at a hub most of the config claimed it
// at once and it cancelled out. Measured at EDDM over 480 flights: easyJet 11%, KLM
// 10%, Lufthansa 9%, and Lufthansa Cityline on 86 of 180 stands nowhere in the top
// twelve. On this fixture before the fix: DLH 16 flights against easyJet's 18, with
// DLH holding half the stands and easyJet a sixteenth of the other half.
//
// EDDP is used deliberately: no built-in airline is based there, so the stands are
// the only thing deciding.
func TestHubCarrierLeadsItsHub(t *testing.T) {
	pool := []string{"KLM", "AFR", "BAW", "EZY", "THY", "LOT", "AUA", "SWR"}
	var parking []airport.Parking
	for i := 0; i < 12; i++ {
		parking = append(parking, airport.Parking{Type: 9, Radius: 26, Airlines: []string{"DLH"}})
	}
	for i := 0; i < 12; i++ {
		parking = append(parking, airport.Parking{Type: 9, Radius: 26, Airlines: pool})
	}
	l := &airport.Layout{ICAO: "EDDP", Latitude: 51.42, Longitude: 12.23,
		Runways: []airport.Runway{{Length: 3600}}, Parking: parking}
	from := time.Date(2026, 10, 24, 6, 0, 0, 0, time.UTC)
	fl := Schedule(DefaultScheduleConfig(), ScheduleOptions{
		Focus: []string{"EDDP"}, Density: 3, Seed: 42,
		Layouts: map[string]*airport.Layout{"EDDP": l},
	}, from, from.Add(12*time.Hour))
	count := map[string]int{}
	for _, f := range fl {
		count[f.Airline]++
	}
	if len(fl) == 0 {
		t.Fatal("no flights at all — this proves nothing")
	}
	for _, other := range pool {
		if count[other] >= count["DLH"] {
			t.Errorf("DLH %d flights, %s %d — the field's own carrier does not lead it",
				count["DLH"], other, count[other])
		}
	}
	t.Logf("%d flights: DLH %d, pool %v", len(fl), count["DLH"], pool)
}
