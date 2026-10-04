package world

import (
	"net"
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
				if l, ok := greeted(c, "s3cret"); ok {
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
