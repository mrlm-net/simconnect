package world

import (
	"net"
	"net/http"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Network play (#511): several people on the LAN, each working one
// position. Start the map with -addr :8080 (every interface) and open it on
// the other devices. A client says which position it works with ?as= (or
// the X-ATC-Position header): delivery, ground, tower, approach (with
// departure) or all; a clearance for an aircraft another position works is
// refused. Each client can play the radio itself: GET /api/voice/clip gives
// a transmission as a WAV in the voice the server would say it in.
//
// Without -token there is no login: serve it on a network you trust. With
// it, see access.go: a token to control, another to watch.

// positionOf is the position a request is made as ("" or "all": any).
func positionOf(r *http.Request) string {
	as := r.URL.Query().Get("as")
	if as == "" {
		as = r.Header.Get("X-ATC-Position")
	}
	return strings.ToLower(strings.TrimSpace(as))
}

// mayClear reports whether a client working position as may clear it: the
// aircraft is on that position's frequency (approach also works departures).
func mayClear(as string, it *controlled) bool {
	if as == "" || as == "all" {
		return true
	}
	it.mu.Lock()
	atc := it.atc
	it.mu.Unlock()
	if atc == "" {
		return true // not handed to anyone yet
	}
	if as == string(traffic.PosApproach) && atc == traffic.PosDeparture {
		return true
	}
	return string(atc) == as
}

// listenAddr is the address the map serves on (-addr).
var listenAddr string

// networkURLs are the addresses other devices open the map on: one per
// network interface when it listens on all of them (-addr :8080), none
// when it listens on this computer only (127.0.0.1).
func networkURLs() []string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() || host == "localhost" {
			return nil
		}
		return []string{"http://" + net.JoinHostPort(host, port)}
	}
	var out []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil && !n.IP.IsLinkLocalUnicast() {
			out = append(out, "http://"+net.JoinHostPort(n.IP.String(), port))
		}
	}
	return out
}

// registerStatus serves GET /api/status — {connected}: the simulator is
// connected (also in its menu, without an aircraft), with the network
// addresses and the caller's role.
func registerStatus(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		connected := st.control != nil
		st.mu.Unlock()
		out := map[string]any{"connected": connected, "network": networkURLs(), "addr": listenAddr, "role": requestRole(r), "tokens": tokensOn()}
		if fromThisComputer(r) {
			out["links"] = shareLinks() // the links to give out, with their tokens: for the host only
		}
		writeJSON(w, out)
	})
}

// Guard wraps the map's HTTP handler with the network-play access check
// (SetTokens): every request but the page needs a token when tokens are set.
func Guard(next http.Handler) http.Handler { return guard(next) }

// SetTokens sets the network-play tokens: control (to control the traffic)
// and view (to watch); "auto" a random one, "" none needed.
func SetTokens(control, view string) { setTokens(control, view) }

// ShareLinks are the links to give out per role, with their tokens.
func ShareLinks() map[string][]string { return shareLinks() }
