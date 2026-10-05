package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

func TestDefaultStationsShareTheFallbackFrequency(t *testing.T) {
	l := &airport.Layout{ICAO: "LKXX", Frequencies: []airport.Frequency{
		{Kind: airport.FreqTower, Name: "XX TOWER", MHz: 118.1},
	}}
	st := DefaultStations(l)
	g, ok1 := PickStation(st, PosGround, Where{})
	tw, ok2 := PickStation(st, PosTower, Where{})
	if !ok1 || !ok2 || g.Freq != tw.Freq || g.Controller != tw.Controller || g.Controller == "" {
		t.Fatalf("ground %+v, tower %+v", g, tw)
	}
}

func TestPickStationBySector(t *testing.T) {
	apron := []airport.LatLon{{Lat: 50, Lon: 14}, {Lat: 50, Lon: 14.01}, {Lat: 50.01, Lon: 14.01}, {Lat: 50.01, Lon: 14}}
	st := StationsWith("LKPR", []Station{{Position: PosGround, Name: "Ruzyne Ground", Freq: "121.91", Controller: "LKPR 121.91"}},
		[]Station{
			{Position: PosGround, Name: "Ruzyne Ground", Freq: "121.91"},
			{Position: PosGround, Name: "Ruzyne Ground North", Freq: "121.71", Taxiways: []string{"F", "L"}},
			{Position: PosGround, Name: "Ruzyne Apron", Freq: "121.8", Area: apron},
			{Position: PosTower, Name: "Ruzyne Tower", Freq: "118.1", Runways: []string{"06/24"}},
			{Position: PosTower, Name: "Ruzyne Tower", Freq: "134.56", Runways: []string{"30"}, Controller: "LKPR twr"},
		})
	in := airport.LatLon{Lat: 50.005, Lon: 14.005}
	for _, c := range []struct {
		pos  Position
		w    Where
		want string
	}{
		{PosGround, Where{Taxiways: []string{"l"}}, "121.71"},
		{PosGround, Where{At: &in}, "121.8"},
		{PosGround, Where{Taxiways: []string{"A"}}, "121.91"},
		{PosTower, Where{Runway: "24"}, "118.1"},
		{PosTower, Where{Runway: "30"}, "134.56"},
		{PosTower, Where{Runway: "12"}, "118.1"},
	} {
		if s, _ := PickStation(st, c.pos, c.w); s.Freq != c.want {
			t.Errorf("%s at %+v: %s, want %s", c.pos, c.w, s.Freq, c.want)
		}
	}
	if len(st) != 5 || ControllerOn(st, "121.71") != "LKPR 121.71" || ControllerOn(st, "134.56") != "LKPR twr" {
		t.Errorf("stations %+v", st)
	}
}

// One controller on two frequencies says one thing at a time; two
// controllers talk at once.
func TestOneControllerOneMouth(t *testing.T) {
	now := time.Date(2026, 10, 5, 22, 0, 0, 0, time.UTC)
	ctl := map[string]string{"121.91": "night", "118.1": "night", "120.06": "delivery"}
	r := NewRadio(RadioOptions{Now: func() time.Time { return now },
		ControllerOf: func(_, f string) string { return ctl[f] }})
	g := r.Transmit("LKPR", Transmission{Position: PosGround, Frequency: "121.91", Callsign: "CSA1", Text: "CSA1, taxi to holding point A1 runway 24"})
	tw := r.Transmit("LKPR", Transmission{Position: PosTower, Frequency: "118.1", Callsign: "CSA2", Text: "CSA2, runway 24, cleared to land"})
	d := r.Transmit("LKPR", Transmission{Position: PosDelivery, Frequency: "120.06", Callsign: "CSA3", Text: "CSA3, cleared to Frankfurt"})
	if g.Controller != "night" || tw.Controller != "night" {
		t.Fatalf("controllers %q %q", g.Controller, tw.Controller)
	}
	if tw.At.Before(g.At.Add(SpeakingTime(g.Text))) {
		t.Errorf("one controller said two things at once: %v, %v", g.At, tw.At)
	}
	if !d.At.Equal(now) {
		t.Errorf("another controller waited: %v", d.At)
	}
}
