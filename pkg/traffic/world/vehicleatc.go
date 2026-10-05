package world

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Service vehicles under ATC (#752): a tug or fuel truck about to drive
// along taxiways calls ground and goes once told "proceed"; one about to
// cross a runway calls the tower, waits in the tower's crossing queue as
// an aircraft does and crosses once cleared; off it, it reports vacated.

// vehicleATC answers the vehicles of all airports (traffic.VehicleATC
// through vehicleAt, one per airport).
type vehicleATC struct {
	cc    *controlCenter
	mu    sync.Mutex
	reqs  map[string]*vehicleReq
	names map[uint32]string
	n     map[string]int // by kind: the next number
}

type vehicleReq struct {
	icao    string
	r       traffic.VehicleRequest
	name    string
	cleared bool
	waiting bool // a runway crossing in the tower's queue
	given   bool // the tower's clearance said
}

// vehicleAt is the vehicles' ATC at one airport.
type vehicleAt struct {
	v    *vehicleATC
	icao string
}

func (a vehicleAt) Cleared(r traffic.VehicleRequest) bool { return a.v.cleared(a.icao, r) }
func (a vehicleAt) Vacated(r traffic.VehicleRequest)      { a.v.vacated(a.icao, r) }

// vehicles is the control center's vehicle ATC, made once.
func (cc *controlCenter) vehicles() *vehicleATC {
	cc.vehOnce.Do(func() {
		cc.vehATC = &vehicleATC{cc: cc, reqs: map[string]*vehicleReq{}, names: map[uint32]string{}, n: map[string]int{}}
	})
	return cc.vehATC
}

// giveATC has vehicle v (a tug, a fuel truck) ask ATC at icao (#752).
func (cc *controlCenter) giveATC(v any, l *airport.Layout, kind string) {
	if a, ok := v.(traffic.ATCAware); ok && l != nil {
		a.SetATC(vehicleAt{v: cc.vehicles(), icao: l.ICAO}, l, kind)
	}
}

// name is how vehicle id of kind is called: "Tug 3", "Fuel 2".
func (v *vehicleATC) name(id uint32, kind string) string {
	if n, ok := v.names[id]; ok {
		return n
	}
	word := "Tug"
	if strings.Contains(kind, "fuel") {
		word = "Fuel"
	}
	v.n[word]++
	n := fmt.Sprintf("%s %d", word, v.n[word])
	v.names[id] = n
	return n
}

func reqKey(icao string, r traffic.VehicleRequest) string {
	return fmt.Sprintf("%s %d %s %s %s", icao, r.Vehicle, r.Gate, r.Runway, strings.Join(r.Taxiways, ","))
}

// cleared answers a vehicle's ask at a gate: the first ask makes its call.
func (v *vehicleATC) cleared(icao string, r traffic.VehicleRequest) bool {
	v.mu.Lock()
	k := reqKey(icao, r)
	q := v.reqs[k]
	if q != nil {
		ok := q.cleared
		v.mu.Unlock()
		return ok
	}
	q = &vehicleReq{icao: icao, r: r, name: v.name(r.Vehicle, r.Kind)}
	v.reqs[k] = q
	v.mu.Unlock()
	cc := v.cc
	switch r.Gate {
	case traffic.GateTaxiway:
		station, freq := cc.stationOf(icao, traffic.PosGround)
		via := strings.Join(r.Taxiways, ", ")
		cc.radio.Transmit(icao, traffic.Transmission{Position: traffic.PosGround, Frequency: freq, Pilot: true, Callsign: q.name,
			Intent: traffic.IntentVehicleRequest, Params: map[string]string{traffic.ParamVia: via},
			Text: station + ", " + q.name + ", request proceed via " + via})
		p := cc.pending
		p.later(cc.radio.ClearAt(icao, freq).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() {
			cc.radio.Transmit(icao, traffic.Transmission{Position: traffic.PosGround, Frequency: freq, Callsign: q.name,
				Intent: traffic.IntentVehicleProceed, Params: map[string]string{traffic.ParamVia: via},
				Text: q.name + ", proceed via " + via})
			cc.radio.Transmit(icao, traffic.Transmission{Position: traffic.PosGround, Frequency: freq, Pilot: true, Callsign: q.name,
				Intent: traffic.IntentReadback, Params: map[string]string{traffic.ParamIntent: string(traffic.IntentVehicleProceed)},
				Text: "Proceed via " + via + ", " + q.name})
			p.later(cc.radio.ClearAt(icao, freq), func() {
				v.mu.Lock()
				q.cleared = true
				v.mu.Unlock()
			})
		})
	case traffic.GateRunway:
		station, freq := cc.stationOf(icao, traffic.PosTower)
		cc.radio.Transmit(icao, traffic.Transmission{Position: traffic.PosTower, Frequency: freq, Pilot: true, Callsign: q.name,
			Intent: traffic.IntentVehicleRequest, Params: map[string]string{traffic.ParamRunway: v.said(icao, r.Runway)},
			Text: station + ", " + q.name + ", request cross runway " + v.said(icao, r.Runway)})
		v.mu.Lock()
		q.waiting = true // the tower decides in its turn (crossingUsers, crossCleared)
		v.mu.Unlock()
	}
	cc.log.printf("%-6s %s: asks %s %s%s", q.name, r.Kind, r.Gate, r.Runway, strings.Join(r.Taxiways, ","))
	return false
}

// said is a runway as the tower says it: the end in use, else the first.
func (v *vehicleATC) said(icao, rwy string) string {
	end, _, _ := strings.Cut(rwy, "/")
	if g, err := v.cc.graph(icao); err == nil {
		inUse := v.cc.activeRunway(g, false)
		if slices := strings.Split(rwy, "/"); inUse != "" && (slices[0] == inUse || len(slices) > 1 && slices[1] == inUse) {
			return inUse
		}
	}
	return end
}

// crossingUsers are the vehicles holding short of icao's runway rwy for
// the tower's queue (Crossing, never a departure).
func (v *vehicleATC) crossingUsers(icao, rwy string) []traffic.RunwayUser {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []traffic.RunwayUser
	for _, q := range v.reqs {
		if q.icao == icao && q.r.Gate == traffic.GateRunway && q.r.Runway == rwy && !q.cleared {
			out = append(out, traffic.RunwayUser{Callsign: q.name, Phase: traffic.RunwayHoldingShort, Crossing: true, Wake: traffic.WakeFor("")})
		}
	}
	return out
}

// crossCleared gives the vehicles in cross their crossing: "Tug 3, cross
// runway 24", read back; it drives once it has read it back.
func (v *vehicleATC) crossCleared(icao string, cross []string) {
	cc := v.cc
	for _, name := range cross {
		v.mu.Lock()
		var q *vehicleReq
		for _, x := range v.reqs {
			if x.icao == icao && x.name == name && x.waiting && !x.given {
				q = x
			}
		}
		if q != nil {
			q.given = true
		}
		v.mu.Unlock()
		if q == nil {
			continue
		}
		_, freq := cc.stationOf(icao, traffic.PosTower)
		t := traffic.ClearedCross(name, v.said(icao, q.r.Runway))
		t.Position, t.Frequency = traffic.PosTower, freq
		cc.radio.Transmit(icao, t) // read back by the crew (the radio's ReadBack)
		p := cc.pending
		p.later(cc.radio.ClearAt(icao, freq), func() {
			v.mu.Lock()
			q.cleared = true
			v.mu.Unlock()
		})
	}
}

// vacated is a vehicle off the runway it crossed: it reports it.
func (v *vehicleATC) vacated(icao string, r traffic.VehicleRequest) {
	v.mu.Lock()
	name := v.name(r.Vehicle, r.Kind)
	delete(v.reqs, reqKey(icao, r))
	v.mu.Unlock()
	_, freq := v.cc.stationOf(icao, traffic.PosTower)
	t := traffic.Vacated(name, v.said(icao, r.Runway))
	t.Position, t.Frequency = traffic.PosTower, freq
	v.cc.radio.Transmit(icao, t)
}

// forget drops a vehicle's requests once it is gone.
func (v *vehicleATC) forget(id uint32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for k, q := range v.reqs {
		if q.r.Vehicle == id {
			delete(v.reqs, k)
		}
	}
	delete(v.names, id)
}

// runwaysWaiting are the (airport, runway) pairs with a vehicle holding
// short to cross.
func (v *vehicleATC) runwaysWaiting() [][2]string {
	v.mu.Lock()
	defer v.mu.Unlock()
	seen := map[[2]string]bool{}
	var out [][2]string
	for _, q := range v.reqs {
		k := [2]string{q.icao, q.r.Runway}
		if q.r.Gate == traffic.GateRunway && q.waiting && !q.cleared && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}
