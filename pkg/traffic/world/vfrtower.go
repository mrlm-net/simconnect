package world

import (
	"fmt"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// circuitPlace is a VFR arrival's place in the landing sequence as the
// tower gives it on its downwind report (#569, Doc 4444 12.3.4.14 b): its
// number and the traffic it follows as said, "the Airbus A320 on 4 mile
// final" ("" when number 1), and its call sign; 0 when it is not sequenced.
func (it *controlled) circuitPlace() (int, string, string) {
	if it.cc.sequencesAt == nil {
		return 0, "", ""
	}
	for _, seq := range it.cc.sequencesAt(it.ICAO) {
		for _, e := range seq {
			if e.Callsign != it.Tail {
				continue
			}
			if e.Leader == "" {
				return e.Number, "", ""
			}
			typ, inCircuit := "aircraft", false
			if l := it.cc.byTail(e.Leader); l != nil {
				l.mu.Lock()
				typ, inCircuit = typeSaid(traffic.ProfileFor(l.view.Model).Type), l.circuit != nil
				l.mu.Unlock()
			}
			where := ""
			for _, x := range seq {
				if x.Callsign != e.Leader {
					continue
				}
				switch d := x.DistanceToGoNM; {
				case d <= 1.5:
					where = " on short final"
				case inCircuit:
					where = " in the circuit"
				case d <= 12:
					where = fmt.Sprintf(" on %.0f mile final", d)
				}
			}
			return e.Number, "the " + typ + where, e.Leader
		}
	}
	return 0, "", ""
}

// wakeCautionWindow: a landing this long ago still counts as the preceding
// one for a wake caution: the longest time minimum for landing behind another
// (Doc 4444 5.8, 3 min).
const wakeCautionWindow = 3 * time.Minute

// landedRecord is a landing as noteLanded keeps it.
type landedRecord struct {
	model string
	at    time.Time
}

// noteLanded notes model's landing on rwy at icao.
func (cc *controlCenter) noteLanded(icao, rwy, model string) {
	cc.landed.Store(icao+" "+rwy, landedRecord{model, cc.clock.Now()})
}

// wakeCaution tells a VFR arrival in the circuit of a heavier aircraft ahead,
// in the sequence or just landed on its runway, once a circuit (#1307). An
// IFR arrival stays wake separated by the sequence, so gets none.
func (it *controlled) wakeCaution() {
	if it.circuit == nil || it.wakeCautioned.Load() {
		return
	}
	it.mu.Lock()
	model, rwy := it.view.Model, it.view.Runway
	it.mu.Unlock()
	_, ahead, lead := it.circuitPlace()
	leadModel := ""
	if l := it.cc.byTail(lead); lead != "" && l != nil {
		l.mu.Lock()
		leadModel = l.view.Model
		l.mu.Unlock()
	} else if v, ok := it.cc.landed.Load(it.ICAO + " " + rwy); ok {
		if r := v.(landedRecord); it.cc.clock.Now().Sub(r.at) <= wakeCautionWindow {
			leadModel, ahead = r.model, "the "+typeSaid(traffic.ProfileFor(r.model).Type)+" landing ahead"
		}
	}
	if leadModel == "" {
		return
	}
	wake := func(m string) traffic.WakeCategory { return traffic.WakeFor(traffic.ProfileFor(m).Type).ICAO }
	if traffic.WakeCautionDue(wake(leadModel), wake(model)) && it.wakeCautioned.CompareAndSwap(false, true) {
		it.say(traffic.WakeCaution(it.Tail, ahead))
	}
}

// leaderToGo is the arrival this one lands behind ("" none, or landed
// already), how far it still is from the threshold and the spacing kept
// behind it (NM).
func (it *controlled) leaderToGo() (string, float64, float64) {
	if it.cc.sequencesAt == nil {
		return "", 0, 0
	}
	for _, seq := range it.cc.sequencesAt(it.ICAO) {
		for _, e := range seq {
			if e.Callsign != it.Tail || e.Leader == "" {
				continue
			}
			for _, l := range seq {
				if l.Callsign == e.Leader && l.Runway == "" {
					return l.Callsign, l.DistanceToGoNM, max(e.MinimumNM, e.SeparationNM)
				}
			}
		}
	}
	return "", 0, 0
}

// fnv32 is a small stable hash of s (a call sign: the same choice each time).
func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h
}

// offsetLatLon is p moved meters along true bearing brg.
func offsetLatLon(p airport.LatLon, brg, meters float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, brg, meters)
	return airport.LatLon{Lat: lat, Lon: lon}
}
