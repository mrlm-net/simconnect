package world

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Late followers (#964). An actuator that follows a director gets the same
// commands as the primary, so its sim flies the same traffic, but only the
// flights started after it joined. The flights already going on when it
// joins are puppets there: the primary records each (flight.Recorder, five
// samples a second), the director relays the samples to the followers that
// joined after the flight began, and each of those creates the aircraft and
// flies it as recorded (flight.Ghost), a moment behind the newest sample so
// it is always between two. When the flight ends on the primary, its puppets
// go.

const (
	// puppetEveryFrames: the primary samples a puppeted aircraft every
	// this many sim frames (5 a second at 60).
	puppetEveryFrames = 12
	// puppetDelay: a puppet flies this far behind the newest sample.
	puppetDelay = time.Second
	// puppetKeep: samples older than this behind the newest are dropped.
	puppetKeep = 10 * time.Second
	// puppetResync: a puppet further behind than this jumps to the newest.
	puppetResync = 3 * time.Second
	// puppetRequests: request IDs for creating and removing puppets.
	puppetRequests = 300
	// puppetFrame: a puppet is placed at most this often.
	puppetFrame = time.Second / 60
)

// Default ID bases (Options.IDBase +2100, +2200): the primary's recorder
// (a definition, a request per aircraft) and the followers' creations.
const (
	puppetRecBase uint32 = 42100
	puppetReqBase uint32 = 42200
)

// puppetStart is a puppet to create on a follower: the primary's flight
// (Target), its model and tail, its first sample as a row of Fields.
type puppetStart struct {
	Target string    `json:"target"`
	Model  string    `json:"model"`
	Tail   string    `json:"tail,omitempty"`
	Fields []string  `json:"fields"`
	Row    []float64 `json:"row"`
}

// ── The primary: its flights recorded for the director ────────────────────

type liveFlight struct {
	ctl         interface{ ObjectID() uint32 }
	model, tail string
	streaming   uint32 // the object recorded, 0 none
}

// addLive notes a flight started here (StartDeparture, StartArrival).
func (a *actuatorSim) addLive(target string, ctl any, model, tail string) {
	c, ok := ctl.(interface{ ObjectID() uint32 })
	if !ok {
		return
	}
	a.mu.Lock()
	if a.live == nil {
		a.live = map[string]*liveFlight{}
	}
	a.live[target] = &liveFlight{ctl: c, model: model, tail: tail}
	a.mu.Unlock()
}

// endLive forgets a flight ended here; one streamed is told ended.
func (a *actuatorSim) endLive(target string) {
	a.mu.Lock()
	f := a.live[target]
	delete(a.live, target)
	delete(a.puppetWant, target)
	rec := a.rec
	a.mu.Unlock()
	if f == nil || f.streaming == 0 {
		return
	}
	if rec != nil {
		rec.Stop(f.streaming)
	}
	a.feedOut("puppetEnd", target)
}

// LiveTargets are the flights this actuator flies now (for a follower
// joining late).
func (a *actuatorSim) LiveTargets() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.live))
	for t := range a.live {
		out = append(out, t)
	}
	return out
}

// StreamPuppets streams the samples of targets to the director (none:
// stops); the samples' field names go first.
func (a *actuatorSim) StreamPuppets(targets []string) error {
	want := map[string]bool{}
	for _, t := range targets {
		want[t] = true
	}
	a.mu.Lock()
	a.puppetWant = want
	a.mu.Unlock()
	if len(want) > 0 {
		a.feedOut("puppetFields", flight.SampleFields())
	}
	a.syncPuppetStream()
	return nil
}

// syncPuppetStream records the flights wanted whose aircraft is in the sim
// now, and stops those no longer wanted (each second, and when asked).
func (a *actuatorSim) syncPuppetStream() {
	a.mu.Lock()
	if len(a.puppetWant) > 0 && a.rec == nil && a.localSim != nil {
		a.rec = flight.NewRecorder(a.localSim.client, a.puppetRec)
		a.recFor = map[uint32]string{}
		a.rec.OnSample = func(obj uint32, s flight.Sample) {
			a.mu.Lock()
			target := a.recFor[obj]
			a.mu.Unlock()
			if target != "" {
				a.feedOut("puppet", target, s.Row())
			}
		}
		a.ctls = append(a.ctls, a.rec)
	}
	rec := a.rec
	type change struct {
		target string
		f      *liveFlight
		start  bool
	}
	var changes []change
	for t, f := range a.live {
		switch {
		case a.puppetWant[t] && f.streaming == 0 && f.ctl.ObjectID() != 0:
			changes = append(changes, change{t, f, true})
		case !a.puppetWant[t] && f.streaming != 0:
			changes = append(changes, change{t, f, false})
		}
	}
	a.mu.Unlock()
	if rec == nil {
		return
	}
	for _, c := range changes {
		if !c.start {
			rec.Stop(c.f.streaming)
			a.mu.Lock()
			delete(a.recFor, c.f.streaming)
			c.f.streaming = 0
			a.mu.Unlock()
			continue
		}
		obj := c.f.ctl.ObjectID()
		a.feedOut("puppetMeta", c.target, c.f.model, c.f.tail)
		if err := rec.Start(obj, flight.RecordOptions{Title: c.f.model, EveryFrames: puppetEveryFrames}); err != nil {
			continue
		}
		a.mu.Lock()
		c.f.streaming = obj
		a.recFor[obj] = c.target
		a.mu.Unlock()
	}
}

// feedOut sends a feed message of method with args to the director.
func (a *actuatorSim) feedOut(method string, args ...any) {
	m := wireMsg{Kind: wireFeed, Method: method}
	for _, v := range args {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		m.Args = append(m.Args, b)
	}
	if a.send != nil {
		_ = a.send(m)
	}
}

// ── The director: the samples relayed to the late followers ───────────────

// puppetRelay is the director's side: which followers puppet which of the
// primary's flights, and what it needs to start a puppet.
type puppetRelay struct {
	fan  *fanLink
	call func(method string, args []any, outs ...any) error
	logf func(string, ...any)

	mu      sync.Mutex
	fields  []string
	meta    map[string][2]string          // target → model, tail
	wants   map[*connLink]map[string]bool // follower → flights to puppet
	started map[*connLink]map[string]bool // follower → puppets started
}

func newPuppetRelay(fan *fanLink, call func(method string, args []any, outs ...any) error, logf func(string, ...any)) *puppetRelay {
	r := &puppetRelay{fan: fan, call: call, logf: logf, meta: map[string][2]string{},
		wants: map[*connLink]map[string]bool{}, started: map[*connLink]map[string]bool{}}
	fan.mu.Lock()
	fan.onFollow, fan.onDrop = r.joined, r.left
	fan.mu.Unlock()
	return r
}

// joined: a follower attached; the primary's flights now are its puppets.
func (r *puppetRelay) joined(l *connLink) {
	go func() {
		var live []string
		if err := r.call("LiveTargets", nil, &live); err != nil {
			r.logf("director: late follower: flights going on: %v", err)
			return
		}
		r.mu.Lock()
		want := map[string]bool{}
		for _, t := range live {
			want[t] = true
		}
		r.wants[l], r.started[l] = want, map[string]bool{}
		r.mu.Unlock()
		if len(live) > 0 {
			r.logf("director: late follower: %d flights going on flown as puppets there", len(live))
		}
		r.stream()
	}()
}

// left: a follower gone.
func (r *puppetRelay) left(l *connLink) {
	r.mu.Lock()
	_, had := r.wants[l]
	delete(r.wants, l)
	delete(r.started, l)
	r.mu.Unlock()
	if had {
		r.stream()
	}
}

// stream asks the primary for the flights any follower puppets.
func (r *puppetRelay) stream() {
	r.mu.Lock()
	set := map[string]bool{}
	for _, w := range r.wants {
		for t := range w {
			set[t] = true
		}
	}
	r.mu.Unlock()
	targets := make([]string, 0, len(set))
	for t := range set {
		targets = append(targets, t)
	}
	if err := r.call("StreamPuppets", []any{targets}); err != nil {
		r.logf("director: puppets: %v", err)
	}
}

// feed takes the primary's puppet feed; false when m is none of it.
func (r *puppetRelay) feed(m wireMsg) bool {
	str := func(i int) string {
		var s string
		if i < len(m.Args) {
			json.Unmarshal(m.Args[i], &s)
		}
		return s
	}
	switch m.Method {
	case "puppetFields":
		var f []string
		if len(m.Args) > 0 && json.Unmarshal(m.Args[0], &f) == nil {
			r.mu.Lock()
			r.fields = f
			r.mu.Unlock()
		}
	case "puppetMeta":
		r.mu.Lock()
		r.meta[str(0)] = [2]string{str(1), str(2)}
		r.mu.Unlock()
	case "puppet":
		target := str(0)
		var row []float64
		if len(m.Args) < 2 || json.Unmarshal(m.Args[1], &row) != nil {
			return true
		}
		r.mu.Lock()
		var sends []struct {
			l   *connLink
			msg wireMsg
		}
		for l, want := range r.wants {
			if !want[target] {
				continue
			}
			if r.started[l][target] {
				sends = append(sends, struct {
					l   *connLink
					msg wireMsg
				}{l, puppetCall("PuppetPose", target, row)})
				continue
			}
			meta, ok := r.meta[target]
			if !ok || r.fields == nil {
				continue
			}
			r.started[l][target] = true
			sends = append(sends, struct {
				l   *connLink
				msg wireMsg
			}{l, puppetCall("PuppetStart", puppetStart{Target: target, Model: meta[0], Tail: meta[1], Fields: r.fields, Row: row})})
		}
		r.mu.Unlock()
		for _, s := range sends {
			if s.l.Send(s.msg) != nil {
				r.fan.drop(s.l)
			}
		}
	case "puppetEnd":
		target := str(0)
		r.mu.Lock()
		var ends []*connLink
		for l, want := range r.wants {
			if want[target] {
				delete(want, target)
				if r.started[l][target] {
					ends = append(ends, l)
					delete(r.started[l], target)
				}
			}
		}
		delete(r.meta, target)
		r.mu.Unlock()
		for _, l := range ends {
			_ = l.Send(puppetCall("PuppetEnd", target))
		}
	default:
		return false
	}
	return true
}

// puppetCall is a call to a follower's sim port; its reply is dropped.
func puppetCall(method string, args ...any) wireMsg {
	m := wireMsg{Kind: wireCall, Target: "sim", Method: method}
	for _, a := range args {
		b, _ := json.Marshal(a)
		m.Args = append(m.Args, b)
	}
	return m
}

// ── A follower: the puppets flown ─────────────────────────────────────────

// puppet is one of the primary's flights flown here as recorded.
type puppet struct {
	client engine.Client
	inj    *traffic.Injector
	req    uint32
	decode func([]float64) flight.Sample

	mu      sync.Mutex
	track   flight.Track
	offset  float64 // wall seconds − sample time, set by the first sample
	obj     uint32
	ghost   *flight.Ghost
	placed  time.Time
	removed bool
}

// PuppetStart creates one of the primary's flights here as a puppet.
func (a *actuatorSim) PuppetStart(p puppetStart) error {
	if a.localSim == nil {
		return nil
	}
	a.mu.Lock()
	if a.puppets == nil {
		a.puppets = map[string]*puppet{}
	}
	if _, ok := a.puppets[p.Target]; ok {
		a.mu.Unlock()
		return nil
	}
	a.puppetSeq++
	pp := &puppet{client: a.localSim.client, inj: a.localSim.inj, req: a.puppetReq + a.puppetSeq%puppetRequests, decode: flight.RowDecoder(p.Fields)}
	a.puppets[p.Target] = pp
	a.ctls = append(a.ctls, pp)
	a.mu.Unlock()
	s := pp.decode(p.Row)
	pp.add(s)
	tail := p.Tail
	if tail == "" {
		tail = "PUPPET"
	}
	init := types.SIMCONNECT_DATA_INITPOSITION{Latitude: s.Lat, Longitude: s.Lon, Altitude: s.AltFt, Pitch: -s.Pitch, Bank: -s.Bank,
		Heading: s.Heading, Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(s.GS)}
	if s.OnGround {
		init.OnGround = 1
	}
	return pp.client.AICreateNonATCAircraft(p.Model, tail, init, pp.req)
}

// PuppetPose is a puppet's next sample.
func (a *actuatorSim) PuppetPose(target string, row []float64) error {
	a.mu.Lock()
	pp := a.puppets[target]
	a.mu.Unlock()
	if pp != nil {
		pp.add(pp.decode(row))
	}
	return nil
}

// PuppetEnd removes a puppet: its flight ended on the primary.
func (a *actuatorSim) PuppetEnd(target string) error {
	a.mu.Lock()
	pp := a.puppets[target]
	delete(a.puppets, target)
	a.mu.Unlock()
	if pp == nil {
		return nil
	}
	a.drop(pp)
	pp.remove()
	return nil
}

// add keeps a sample, the oldest beyond puppetKeep dropped.
func (p *puppet) add(s flight.Sample) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.track.Samples) == 0 {
		p.offset = float64(time.Now().UnixNano())/1e9 - s.T
	}
	if n := len(p.track.Samples); n > 0 && s.T <= p.track.Samples[n-1].T {
		return // out of order or repeated
	}
	p.track.Samples = append(p.track.Samples, s)
	keep := puppetKeep.Seconds()
	i := 0
	for i < len(p.track.Samples)-2 && s.T-p.track.Samples[i].T > keep {
		i++
	}
	p.track.Samples = p.track.Samples[i:]
}

// at is the sample to fly at now: puppetDelay behind the newest by the
// wall clock, jumping on when it fell behind.
func (p *puppet) at(now time.Time) (flight.Sample, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.track.Samples)
	if n == 0 {
		return flight.Sample{}, false
	}
	newest := p.track.Samples[n-1].T
	t := float64(now.UnixNano())/1e9 - p.offset - puppetDelay.Seconds()
	if newest-t > puppetResync.Seconds() {
		p.offset = float64(now.UnixNano())/1e9 - newest
		t = newest - puppetDelay.Seconds()
	}
	return p.track.At(t)
}

// Handle takes its creation's object ID, and flies it on each message at
// most every puppetFrame.
func (p *puppet) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil {
		return false
	}
	p.mu.Lock()
	removed, obj := p.removed, p.obj
	p.mu.Unlock()
	if removed {
		return false
	}
	if obj == 0 {
		if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID {
			return false
		}
		m := msg.AsAssignedObjectID()
		if m == nil || uint32(m.DwRequestID) != p.req {
			return false
		}
		obj = uint32(m.DwObjectID)
		if err := p.inj.Takeover(obj); err != nil {
			return true
		}
		p.mu.Lock()
		p.obj, p.ghost = obj, flight.NewGhost(p.inj, obj)
		p.mu.Unlock()
		return true
	}
	now := time.Now()
	p.mu.Lock()
	due := now.Sub(p.placed) >= puppetFrame
	if due {
		p.placed = now
	}
	g := p.ghost
	p.mu.Unlock()
	if due && g != nil {
		if s, ok := p.at(now); ok {
			_ = g.Apply(s)
		}
	}
	return false // the message is still everyone's
}

// remove takes the puppet out of the sim.
func (p *puppet) remove() {
	p.mu.Lock()
	p.removed = true
	obj := p.obj
	p.mu.Unlock()
	if obj != 0 {
		p.inj.Forget(obj)
		_ = p.client.AIRemoveObject(obj, p.req)
	}
}
