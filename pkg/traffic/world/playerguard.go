package world

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The user aircraft and the World's runways (the player's ATC does not
// know our traffic; reviewed after the player was cleared for take-off
// with OKRAX lined up): its clearances, and what it is seen doing even
// without one — on a runway, on a final — on that runway and on every
// runway crossing it; a clearance left stale expires.

// Stale player clearances: lined up or taking off, over once the user
// aircraft is airborne and off the runway or after playerRunwayStale;
// landing, once it is on the ground off the runway or after
// playerLandingStale; crossing, once it has been on the runway and is off
// it, or after playerCrossStale; any other after playerOtherStale.
const (
	playerRunwayStale  = 10 * time.Minute
	playerLandingStale = 20 * time.Minute
	playerCrossStale   = 3 * time.Minute
	playerOtherStale   = 30 * time.Minute
	// playerFinalNM, playerFinalFt: the user aircraft lined up with a
	// runway within this distance and below this height counts as landing
	// on it, cleared or not.
	playerFinalNM = 4.0
	playerFinalFt = 2500.0
)

// expirePlayer ends the host's clearance of the user aircraft ua when it
// is done or too old (nil ua: by age only); l is the clearance's airport.
func (k *core) expirePlayer(ua *traffic.TrackedAircraft, l *airport.Layout) {
	k.player.mu.Lock()
	defer k.player.mu.Unlock()
	c := k.player.c
	if c == nil {
		return
	}
	age := time.Since(k.player.since)
	var r airport.Runway
	haveRwy := false
	if l != nil {
		r, haveRwy = runwayOf(l, c.Runway)
	}
	on := ua != nil && haveRwy && onRunwayWithin(r, ua.Position, 40)
	if on {
		k.player.onRunway = true
	}
	done := false
	switch c.Phase {
	case PlayerLineUp, PlayerTakeoff:
		done = age > playerRunwayStale || ua != nil && !ua.OnGround && !on
	case PlayerLanding:
		done = age > playerLandingStale || ua != nil && ua.OnGround && haveRwy && !on
	case PlayerCrossing:
		done = age > playerCrossStale || k.player.onRunway && ua != nil && !on
	default:
		done = age > playerOtherStale
	}
	if done {
		k.player.c = nil
	}
}

// userAircraft is the user aircraft among aircraft, nil when not seen.
func userAircraft(aircraft []traffic.TrackedAircraft) *traffic.TrackedAircraft {
	for i := range aircraft {
		if aircraft[i].User {
			return &aircraft[i]
		}
	}
	return nil
}

// playerUsers are the user aircraft's places on runways, by airport and
// runway: as its host's ATC cleared it (holding short in the queue, lining
// up, rolling, crossing) and as it is seen (on a runway, on a final within
// playerFinalNM below playerFinalFt), each on every runway crossing that
// one too. airports are where to look; layout gives each.
func playerUsers(p *PlayerClearance, ua *traffic.TrackedAircraft, airports []string, layout func(string) *airport.Layout) map[[2]string][]traffic.RunwayUser {
	out := map[[2]string][]traffic.RunwayUser{}
	name, wake := "Player", traffic.WakeFor("")
	if p != nil {
		if p.Callsign != "" {
			name = p.Callsign
		}
		wake = traffic.WakeFor(p.Model)
	}
	if ua != nil && p == nil && ua.Title != "" {
		wake = traffic.WakeFor(ua.Title)
	}
	seen := map[[2]string]bool{}
	add := func(l *airport.Layout, icao string, r airport.Runway, u traffic.RunwayUser, crossing bool) {
		k := [2]string{icao, r.Name()}
		if !seen[k] {
			seen[k] = true
			out[k] = append(out[k], u)
		} else if us := out[k]; len(us) > 0 && us[len(us)-1].Phase == traffic.RunwayHoldingShort && u.Phase != traffic.RunwayHoldingShort {
			// Seen on the runway or its final, cleared to hold short only
			// (a stale clearance; stopped on it after a rejected take-off):
			// where it is wins.
			us[len(us)-1] = u
		}
		if !crossing {
			return
		}
		// On the runway or close in on its final: on every runway crossing
		// it as well (a take-off on 24 runs through 12/30).
		for _, x := range l.Runways {
			if x.Name() == r.Name() || !runwaysCross(r, x) {
				continue
			}
			kx := [2]string{icao, x.Name()}
			if !seen[kx] {
				seen[kx] = true
				v := u
				v.Phase, v.Arrival, v.Host = traffic.RunwayRolling, false, false
				out[kx] = append(out[kx], v)
			}
		}
	}
	// As cleared.
	if p != nil {
		if l := layout(p.ICAO); l != nil {
			if r, ok := runwayOf(l, p.Runway); ok {
				u := traffic.RunwayUser{Callsign: name, Wake: wake}
				switch p.Phase {
				case PlayerHoldingShort:
					u.Phase, u.Host = traffic.RunwayHoldingShort, true
					add(l, p.ICAO, r, u, false)
				case PlayerLineUp:
					u.Phase, u.Other = traffic.RunwayLinedUp, true
					add(l, p.ICAO, r, u, true)
				case PlayerTakeoff:
					u.Phase, u.Other = traffic.RunwayRolling, true
					add(l, p.ICAO, r, u, true)
				case PlayerCrossing:
					u.Phase, u.Other, u.Crossing = traffic.RunwayRolling, true, true
					add(l, p.ICAO, r, u, false)
				}
			}
		}
	}
	if ua == nil {
		return out
	}
	// As seen: whatever its ATC said.
	for _, icao := range airports {
		l := layout(icao)
		if l == nil {
			continue
		}
		if calc.HaversineNM(l.Latitude, l.Longitude, ua.Position.Lat, ua.Position.Lon) > 15 {
			continue
		}
		for _, r := range l.Runways {
			u := traffic.RunwayUser{Callsign: name, Wake: wake, Other: true}
			if ua.OnGround || ua.AGLFt < 100 {
				if onRunway(r, ua.Position) {
					u.Phase = traffic.RunwayRolling
					add(l, icao, r, u, true)
				}
				continue
			}
			if ua.AGLFt > playerFinalFt {
				continue
			}
			for _, end := range []airport.RunwayEnd{r.Primary, r.Secondary} {
				d := calc.HaversineNM(ua.Position.Lat, ua.Position.Lon, end.Threshold.Lat, end.Threshold.Lon)
				if overRunway(ua.Position, ua.Heading, end, r.Length) {
					d = 0
				} else if !onFinalNear(ua.Position, ua.Heading, d, end) || d > playerFinalNM {
					continue
				}
				u.Phase, u.Arrival, u.DistanceNM, u.GroundKts = traffic.RunwayFinal, true, d, ua.GroundKts
				add(l, icao, r, u, d < 2)
			}
		}
	}
	return out
}

// playerBlocks reports whether the host's clearance of the user aircraft
// keeps ours off icao's runway rwy: cleared onto it, or onto a runway
// crossing it.
func (k *core) playerBlocks(icao, rwy string, l *airport.Layout) bool {
	if _, ok := k.playerOn(icao, rwy); ok {
		return true
	}
	p, ok := k.playerClearance()
	if !ok || l == nil || !strings.EqualFold(p.ICAO, icao) || p.Phase == PlayerHoldingShort || p.Phase == PlayerCrossing {
		return false
	}
	if p.Phase != PlayerLineUp && p.Phase != PlayerTakeoff && p.Phase != PlayerLanding {
		return false
	}
	a, okA := runwayOf(l, p.Runway)
	b, okB := runwayOf(l, rwy)
	return okA && okB && a.Name() != b.Name() && runwaysCross(a, b)
}

// reportUserGround tells the ground picture of the airport the user
// aircraft ua is at where it is going (ReportUserMotion): ours give way to
// its way ahead, or to its push — cleared to push by its ATC, or seen
// moving backwards — not only to its body. prev is where it was a moment
// ago (zero none), for which way it moves.
func (t *towers) reportUserGround(ua *traffic.TrackedAircraft, prev airport.LatLon, airports []string, layout func(string) *airport.Layout) {
	if ua == nil || !ua.OnGround {
		return
	}
	for _, icao := range airports {
		l := layout(icao)
		if l == nil || calc.HaversineNM(l.Latitude, l.Longitude, ua.Position.Lat, ua.Position.Lon) > 5 {
			continue
		}
		pushing := false
		if p, ok := t.cc.core.playerClearance(); ok && p.Phase == PlayerPushback && strings.EqualFold(p.ICAO, icao) {
			pushing = true
		}
		if prev != (airport.LatLon{}) && calc.HaversineMeters(prev.Lat, prev.Lon, ua.Position.Lat, ua.Position.Lon) > 1 {
			track := calc.BearingDegrees(prev.Lat, prev.Lon, ua.Position.Lat, ua.Position.Lon)
			if d := headingDiff(ua.Heading, track); d > 120 || d < -120 {
				pushing = true // moving backwards
			}
		}
		half := ua.SpanM / 2
		if half <= 0 {
			half = 18
		}
		t.cc.world.Ground(icao).ReportUserMotion(ua.ObjectID, ua.Position, ua.Heading, ua.GroundKts, half, half*0.9, pushing)
		return
	}
}

// standIndex is the parking spot labelled label: of several with the label
// (a scenery's duplicates), the one nearest the user aircraft ua; an error
// without it.
func standIndex(l *airport.Layout, label string, ua *traffic.TrackedAircraft) (int, error) {
	idx, err := l.ParkingIndex(label)
	if !errors.Is(err, airport.ErrAmbiguousParking) || ua == nil {
		return idx, err
	}
	best := math.Inf(1)
	for _, p := range l.ParkingByLabel(label) {
		if d := calc.HaversineNM(ua.Position.Lat, ua.Position.Lon, p.Position.Lat, p.Position.Lon); d < best {
			best, idx = d, p.Index
		}
	}
	return idx, nil
}

// syncPlayerStand holds the stand the host gave the user aircraft (pc's
// Stand) for it: an arrival of ours that has not landed yet and holds it
// gives it up and is moved (recheckArrivalStands); one taxiing in or
// parked there keeps it, logged. Without a stand, the one held is freed.
func (t *towers) syncPlayerStand(pc *PlayerClearance, ua *traffic.TrackedAircraft) {
	t.mu.Lock()
	had := t.playerStand
	t.mu.Unlock()
	want := ""
	if pc != nil && pc.Stand != "" {
		want = strings.ToUpper(pc.ICAO) + " " + strings.ToUpper(pc.Stand)
	}
	if want == had {
		return
	}
	if had != "" {
		icao, _, _ := strings.Cut(had, " ")
		if g, err := t.cc.graph(icao); err == nil {
			t.cc.allocator(g).ReleaseOwner(traffic.PlayerStandOwner)
		}
	}
	t.mu.Lock()
	t.playerStand = ""
	t.mu.Unlock()
	if want == "" {
		return
	}
	g, err := t.cc.graph(pc.ICAO)
	if err != nil {
		return
	}
	idx, err := standIndex(g.Layout, pc.Stand, ua)
	if err != nil {
		t.cc.log.printf("player: stand %s not held for it: %v", pc.Stand, err)
		return
	}
	alloc := t.cc.allocator(g)
	half := traffic.ProfileFor(pc.Model).WingspanM / 2
	if err := alloc.Occupy(idx, traffic.PlayerStandOwner, half); err != nil {
		// Held by one of ours: given up if it has not landed yet.
		o, held := alloc.Occupant(idx)
		it := t.cc.byTail(o.Owner)
		if !held || it == nil || it.arr == nil || it.arr.State() > traffic.ArrivalLanding {
			t.cc.log.printf("player: stand %s not held for it: %v", pc.Stand, err)
			return
		}
		alloc.Release(idx)
		if err := alloc.Occupy(idx, traffic.PlayerStandOwner, half); err != nil {
			_ = alloc.Occupy(idx, o.Owner, o.HalfSpan)
			t.cc.log.printf("player: stand %s not held for it: %v", pc.Stand, err)
			return
		}
		t.cc.log.printf("player: stand %s held for it, %s moves", pc.Stand, o.Owner)
	}
	t.mu.Lock()
	t.playerStand = want
	t.mu.Unlock()
}
