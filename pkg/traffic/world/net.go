package world

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// The split across a network (#710): the actuator listens beside the
// simulator, a director dials it. JSON lines over TCP, a token first.

// connLink is a link on a network connection: one JSON message a line.
type connLink struct {
	c   net.Conn
	mu  sync.Mutex // writes
	enc *json.Encoder
	dec *json.Decoder
}

func newConnLink(c net.Conn) *connLink {
	return &connLink{c: c, enc: json.NewEncoder(c), dec: json.NewDecoder(bufio.NewReaderSize(c, 1<<16))}
}

func (l *connLink) Send(m wireMsg) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enc.Encode(m)
}

func (l *connLink) Recv() (wireMsg, error) {
	var m wireMsg
	err := l.dec.Decode(&m)
	return m, err
}

func (l *connLink) Close() error { return l.c.Close() }

// hello is the director's first line: the token the actuator wants.
type hello struct {
	Token string `json:"token"`
}

// hubLink is the actuator's end: the director attached now, if any.
// Messages to no director are dropped; Recv waits for one.
type hubLink struct {
	mu   sync.Mutex
	cur  *connLink
	next chan *connLink
	done chan struct{}
	once sync.Once
	in   chan wireMsg // what the director sends, for whoever reads now
	// state: "dialling", "attached", "gone" (LinkState).
	state string
}

func newHubLink() *hubLink {
	h := &hubLink{next: make(chan *connLink, 1), done: make(chan struct{}), in: make(chan wireMsg, 256)}
	go h.pump()
	return h
}

// pump reads the director attached now into in, until the hub closes.
func (h *hubLink) pump() {
	for {
		m, err := h.recvDirect()
		if err != nil {
			return
		}
		select {
		case h.in <- m:
		case <-h.done:
			return
		}
	}
}

// RecvCtx is Recv until ctx ends: a connection's reader stops with it and
// leaves the link to the next one (#779).
func (h *hubLink) RecvCtx(ctx context.Context) (wireMsg, error) {
	select {
	case m := <-h.in:
		return m, nil
	case <-h.done:
		return wireMsg{}, errLinkClosed
	case <-ctx.Done():
		return wireMsg{}, ctx.Err()
	}
}

func (h *hubLink) setState(s string) {
	h.mu.Lock()
	h.state = s
	h.mu.Unlock()
}

// attach makes c the director, dropping the one before.
func (h *hubLink) attach(c *connLink) {
	h.mu.Lock()
	old := h.cur
	h.cur = c
	h.mu.Unlock()
	if old != nil {
		old.Close()
	}
	select {
	case h.next <- c:
	default:
	}
}

func (h *hubLink) Send(m wireMsg) error {
	h.mu.Lock()
	c := h.cur
	h.mu.Unlock()
	if c == nil {
		return nil // no director: nobody to tell
	}
	if err := c.Send(m); err != nil {
		h.drop(c)
	}
	return nil
}

func (h *hubLink) Recv() (wireMsg, error) { return h.RecvCtx(context.Background()) }

// recvDirect reads the director attached now, waiting for one.
func (h *hubLink) recvDirect() (wireMsg, error) {
	for {
		h.mu.Lock()
		c := h.cur
		h.mu.Unlock()
		if c == nil {
			select {
			case <-h.done:
				return wireMsg{}, errLinkClosed
			case <-h.next:
				continue
			}
		}
		m, err := c.Recv()
		if err == nil {
			return m, nil
		}
		h.drop(c)
	}
}

// drop forgets c, if it is still the director.
func (h *hubLink) drop(c *connLink) {
	h.mu.Lock()
	if h.cur == c {
		h.cur = nil
	}
	h.mu.Unlock()
	c.Close()
}

func (h *hubLink) Close() error {
	h.once.Do(func() { close(h.done) })
	h.mu.Lock()
	c := h.cur
	h.cur = nil
	h.mu.Unlock()
	if c != nil {
		c.Close()
	}
	return nil
}

// ServeActuator runs w as an actuator on its own simulator connection
// (Run) and serves it to directors on addr ("host:port"), one at a time,
// each greeting with token (any when token is ""). It returns when ctx ends
// or the listener fails.
func ServeActuator(ctx context.Context, w *World, addr, token string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	hub := newHubLink()
	w.st.actLink = hub
	go w.Run(ctx)
	go func() {
		<-ctx.Done()
		ln.Close()
		hub.Close()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func() {
			if l, ok := greeted(c, token); ok {
				w.st.core.log.printf("actuator: director %s attached", c.RemoteAddr())
				hub.attach(l)
			}
		}()
	}
}

// DialDirector runs w as a director of the actuator at addr, greeting with
// token, until ctx ends; a lost actuator is dialled again.
func DialDirector(ctx context.Context, w *World, addr, token string) error {
	for {
		err := dialOnce(ctx, w, addr, token)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			w.st.core.log.printf("director: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func dialOnce(ctx context.Context, w *World, addr, token string) error {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	l := newConnLink(c)
	if err := l.enc.Encode(hello{Token: token}); err != nil {
		c.Close()
		return err
	}
	w.st.core.log.printf("director: actuator %s", addr)
	err = w.runDirector(ctx, l)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("actuator %s gone", addr)
	}
	return err
}

// greeted reads a director's hello on c: its link, or false (c closed)
// when the token is wrong or none comes within 10 s.
func greeted(c net.Conn, token string) (*connLink, bool) {
	l := newConnLink(c)
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	var h hello
	if err := l.dec.Decode(&h); err != nil || token != "" && subtle.ConstantTimeCompare([]byte(h.Token), []byte(token)) != 1 {
		c.Close()
		return nil, false
	}
	c.SetReadDeadline(time.Time{})
	return l, true
}

// DialActuator runs w as an actuator on its own simulator connection (Run)
// and dials the director at addr ("host:port") with token, for a player
// behind a router (#774): the director needs no way in. A lost director
// is dialled again every 5 s until ctx ends.
func DialActuator(ctx context.Context, w *World, addr, token string) error {
	hub := newHubLink()
	w.st.actLink = hub
	go w.Run(ctx)
	defer hub.Close()
	return hub.dialOut(ctx, addr, token, w.st.core.log.printf)
}

// dialOut keeps the hub attached to the director at addr: dialled, greeted
// with token, dialled again 5 s after it is lost, until ctx ends.
func (h *hubLink) dialOut(ctx context.Context, addr, token string, logf func(string, ...any)) error {
	for {
		h.setState("dialling")
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			l := newConnLink(c)
			if err = l.enc.Encode(hello{Token: token}); err == nil {
				logf("actuator: attached to director %s", addr)
				h.attach(l)
				h.setState("attached")
				// Until this director is gone (dropped on a failed read or send).
				for h.current() == l && ctx.Err() == nil {
					select {
					case <-ctx.Done():
					case <-time.After(250 * time.Millisecond):
					}
				}
				logf("actuator: director %s gone", addr)
				h.setState("gone")
			} else {
				c.Close()
			}
		}
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			logf("actuator: director %s: %v", addr, err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(redialEvery):
		}
	}
}

// redialEvery: a lost director is dialled again after this (tests shorten it).
var redialEvery = 5 * time.Second

// ListenDirector runs w as a director waiting on addr for actuators that
// dial in with token (any when ""), #774, #779. The first is the primary:
// its replies, its aircraft's events and its simulator's feed drive the
// director. Every later one follows: it gets the same commands, so each
// player's sim creates and moves the same traffic locally; what it sends
// back is dropped. A follower joining late gets the flights started after
// it. When the primary is gone the director starts again with the next
// actuator to dial in (the followers dial again too). It returns when ctx
// ends or the listener fails.
func ListenDirector(ctx context.Context, w *World, addr, token string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	var mu sync.Mutex
	var cur *fanLink
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func() {
			l, ok := greeted(c, token)
			if !ok {
				return
			}
			mu.Lock()
			if cur != nil {
				cur.follow(l)
				mu.Unlock()
				w.st.core.log.printf("director: actuator %s follows", c.RemoteAddr())
				return
			}
			fan := &fanLink{primary: l}
			cur = fan
			mu.Unlock()
			w.st.core.log.printf("director: actuator %s attached (primary)", c.RemoteAddr())
			err := w.runDirector(ctx, fan)
			mu.Lock()
			cur = nil
			mu.Unlock()
			fan.Close()
			if ctx.Err() == nil {
				w.st.core.log.printf("director: primary actuator %s gone: %v", c.RemoteAddr(), err)
			}
		}()
	}
}

// fanLink is a director's link to several actuators (#779): sent to all,
// received from the primary.
type fanLink struct {
	primary   *connLink
	mu        sync.Mutex
	followers []*connLink
}

// follow adds a follower: what it sends is read and dropped.
func (f *fanLink) follow(l *connLink) {
	f.mu.Lock()
	f.followers = append(f.followers, l)
	f.mu.Unlock()
	go func() {
		for {
			if _, err := l.Recv(); err != nil {
				f.drop(l)
				return
			}
		}
	}()
}

func (f *fanLink) drop(l *connLink) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, x := range f.followers {
		if x == l {
			f.followers = append(f.followers[:i], f.followers[i+1:]...)
			break
		}
	}
	l.Close()
}

func (f *fanLink) Send(m wireMsg) error {
	err := f.primary.Send(m)
	f.mu.Lock()
	fs := append([]*connLink(nil), f.followers...)
	f.mu.Unlock()
	for _, l := range fs {
		if l.Send(m) != nil {
			f.drop(l)
		}
	}
	return err
}

func (f *fanLink) Recv() (wireMsg, error) { return f.primary.Recv() }

func (f *fanLink) Close() error {
	f.mu.Lock()
	fs := f.followers
	f.followers = nil
	f.mu.Unlock()
	for _, l := range fs {
		l.Close()
	}
	return f.primary.Close()
}

// current is the director attached now (nil none).
func (h *hubLink) current() *connLink {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cur
}

// LinkDirector makes w an actuator of the director at addr ("host:port",
// greeting with token) on the host's own simulator connection: the host
// keeps calling RunOn per connection (#779). It dials out (a player behind
// a router needs no way in), dials again 5 s after the director is lost,
// and keeps the link across sim reconnects. Call it before RunOn: a
// connection already running stays as it was until the next one. When ctx
// ends the World decides on its own again from the next connection.
// Snapshot.Link says how the link is.
func (w *World) LinkDirector(ctx context.Context, addr, token string) error {
	hub := newHubLink()
	w.st.mu.Lock()
	w.st.actLink = hub
	w.st.mu.Unlock()
	defer func() {
		w.st.mu.Lock()
		if w.st.actLink == link(hub) {
			w.st.actLink = nil
		}
		w.st.mu.Unlock()
		hub.Close()
	}()
	return hub.dialOut(ctx, addr, token, w.st.core.log.printf)
}
