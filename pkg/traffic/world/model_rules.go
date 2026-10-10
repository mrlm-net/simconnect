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

// The model rules (#1024): simulator models our traffic never uses. The
// library's (traffic.modelRules) with the local ones over them
// (DataDir/model-rules.json, the GSX way: what is there wins, per model),
// set on the map ("never use this model") or by the host.

// modelRuleStore keeps the local rules.
type modelRuleStore struct {
	mu    sync.Mutex
	file  string
	rules []traffic.ModelRuleItem
}

// loadModelRules reads file (missing: none) and puts its rules in use.
func loadModelRules(file string) *modelRuleStore {
	s := &modelRuleStore{file: file}
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
func (s *modelRuleStore) apply() error {
	b, err := json.Marshal(s.rules)
	if err != nil {
		return err
	}
	return dict.Use("traffic.modelRules", b)
}

// set adds or replaces the local rule of r.Title, puts it in use and
// saves.
func (s *modelRuleStore) set(r traffic.ModelRuleItem) error {
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return fmt.Errorf("a model rule needs a title")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.rules, func(x traffic.ModelRuleItem) bool { return strings.EqualFold(x.Title, r.Title) })
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

// ModelRules are the model rules in use: the library's with the local ones
// over them.
func (w *World) ModelRules() []traffic.ModelRuleItem { return traffic.ModelRules() }

// SetModelRule sets a local model rule (DataDir/model-rules.json): never
// use the model r.Title (r.Never), or use it again, a shipped rule
// included. Models in use keep going; the next choice passes it over.
func (w *World) SetModelRule(r traffic.ModelRuleItem) error {
	if err := w.st.modelRules.set(r); err != nil {
		return err
	}
	verb := "used again"
	if r.Never {
		verb = "never used"
	}
	w.st.core.log.printf("model %q (%s) %s", r.Title, r.Kind, verb)
	return nil
}

// registerModelRules serves GET /api/models/rules (the rules in use) and
// POST /api/models/rules {"title": …, "kind": …, "never": true, "reason": …}.
func registerModelRules(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/models/rules", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, w.ModelRules())
	})
	mux.HandleFunc("POST /api/models/rules", func(rw http.ResponseWriter, r *http.Request) {
		var req traffic.ModelRuleItem
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if err := w.SetModelRule(req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(rw, w.ModelRules())
	})
}
