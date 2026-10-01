//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
	voicegoio "github.com/mrlm-net/voice-goio"
	"github.com/mrlm-net/voice-goio/audio"
	"github.com/mrlm-net/voice-goio/audio/radio"
	"github.com/mrlm-net/voice-goio/normalise"
	"github.com/mrlm-net/voice-goio/tts"
	"github.com/mrlm-net/voice-goio/tts/piper"
	"github.com/mrlm-net/voice-goio/voices"
)

// The radio, heard (#419): what is said on the frequency the Radio tab
// follows (or all of them) goes through voice-goio — a voice per controller
// position, each pilot in a voice of their own, the ATIS in its voice on a
// loop while its frequency is followed — with the radio chain of each
// position. Voice needs piper and a voice model (see the README); without
// them the button says so and the map stays silent.

// voiceMaxLagSeconds: a transmission not yet said this long after it was
// made is dropped, so a busy frequency stays live rather than minutes
// behind.
const voiceMaxLagSeconds = 60

// voiceQueueKey plays everything through one player queue: one frequency or
// all, one thing at a time, as on a single receiver.
const voiceQueueKey = "radio"

// The pauses a frequency has (#419; voice-goio waits for each transmission
// to finish): a reply to the same aircraft follows after voiceReplyGap, a new
// exchange after voiceExchangeGap, each with up to voiceGapJitter more.
const (
	voiceReplyGap    = 800 * time.Millisecond
	voiceExchangeGap = 2 * time.Second
	voiceGapJitter   = 1500 * time.Millisecond
)

type voiceOut struct {
	mu      sync.Mutex
	on      bool
	freq    string // followed; "" all
	status  string // why it is silent, or the backend speaking
	backend string
	// syncCom follows the user aircraft's COM1 (com1): tuning the radio in
	// the simulator picks the frequency heard. Off by default.
	syncCom bool
	com     string

	engine voicegoio.TTS
	pool   *voices.Pool
	chain  *radio.Set
	norm   *normalise.Normaliser
	player *audio.Player

	queue chan voiceItem
	// The ATIS broadcast: its text, its audio, when its loop started.
	atisText  string
	atisPCM   []int16
	atisStart time.Time
	// The pauses between transmissions: when the last one ends and who it
	// was to or from, so a readback follows its clearance closely and a new
	// exchange after a breath.
	lastEnd time.Time
	lastCS  string
	rng     *rand.Rand
	// piper: the piper executable and the voices folder ("" defaults).
	piperPath, voicesDir string
	// atis is the current ATIS of the airport broadcasting on freq.
	atis func(freq string) (icao, text string, ok bool)
}

type voiceItem struct {
	t    traffic.Transmission
	when time.Time
}

func newVoice() *voiceOut {
	v := &voiceOut{queue: make(chan voiceItem, 64), status: "off", rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x70ce))}
	go v.run()
	return v
}

// open starts the voice pipeline on first use; v.mu held.
func (v *voiceOut) open() error {
	if v.engine == nil {
		engine, backend, err := tts.Open(tts.Options{Piper: piper.Options{PiperPath: v.piperPath, VoicesDir: v.voicesDir}})
		if err != nil {
			return err
		}
		if backend != tts.BackendPiper {
			engine.Close()
			return errors.New("no voice: piper and a voice model are needed (see the airport-map README)")
		}
		man, err := voices.LoadDefault()
		if err != nil {
			engine.Close()
			return err
		}
		v.engine, v.backend = engine, backend
		// Not the Czech model reading English: it sounds wrong on the radio.
		v.pool = voices.NewPool(man, voices.PoolOptions{Seed: time.Now().UnixNano(), AllowUnaudited: true, Dir: v.voicesDir, Exclude: []string{"cs_CZ-jirka-medium"}})
		v.chain, v.norm = radio.Default(), normalise.New()
	}
	if v.player == nil {
		p, err := audio.NewPlayer(audio.Options{})
		if err != nil {
			return err
		}
		go func() {
			for range p.Events() { // drained: the player needs it
			}
		}()
		v.player = p
	}
	return nil
}

// set turns the voice on or off and picks the frequency followed.
func (v *voiceOut) set(on bool, freq string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if freq != v.freq && v.player != nil {
		v.player.Close() // another frequency: what the last one still had goes
		v.player = nil
		for len(v.queue) > 0 {
			<-v.queue
		}
	}
	v.freq = freq
	if !on {
		v.on, v.status = false, "off"
		if v.player != nil {
			v.player.Close() // what is queued goes with it
			v.player = nil
		}
		return
	}
	if err := v.open(); err != nil {
		v.on, v.status = false, err.Error()
		log.Printf("voice: %v", err)
		return
	}
	v.on, v.status = true, "on ("+v.backend+")"
}

type voiceState struct {
	On        bool   `json:"on"`
	Frequency string `json:"frequency"`
	Status    string `json:"status"`
	SyncCom   bool   `json:"syncCom"`
	Com1      string `json:"com1,omitempty"`
}

func (v *voiceOut) state() voiceState {
	v.mu.Lock()
	defer v.mu.Unlock()
	return voiceState{On: v.on, Frequency: v.freq, Status: v.status, SyncCom: v.syncCom, Com1: v.com}
}

// hear takes a transmission from the radio: said if the voice is on and it
// is on the frequency followed. It never blocks the radio.
func (v *voiceOut) hear(t traffic.Transmission) {
	v.mu.Lock()
	// One frequency, as on a receiver: nothing without one (#462).
	skip := !v.on || v.freq == "" || t.Frequency != v.freq || t.Intent == traffic.IntentATIS
	v.mu.Unlock()
	if skip {
		return
	}
	select {
	case v.queue <- voiceItem{t, time.Now()}:
	default: // behind: drop it
	}
}

// run says what is queued; while the followed frequency is an ATIS and
// nothing else is to be said, it says the ATIS again and again.
func (v *voiceOut) run() {
	for {
		select {
		case it := <-v.queue:
			if time.Since(it.when) > voiceMaxLagSeconds*time.Second {
				continue
			}
			v.say(it.t, false)
		case <-time.After(voiceTick):
			v.mu.Lock()
			on, freq, atis := v.on, v.freq, v.atis
			v.mu.Unlock()
			if !on || freq == "" || atis == nil {
				continue
			}
			if icao, text, ok := atis(freq); ok {
				v.broadcast(icao, freq, text)
			}
		}
	}
}

// atisGap is the silence between two runs of an ATIS broadcast.
const atisGap = 3 * time.Second

// broadcast plays the ATIS on freq as a continuous broadcast (#462): each
// information is synthesised once and loops from a fixed start, so tuning
// in joins it where it is, mid-sentence, as on a real receiver. It returns
// after one run, or when the frequency or the information changes.
func (v *voiceOut) broadcast(icao, freq, text string) {
	v.mu.Lock()
	if !v.on || v.player == nil || v.engine == nil {
		v.mu.Unlock()
		return
	}
	engine, pool, chain, norm, player := v.engine, v.pool, v.chain, v.norm, v.player
	cached := v.atisText == text && v.atisPCM != nil
	v.mu.Unlock()
	if !cached {
		voice := pool.Assign(icao, voicegoio.ATIS)
		pcm, err := engine.Synthesize(context.Background(), voice, norm.Spoken(text, voicegoio.ICAO))
		if err != nil {
			log.Printf("voice: %v", err)
			return
		}
		out := chain.Apply(pcm, engine.SampleRate(voice), voice.Radio, player.SampleRate(), 1)
		v.mu.Lock()
		v.atisText, v.atisPCM, v.atisStart = text, out, time.Now()
		v.mu.Unlock()
	}
	v.mu.Lock()
	out, start := v.atisPCM, v.atisStart
	v.mu.Unlock()
	rate := player.SampleRate()
	length := time.Duration(float64(len(out)) / float64(rate) * float64(time.Second))
	period := length + atisGap
	into := time.Since(start) % period
	if into >= length {
		// Between two runs: the next one from its start, unless the
		// frequency changes meanwhile.
		if !v.wait(period-into, freq, player) {
			return
		}
		into = 0
	}
	from := int(into.Seconds() * float64(rate))
	if err := player.Play(voicegoio.Transmission{Frequency: voiceQueueKey, ControllerID: "atis", Phraseology: voicegoio.ICAO, Text: text}, out[from:], rate); err != nil {
		return
	}
	// Wait while it plays, but stop listening for it at a change of
	// frequency (set closes the player).
	v.wait(length-into, freq, player)
}

// voiceTick is how often the voice looks for an ATIS to broadcast and,
// while waiting, for a change of frequency.
const voiceTick = 100 * time.Millisecond

// wait waits d while the voice stays on freq with player, and reports
// whether it did: a change of frequency (set closes the player) ends it at
// once, so the new frequency is heard without the old one's delay.
func (v *voiceOut) wait(d time.Duration, freq string, player *audio.Player) bool {
	for end := time.Now().Add(d); time.Now().Before(end); {
		v.mu.Lock()
		same := v.freq == freq && v.player == player && v.on
		v.mu.Unlock()
		if !same {
			return false
		}
		time.Sleep(min(voiceTick, time.Until(end)))
	}
	return true
}

// say synthesises t in its speaker's voice and waits while it is played;
// force says it with the voice off (a one-off asked for).
func (v *voiceOut) say(t traffic.Transmission, force bool) {
	v.mu.Lock()
	if !v.on && !force || v.player == nil {
		v.mu.Unlock()
		return
	}
	engine, pool, chain, norm, player, freq := v.engine, v.pool, v.chain, v.norm, v.player, v.freq
	v.mu.Unlock()

	var voice voicegoio.VoiceProfile
	if t.Pilot {
		voice = pool.Assign(t.Callsign, voicegoio.Center) // each crew its own voice
	} else {
		voice = pool.Assign(t.Airport, controllerKind(t.Position))
	}
	pcm, err := engine.Synthesize(context.Background(), voice, norm.Spoken(t.Text, voicegoio.ICAO))
	if err != nil {
		log.Printf("voice: %v", err)
		return
	}
	out := chain.Apply(pcm, engine.SampleRate(voice), voice.Radio, player.SampleRate(), int64(len(t.Text)))
	// The pause since the last transmission, synthesis included.
	v.mu.Lock()
	gap := voiceExchangeGap + time.Duration(v.rng.Int64N(int64(voiceGapJitter)))
	if t.Callsign != "" && t.Callsign == v.lastCS { // a reply: short
		gap = voiceReplyGap + time.Duration(v.rng.Int64N(int64(voiceGapJitter/3)))
	}
	if len(v.queue) > 2 {
		gap /= 2 // behind: shorter pauses rather than dropping calls
	}
	wait := time.Until(v.lastEnd.Add(gap))
	v.mu.Unlock()
	if wait > 0 && !v.wait(wait, freq, player) {
		return // another frequency meanwhile
	}
	who := string(t.Position)
	if t.Pilot {
		who = t.Callsign
	}
	if err := player.Play(voicegoio.Transmission{Frequency: voiceQueueKey, ControllerID: who, Phraseology: voicegoio.ICAO, Text: t.Text}, out, player.SampleRate()); err != nil {
		return // turned off meanwhile
	}
	// Wait while it is said, so the queue stays on the lag it has.
	said := time.Duration(float64(len(out)) / float64(player.SampleRate()) * float64(time.Second))
	v.mu.Lock()
	v.lastEnd, v.lastCS = time.Now().Add(said), t.Callsign
	v.mu.Unlock()
	v.wait(said, freq, player)
}

// controllerKind is voice-goio's kind for a position: its voice and radio.
func controllerKind(p traffic.Position) voicegoio.ControllerKind {
	switch p {
	case traffic.PosDelivery, traffic.PosGround:
		return voicegoio.Ground
	case traffic.PosApproach, traffic.PosDeparture:
		return voicegoio.Approach
	case traffic.PosCenter:
		return voicegoio.Center
	case traffic.PosATIS:
		return voicegoio.ATIS
	default:
		return voicegoio.Tower
	}
}

// sayOnce says t now, whatever the frequency followed (the airport panel's
// ATIS button); false when the voice is off or unavailable.
func (v *voiceOut) sayOnce(t traffic.Transmission) bool {
	v.mu.Lock()
	if !v.on {
		if err := v.open(); err != nil {
			v.status = err.Error()
			v.mu.Unlock()
			return false
		}
	}
	player := v.player
	v.mu.Unlock()
	if player == nil {
		return false
	}
	go v.say(t, true)
	return true
}

// com1 takes the user aircraft's COM1 active frequency: followed at once
// while syncCom is on.
func (v *voiceOut) com1(freq string) {
	v.mu.Lock()
	v.com = freq
	follow := v.syncCom && freq != "" && freq != v.freq
	on := v.on
	v.mu.Unlock()
	if follow {
		v.set(on, freq)
	}
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
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.SyncCom != nil {
			v.follow(*req.SyncCom)
		}
		if st := v.state(); st.SyncCom && st.Com1 != "" {
			req.Frequency = st.Com1 // following COM1
		}
		v.set(req.On, req.Frequency)
		writeJSON(w, v.state())
	})
}

// speaker is the map's voice, one for the process.
var speaker = newVoice()
