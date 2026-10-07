package world

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestNetLink: over TCP, a director greeting with the token calls the
// actuator's objects and hears its feed; a second director takes over; a
// wrong token is turned away.
func TestNetLink(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	hub := newHubLink()
	defer hub.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				if l, ok := (LinkOptions{Token: "s3cret"}).greeted(context.Background(), c, t.Logf); ok {
					hub.attach(l)
				}
			}()
		}
	}()
	srv := newWireServer()
	srv.add("dep/1", &fakeDep{})
	go serve(hub, srv)

	dial := func(token string) (*wireClient, chan wireMsg) {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		l := newConnLink(c)
		if err := l.enc.Encode(hello{Token: token}); err != nil {
			t.Fatal(err)
		}
		feed := make(chan wireMsg, 8)
		return newWireClient(l, func(m wireMsg) { feed <- m }), feed
	}
	c1, feed1 := dial("s3cret")
	if s := (&remoteDep{c: c1, t: "dep/1"}).State(); s != traffic.TaxiHoldingShort {
		t.Fatalf("state over TCP: %v", s)
	}
	(&wireFeedOut{send: hub.Send}).Paused(true)
	select {
	case m := <-feed1:
		if m.Method != "paused" {
			t.Errorf("fed %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("feed not delivered over TCP")
	}

	c2, _ := dial("s3cret") // takes over
	time.Sleep(100 * time.Millisecond)
	if s := (&remoteDep{c: c2, t: "dep/1"}).State(); s != traffic.TaxiHoldingShort {
		t.Errorf("second director: %v", s)
	}
	select {
	case <-c1.done:
	case <-time.After(2 * time.Second):
		t.Error("the first director was not dropped")
	}

	c3, _ := dial("wrong")
	select {
	case <-c3.done:
	case <-time.After(12 * time.Second):
		t.Error("a wrong token was let in")
	}
}

// TestDialOut: the actuator dials the director (#774): the director greets
// it, calls its objects and hears its feed; a director that drops the link
// is dialled again; a director listening with another token turns it away.
func TestDialOut(t *testing.T) {
	redialEvery = 200 * time.Millisecond
	defer func() { redialEvery = 5 * time.Second }()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	hub := newHubLink()
	defer hub.Close()
	srv := newWireServer()
	srv.add("dep/1", &fakeDep{})
	go serve(hub, srv)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.dialOut(ctx, ln.Addr().String(), LinkOptions{Token: "s3cret"}, func(string, ...any) {})

	accept := func(token string) (*wireClient, chan wireMsg, bool) {
		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		l, ok := LinkOptions{Token: token}.greeted(context.Background(), c, func(string, ...any) {})
		if !ok {
			return nil, nil, false
		}
		feed := make(chan wireMsg, 8)
		return newWireClient(l, func(m wireMsg) { feed <- m }), feed, true
	}
	c1, feed1, ok := accept("s3cret")
	if !ok {
		t.Fatal("the actuator's hello refused")
	}
	if s := (&remoteDep{c: c1, t: "dep/1"}).State(); s != traffic.TaxiHoldingShort {
		t.Fatalf("state over the dialled-out link: %v", s)
	}
	(&wireFeedOut{send: hub.Send}).Paused(true)
	select {
	case m := <-feed1:
		if m.Method != "paused" {
			t.Errorf("fed %+v", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("feed not delivered")
	}
	// The director drops it: the actuator dials again.
	c1.l.Close()
	c2, _, ok := accept("s3cret")
	if !ok {
		t.Fatal("not dialled again")
	}
	if s := (&remoteDep{c: c2, t: "dep/1"}).State(); s != traffic.TaxiHoldingShort {
		t.Errorf("after the redial: %v", s)
	}
	// A director with another token turns it away.
	c2.l.Close()
	if _, _, ok := accept("other"); ok {
		t.Error("a wrong token let in")
	}
}

// A connection's reader stops with its connection; the link and what the
// director sends next go to the next connection's reader (#779: a sim
// reconnect under LinkDirector).
func TestHubOutlivesConnection(t *testing.T) {
	hub := newHubLink()
	defer hub.Close()
	a, b := net.Pipe()
	hub.attach(newConnLink(a))
	dir := newConnLink(b)
	ctx1, cancel1 := context.WithCancel(context.Background())
	cancel1() // the first connection is gone
	if _, err := hub.RecvCtx(ctx1); err == nil {
		t.Fatal("a reader of an ended connection got a message")
	}
	go dir.Send(wireMsg{Kind: wireCall, ID: 7, Method: "State"})
	m, err := hub.RecvCtx(context.Background())
	if err != nil || m.ID != 7 {
		t.Fatalf("the next connection's reader: %+v %v", m, err)
	}
}

// bumper counts the calls it gets.
type bumper struct{ n atomic.Int32 }

func (b *bumper) Bump() error { b.n.Add(1); return nil }

// One director, two actuators (#779): a command reaches both, so each sim
// does the same; the reply comes from the primary; a follower that leaves
// does not stop the director.
func TestFanOut(t *testing.T) {
	a1, b1 := net.Pipe()
	a2, b2 := net.Pipe()
	var p, f bumper
	s1, s2 := newWireServer(), newWireServer()
	s1.add("ctr", &p)
	s2.add("ctr", &f)
	go serve(newConnLink(b1), s1)
	go serve(newConnLink(b2), s2)
	fan := &fanLink{primary: newConnLink(a1)}
	fan.follow(newConnLink(a2))
	c := newWireClient(fan, func(wireMsg) {})
	for range 3 {
		if err := c.call("ctr", "Bump", nil); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for f.n.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if p.n.Load() != 3 || f.n.Load() != 3 {
		t.Fatalf("primary %d, follower %d calls: want 3 each", p.n.Load(), f.n.Load())
	}
	b2.Close() // the follower leaves
	time.Sleep(50 * time.Millisecond)
	if err := c.call("ctr", "Bump", nil); err != nil || p.n.Load() != 4 {
		t.Errorf("after the follower left: %v, primary %d", err, p.n.Load())
	}
}

// A hello larger than helloMaxBytes is refused before any token is looked
// at (#54).
func TestGreetedHelloTooLarge(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return
		}
		defer c.Close()
		big := strings.Repeat("x", 4*helloMaxBytes)
		fmt.Fprintf(c, `{"token":%q}`+"\n", big)
	}()
	c, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := (LinkOptions{Token: "s3cret"}).greeted(context.Background(), c, func(string, ...any) {}); ok {
		t.Error("an oversized hello was taken")
	}
}
