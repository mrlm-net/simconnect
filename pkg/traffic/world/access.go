package world

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Access for network play: without tokens the map is open to anyone who
// can reach it (serve it on a network you trust). With -token, another
// device needs that token to control the traffic; with -view-token, a
// second token lets one watch (spectator: it reads everything, changes
// nothing). This computer itself always has full access.
//
// A device opens the link with ?token=…; the map keeps it in a cookie and
// takes it out of the address, so it is not left in the address bar,
// history or a shared screenshot. Scripts may send it as a bearer token.

// Roles a request is made with.
const (
	roleControl   = "control"
	roleSpectator = "spectator"
)

const tokenCookie = "airport_map_token"

// access holds the tokens (empty: not required).
var access struct {
	control, view string
	// verify checks a session token (SetTokenVerifier); it gives verifyRole.
	verify     func(ctx context.Context, token string) error
	verifyRole string
}

// SetTokenVerifier lets a request in with a token verify accepts (the
// MyCrew API's session tokens, JWKS.Verify, #792), as a controller when
// control, else as a spectator; nil: none. The static tokens still work.
func SetTokenVerifier(verify func(ctx context.Context, token string) error, control bool) {
	access.verify, access.verifyRole = verify, roleSpectator
	if control {
		access.verifyRole = roleControl
	}
}

// newToken is a random token for -token auto.
func newToken() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// setTokens sets the tokens from the flags ("auto": a random one).
func setTokens(control, view string) {
	if control == "auto" {
		control = newToken()
	}
	if view == "auto" {
		view = newToken()
	}
	access.control, access.view = control, view
}

func tokensOn() bool { return access.control != "" || access.view != "" || access.verify != nil }

// fromThisComputer reports whether r comes from the map's own computer.
func fromThisComputer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func same(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// roleOf is the role of a token ("" none).
func roleOf(token string) string {
	switch {
	case same(token, access.control):
		return roleControl
	case same(token, access.view):
		return roleSpectator
	case token != "" && access.verify != nil:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if access.verify(ctx, token) == nil {
			return access.verifyRole
		}
	}
	return ""
}

// requestRole is the role r is made with: control without tokens or from
// this computer, else the role of its token ("" none).
func requestRole(r *http.Request) string {
	if !tokensOn() || fromThisComputer(r) {
		return roleControl
	}
	if c, err := r.Cookie(tokenCookie); err == nil {
		if role := roleOf(c.Value); role != "" {
			return role
		}
	}
	if t, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return roleOf(strings.TrimSpace(t))
	}
	return ""
}

// guard enforces the tokens: a link with ?token= sets the cookie and comes
// back without it; no token, 401; a spectator, reads only (GET, HEAD).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t := r.URL.Query().Get("token"); t != "" && r.Method == http.MethodGet {
			if roleOf(t) != "" {
				http.SetCookie(w, &http.Cookie{Name: tokenCookie, Value: t, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 3600})
			}
			q := r.URL.Query()
			q.Del("token")
			u := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
			http.Redirect(w, r, u.String(), http.StatusSeeOther)
			return
		}
		switch requestRole(r) {
		case roleControl:
			next.ServeHTTP(w, r)
		case roleSpectator:
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.Error(w, "spectating: this device can watch, not change anything", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Airport map</title>` +
				`<body style="font:16px system-ui;max-width:32em;margin:15vh auto;padding:0 16px;color:#222"><h1 style="font-size:20px">This map needs a link with a token</h1>` +
				`<p>Ask whoever runs it for the link: one to control the traffic, or one to watch.</p></body>`))
		}
	})
}

// shareLinks are the links to give out per network address, for the
// host's own page: with the control and the view token.
func shareLinks() map[string][]string {
	out := map[string][]string{}
	for _, u := range networkURLs() {
		if access.control != "" {
			out[roleControl] = append(out[roleControl], u+"/?token="+url.QueryEscape(access.control))
		}
		if access.view != "" {
			out[roleSpectator] = append(out[roleSpectator], u+"/?token="+url.QueryEscape(access.view))
		}
	}
	return out
}
