package world

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/flight"
)

// primarySim is the primary's sim port for the relay: its flights going on
// and what it was asked to stream.
type primarySim struct {
	mu       sync.Mutex
	streamed [][]string
}

func (p *primarySim) LiveTargets() []string { return []string{"dep/1/1"} }
func (p *primarySim) StreamPuppets(t []string) error {
	p.mu.Lock()
	p.streamed = append(p.streamed, t)
	p.mu.Unlock()
	return nil
}

// followerSim is a late follower's sim port: the puppet calls it got.
type followerSim struct {
	mu     sync.Mutex
	calls  []string
	start  puppetStart
	poses  int
	ended  string
	stream int
}

func (f *followerSim) PuppetStart(p puppetStart) error {
	f.mu.Lock()
	f.calls, f.start = append(f.calls, "start"), p
	f.mu.Unlock()
	return nil
}
func (f *followerSim) PuppetPose(string, []float64) error {
	f.mu.Lock()
	f.calls, f.poses = append(f.calls, "pose"), f.poses+1
	f.mu.Unlock()
	return nil
}
func (f *followerSim) PuppetEnd(target string) error {
	f.mu.Lock()
	f.calls, f.ended = append(f.calls, "end"), target
	f.mu.Unlock()
	return nil
}
func (f *followerSim) StreamPuppets([]string) error {
	f.mu.Lock()
	f.stream++
	f.mu.Unlock()
	return nil
}
func (f *followerSim) LiveTargets() []string { return nil }

func feedMsg(method string, args ...any) wireMsg {
	m := wireMsg{Kind: wireFeed, Method: method}
	for _, a := range args {
		b, _ := json.Marshal(a)
		m.Args = append(m.Args, b)
	}
	return m
}

// TestPuppetRelay (#964): a follower joining late is given the primary's
// flights going on: the primary (only) is asked to stream them; its first
// sample starts the puppet with the fields and model, the next move it, its
// end ends it; the follower gone, the stream stops.
func TestPuppetRelay(t *testing.T) {
	a1, b1 := net.Pipe()
	a2, b2 := net.Pipe()
	var p primarySim
	var f followerSim
	s1, s2 := newWireServer(), newWireServer()
	s1.add("sim", &p)
	s2.add("sim", &f)
	go serve(newConnLink(b1), s1)
	go serve(newConnLink(b2), s2)
	fan := &fanLink{primary: newConnLink(a1)}
	c := newWireClient(fan, func(wireMsg) {})
	relay := newPuppetRelay(fan, func(method string, args []any, outs ...any) error {
		return c.callVia(fan.primary.Send, "sim", method, args, outs...)
	}, t.Logf)
	fan.follow(newConnLink(a2))
	wait := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("waiting for %s", what)
			}
		}
	}
	wait("the stream asked", func() bool { p.mu.Lock(); defer p.mu.Unlock(); return len(p.streamed) == 1 })
	if got := p.streamed[0]; len(got) != 1 || got[0] != "dep/1/1" {
		t.Fatalf("streamed %v", got)
	}
	row := flight.Sample{T: 10, Lat: 50.1, Lon: 14.2, AltFt: 1200}.Row()
	relay.feed(feedMsg("puppetFields", flight.SampleFields()))
	relay.feed(feedMsg("puppetMeta", "dep/1/1", "FSLTL A320 CSA", "OK-NEM"))
	relay.feed(feedMsg("puppet", "dep/1/1", row))
	relay.feed(feedMsg("puppet", "dep/1/1", row))
	relay.feed(feedMsg("puppet", "dep/9/9", row)) // started after it joined: its own
	wait("start and pose", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.calls) == 2 })
	if f.start.Model != "FSLTL A320 CSA" || f.start.Tail != "OK-NEM" || len(f.start.Fields) == 0 || f.poses != 1 {
		t.Errorf("start %+v, poses %d", f.start, f.poses)
	}
	if s := flight.RowDecoder(f.start.Fields)(f.start.Row); s.Lat != 50.1 || s.AltFt != 1200 {
		t.Errorf("first sample %+v", s)
	}
	relay.feed(feedMsg("puppetEnd", "dep/1/1"))
	wait("end", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.ended == "dep/1/1" })
	if f.stream != 0 {
		t.Error("the follower was asked to stream")
	}
	fan.drop(fan.followers[0])
	wait("the stream stopped", func() bool { p.mu.Lock(); defer p.mu.Unlock(); return len(p.streamed) == 2 && len(p.streamed[1]) == 0 })
}

// TestPuppetTiming: a puppet flies puppetDelay behind the newest sample,
// jumps on when it falls further behind than puppetResync, and keeps
// puppetKeep of samples.
func TestPuppetTiming(t *testing.T) {
	p := &puppet{}
	for i := range 101 {
		p.add(flight.Sample{T: float64(i) * 0.2, AltFt: float64(i) * 20})
	}
	if n := len(p.track.Samples); n > int(puppetKeep.Seconds()/0.2)+3 {
		t.Errorf("%d samples kept", n)
	}
	newest := 20.0
	now := time.Unix(0, int64((p.offset+newest)*1e9))
	s, ok := p.at(now)
	if !ok || s.T < newest-puppetDelay.Seconds()-0.01 || s.T > newest-puppetDelay.Seconds()+0.01 {
		t.Errorf("at the newest's time: sample at %v, want %v", s.T, newest-puppetDelay.Seconds())
	}
	p.add(flight.Sample{T: 30}) // a jump: far behind now
	s, _ = p.at(now)
	if s.T < 29 {
		t.Errorf("behind by %v s: not jumped on", 30-s.T)
	}
	p.add(flight.Sample{T: 29}) // older: dropped
	if p.track.Samples[len(p.track.Samples)-1].T != 30 {
		t.Error("an older sample taken")
	}
}
