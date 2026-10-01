//go:build windows
// +build windows

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	voicegoio "github.com/mrlm-net/voice-goio"
)

// Network play (#511): several people on the LAN, each working one
// position. Start the map with -addr :8080 (every interface) and open it on
// the other devices. A client says which position it works with ?as= (or
// the X-ATC-Position header): delivery, ground, tower, approach (with
// departure) or all; a clearance for an aircraft another position works is
// refused. Each client can play the radio itself: GET /api/voice/clip gives
// a transmission as a WAV in the voice the server would say it in.
//
// There is no login: serve it on a network you trust.

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

// clips caches the WAVs of recent transmissions, by key.
var clips = struct {
	sync.Mutex
	m     map[string][]byte
	order []string
}{m: map[string][]byte{}}

const clipCacheSize = 200

// clipKey names a transmission: its time and call sign.
func clipKey(t traffic.Transmission) string {
	return t.At.Format(time.RFC3339Nano) + " " + t.Callsign + " " + string(t.Intent)
}

// clip is transmission t as said on the radio, a 16-bit mono WAV.
func (v *voiceOut) clip(t traffic.Transmission) ([]byte, error) {
	key := clipKey(t)
	clips.Lock()
	if b, ok := clips.m[key]; ok {
		clips.Unlock()
		return b, nil
	}
	clips.Unlock()
	v.mu.Lock()
	err := v.openEngine()
	engine, pool, chain, norm := v.engine, v.pool, v.chain, v.norm
	v.mu.Unlock()
	if err != nil {
		return nil, err
	}
	var voice voicegoio.VoiceProfile
	switch {
	case t.Intent == traffic.IntentATIS:
		voice = pool.Assign(t.Airport, voicegoio.ATIS)
	case t.Pilot:
		voice = pool.Assign(t.Callsign, voicegoio.Center)
	default:
		voice = pool.Assign(v.onShift(t.Airport, t.Position), controllerKind(t.Position))
	}
	pcm, err := engine.Synthesize(context.Background(), voice, norm.Spoken(t.Text, phraseologyOf(t)))
	if err != nil {
		return nil, err
	}
	rate := engine.SampleRate(voice)
	out := chain.Apply(pcm, rate, voice.Radio, rate, int64(len(t.Text)))
	b := wav(out, rate)
	clips.Lock()
	clips.m[key] = b
	clips.order = append(clips.order, key)
	for len(clips.order) > clipCacheSize {
		delete(clips.m, clips.order[0])
		clips.order = clips.order[1:]
	}
	clips.Unlock()
	return b, nil
}

// wav is pcm (16-bit mono at rate) as a WAV file.
func wav(pcm []int16, rate int) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	data := uint32(len(pcm) * 2)
	b.WriteString("RIFF")
	binary.Write(&b, le, 36+data)
	b.WriteString("WAVEfmt ")
	binary.Write(&b, le, uint32(16))
	binary.Write(&b, le, uint16(1)) // PCM
	binary.Write(&b, le, uint16(1)) // mono
	binary.Write(&b, le, uint32(rate))
	binary.Write(&b, le, uint32(rate*2))
	binary.Write(&b, le, uint16(2))
	binary.Write(&b, le, uint16(16))
	b.WriteString("data")
	binary.Write(&b, le, data)
	binary.Write(&b, le, pcm)
	return b.Bytes()
}

// registerNetwork serves the clips:
//
//	GET /api/voice/clip?icao=&at=&cs=&intent= — a transmission of /api/radio
//	    (its at, callsign and intent) as a WAV, for a client playing the
//	    radio on its own device.
func registerNetwork(mux *http.ServeMux, st *state) {
	// GET /api/status — {connected}: the simulator is connected (also in its
	// menu, without an aircraft).
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		connected := st.control != nil
		st.mu.Unlock()
		writeJSON(w, map[string]any{"connected": connected, "network": networkURLs(), "addr": listenAddr})
	})
	mux.HandleFunc("GET /api/voice/clip", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		if cc == nil {
			http.Error(w, "not connected to the simulator", http.StatusServiceUnavailable)
			return
		}
		q := r.URL.Query()
		var found *traffic.Transmission
		for _, t := range cc.radio.Recent(strings.ToUpper(q.Get("icao")), clipCacheSize) {
			if t.At.Format(time.RFC3339Nano) == q.Get("at") && t.Callsign == q.Get("cs") && string(t.Intent) == q.Get("intent") {
				t := t
				found = &t
				break
			}
		}
		if found == nil {
			http.Error(w, "no such transmission", http.StatusNotFound)
			return
		}
		b, err := speaker.clip(*found)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Write(b)
	})
}
