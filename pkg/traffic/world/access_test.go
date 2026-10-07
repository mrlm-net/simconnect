package world

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The tokens: a link sets the cookie and comes back without the token; no
// token, 401; a spectator reads but cannot change; this computer and a
// control token can do anything.
func TestGuard(t *testing.T) {
	defer setTokens("", "")
	setTokens("ctl123", "view456")
	ok := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	do := func(method, target, remote, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, nil)
		r.RemoteAddr = remote
		if remote == "127.0.0.1:5000" {
			r.Host = "127.0.0.1:8080" // the map asked for by its loopback name
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: tokenCookie, Value: cookie})
		}
		w := httptest.NewRecorder()
		ok.ServeHTTP(w, r)
		return w
	}
	const lan, here = "10.0.0.7:5000", "127.0.0.1:5000"
	if c := do("GET", "/api/traffic", lan, "").Code; c != http.StatusUnauthorized {
		t.Errorf("no token: %d", c)
	}
	if c := do("POST", "/api/control", here, "").Code; c != http.StatusOK {
		t.Errorf("this computer: %d", c)
	}
	w := do("GET", "/?icao=LKPR&token=view456", lan, "")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/?icao=LKPR" {
		t.Errorf("link: %d to %q", w.Code, w.Header().Get("Location"))
	}
	if ck := w.Result().Cookies(); len(ck) != 1 || ck[0].Value != "view456" || !ck[0].HttpOnly {
		t.Errorf("cookie: %+v", ck)
	}
	if c := do("GET", "/api/traffic", lan, "view456").Code; c != http.StatusOK {
		t.Errorf("spectator reads: %d", c)
	}
	if c := do("POST", "/api/control", lan, "view456").Code; c != http.StatusForbidden {
		t.Errorf("spectator changes: %d", c)
	}
	if c := do("POST", "/api/control", lan, "ctl123").Code; c != http.StatusOK {
		t.Errorf("control: %d", c)
	}
	if c := do("GET", "/api/traffic", lan, "wrong").Code; c != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", c)
	}
	// A bad link sets no cookie.
	if ck := do("GET", "/?token=nope", lan, "").Result().Cookies(); len(ck) != 0 {
		t.Errorf("bad link set %+v", ck)
	}
	// Not off-site from a link (#61).
	if loc := do("GET", "//evil.example/x?token=view456", lan, "").Header().Get("Location"); loc != "/evil.example/x" {
		t.Errorf("redirect to %q", loc)
	}
	// This computer only by a loopback name, and not from another site's
	// page (#60).
	rebound := httptest.NewRequest("POST", "/api/control", nil)
	rebound.RemoteAddr, rebound.Host = here, "evil.example:8080"
	if fromThisComputer(rebound) {
		t.Error("a rebound name came in as this computer")
	}
	csrf := httptest.NewRequest("POST", "/api/control", nil)
	csrf.RemoteAddr, csrf.Host = here, "127.0.0.1:8080"
	csrf.Header.Set("Origin", "https://evil.example")
	if fromThisComputer(csrf) {
		t.Error("another site's POST came in as this computer")
	}
	own := httptest.NewRequest("POST", "/api/control", nil)
	own.RemoteAddr, own.Host = here, "127.0.0.1:8080"
	own.Header.Set("Origin", "http://127.0.0.1:8080")
	if !fromThisComputer(own) {
		t.Error("the map's own POST refused")
	}
	setTokens("", "")
	if c := do("POST", "/api/control", lan, "").Code; c != http.StatusOK {
		t.Errorf("no tokens: %d", c)
	}
}
