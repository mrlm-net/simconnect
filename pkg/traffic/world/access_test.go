//go:build windows
// +build windows

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
	setTokens("", "")
	if c := do("POST", "/api/control", lan, "").Code; c != http.StatusOK {
		t.Errorf("no tokens: %d", c)
	}
}
