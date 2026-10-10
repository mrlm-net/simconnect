package world

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A rule set on the map (POST /api/models/rules) is in use at once, saved
// beside the settings and read again by the next World (#1024).
func TestModelRulesAPI(t *testing.T) {
	defer dict.Reset("traffic.modelRules")
	dir := t.TempDir()
	w := New(Options{DataDir: dir})
	mux := http.NewServeMux()
	registerModelRules(mux, w)
	body, _ := json.Marshal(traffic.ModelRuleItem{Title: "FSDT_GPU_TLD_406", Kind: "gpu", Never: true, Reason: "set on the map"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/models/rules", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: %d %s", rec.Code, rec.Body)
	}
	if !traffic.ModelNever("FSDT_GPU_TLD_406") || !traffic.ModelNever("Car Ground Power Unit") {
		t.Error("the rule set, or the shipped ones, not in use")
	}
	if _, err := os.Stat(filepath.Join(dir, "model-rules.json")); err != nil {
		t.Fatal(err)
	}
	if err := dict.Reset("traffic.modelRules"); err != nil {
		t.Fatal(err)
	}
	New(Options{DataDir: dir})
	if !traffic.ModelNever("FSDT_GPU_TLD_406") {
		t.Error("the saved rule not read again")
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/models/rules", bytes.NewReader([]byte(`{"title":""}`))))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a rule without a title: %d", rec.Code)
	}
}
