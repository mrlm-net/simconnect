//go:build windows
// +build windows

package world

import (
	"net/http"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Procedures on the map (#314): SIDs, STARs and approaches as paths, and
// the fixes they use, as charts show them (airport.Charts).

// registerProcedures serves GET /api/procedures?icao=X.
func registerProcedures(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/procedures", func(w http.ResponseWriter, r *http.Request) {
		icao := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("icao")))
		g, err := st.cache.Graph(icao)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		st.mu.Lock()
		p, ok := st.procedures[g.Layout.ICAO]
		st.mu.Unlock()
		if !ok {
			http.Error(w, "procedures of "+g.Layout.ICAO+" not loaded (yet)", http.StatusNotFound)
			return
		}
		writeJSON(w, airport.Charts(g.Layout, p))
	})
}
