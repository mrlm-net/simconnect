package world

//go:generate node gen/remote.js .

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// The wire between a director (the World's decisions, anywhere) and an
// actuator (its simulator side, beside the simulator) (#710, option 3).
// The director calls methods of the actuator's objects — the sim port
// ("sim") and each aircraft's controller ("dep/3", "arr/5") — and the
// actuator sends back their replies, the controllers' events and what the
// simulator tells (the feed).

// wireMsg is one message on a link.
type wireMsg struct {
	Kind   string            `json:"k"`            // call, reply, event, feed
	ID     uint64            `json:"id,omitempty"` // a call and its reply
	Target string            `json:"t,omitempty"`  // "sim", "dep/3", "arr/5"; an event's or feed's source
	Method string            `json:"m,omitempty"`  // a call's method; a feed's kind
	Args   []json.RawMessage `json:"a,omitempty"`  // a call's arguments; an event or a feed
	Result []json.RawMessage `json:"r,omitempty"`  // a reply's results, the error left out
	Err    string            `json:"e,omitempty"`  // a reply's error; an event's
}

const (
	wireCall  = "call"
	wireReply = "reply"
	wireEvent = "event"
	wireFeed  = "feed"
)

// link carries wire messages both ways, in order.
type link interface {
	Send(m wireMsg) error
	Recv() (wireMsg, error)
	Close() error
}

// errLinkClosed: the other side is gone.
var errLinkClosed = errors.New("world: wire closed")

// pipeLink is one end of an in-process link (pipe).
type pipeLink struct {
	in, out   chan wireMsg
	done      chan struct{}
	closeOnce *sync.Once
}

// pipe makes a connected pair of in-process links.
func pipe() (link, link) {
	a, b := make(chan wireMsg, 1024), make(chan wireMsg, 1024)
	done, once := make(chan struct{}), &sync.Once{}
	return &pipeLink{in: a, out: b, done: done, closeOnce: once}, &pipeLink{in: b, out: a, done: done, closeOnce: once}
}

func (p *pipeLink) Send(m wireMsg) error {
	select {
	case <-p.done:
		return errLinkClosed
	case p.out <- m:
		return nil
	}
}

func (p *pipeLink) Recv() (wireMsg, error) {
	select {
	case <-p.done:
		return wireMsg{}, errLinkClosed
	case m := <-p.in:
		return m, nil
	}
}

func (p *pipeLink) Close() error {
	p.closeOnce.Do(func() { close(p.done) })
	return nil
}

// ── The director's side ────────────────────────────────────────────────────

// wireClient calls the actuator's objects and hands their events out.
type wireClient struct {
	l       link
	mu      sync.Mutex
	next    uint64
	waiting map[uint64]chan wireMsg
	events  map[string]chan wireMsg // by target
	// cache: each controller's reads from the actuator's snapshot;
	// inflight: commands on their way, by target (wirecache.go).
	cache    map[string]map[string][]json.RawMessage
	inflight map[string]int
	onFeed   func(wireMsg)
	onError  func(error)   // a call's error with nowhere to go
	done     chan struct{} // closed once the link is gone
	err      error         // the link's, once it broke
}

func newWireClient(l link, onFeed func(wireMsg)) *wireClient {
	c := &wireClient{l: l, waiting: map[uint64]chan wireMsg{}, events: map[string]chan wireMsg{}, onFeed: onFeed, done: make(chan struct{}),
		cache: map[string]map[string][]json.RawMessage{}, inflight: map[string]int{}}
	go c.read()
	return c
}

func (c *wireClient) read() {
	for {
		m, err := c.l.Recv()
		if err != nil {
			c.mu.Lock()
			c.err = err
			for id, ch := range c.waiting {
				close(ch)
				delete(c.waiting, id)
			}
			for t, ch := range c.events {
				close(ch)
				delete(c.events, t)
			}
			c.mu.Unlock()
			close(c.done)
			return
		}
		switch m.Kind {
		case wireReply:
			c.mu.Lock()
			ch := c.waiting[m.ID]
			delete(c.waiting, m.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		case wireEvent:
			c.mu.Lock()
			ch := c.events[m.Target]
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		case wireFeed:
			if m.Method == "ctlstate" {
				c.takeSnapshot(m)
				continue
			}
			if c.onFeed != nil {
				c.onFeed(m)
			}
		}
	}
}

// subscribe is target's events, from now on.
func (c *wireClient) subscribe(target string) <-chan wireMsg {
	ch := make(chan wireMsg, 256)
	c.mu.Lock()
	c.events[target] = ch
	c.mu.Unlock()
	return ch
}

// call calls target's method with args and decodes its results into outs
// (pointers, the error left out): its error, or the wire's.
func (c *wireClient) call(target, method string, args []any, outs ...any) error {
	if len(args) == 0 && c.fromCache(target, method, outs) {
		return nil
	}
	defer c.commanding(target, method)()
	m := wireMsg{Kind: wireCall, Target: target, Method: method}
	for _, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return fmt.Errorf("world: wire %s.%s: %w", target, method, err)
		}
		m.Args = append(m.Args, b)
	}
	ch := make(chan wireMsg, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return c.err
	}
	c.next++
	m.ID = c.next
	c.waiting[m.ID] = ch
	c.mu.Unlock()
	if err := c.l.Send(m); err != nil {
		c.mu.Lock()
		delete(c.waiting, m.ID)
		c.mu.Unlock()
		return err
	}
	r, ok := <-ch
	if !ok {
		return errLinkClosed
	}
	for i, out := range outs {
		if i < len(r.Result) && out != nil {
			if err := json.Unmarshal(r.Result[i], out); err != nil {
				return fmt.Errorf("world: wire %s.%s result %d: %w", target, method, i, err)
			}
		}
	}
	if r.Err != "" {
		return errors.New(r.Err)
	}
	return nil
}

// ── The actuator's side ────────────────────────────────────────────────────

// wireServer answers calls on its objects (dispatch, by reflection).
type wireServer struct {
	mu      sync.Mutex
	targets map[string]any
}

func newWireServer() *wireServer { return &wireServer{targets: map[string]any{}} }

func (s *wireServer) add(target string, obj any) {
	s.mu.Lock()
	s.targets[target] = obj
	s.mu.Unlock()
}

func (s *wireServer) remove(target string) {
	s.mu.Lock()
	delete(s.targets, target)
	s.mu.Unlock()
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// dispatch runs call m on its target and is its reply.
func (s *wireServer) dispatch(m wireMsg) wireMsg {
	r := wireMsg{Kind: wireReply, ID: m.ID}
	s.mu.Lock()
	obj := s.targets[m.Target]
	s.mu.Unlock()
	if obj == nil {
		r.Err = "world: wire: no " + m.Target
		return r
	}
	fn := reflect.ValueOf(obj).MethodByName(m.Method)
	if !fn.IsValid() {
		r.Err = "world: wire: " + m.Target + " has no " + m.Method
		return r
	}
	ft := fn.Type()
	if ft.NumIn() != len(m.Args) {
		r.Err = fmt.Sprintf("world: wire: %s.%s takes %d arguments, got %d", m.Target, m.Method, ft.NumIn(), len(m.Args))
		return r
	}
	in := make([]reflect.Value, ft.NumIn())
	for i := range in {
		v := reflect.New(ft.In(i))
		if err := json.Unmarshal(m.Args[i], v.Interface()); err != nil {
			r.Err = fmt.Sprintf("world: wire: %s.%s argument %d: %v", m.Target, m.Method, i, err)
			return r
		}
		in[i] = v.Elem()
	}
	for _, out := range fn.Call(in) {
		if out.Type() == errorType {
			if !out.IsNil() {
				r.Err = out.Interface().(error).Error()
			}
			continue
		}
		b, err := json.Marshal(out.Interface())
		if err != nil {
			r.Err = fmt.Sprintf("world: wire: %s.%s result: %v", m.Target, m.Method, err)
			return r
		}
		r.Result = append(r.Result, b)
	}
	return r
}

// dropped logs a wire error of a call whose method has no error to
// return it in.
func (c *wireClient) dropped(err error) {
	if err != nil && c.onError != nil {
		c.onError(err)
	}
}

// remoteDep, remoteArr and remoteSim are the director's stand-ins for the
// actuator's objects (remote_gen.go): t is the target on the wire.
type (
	remoteDep struct {
		c *wireClient
		t string
	}
	remoteArr struct {
		c *wireClient
		t string
	}
	remoteSim struct {
		c *wireClient
		t string
	}
)

// unsubscribe stops handing target's events out.
func (c *wireClient) unsubscribe(target string) {
	c.mu.Lock()
	if ch := c.events[target]; ch != nil {
		close(ch)
		delete(c.events, target)
	}
	c.mu.Unlock()
}
