package flight

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// GhostFleet replays a Scene's aircraft in sync with a Player (#1013): the
// player's replay or the ghost's clock, so pause, seek and rate follow it.
// Each aircraft is created (NonATC, its title, else the fallback) when its
// time in the scene comes, flown by the Injector as its Track was (Ghost),
// and removed when its track ends, it falls out of the object budget
// (nearest the player first), or on Stop. A seek is just another moment:
// those due then are created, the rest removed.

// DefaultFleetBase is the first request ID a GhostFleet takes; FleetIDs
// how many (cycled).
const (
	DefaultFleetBase uint32 = 0x7E00
	FleetIDs                = 256
)

// DefaultFleetBudget: aircraft flown at once.
const DefaultFleetBudget = 20

// fleetRetry: an aircraft not created within this is created again with
// the fallback title (its own was refused), once.
const fleetRetry = 5 * time.Second

// FleetInjector is what a GhostFleet needs of the traffic Injector.
type FleetInjector interface {
	GhostInjector
	Takeover(objectID uint32) error
	Forget(objectID uint32)
}

// FleetOptions tune a GhostFleet; zero values take the defaults.
type FleetOptions struct {
	IDBase uint32 // DefaultFleetBase
	Budget int    // DefaultFleetBudget
	// Fallback is the model to create an aircraft as when its own title is
	// refused ("" none: it is left out).
	Fallback func(a *SceneAircraft) string
	// Centre is where "nearest" is measured from; nil: the Player's own
	// aircraft at the moment.
	Centre func() (lat, lon float64, ok bool)
}

type fleetGhost struct {
	a        *SceneAircraft
	req      uint32
	obj      uint32
	ghost    *Ghost
	asked    time.Time
	fallback bool
}

// GhostFleet is one scene replayed.
type GhostFleet struct {
	client GhostClient
	inj    FleetInjector
	scene  *Scene
	player *Player
	opt    FleetOptions

	mu      sync.Mutex
	live    map[string]*fleetGhost // by aircraft key
	byReq   map[uint32]string
	nextReq uint32
	placed  time.Time
	stopped bool
}

// NewGhostFleet replays scene on player's clock.
func NewGhostFleet(client GhostClient, inj FleetInjector, scene *Scene, player *Player, o FleetOptions) *GhostFleet {
	if o.IDBase == 0 {
		o.IDBase = DefaultFleetBase
	}
	if o.Budget <= 0 {
		o.Budget = DefaultFleetBudget
	}
	return &GhostFleet{client: client, inj: inj, scene: scene, player: player, opt: o, live: map[string]*fleetGhost{}, byReq: map[uint32]string{}}
}

// sceneTime is the scene's simulation time at now: the Player's track's
// first sample plus where the Player is.
func (f *GhostFleet) sceneTime(now time.Time) float64 {
	tr := f.player.Track()
	if tr == nil || len(tr.Samples) == 0 {
		return 0
	}
	return tr.Samples[0].T + f.player.Time(now)
}

// Live is how many aircraft are created or being created.
func (f *GhostFleet) Live() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.live)
}

// Tick brings the fleet to now: the aircraft due created, those no longer
// due removed, the rest placed. Handle calls it each frame; call it after
// a Seek for an immediate change.
func (f *GhostFleet) Tick(now time.Time) {
	f.mu.Lock()
	if f.stopped {
		f.mu.Unlock()
		return
	}
	due := f.dueLocked(now)
	want := map[string]SceneSample{}
	for _, d := range due {
		want[d.Aircraft.Key] = d
	}
	var remove []uint32
	for key, g := range f.live {
		if _, ok := want[key]; !ok {
			delete(f.live, key)
			delete(f.byReq, g.req)
			if g.obj != 0 {
				remove = append(remove, g.obj)
			}
		}
	}
	type create struct {
		title, tail string
		init        types.SIMCONNECT_DATA_INITPOSITION
		req         uint32
	}
	var creates []create
	var place []struct {
		g *Ghost
		s Sample
	}
	for _, d := range due {
		g := f.live[d.Aircraft.Key]
		switch {
		case g == nil:
			g = &fleetGhost{a: d.Aircraft, req: f.reqLocked(), asked: now}
			f.live[d.Aircraft.Key], f.byReq[g.req] = g, d.Aircraft.Key
			creates = append(creates, create{d.Aircraft.Title, tailOf(d.Aircraft), initOf(d.Sample), g.req})
		case g.obj == 0 && !g.fallback && now.Sub(g.asked) > fleetRetry && f.opt.Fallback != nil:
			// Its own title refused: once more as the fallback.
			if title := f.opt.Fallback(d.Aircraft); title != "" && title != d.Aircraft.Title {
				delete(f.byReq, g.req)
				g.req, g.asked, g.fallback = f.reqLocked(), now, true
				f.byReq[g.req] = d.Aircraft.Key
				creates = append(creates, create{title, tailOf(d.Aircraft), initOf(d.Sample), g.req})
			}
		case g.ghost != nil:
			place = append(place, struct {
				g *Ghost
				s Sample
			}{g.ghost, d.Sample})
		}
	}
	f.mu.Unlock()
	for _, obj := range remove {
		f.inj.Forget(obj)
		_ = f.client.AIRemoveObject(obj, f.opt.IDBase)
	}
	for _, c := range creates {
		_ = f.client.AICreateNonATCAircraft(c.title, c.tail, c.init, c.req)
	}
	for _, p := range place {
		_ = p.g.Apply(p.s)
	}
}

// dueLocked are the scene's aircraft at now, nearest first, within the
// budget; f.mu held.
func (f *GhostFleet) dueLocked(now time.Time) []SceneSample {
	at := f.scene.At(f.sceneTime(now))
	lat, lon, ok := 0.0, 0.0, false
	if f.opt.Centre != nil {
		lat, lon, ok = f.opt.Centre()
	} else if s, _ := f.player.Sample(now); s.Lat != 0 || s.Lon != 0 {
		lat, lon, ok = s.Lat, s.Lon, true
	}
	if ok {
		sort.SliceStable(at, func(i, j int) bool {
			return calc.HaversineMeters(lat, lon, at[i].Sample.Lat, at[i].Sample.Lon) < calc.HaversineMeters(lat, lon, at[j].Sample.Lat, at[j].Sample.Lon)
		})
	}
	if len(at) > f.opt.Budget {
		at = at[:f.opt.Budget]
	}
	return at
}

// reqLocked is the next request ID; f.mu held.
func (f *GhostFleet) reqLocked() uint32 {
	r := f.opt.IDBase + 1 + f.nextReq%(FleetIDs-1)
	f.nextReq++
	return r
}

func tailOf(a *SceneAircraft) string {
	if a.Callsign != "" {
		return a.Callsign
	}
	return "REPLAY"
}

func initOf(s Sample) types.SIMCONNECT_DATA_INITPOSITION {
	init := types.SIMCONNECT_DATA_INITPOSITION{Latitude: s.Lat, Longitude: s.Lon, Altitude: s.AltFt, Pitch: -s.Pitch, Bank: -s.Bank,
		Heading: s.Heading, Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(s.GS))}
	if s.OnGround {
		init.OnGround = 1
	}
	return init
}

// Handle takes the fleet's object IDs (true), and on the other messages
// brings the fleet to now, at most every frame (false).
func (f *GhostFleet) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil {
		return false
	}
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
		m := msg.AsAssignedObjectID()
		if m == nil {
			return false
		}
		req, obj := uint32(m.DwRequestID), uint32(m.DwObjectID)
		f.mu.Lock()
		key, ours := f.byReq[req]
		g := f.live[key]
		stopped := f.stopped
		f.mu.Unlock()
		if !ours && !(req > f.opt.IDBase && req <= f.opt.IDBase+FleetIDs) {
			return false
		}
		if !ours || g == nil || stopped {
			// Created after it stopped being due: gone again.
			_ = f.client.AIRemoveObject(obj, f.opt.IDBase)
			return true
		}
		if err := f.inj.Takeover(obj); err != nil {
			return true
		}
		f.mu.Lock()
		g.obj, g.ghost = obj, NewGhost(f.inj, obj)
		f.mu.Unlock()
		return true
	}
	now := time.Now()
	f.mu.Lock()
	due := now.Sub(f.placed) >= ghostFrame
	if due {
		f.placed = now
	}
	f.mu.Unlock()
	if due {
		f.Tick(now)
	}
	return false
}

// Stop removes every aircraft of the fleet.
func (f *GhostFleet) Stop() {
	f.mu.Lock()
	f.stopped = true
	var objs []uint32
	for _, g := range f.live {
		if g.obj != 0 {
			objs = append(objs, g.obj)
		}
	}
	f.live, f.byReq = map[string]*fleetGhost{}, map[uint32]string{}
	f.mu.Unlock()
	for _, obj := range objs {
		f.inj.Forget(obj)
		_ = f.client.AIRemoveObject(obj, f.opt.IDBase)
	}
}
