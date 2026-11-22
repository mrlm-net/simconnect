package world

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The centre's cap holds whatever the crew answers (#1662: a hand-off with
// room for it and its readback became four with a "say again", a peak of 8
// against a cap of 6); with room to spare the crews slip as before.
func TestCentreCapWithSlips(t *testing.T) {
	st := traffic.Station{Position: traffic.PosCenter, Name: "Test Centre", Freq: "127.000"}
	for _, c := range []struct {
		name        string
		booked, max int
		slips       bool
	}{{"room for two", centreCallsPerMinute - 2, centreCallsPerMinute, false}, {"room to spare", 0, centreCallsPerMinute, true}} {
		t.Run(c.name, func(t *testing.T) {
			w := New(Options{DataDir: t.TempDir()})
			cc := newControlCenter(nil, w.st.core)
			cc.localStations = func(string) []traffic.Station {
				s := st
				s.Area = []airport.LatLon{{Lat: 40, Lon: 5}, {Lat: 60, Lon: 5}, {Lat: 60, Lon: 25}, {Lat: 40, Lon: 25}}
				return []traffic.Station{s}
			}
			// Every crew asks for it again.
			cc.radio.SetVariety(&traffic.Variety{Seed: 1662, SayAgain: 1, ReadbackError: -1, MissedCall: -1})
			s := &scheduler{st: w.st, cc: cc}
			s.over.cr.sent = map[string][]time.Time{}
			now := cc.clock.Now()
			for range c.booked {
				if !s.centreSay(now, "LKPR", st, traffic.CheckIn(traffic.PosCenter, st.Name, "DLH836", "flight level 370", ""), 1) {
					t.Fatal("a check-in under the cap did not go out")
				}
			}
			if !s.centreSay(now, "LKPR", st, traffic.Handoff("KLM221", traffic.PosCenter, traffic.PosCenter, "", ""), 2) {
				t.Fatal("the hand-off did not go out")
			}
			n, slipped := 0, false
			for _, x := range cc.radio.Recent("LKPR", 0) {
				if x.Frequency == st.Freq {
					n++
					slipped = slipped || x.Intent == traffic.IntentPilotSayAgain
				}
			}
			if n > c.max {
				t.Errorf("%d transmissions booked in the minute, cap %d", n, c.max)
			}
			if slipped != c.slips {
				t.Errorf("a say again: %v, want %v", slipped, c.slips)
			}
		})
	}
}
