package world

import (
	"math"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Moments: the instants worth a cut, on any aircraft the World sees. Ours say
// them step by step as their take-off or landing goes (the controllers'
// Sequence); the player's and other traffic's are read off the scan, so they come
// at its rate and only those the simulator shows: lift-off, touchdown, gear up,
// vacated.

// The moments.
const (
	MomentFlare     = "flare"
	MomentTouchdown = "touchdown"
	MomentReversers = "reversers"
	MomentVacated   = "vacated"
	MomentRotate    = "rotate"
	MomentLiftOff   = "liftoff"
	MomentGearUp    = "gearup"
)

// Moment is one of them happening. Seq counts them from 1, for GET
// /api/camera/moments?since=SEQ.
type Moment struct {
	Seq      uint64    `json:"seq"`
	Name     string    `json:"name"`
	Tail     string    `json:"tail"`
	ObjectID uint32    `json:"objectId"`
	At       time.Time `json:"at"`
	User     bool      `json:"user,omitempty"`
	Ours     bool      `json:"ours,omitempty"`
}

const (
	// momentsKept: how many stay for those who ask; momentQueue how many wait
	// for a slow host, more are dropped.
	momentsKept = 256
	momentQueue = 256
	// momentFlewFt: this high it has flown, so a bounce is no lift-off.
	momentFlewFt = 20.0
	// momentOffLineM: this far off the line it touched down on, it has left the
	// runway. Past the edge of the widest one, wings clear of it.
	momentOffLineM = 50.0
)

// isMoment says name is one of the moments.
func isMoment(name string) bool {
	switch name {
	case MomentFlare, MomentTouchdown, MomentReversers, MomentVacated, MomentRotate, MomentLiftOff, MomentGearUp:
		return true
	}
	return false
}

// momentOfStep is the moment a controller's sequence step is, "" if none.
func momentOfStep(step string) string {
	for _, m := range []struct{ prefix, name string }{
		{"flare", MomentFlare}, {"touchdown", MomentTouchdown}, {"reverse thrust", MomentReversers},
		{"vacated", MomentVacated}, {"rotate", MomentRotate}, {"lift-off", MomentLiftOff}, {"gear up", MomentGearUp},
	} {
		if strings.HasPrefix(step, m.prefix) {
			return m.name
		}
	}
	return ""
}

// momentTrack is what the scan said of an aircraft last time.
type momentTrack struct {
	flying bool
	gear   float64
	// rolling: touched down from at heading, until it leaves that line.
	rolling bool
	from    airport.LatLon
	heading float64
}

// momentLog keeps the moments and hands them to the host.
type momentLog struct {
	mu     sync.Mutex
	seq    uint64
	kept   []Moment
	tracks map[uint32]momentTrack
	// queue: to the host, once one listens.
	queue chan Moment
	once  sync.Once
	on    atomic.Pointer[func(Moment)]
}

func newMomentLog() *momentLog {
	return &momentLog{tracks: map[uint32]momentTrack{}}
}

// listen hands f the moments from now on, nil for none.
func (l *momentLog) listen(f func(Moment)) {
	if f == nil {
		l.on.Store(nil)
		return
	}
	l.on.Store(&f)
	l.once.Do(func() {
		q := make(chan Moment, momentQueue)
		l.mu.Lock()
		l.queue = q
		l.mu.Unlock()
		go func() {
			for m := range q {
				if f := l.on.Load(); f != nil {
					(*f)(m)
				}
			}
		}()
	})
}

func (l *momentLog) add(m Moment) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	m.Seq = l.seq
	l.kept = append(l.kept, m)
	if len(l.kept) > momentsKept {
		l.kept = slices.Delete(l.kept, 0, len(l.kept)-momentsKept)
	}
	if l.queue != nil {
		select {
		case l.queue <- m:
		default:
		}
	}
}

// since is what came after seq, oldest first.
func (l *momentLog) since(seq uint64) []Moment {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	i, _ := slices.BinarySearchFunc(l.kept, seq+1, func(m Moment, s uint64) int {
		switch {
		case m.Seq < s:
			return -1
		case m.Seq > s:
			return 1
		}
		return 0
	})
	return slices.Clone(l.kept[i:])
}

// last is the latest Seq.
func (l *momentLog) last() uint64 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.seq
}

// scan reads the moments of what is not ours off a labelled scan. The first look
// at an aircraft only notes where it is.
func (l *momentLog) scan(now time.Time, list []Traffic) {
	if l == nil {
		return
	}
	type found struct {
		t    *Traffic
		name string
	}
	var out []found
	l.mu.Lock()
	seen := make(map[uint32]momentTrack, len(list))
	for i := range list {
		t := &list[i]
		if t.Ours {
			continue
		}
		pos := airport.LatLon{Lat: t.Latitude, Lon: t.Longitude}
		was, known := l.tracks[t.ObjectID]
		h := was
		h.gear = t.Gear
		switch {
		case !known:
			h = momentTrack{flying: !t.OnGround && t.AGL >= momentFlewFt, gear: t.Gear}
		case was.flying && t.OnGround:
			h.flying, h.rolling, h.from, h.heading = false, true, pos, t.Heading
			out = append(out, found{t, MomentTouchdown})
		case !was.flying && !t.OnGround && t.AGL >= momentFlewFt:
			h.flying, h.rolling = true, false
			out = append(out, found{t, MomentLiftOff})
		}
		if known && h.flying && was.gear > 0 && t.Gear <= 0 {
			out = append(out, found{t, MomentGearUp})
		}
		if h.rolling && !h.flying && offLine(h.from, h.heading, pos) > momentOffLineM {
			h.rolling = false
			out = append(out, found{t, MomentVacated})
		}
		seen[t.ObjectID] = h
	}
	l.tracks = seen
	l.mu.Unlock()
	for _, f := range out {
		l.add(Moment{Name: f.name, Tail: f.t.Tail, ObjectID: f.t.ObjectID, At: now, User: f.t.User})
	}
}

// offLine is how far p is off the line from a at heading, meters.
func offLine(a airport.LatLon, heading float64, p airport.LatLon) float64 {
	d := calc.HaversineMeters(a.Lat, a.Lon, p.Lat, p.Lon)
	return math.Abs(d * math.Sin((calc.BearingDegrees(a.Lat, a.Lon, p.Lat, p.Lon)-heading)*math.Pi/180))
}

// stepMoments logs the moments in the steps of ours it has not read yet: count is
// how many there are now, fewer when a new sequence began. Under it.mu.
func (it *controlled) stepMoments(count int, read func() []traffic.SequenceStep) {
	if count == it.stepsRead {
		return
	}
	list := read()
	if len(list) < it.stepsRead {
		it.stepsRead = 0
	}
	now := it.cc.clock.Now()
	for _, s := range list[it.stepsRead:] {
		if name := momentOfStep(s.Step); name != "" {
			it.cc.core.moments.add(Moment{Name: name, Tail: it.view.Tail, ObjectID: it.objectID, At: now, Ours: true})
		}
	}
	it.stepsRead = len(list)
}

// OnMoment hands f each moment as it happens, in order, on a goroutine of its own;
// nil for none. A host too slow for many at once loses the excess, which
// GET /api/camera/moments still has.
func (w *World) OnMoment(f func(Moment)) { w.st.core.moments.listen(f) }
