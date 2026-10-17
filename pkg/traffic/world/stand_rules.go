package world

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/dict"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The stand rules: where our aircraft stop on a stand, corrected per stand.
// The library's (traffic.standRules) with the local ones over them
// (DataDir/stand-rules.json, the GSX way: what is there wins, per stand),
// set on the map or by the host. Aircraft placed already keep their place;
// the next on the stand stops by it.

// standRuleStore keeps the local rules.
type standRuleStore struct {
	mu    sync.Mutex
	file  string
	rules []traffic.StandRuleItem
}

// loadStandRules reads file (missing: none) and puts its rules in use.
func loadStandRules(file string) *standRuleStore {
	s := &standRuleStore{file: file}
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.rules); err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
	if len(s.rules) > 0 {
		if err := s.apply(); err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
	return s
}

// apply puts the shipped rules with the local ones over them in use. s.mu
// held or not yet shared.
func (s *standRuleStore) apply() error {
	b, err := json.Marshal(s.rules)
	if err != nil {
		return err
	}
	return dict.Use("traffic.standRules", b)
}

// set adds or replaces the local rule of r.Stand, puts it in use and saves.
func (s *standRuleStore) set(r traffic.StandRuleItem) error {
	r.Stand = strings.ToUpper(strings.Join(strings.Fields(r.Stand), " "))
	if len(strings.Fields(r.Stand)) != 2 {
		return fmt.Errorf("a stand rule needs a stand as \"ICAO LABEL\", like \"LKPR B14\"")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.rules, func(x traffic.StandRuleItem) bool { return x.Stand == r.Stand })
	if i >= 0 {
		s.rules[i] = r
	} else {
		s.rules = append(s.rules, r)
	}
	if err := s.apply(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0o644)
}

// StandRules are the stand rules in use: the library's with the local ones
// over them.
func (w *World) StandRules() []traffic.StandRuleItem { return traffic.StandRules() }

// SetStandRule sets a local stand rule (DataDir/stand-rules.json); a rule
// with nothing to move ({"stand": "LKPR B14"}) takes a shipped one back.
func (w *World) SetStandRule(r traffic.StandRuleItem) error {
	if err := w.st.standRules.set(r); err != nil {
		return err
	}
	w.st.core.log.printf("stand %s: stop %+.1f m forward, %+.1f m right (wingspan up to %.0f m, 0 any)", r.Stand, r.ForwardM, r.RightM, r.MaxSpanM)
	return nil
}

// registerStandRules serves GET /api/stands/rules (the rules in use) and
// POST /api/stands/rules {"stand": "LKPR B14", "forwardM": -13, "rightM": 0,
// "maxSpanM": 36.5, "reason": …}.
func registerStandRules(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/stands/rules", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, w.StandRules())
	})
	mux.HandleFunc("POST /api/stands/rules", func(rw http.ResponseWriter, r *http.Request) {
		var req traffic.StandRuleItem
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if err := w.SetStandRule(req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(rw, w.StandRules())
	})
}
