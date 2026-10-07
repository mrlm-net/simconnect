package world

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Pushbacks drawn by hand (traffic.SetCustomPush), kept beside the other
// local settings (DataDir/custom-pushes.json) and set again at start:
//
//	GET    /api/pushback?icao=LKPR&stand=S6&model=… — {standard, custom}: the stand's
//	       standard push (planned now if not yet) and its drawn one
//	PUT    /api/pushback?icao=LKPR&stand=S6         — a traffic.PushRoute: flown as drawn from now on
//	DELETE /api/pushback?icao=LKPR&stand=S6         — back to the planned push

// pushStore keeps the drawn pushes: ICAO → stand name → route.
type pushStore struct {
	mu     sync.Mutex
	file   string
	pushes map[string]map[string]traffic.PushRoute
}

// loadPushStore reads file and sets every push in it.
func loadPushStore(file string) *pushStore {
	s := &pushStore{file: file, pushes: map[string]map[string]traffic.PushRoute{}}
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.pushes); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", file, err)
		}
	}
	for icao, stands := range s.pushes {
		for stand, r := range stands {
			traffic.SetCustomPush(icao, stand, r)
		}
	}
	return s
}

// set draws (r non-nil) or clears stand's push at icao and saves.
func (s *pushStore) set(icao, stand string, r *traffic.PushRoute) error {
	icao, stand = strings.ToUpper(icao), strings.ToUpper(stand)
	s.mu.Lock()
	defer s.mu.Unlock()
	if r == nil {
		traffic.ClearCustomPush(icao, stand)
		delete(s.pushes[icao], stand)
		if len(s.pushes[icao]) == 0 {
			delete(s.pushes, icao)
		}
	} else {
		traffic.SetCustomPush(icao, stand, *r)
		r, _ := traffic.CustomPush(icao, stand)
		if s.pushes[icao] == nil {
			s.pushes[icao] = map[string]traffic.PushRoute{}
		}
		s.pushes[icao][stand] = r
	}
	b, err := json.MarshalIndent(s.pushes, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil { // a data folder not made yet (#86)
		return err
	}
	return os.WriteFile(s.file, b, 0o644)
}

func registerPushback(mux *http.ServeMux, st *state) {
	stand := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		icao, name := strings.ToUpper(r.URL.Query().Get("icao")), strings.ToUpper(r.URL.Query().Get("stand"))
		if icao == "" || name == "" {
			http.Error(w, "icao and stand required", http.StatusBadRequest)
			return "", "", false
		}
		return icao, name, true
	}
	mux.HandleFunc("GET /api/pushback", func(w http.ResponseWriter, r *http.Request) {
		icao, name, ok := stand(w, r)
		if !ok {
			return
		}
		g, err := st.cache.Graph(icao)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		idx, err := g.Layout.ParkingIndex(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		model := r.URL.Query().Get("model")
		if model == "" {
			model = standardPushModel
		}
		var out struct {
			Standard *traffic.PushRoute `json:"standard,omitempty"`
			Custom   *traffic.PushRoute `json:"custom,omitempty"`
		}
		if p, ok := traffic.StandardPush(g, idx, model); ok {
			out.Standard = &p
		}
		if p, ok := traffic.CustomPush(icao, name); ok {
			out.Custom = &p
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("PUT /api/pushback", func(w http.ResponseWriter, r *http.Request) {
		icao, name, ok := stand(w, r)
		if !ok {
			return
		}
		var p traffic.PushRoute
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || len(p.Points) < 2 {
			http.Error(w, "a push route with at least two points", http.StatusBadRequest)
			return
		}
		if err := st.pushes.set(icao, name, &p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/pushback", func(w http.ResponseWriter, r *http.Request) {
		icao, name, ok := stand(w, r)
		if !ok {
			return
		}
		if err := st.pushes.set(icao, name, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
