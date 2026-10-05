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
}

func newHubLink() *hubLink {
	return &hubLink{next: make(chan *connLink, 1), done: make(chan struct{})}
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

func (h *hubLink) Recv() (wireMsg, error) {
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
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			l := newConnLink(c)
			if err = l.enc.Encode(hello{Token: token}); err == nil {
				logf("actuator: attached to director %s", addr)
				h.attach(l)
				// Until this director is gone (dropped on a failed read or send).
				for h.current() == l && ctx.Err() == nil {
					select {
					case <-ctx.Done():
					case <-time.After(250 * time.Millisecond):
					}
				}
				logf("actuator: director %s gone", addr)
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

// ListenDirector runs w as a director waiting on addr for an actuator that
// dials in with token (any when ""), #774: one at a time; when it is gone,
// the next one that dials in. It returns when ctx ends or the listener
// fails.
func ListenDirector(ctx context.Context, w *World, addr, token string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		l, ok := greeted(c, token)
		if !ok {
			continue
		}
		w.st.core.log.printf("director: actuator %s attached", c.RemoteAddr())
		err = w.runDirector(ctx, l)
		l.Close()
		if ctx.Err() != nil {
			return nil
		}
		w.st.core.log.printf("director: actuator %s gone: %v", c.RemoteAddr(), err)
	}
}

// current is the director attached now (nil none).
func (h *hubLink) current() *connLink {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cur
}
