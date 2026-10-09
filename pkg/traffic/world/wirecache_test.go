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

// TestCtlDelta (E24): whole snapshots every ctlFullEvery sends, deltas of
// the changed reads and the gone controllers between; the director's cache
// ends as the snapshot; a delta taken as a whole snapshot by a director
// from before is refused, its cache kept.
func TestCtlDelta(t *testing.T) {
	raw := func(s string) []json.RawMessage { return []json.RawMessage{json.RawMessage(s)} }
	snap1 := map[string]map[string][]json.RawMessage{
		"dep/1": {"State": raw(`3`), "Route": raw(`["A","B"]`)},
		"arr/2": {"State": raw(`1`)},
	}
	snap2 := map[string]map[string][]json.RawMessage{
		"dep/1": {"State": raw(`4`), "Route": raw(`["A","B"]`)},
	}
	var sent []wireMsg
	put := func(kind string, vs ...any) {
		m := wireMsg{Kind: wireFeed, Method: kind}
		for _, v := range vs {
			b, _ := json.Marshal(v)
			m.Args = append(m.Args, b)
		}
		sent = append(sent, m)
	}
	var s ctlSender
	s.send(snap1, put)
	s.send(snap1, put) // nothing changed: nothing sent
	s.send(snap2, put)
	if len(sent) != 2 || len(sent[0].Args) != 1 || len(sent[1].Args) != 2 {
		t.Fatalf("sent %d messages %+v, want a whole snapshot then a delta", len(sent), sent)
	}
	if d := string(sent[1].Args[1]); d != `{"set":{"dep/1":{"State":[4]}},"gone":["arr/2"]}` {
		t.Errorf("delta %s", d)
	}

	c := &wireClient{cache: map[string]map[string][]json.RawMessage{}, inflight: map[string]int{}}
	for _, m := range sent {
		c.takeSnapshot(m)
	}
	if got := string(c.cache["dep/1"]["State"][0]); got != "4" {
		t.Errorf("dep/1 State %s, want 4", got)
	}
	if got := string(c.cache["dep/1"]["Route"][0]); got != `["A","B"]` {
		t.Errorf("dep/1 Route %s, kept from the whole snapshot", got)
	}
	if _, ok := c.cache["arr/2"]; ok {
		t.Error("arr/2 gone on the actuator, still cached")
	}

	// A director from before reads only the first argument.
	var old map[string]map[string][]json.RawMessage
	if json.Unmarshal(sent[1].Args[0], &old) == nil {
		t.Error("a delta's first argument reads as a whole snapshot: an old director would drop its cache")
	}

	for range ctlFullEvery - 2 { // sends 4 to 11: the 11th whole
		s.send(snap2, put)
	}
	if last := sent[len(sent)-1]; len(last.Args) != 1 {
		t.Errorf("no whole snapshot after %d sends", ctlFullEvery)
	}
}

// TestCtlDeltaReattach: a director attached again gets the whole snapshot
// at the next send, not only what changed.
func TestCtlDeltaReattach(t *testing.T) {
	snap := map[string]map[string][]json.RawMessage{"dep/1": {"State": []json.RawMessage{json.RawMessage(`3`)}}}
	var args []int
	put := func(kind string, vs ...any) { args = append(args, len(vs)) }
	attached := uint64(1)
	s := ctlSender{attached: func() uint64 { return attached }}
	s.send(snap, put)
	snap = map[string]map[string][]json.RawMessage{"dep/1": {"State": []json.RawMessage{json.RawMessage(`4`)}}}
	s.send(snap, put)
	attached++
	snap = map[string]map[string][]json.RawMessage{"dep/1": {"State": []json.RawMessage{json.RawMessage(`5`)}}}
	s.send(snap, put)
	if len(args) != 3 || args[0] != 1 || args[1] != 2 || args[2] != 1 {
		t.Errorf("sends %v, want whole, delta, whole", args)
	}
}
