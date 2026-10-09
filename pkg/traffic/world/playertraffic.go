package world

import (
	"sort"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// PlayerTrafficInfo is one of the World's aircraft near the user aircraft,
// as its ATC tells it: "traffic, 2 o'clock, 4 miles, opposite direction,
// Airbus A320, 3000 feet" (Text, for the user aircraft's call sign given).
type PlayerTrafficInfo struct {
	Callsign  string  `json:"callsign"`
	Clock     int     `json:"clock"`
	NM        float64 `json:"nm"`
	Direction string  `json:"direction"` // "same direction", "opposite direction", "crossing left to right", …
	Type      string  `json:"type"`      // as said: "Airbus A320"
	Level     string  `json:"level"`     // as said: "3000 feet", "flight level 120"
	Text      string  `json:"text"`
}

// Traffic near the user aircraft its ATC is told of (PlayerTraffic): within
// playerTrafficNM and playerTrafficFt, airborne, closing in.
const (
	playerTrafficNM = 6.0
	playerTrafficFt = 2000.0
)

// PlayerTraffic is the World's traffic the user aircraft's ATC tells it of
// (its ATC knows none of ours otherwise): our aircraft airborne within
// playerTrafficNM and playerTrafficFt of it and not moving apart, nearest
// first; none while it is on the ground. callsign is the user aircraft as
// said in Text ("" none), icao the airport whose transition altitude says
// levels ("" 5500 ft).
func (w *World) PlayerTraffic(callsign, icao string) []PlayerTrafficInfo {
	st := w.st
	st.mu.Lock()
	cc := st.control
	st.mu.Unlock()
	if cc == nil {
		return nil
	}
	air := cc.world.Aircraft()
	me := userAircraft(air)
	if me == nil || me.OnGround {
		return nil
	}
	ta := 0.0
	if icao != "" {
		ta = cc.taOf(icao)
	}
	var out []PlayerTrafficInfo
	for _, o := range air {
		if !o.Ours || o.OnGround || o.User {
			continue
		}
		nm := calc.HaversineNM(me.Position.Lat, me.Position.Lon, o.Position.Lat, o.Position.Lon)
		if nm > playerTrafficNM || abs(o.AltFt-me.AltFt) > playerTrafficFt {
			continue
		}
		clock, saidNM, dir := traffic.TrafficRelative(me.Position, me.Heading, o.Position, o.Heading)
		if behindAway(clock, *me, o) {
			continue
		}
		typ := typeSaid(traffic.ProfileFor(o.Title).Type)
		level := levelSaid(ta, o.AltFt)
		tx := traffic.TrafficInformation(traffic.PosApproach, callsign, clock, saidNM, dir, typ, level)
		out = append(out, PlayerTrafficInfo{Callsign: o.Tail, Clock: clock, NM: nm, Direction: dir, Type: typ, Level: level, Text: tx.Text})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NM < out[j].NM })
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
