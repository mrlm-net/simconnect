package world

import (
	"bytes"
	"encoding/json"
	"reflect"
)

// The director's reads of a controller's state answered from the actuator's
// snapshot instead of a call each (#710): the actuator sends, each second,
// the results of each controller's reads that take no arguments
// (cachedReads); a command to a controller drops its snapshot, and a
// snapshot that comes while a command is on its way is not taken (the link
// is ordered: the next one is from after it).

// cachedReads are the reads answered from the snapshot. VectorDue is not
// one: it hands the vector out.
var cachedReads = map[string]bool{
	"State": true, "Route": true, "PushFacingSaid": true, "FacesOut": true,
	"Plan": true, "ProcedurePlan": true, "ProcedureRoute": true, "Holding": true,
	"TouchAndGosLeft": true, "TurningFinal": true, "CircuitFixes": true, "InterceptHeading": true,
}

// readsWithArgs read and change nothing, but take arguments: called each
// time, they do not drop the snapshot.
var readsWithArgs = map[string]bool{"ClimbPlan": true, "ClimbRoute": true, "HoldFix": true}

// fromCache answers target's method from its snapshot into outs.
func (c *wireClient) fromCache(target, method string, outs []any) bool {
	if !cachedReads[method] {
		return false
	}
	c.mu.Lock()
	r, ok := c.cache[target][method]
	c.mu.Unlock()
	if !ok {
		return false
	}
	for i, out := range outs {
		if i < len(r) && out != nil {
			if json.Unmarshal(r[i], out) != nil {
				return false
			}
		}
	}
	return true
}

// commanding marks a command to target on its way: its snapshot dropped
// until the reply; done ends it.
func (c *wireClient) commanding(target, method string) (done func()) {
	if target == "sim" || cachedReads[method] || readsWithArgs[method] {
		return func() {}
	}
	c.mu.Lock()
	delete(c.cache, target)
	c.inflight[target]++
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		if c.inflight[target]--; c.inflight[target] <= 0 {
			delete(c.inflight, target)
		}
		c.mu.Unlock()
	}
}

// takeSnapshot keeps the controllers' snapshots of a "ctlstate" feed, but
// not of a controller a command is on its way to.
func (c *wireClient) takeSnapshot(m wireMsg) {
	if len(m.Args) == 0 {
		return
	}
	if len(m.Args) > 1 {
		c.takeDelta(m.Args[1]) // a delta (E24)
		return
	}
	var snap map[string]map[string][]json.RawMessage
	if json.Unmarshal(m.Args[0], &snap) != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for t, reads := range snap {
		if c.inflight[t] == 0 {
			c.cache[t] = reads
		}
	}
	for t := range c.cache {
		if _, ok := snap[t]; !ok {
			delete(c.cache, t) // gone on the actuator
		}
	}
}

// snapshot is the actuator's controllers' cached reads now, by target.
func (s *wireServer) snapshot() map[string]map[string][]json.RawMessage {
	s.mu.Lock()
	targets := make(map[string]any, len(s.targets))
	for t, o := range s.targets {
		if t != "sim" {
			targets[t] = o
		}
	}
	s.mu.Unlock()
	out := map[string]map[string][]json.RawMessage{}
	for t, o := range targets {
		v := reflect.ValueOf(o)
		reads := map[string][]json.RawMessage{}
		for name := range cachedReads {
			fn := v.MethodByName(name)
			if !fn.IsValid() || fn.Type().NumIn() != 0 {
				continue
			}
			var res []json.RawMessage
			ok := true
			for _, r := range safeCall(fn) {
				if r.Type() == errorType {
					continue
				}
				b, err := json.Marshal(r.Interface())
				if err != nil {
					ok = false
					break
				}
				res = append(res, b)
			}
			if ok && res != nil {
				reads[name] = res
			}
		}
		out[t] = reads
	}
	return out
}

// Deltas (review E24): the actuator sends the whole snapshot every
// ctlFullEvery seconds and on a new link, and in between only the reads that
// changed and the controllers gone, as a "ctlstate" feed with an empty array
// first and the delta second. A director from before takes the first
// argument for a snapshot, fails to read an array as one and leaves its
// cache as it is: it keeps the whole snapshots.
const ctlFullEvery = 10

// ctlDelta is what changed between two snapshots.
type ctlDelta struct {
	Set  map[string]map[string][]json.RawMessage `json:"set,omitempty"`
	Gone []string                                `json:"gone,omitempty"`
}

// ctlDiff is the delta from last to now; false when nothing changed.
func ctlDiff(last, now map[string]map[string][]json.RawMessage) (ctlDelta, bool) {
	var d ctlDelta
	for t, reads := range now {
		for name, res := range reads {
			if sameResults(last[t][name], res) {
				continue
			}
			if d.Set == nil {
				d.Set = map[string]map[string][]json.RawMessage{}
			}
			if d.Set[t] == nil {
				d.Set[t] = map[string][]json.RawMessage{}
			}
			d.Set[t][name] = res
		}
	}
	for t := range last {
		if _, ok := now[t]; !ok {
			d.Gone = append(d.Gone, t)
		}
	}
	return d, d.Set != nil || d.Gone != nil
}

func sameResults(a, b []json.RawMessage) bool {
	if len(a) != len(b) || a == nil {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// ctlSender sends the controllers' snapshots over one link: whole or as a
// delta (put: the feed's put).
type ctlSender struct {
	last map[string]map[string][]json.RawMessage
	n    int
	// attached, when set, counts the directors attached to the link (a
	// hub's): a new one gets the whole snapshot at once.
	attached func() uint64
	seen     uint64
}

func (s *ctlSender) send(now map[string]map[string][]json.RawMessage, put func(kind string, vs ...any)) {
	fresh := false
	if s.attached != nil {
		if a := s.attached(); a != s.seen {
			s.seen, fresh = a, true
		}
	}
	switch {
	case fresh || s.last == nil || s.n%ctlFullEvery == 0:
		put("ctlstate", now)
	default:
		if d, ok := ctlDiff(s.last, now); ok {
			put("ctlstate", []any{}, d)
		}
	}
	s.last = now
	s.n++
}

// takeDelta applies a delta to the snapshots kept, but not to a controller
// a command is on its way to (its snapshot dropped; a read missing is
// asked over the link).
func (c *wireClient) takeDelta(raw json.RawMessage) {
	var d ctlDelta
	if json.Unmarshal(raw, &d) != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for t, reads := range d.Set {
		if c.inflight[t] != 0 {
			continue
		}
		if c.cache[t] == nil {
			c.cache[t] = map[string][]json.RawMessage{}
		}
		for name, res := range reads {
			c.cache[t][name] = res
		}
	}
	for _, t := range d.Gone {
		delete(c.cache, t)
	}
}

// safeCall calls fn with no arguments; nil when it panics (a read that
// fails is left out of the snapshot, not the actuator's end).
func safeCall(fn reflect.Value) (out []reflect.Value) {
	defer func() {
		if recover() != nil {
			out = nil
		}
	}()
	return fn.Call(nil)
}
