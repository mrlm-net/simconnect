package world

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// The approach aids of an airport: each runway end's ILS (its ident from
// the RUNWAY record, PRIMARY_ILS_ICAO), and from its navaid record the
// frequency and name, loaded when the airport is.

// ilsInfo is one runway end's ILS.
type ilsInfo struct {
	Runway string  `json:"runway"`
	Ident  string  `json:"ident"`
	MHz    float64 `json:"mhz"`
	Name   string  `json:"name,omitempty"`
}

// ilsAids holds the airports' ILS, and the lookups under way.
type ilsStore struct {
	sync.Mutex
	byICAO  map[string]map[string]ilsInfo // ICAO → runway end → ILS
	pending map[nav.FixKey][]ilsRef
	// queued: lookups the loader had no room for yet, asked again each
	// second (#80: more than its slots at once were dropped for good).
	queued []nav.FixKey
	tries  map[nav.FixKey]int // retries of each queued lookup, at most ilsRetries
}

type ilsRef struct{ icao, runway string }

// resetILS forgets the lookups under way: on a new connection they would
// never be answered, and an airport asked for again must be asked anew.
func (k *core) resetILS() {
	k.ils.Lock()
	k.ils.pending = map[nav.FixKey][]ilsRef{}
	k.ils.queued = nil
	k.ils.Unlock()
}

// requestILS asks loader for the ILS of every runway end of l. On the
// connection's goroutine.
func (k *core) requestILS(loader *nav.NavLoader, l *airport.Layout) {
	for _, r := range l.Runways {
		for _, e := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			if e.ILS == "" {
				continue
			}
			key := nav.Key(e.ILS, e.ILSRegion, nav.KindVOR)
			k.ils.Lock()
			first := len(k.ils.pending[key]) == 0
			k.ils.pending[key] = append(k.ils.pending[key], ilsRef{l.ICAO, e.Name})
			k.ils.Unlock()
			if !first {
				continue
			}
			if err := loader.RequestNavaid(key); err != nil {
				k.ils.Lock()
				k.ils.queued = append(k.ils.queued, key) // asked again (retryILS)
				k.ils.Unlock()
			}
		}
	}
}

// gotILS stores a loaded ILS for the runway ends that asked for it.
func (k *core) gotILS(r nav.NavResult) {
	k.ils.Lock()
	refs := k.ils.pending[r.Key]
	delete(k.ils.pending, r.Key)
	k.ils.Unlock()
	if !r.Found || r.Fix.Freq == 0 {
		for _, ref := range refs {
			fmt.Fprintf(os.Stderr, "⚠️  ILS %s of %s %s: no navaid record\n", r.Key.Ident, ref.icao, ref.runway)
		}
		return
	}
	k.ils.Lock()
	defer k.ils.Unlock()
	for _, ref := range refs {
		if k.ils.byICAO[ref.icao] == nil {
			k.ils.byICAO[ref.icao] = map[string]ilsInfo{}
		}
		k.ils.byICAO[ref.icao][ref.runway] = ilsInfo{Runway: ref.runway, Ident: r.Key.Ident, MHz: r.Fix.Freq, Name: r.Fix.Name}
		fmt.Fprintf(stdout, "📡 %s ILS %s: %s %.2f %s\n", ref.icao, ref.runway, r.Key.Ident, r.Fix.Freq, r.Fix.Name)
	}
}

// ilsOf is icao's ILS by runway end, in runway order.
func (k *core) ilsOf(icao string) []ilsInfo {
	k.ils.Lock()
	defer k.ils.Unlock()
	out := make([]ilsInfo, 0, len(k.ils.byICAO[icao]))
	for _, i := range k.ils.byICAO[icao] {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Runway < out[b].Runway })
	return out
}

// retryILS asks again for the lookups the loader had no room for. On the
// connection's goroutine.
func (k *core) retryILS(loader *nav.NavLoader) {
	k.ils.Lock()
	defer k.ils.Unlock()
	if k.ils.tries == nil {
		k.ils.tries = map[nav.FixKey]int{}
	}
	for len(k.ils.queued) > 0 {
		key := k.ils.queued[0]
		if err := loader.RequestNavaid(key); err != nil {
			if k.ils.tries[key]++; k.ils.tries[key] < ilsRetries {
				return // still full: next time
			}
			fmt.Fprintf(os.Stderr, "❌ ILS %s: %v\n", key.Ident, err)
			delete(k.ils.pending, key)
		}
		delete(k.ils.tries, key)
		k.ils.queued = k.ils.queued[1:]
	}
}

// ilsRetries: a lookup the loader keeps refusing is given up after this
// many seconds.
const ilsRetries = 60
