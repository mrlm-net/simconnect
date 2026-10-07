package world

import (
	"bufio"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	in  *budgetReader
}

func newConnLink(c net.Conn) *connLink {
	in := &budgetReader{r: c, left: math.MaxInt64}
	return &connLink{c: c, enc: json.NewEncoder(c), dec: json.NewDecoder(bufio.NewReaderSize(in, 1<<16)), in: in}
}

// budgetReader reads at most left bytes: the hello before a link is
// trusted is read within helloMaxBytes (#54: an unbounded decode let
// anyone exhaust the director's memory before any token was checked).
type budgetReader struct {
	r    io.Reader
	left int64
}

func (b *budgetReader) Read(p []byte) (int, error) {
	if b.left <= 0 {
		return 0, errHelloTooLarge
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.r.Read(p)
	b.left -= int64(n)
	return n, err
}

// Before a link is trusted: its hello within helloMaxBytes, and at most
// maxGreetings links being greeted at once.
const (
	helloMaxBytes = 16 << 10
	maxGreetings  = 32
)

var (
	errHelloTooLarge = errors.New("world: link hello too large")
	greetSlots       = make(chan struct{}, maxGreetings)
)

func (l *connLink) Send(m wireMsg) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	// A peer that stops reading fails the write, not the sender for ever
	// (#55: a stalled director froze the actuator's sim loop).
	l.c.SetWriteDeadline(time.Now().Add(linkWriteTimeout))
	return l.enc.Encode(m)
}

// linkWriteTimeout: a message not written by then fails its link.
const linkWriteTimeout = 10 * time.Second

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
	in   chan hubMsg // what the director sends, for whoever reads now
	// state: "dialling", "attached", "gone" (LinkState).
	state string
}

func newHubLink() *hubLink {
	h := &hubLink{next: make(chan *connLink, 1), done: make(chan struct{}), in: make(chan hubMsg, 256)}
	go h.pump()
	return h
}

// pump reads the director attached now into in, until the hub closes.
func (h *hubLink) pump() {
	for {
		m, from, err := h.recvDirect()
		if err != nil {
			return
		}
		select {
		case h.in <- hubMsg{m, from}:
		case <-h.done:
			return
		}
	}
}

// hubMsg is a message and the director link it came on: one from a
// director since replaced is not read (#57: its calls reached the next
// one, whose call IDs start at 1 again, and got wrong replies).
type hubMsg struct {
	m    wireMsg
	from *connLink
}

// RecvCtx is Recv until ctx ends: a connection's reader stops with it and
// leaves the link to the next one (#779).
func (h *hubLink) RecvCtx(ctx context.Context) (wireMsg, error) {
	for {
		select {
		case hm := <-h.in:
			h.mu.Lock()
			cur := h.cur
			h.mu.Unlock()
			if hm.from != cur {
				continue // the director before
			}
			return hm.m, nil
		case <-h.done:
			return wireMsg{}, errLinkClosed
		case <-ctx.Done():
			return wireMsg{}, ctx.Err()
		}
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
func (h *hubLink) recvDirect() (wireMsg, *connLink, error) {
	for {
		h.mu.Lock()
		c := h.cur
		h.mu.Unlock()
		if c == nil {
			select {
			case <-h.done:
				return wireMsg{}, nil, errLinkClosed
			case <-h.next:
				continue
			}
		}
		m, err := c.Recv()
		if err == nil {
			return m, c, nil
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
	return ServeActuatorWith(ctx, w, addr, LinkOptions{Token: token})
}

// ServeActuatorWith is ServeActuator with o's TLS and token check (#792).
func ServeActuatorWith(ctx context.Context, w *World, addr string, o LinkOptions) error {
	ln, err := o.listen(addr)
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
			if l, ok := o.greeted(ctx, c, w.st.core.log.printf); ok {
				w.st.core.log.printf("actuator: director %s attached", c.RemoteAddr())
				hub.attach(l)
			}
		}()
	}
}

// DialDirector runs w as a director of the actuator at addr, greeting with
// token, until ctx ends; a lost actuator is dialled again.
func DialDirector(ctx context.Context, w *World, addr, token string) error {
	return DialDirectorWith(ctx, w, addr, LinkOptions{Token: token})
}

// DialDirectorWith is DialDirector with o's TLS and token (#792).
func DialDirectorWith(ctx context.Context, w *World, addr string, o LinkOptions) error {
	for {
		err := dialOnce(ctx, w, addr, o)
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

func dialOnce(ctx context.Context, w *World, addr string, o LinkOptions) error {
	token, err := o.token(ctx)
	if err != nil {
		return err
	}
	c, err := o.dial(ctx, addr)
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

// greeted reads the other side's hello on c: its link, or false (c
// closed) when the token is refused or none comes within 10 s. A token
// that expires (Verify) closes c when it does: the other side dials again
// with a fresh one.
func (o LinkOptions) greeted(ctx context.Context, c net.Conn, logf func(string, ...any)) (*connLink, bool) {
	select {
	case greetSlots <- struct{}{}:
		defer func() { <-greetSlots }()
	default:
		logf("link: %s refused: too many links greeting at once", c.RemoteAddr())
		c.Close()
		return nil, false
	}
	l := newConnLink(c)
	l.in.left = helloMaxBytes
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	var h hello
	if err := l.dec.Decode(&h); err != nil {
		c.Close()
		return nil, false
	}
	l.in.left = math.MaxInt64 // trusted from here on: its messages unbounded
	var expires time.Time
	switch {
	case o.Verify != nil:
		var err error
		if expires, err = o.Verify(ctx, h.Token); err != nil {
			logf("link: %s refused: %v", c.RemoteAddr(), err)
			c.Close()
			return nil, false
		}
	case o.Token != "" && subtle.ConstantTimeCompare([]byte(h.Token), []byte(o.Token)) != 1:
		logf("link: %s refused: wrong token", c.RemoteAddr())
		c.Close()
		return nil, false
	}
	c.SetReadDeadline(time.Time{})
	if !expires.IsZero() {
		time.AfterFunc(time.Until(expires), func() { c.Close() })
	}
	return l, true
}

// LinkOptions secure a link between director and actuator (#792).
type LinkOptions struct {
	// Token is the token the dialling side greets with and the listening
	// side wants ("": any, a trusted network only); TokenFunc, when set,
	// gives the dialling side a fresh one for every greeting (a session
	// token that expires).
	Token     string
	TokenFunc func(ctx context.Context) (string, error)
	// Verify, when set, checks the greeting's token on the listening side
	// instead of Token: when it expires (zero: never) the link is closed
	// and the other side dials again with a fresh one. JWKS.LinkVerify
	// checks the MyCrew API's session tokens.
	Verify func(ctx context.Context, token string) (expires time.Time, err error)
	// TLS: the dialling side verifies the server with it (an empty Config:
	// the system roots), the listening side serves its certificate (nil:
	// plain TCP).
	TLS *tls.Config
}

func (o LinkOptions) token(ctx context.Context) (string, error) {
	if o.TokenFunc != nil {
		t, err := o.TokenFunc(ctx)
		if err != nil {
			return "", fmt.Errorf("link token: %w", err)
		}
		return t, nil
	}
	return o.Token, nil
}

func (o LinkOptions) listen(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil || o.TLS == nil {
		return ln, err
	}
	return tls.NewListener(ln, o.TLS), nil
}

func (o LinkOptions) dial(ctx context.Context, addr string) (net.Conn, error) {
	if o.TLS == nil {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
	d := tls.Dialer{Config: o.TLS}
	return d.DialContext(ctx, "tcp", addr)
}

// LinkVerify is j as a LinkOptions.Verify: when the link must close (the
// token's expiry and the leeway Verify allows), or why it is refused.
func (j *JWKS) LinkVerify(ctx context.Context, token string) (time.Time, error) {
	c, err := j.Verify(ctx, token)
	if err != nil || c.Expires.IsZero() {
		return time.Time{}, err
	}
	return c.Expires.Add(tokenLeeway), nil
}

// DialActuator runs w as an actuator on its own simulator connection (Run)
// and dials the director at addr ("host:port") with token, for a player
// behind a router (#774): the director needs no way in. A lost director
// is dialled again every 5 s until ctx ends.
func DialActuator(ctx context.Context, w *World, addr, token string) error {
	return DialActuatorWith(ctx, w, addr, LinkOptions{Token: token})
}

// DialActuatorWith is DialActuator with o's TLS and token (#792).
func DialActuatorWith(ctx context.Context, w *World, addr string, o LinkOptions) error {
	hub := newHubLink()
	w.st.actLink = hub
	go w.Run(ctx)
	defer hub.Close()
	return hub.dialOut(ctx, addr, o, w.st.core.log.printf)
}

// dialOut keeps the hub attached to the director at addr: dialled, greeted
// with token, dialled again 5 s after it is lost, until ctx ends.
func (h *hubLink) dialOut(ctx context.Context, addr string, o LinkOptions, logf func(string, ...any)) error {
	for {
		h.setState("dialling")
		token, err := o.token(ctx)
		var c net.Conn
		if err == nil {
			c, err = o.dial(ctx, addr)
		}
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
	return ListenDirectorWith(ctx, w, addr, LinkOptions{Token: token})
}

// ListenDirectorWith is ListenDirector with o's TLS and token check: the
// MyCrew API's session tokens with Verify (JWKS.LinkVerify), #792.
func ListenDirectorWith(ctx context.Context, w *World, addr string, o LinkOptions) error {
	ln, err := o.listen(addr)
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
			l, ok := o.greeted(ctx, c, w.st.core.log.printf)
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
	return w.LinkDirectorWith(ctx, addr, LinkOptions{Token: token})
}

// LinkDirectorWith is LinkDirector with o's TLS and token: TokenFunc gives
// a fresh session token for every greeting (#792).
func (w *World) LinkDirectorWith(ctx context.Context, addr string, o LinkOptions) error {
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
	return hub.dialOut(ctx, addr, o, w.st.core.log.printf)
}
