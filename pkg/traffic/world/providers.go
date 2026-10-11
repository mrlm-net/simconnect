package world

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Model providers (traffic.ModelProvider): which packages our traffic's
// models come from, by kind, each switched on or off. Flyable add-ons (the
// user's own aircraft) start off; a kind left with none switched on takes
// any (traffic.PreferProviders).

// ProviderView is one provider of a kind of model: "aircraft", or a ground
// vehicle kind ("tug", "fuel", "stairs", "gpu", "bus", "followme"), how
// many of the simulator's titles it has, and whether it is used.
type ProviderView struct {
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Titles   int    `json:"titles"`
	On       bool   `json:"on"`
	Flyable  bool   `json:"flyable,omitempty"` // the user's add-on aircraft (off by default)
}

// ModelProviders are the providers of the simulator's models now, by kind
// (none before it has listed them).
func (w *World) ModelProviders() []ProviderView {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return nil
	}
	cc.mu.Lock()
	ft := cc.fuelTitles
	cc.mu.Unlock()
	kinds := []struct {
		kind   string
		titles []string
	}{
		{"aircraft", cc.modelList()},
		{string(traffic.VehicleTug), ft.tugs},
		{string(traffic.VehicleFuel), concat(ft.trucks, ft.hydrants, ft.stock)},
		{string(traffic.VehicleStairs), concat(ft.stairs, ft.stairsStock)},
		{string(traffic.VehicleGPU), concat(ft.gpus, ft.gpusStock)},
		{string(traffic.VehicleBus), concat(ft.buses, ft.busesStock)},
		{string(traffic.VehicleFollowMe), ft.followMe},
	}
	var out []ProviderView
	for _, k := range kinds {
		count := map[string]int{}
		for _, t := range k.titles {
			count[traffic.ModelProvider(t)]++
		}
		names := make([]string, 0, len(count))
		for p := range count {
			names = append(names, p)
		}
		slices.Sort(names)
		for _, p := range names {
			out = append(out, ProviderView{Kind: k.kind, Provider: p, Titles: count[p], On: traffic.ProviderOn(p), Flyable: traffic.ProviderFlyable(p)})
		}
	}
	return out
}

func concat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

var fallbackLogged sync.Map // first title → logged

// logProviderFallback logs once per choice that no provider switched on
// had a model, so one switched off was taken.
func logProviderFallback(titles []string) {
	if _, done := fallbackLogged.LoadOrStore(titles[0], true); done {
		return
	}
	tlog.printf("models: none of a provider switched on for %s (%d titles): taken from %s", titles[0], len(titles), traffic.ModelProvider(titles[0]))
}

// SetModelProviders switches providers on or off (the others as they
// are): the next model chosen passes over those off. Models in use keep
// going.
func (w *World) SetModelProviders(on map[string]bool) {
	traffic.SetProviders(on)
	for p, v := range on {
		w.st.core.log.printf("model provider %s %s", p, map[bool]string{true: "on", false: "off"}[v])
	}
}

// registerModelProviders serves GET /api/models/providers and POST
// /api/models/providers {"FSLTL": true, "FlyByWire Simulations": false}.
func registerModelProviders(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/models/providers", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, w.ModelProviders())
	})
	mux.HandleFunc("POST /api/models/providers", func(rw http.ResponseWriter, r *http.Request) {
		var req map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		for p := range req {
			if strings.TrimSpace(p) == "" {
				http.Error(rw, "a provider needs a name", http.StatusBadRequest)
				return
			}
		}
		w.SetModelProviders(req)
		writeJSON(rw, w.ModelProviders())
	})
}
