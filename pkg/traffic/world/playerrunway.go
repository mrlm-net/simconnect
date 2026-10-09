package world

import (
	"fmt"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// PlayerQuery is the player's ATC asking before it clears the user
// aircraft onto a runway: to line up, take off or land there.
type PlayerQuery struct {
	ICAO   string      `json:"icao"`
	Runway string      `json:"runway"` // runway end ("24")
	Phase  PlayerPhase `json:"phase"`  // PlayerLineUp, PlayerTakeoff, PlayerLanding or PlayerCrossing
	// Callsign and Model: the user aircraft as said and its model (its wake
	// and departure interval); optional.
	Callsign string `json:"callsign,omitempty"`
	Model    string `json:"model,omitempty"`
}

// RunwayAnswer is the World's tower answering a PlayerQuery: Free when the
// user aircraft may be cleared now; else what to tell it (Say) and why.
type RunwayAnswer struct {
	Free bool `json:"free"`
	// Say is what to clear it to instead: "line up and wait", "hold short"
	// (crossing), "cross behind" (with Behind), "hold
	// position", "continue approach" or "go around"; "" when Free.
	Say string `json:"say,omitempty"`
	// Behind is a conditional line-up's traffic as said: "the departing
	// Airbus A320", "the landing Boeing 737" ("behind the …, line up and
	// wait").
	Behind string `json:"behind,omitempty"`
	// Traffic is why, as said to a pilot: "traffic on the runway", "traffic
	// on a 3 mile final", "departing traffic", "landing traffic"; "" none.
	Traffic string `json:"traffic,omitempty"`
	// Why is the World's own reason, with call signs (for a log).
	Why string `json:"why,omitempty"`
	// Number is its place in the departure queue (0 not in it).
	Number int `json:"number,omitempty"`
	// AskAgainIn is when to ask again.
	AskAgainIn time.Duration `json:"askAgainIn,omitempty"`
}

// playerAskAgain: how soon a player's ATC asks again after a "not now".
const playerAskAgain = 10 * time.Second

// PlayerRunway answers q the way the World's tower decides for its own
// traffic, on a copy of the runway's state: nothing changes by asking. The
// player's ATC asks before it clears the user aircraft (live, the player
// cleared for take-off while OKRAX, cleared already, was lined up on 24).
// Without a connection or traffic on the runway it is free.
func (w *World) PlayerRunway(q PlayerQuery) RunwayAnswer {
	free := RunwayAnswer{Free: true}
	st := w.st
	st.mu.Lock()
	t := st.towers
	st.mu.Unlock()
	if t == nil {
		return free
	}
	icao := strings.ToUpper(q.ICAO)
	g, err := t.cc.graph(icao)
	if err != nil {
		return free
	}
	r, ok := runwayOf(g.Layout, q.Runway)
	if !ok {
		return free
	}
	k := icao + " " + r.Name()
	t.mu.Lock()
	rc := t.ctl[k]
	users := append([]traffic.RunwayUser(nil), t.users[k]...)
	t.mu.Unlock()
	if rc == nil {
		return free
	}
	name := q.Callsign
	if name == "" {
		name = "Player"
	}
	// The user aircraft as it would be: not as the host last cleared it.
	var list []traffic.RunwayUser
	for _, u := range users {
		if u.Callsign != name && !u.Host {
			list = append(list, u)
		}
	}
	u := traffic.RunwayUser{Callsign: name, Wake: traffic.WakeFor(q.Model)}
	landing := q.Phase == PlayerLanding
	if q.Phase == PlayerCrossing {
		u.Phase, u.Crossing = traffic.RunwayHoldingShort, true
	} else if landing {
		_, end, _ := g.Layout.RunwayEnd(q.Runway)
		at := st.core.userAt()
		u.Phase, u.Arrival, u.Established = traffic.RunwayFinal, true, true
		u.DistanceNM = calc.HaversineNM(at.Lat, at.Lon, end.Threshold.Lat, end.Threshold.Lon)
	} else {
		u.Phase = traffic.RunwayHoldingShort
	}
	d := rc.Clone().Decide(t.cc.clock.Now(), append(list, u))
	a := playerAnswer(t, q.Phase, name, d)
	if a.Free && q.Phase != PlayerCrossing {
		// A runway crossing this one in use: not through it (a take-off on
		// 24 runs through 12/30).
		if x, why := t.crossingBusy(icao, r, g.Layout.Runways); x != "" {
			a = RunwayAnswer{Say: "hold position", Traffic: "traffic on runway " + x, Why: why, AskAgainIn: playerAskAgain}
			if landing {
				a.Say = "continue approach"
			}
		}
	}
	return a
}

// crossingBusy is a runway crossing r at icao with traffic on it or close
// in on its final ("" none), and who.
func (t *towers) crossingBusy(icao string, r airport.Runway, runways []airport.Runway) (string, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, x := range runways {
		if x.Name() == r.Name() || !runwaysCross(r, x) {
			continue
		}
		for _, u := range t.users[icao+" "+x.Name()] {
			switch {
			case u.Phase == traffic.RunwayRolling || u.Phase == traffic.RunwayLinedUp:
				return x.Name(), u.Callsign + " on " + x.Name()
			case u.Phase == traffic.RunwayFinal && u.DistanceNM < crossingFinalNM:
				return x.Name(), u.Callsign + " on final " + x.Name()
			}
		}
	}
	return "", ""
}

// crossingFinalNM: traffic this close in on the final of a crossing runway
// keeps the player off its own.
const crossingFinalNM = 3.0

// playerAnswer reads the tower's decision d for the user aircraft name.
func playerAnswer(t *towers, phase PlayerPhase, name string, d traffic.RunwayClearances) RunwayAnswer {
	has := func(l []string) bool {
		for _, s := range l {
			if s == name {
				return true
			}
		}
		return false
	}
	why := d.Waiting[name]
	a := RunwayAnswer{Why: why, Traffic: trafficSaid(why), AskAgainIn: playerAskAgain}
	fmt.Sscanf(why, "number %d for departure", &a.Number)
	switch phase {
	case PlayerCrossing:
		switch {
		case has(d.Cross):
			return RunwayAnswer{Free: true}
		case d.CrossBehind[name] != "":
			a.Say, a.Behind = "cross behind", "the landing "+t.arrivalSaid(d.CrossBehind[name])
		default:
			a.Say = "hold short"
		}
		return a
	case PlayerLanding:
		switch {
		case has(d.Land):
			return RunwayAnswer{Free: true}
		case has(d.GoAround):
			a.Say = "go around"
		default:
			a.Say = "continue approach"
		}
		return a
	case PlayerLineUp:
		if has(d.LineUp) || has(d.Takeoff) {
			return RunwayAnswer{Free: true}
		}
	default: // take-off
		if has(d.Takeoff) {
			return RunwayAnswer{Free: true}
		}
		if has(d.LineUp) {
			a.Say = "line up and wait"
			return a
		}
	}
	switch {
	case d.LineUpBehind[name] != "":
		a.Say, a.Behind = "line up and wait", "the landing "+t.arrivalSaid(d.LineUpBehind[name])
	case d.LineUpBehindDeparting[name] != "":
		a.Say, a.Behind = "line up and wait", "the departing "+t.arrivalSaid(d.LineUpBehindDeparting[name])
	default:
		a.Say = "hold position"
	}
	return a
}

// trafficSaid is the tower's reason why (call signs) as said to a pilot:
// "OKRAX on the runway" → "traffic on the runway", "AFR1 on a 2.4 NM
// final" → "traffic on a 2 mile final", "AFR1 lands in 1m10s" → "landing
// traffic", "1m behind OKRAX" → "departing traffic"; "" none.
func trafficSaid(why string) string {
	var nm float64
	switch {
	case why == "" || strings.HasPrefix(why, "number "):
		return ""
	case strings.HasSuffix(why, " on the runway"):
		return "traffic on the runway"
	case strings.Contains(why, " NM final"):
		if i := strings.Index(why, " on a "); i >= 0 {
			fmt.Sscanf(why[i+len(" on a "):], "%f", &nm)
		}
		return fmt.Sprintf("traffic on a %d mile final", max(1, int(nm+0.5)))
	case strings.Contains(why, " lands in "):
		return "landing traffic"
	case strings.Contains(why, " behind "):
		return "departing traffic"
	}
	return why
}
