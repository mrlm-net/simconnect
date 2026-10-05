package world

import (
	"encoding/json"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestWireCache: a read is answered from the actuator's snapshot without a
// call; a command drops the snapshot, and one taken during the command is
// not kept.
func TestWireCache(t *testing.T) {
	dir, act := pipe()
	defer dir.Close()
	srv := newWireServer()
	srv.add("dep/1", &fakeDep{})
	go serve(act, srv)
	c := newWireClient(dir, nil)
	d := &remoteDep{c: c, t: "dep/1"}

	b, _ := json.Marshal(srv.snapshot())
	c.takeSnapshot(wireMsg{Kind: wireFeed, Method: "ctlstate", Args: []json.RawMessage{b}})
	srv.remove("dep/1") // a call would fail now: the read must come from the snapshot
	if s := d.State(); s != traffic.TaxiHoldingShort {
		t.Fatalf("state from the snapshot: %v", s)
	}

	srv.add("dep/1", &fakeDep{})
	done := c.commanding("dep/1", "ChangeEntry")
	c.takeSnapshot(wireMsg{Kind: wireFeed, Method: "ctlstate", Args: []json.RawMessage{b}}) // during the command
	c.mu.Lock()
	_, cached := c.cache["dep/1"]
	c.mu.Unlock()
	if cached {
		t.Error("a snapshot taken during a command was kept")
	}
	done()
	c.takeSnapshot(wireMsg{Kind: wireFeed, Method: "ctlstate", Args: []json.RawMessage{b}})
	c.mu.Lock()
	_, cached = c.cache["dep/1"]
	c.mu.Unlock()
	if !cached {
		t.Error("the snapshot after the command was not kept")
	}
	if !cachedReads["State"] || cachedReads["VectorDue"] {
		t.Error("VectorDue hands the vector out: never from a snapshot")
	}
}
