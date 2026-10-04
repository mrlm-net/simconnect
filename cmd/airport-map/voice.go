//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	voicegoio "github.com/mrlm-net/voice-goio"
	"github.com/mrlm-net/voice-goio/speaker"
)

// The radio, heard (#419): what is said on the frequency the Radio tab
// follows goes through voice-goio's speaker — a voice per controller
// position, each pilot in a voice of their own, the ATIS in its voice on a
// loop while its frequency is followed — with the radio chain of each
// position. The rules live in the speaker, shared with the MyCrew app; the
// map adds following and tuning the user aircraft's COM1, the HTTP API and
// the camera cutting to the aircraft heard. Voice needs piper and a voice
// model (see the README); without them the button says so and the map
// stays silent.

type voiceOut struct {
	mu sync.Mutex
	sp *speaker.Speaker
	// piper: the piper executable and the voices folder ("" defaults),
	// set before first use.
	piperPath, voicesDir string
	// accents: controllers speak with their airport's accent (-accents;
	// off by default, voice-goio's Options.Accents).
	accents bool
	// syncCom follows the user aircraft's COM1 (com): tuning the radio in
	// the simulator picks the frequency heard. Off by default.
	syncCom bool
	com     string
	// tune sets the user aircraft's COM1 (MHz), in the connection's
	// goroutine; nil while not connected.
	tune func(mhz float64) error
	// atis is the current ATIS of the airport broadcasting on freq.
	atis func(freq string) (icao, text string, ok bool)
}

func newVoice() *voiceOut { return &voiceOut{} }

// speakerOf is the speaker, made on first use with the piper paths set.
func (v *voiceOut) speakerOf() *speaker.Speaker {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.sp == nil {
		v.sp = speaker.New(speaker.Options{
			PiperPath: v.piperPath, VoicesDir: v.voicesDir, Accents: v.accents, Hint: "see the airport-map README",
			ATIS: func(freq string) (string, string, bool) {
				v.mu.Lock()
				atis := v.atis
				v.mu.Unlock()
				if atis == nil {
					return "", "", false
				}
				return atis(freq)
			},
			OnSay: func(u speaker.Utterance) {
				cameraCut(traffic.Transmission{Airport: u.Airport, Position: traffic.Position(u.Position), Callsign: u.Callsign, Pilot: u.Pilot, Frequency: u.Frequency, Text: u.Text}) // the picture with the sound
			},
		})
	}
	return v.sp
}

// setATIS sets where the ATIS broadcast on a frequency comes from.
func (v *voiceOut) setATIS(f func(freq string) (icao, text string, ok bool)) {
	v.mu.Lock()
	v.atis = f
	v.mu.Unlock()
}

// utterance is transmission t as the speaker takes it: FAA numbers and
// frequencies at a US airport (#463).
func utterance(t traffic.Transmission) speaker.Utterance {
	ph := voicegoio.ICAO
	if t.Phraseology == traffic.PhraseologyFAA {
		ph = voicegoio.FAA
	}
	return speaker.Utterance{Airport: t.Airport, Position: string(t.Position), Callsign: t.Callsign,
		Pilot: t.Pilot, Frequency: t.Frequency, Text: t.Text, Phraseology: ph}
}

// set turns the voice on or off and picks the frequency followed.
func (v *voiceOut) set(on bool, freq string) {
	sp := v.speakerOf()
	sp.Set(on, freq)
	if st := sp.State(); on && !st.On {
		log.Printf("voice: %s", st.Status)
	}
}

type voiceState struct {
	speaker.State
	SyncCom bool   `json:"syncCom"`
	Com1    string `json:"com1,omitempty"`
}

func (v *voiceOut) state() voiceState {
	st := v.speakerOf().State()
	v.mu.Lock()
	defer v.mu.Unlock()
	return voiceState{State: st, SyncCom: v.syncCom, Com1: v.com}
}

// hear takes a transmission from the radio: said if the voice is on and it
// is on the frequency followed (one frequency, as on a receiver: nothing
// without one, #462). It never blocks the radio. The ATIS is broadcast
// on its own loop, not heard here.
func (v *voiceOut) hear(t traffic.Transmission) {
	if t.Intent == traffic.IntentATIS {
		return
	}
	v.speakerOf().Hear(utterance(t))
}

// sayOnce says t now, whatever the frequency followed (the airport panel's
// ATIS button); false when the voice is unavailable.
func (v *voiceOut) sayOnce(t traffic.Transmission) bool {
	return v.speakerOf().SayOnce(utterance(t))
}

// clipOf is t as said on the radio (16-bit mono) and its rate, for a
// client on the network that plays the radio itself (network.go).
func (v *voiceOut) clipOf(t traffic.Transmission) ([]int16, int, error) {
	return v.speakerOf().Clip(utterance(t))
}

// com1 takes the user aircraft's COM1 active frequency: followed at once
// while syncCom is on.
func (v *voiceOut) com1(freq string) {
	v.mu.Lock()
	v.com = freq
	sync := v.syncCom
	v.mu.Unlock()
	if !sync || freq == "" {
		return
	}
	if st := v.speakerOf().State(); freq != st.Frequency {
		v.set(st.On, freq)
	}
}

// setTune sets how COM1 is tuned (the connection's TransmitClientEvent).
func (v *voiceOut) setTune(f func(mhz float64) error) {
	v.mu.Lock()
	v.tune = f
	v.mu.Unlock()
}

// tuneCom1 tunes the user aircraft's COM1 to freq (as the radio writes it)
// and follows it at once.
func (v *voiceOut) tuneCom1(freq string) error {
	mhz, err := strconv.ParseFloat(freq, 64)
	if err != nil {
		return err
	}
	// A COM frequency ("NaN" or 1e30 would overflow the event's Hz).
	if !(mhz >= 118 && mhz < 137) {
		return fmt.Errorf("frequency %s: not a COM frequency (118 to 136.975 MHz)", freq)
	}
	v.mu.Lock()
	tune := v.tune
	v.com = freq
	v.mu.Unlock()
	if tune == nil {
		return errors.New("not connected to the simulator")
	}
	return tune(mhz)
}

// follow turns the COM1 sync on or off; on, it follows COM1 now.
func (v *voiceOut) follow(sync bool) {
	v.mu.Lock()
	v.syncCom = sync
	com := v.com
	v.mu.Unlock()
	if sync {
		v.com1(com)
	}
}

// registerVoice serves the voice switch: GET /api/voice is its state, POST
// /api/voice {on, frequency, syncCom} turns it on or off, picks the
// frequency, or follows the user aircraft's COM1.
func registerVoice(mux *http.ServeMux, v *voiceOut) {
	mux.HandleFunc("GET /api/voice", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, v.state())
	})
	mux.HandleFunc("POST /api/voice", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			On        bool   `json:"on"`
			Frequency string `json:"frequency"`
			SyncCom   *bool  `json:"syncCom"`
			// Tune: the frequency was picked on the map and COM1 is to be
			// tuned to it (following COM1, or "Tune my COM1").
			Tune bool `json:"tune"`
			// Device picks the output ("" the system default).
			Device *string `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.SyncCom != nil {
			v.follow(*req.SyncCom)
		}
		if req.Device != nil {
			if err := v.speakerOf().SetDevice(*req.Device); err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
		}
		if req.Tune && req.Frequency != "" {
			if err := v.tuneCom1(req.Frequency); err != nil {
				log.Printf("voice: tune COM1: %v", err)
			}
		}
		if st := v.state(); st.SyncCom && st.Com1 != "" {
			req.Frequency = st.Com1 // following COM1
		}
		v.set(req.On, req.Frequency)
		writeJSON(w, v.state())
	})
}

// radioVoice is the map's voice, one for the process.
var radioVoice = newVoice()

// cameraCut cuts the camera to a transmission's aircraft as the voice says
// it (the World's camera, set in main).
var cameraCut = func(traffic.Transmission) {}
