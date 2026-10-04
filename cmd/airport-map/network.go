//go:build windows
// +build windows

package main

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
	"github.com/mrlm-net/voice-goio/speaker"
)

// clips caches the WAVs of recent transmissions, by key.
var clips = struct {
	sync.Mutex
	m     map[string][]byte
	order []string
}{m: map[string][]byte{}}

const clipCacheSize = 200

// clipMaking makes one clip at a time: devices playing the same call (and a
// browser fetching the next clips ahead) wait for it rather than synthesise
// it again, each at once.
var clipMaking sync.Mutex

// clipKey names a transmission: its time and call sign.
func clipKey(t traffic.Transmission) string {
	return t.At.Format(time.RFC3339Nano) + " " + t.Callsign + " " + string(t.Intent)
}

// clip is transmission t as said on the radio, a 16-bit mono WAV.
func (v *voiceOut) clip(t traffic.Transmission) ([]byte, error) {
	key := clipKey(t)
	cached := func() ([]byte, bool) {
		clips.Lock()
		defer clips.Unlock()
		b, ok := clips.m[key]
		return b, ok
	}
	if b, ok := cached(); ok {
		return b, nil
	}
	clipMaking.Lock()
	defer clipMaking.Unlock()
	if b, ok := cached(); ok {
		return b, nil // made while this one waited
	}
	if t.Intent == traffic.IntentATIS {
		t.Position = traffic.PosATIS // the speaker's ATIS voice
	}
	out, rate, err := v.clipOf(t)
	if err != nil {
		return nil, err
	}
	b := speaker.WAV(out, rate)
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

// registerNetwork serves the clips:
//
//	GET /api/voice/clip?icao=&at=&cs=&intent= — a transmission of /api/radio
//	    (its at, callsign and intent) as a WAV, for a client playing the
//	    radio on its own device.
func registerNetwork(mux *http.ServeMux, wd *world.World) {
	mux.HandleFunc("GET /api/voice/clip", func(w http.ResponseWriter, r *http.Request) {
		if !wd.Connected() {
			http.Error(w, "not connected to the simulator", http.StatusServiceUnavailable)
			return
		}
		q := r.URL.Query()
		var found *traffic.Transmission
		for _, t := range wd.Recent(strings.ToUpper(q.Get("icao")), clipCacheSize) {
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
		b, err := radioVoice.clip(*found)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Write(b)
	})
}
