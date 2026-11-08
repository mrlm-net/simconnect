package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// What one run learnt of the struts is handed to the injector of the next.
func TestStrutsStoreAcrossRuns(t *testing.T) {
	file := filepath.Join(t.TempDir(), "struts.json")
	b, _ := json.Marshal(traffic.StrutsLearnt{Struts: map[int]float64{850: 0.57}, Reach: map[int]float64{850: 0.45}})
	if err := os.WriteFile(file, b, 0o644); err != nil {
		t.Fatal(err)
	}
	s := &strutsStore{}
	s.load(file)
	inj := traffic.NewInjector(nil)
	stop := s.attach(inj)
	l, _ := inj.StrutsLearnt()
	stop()
	if l.Struts[850] != 0.57 || l.Reach[850] != 0.45 {
		t.Errorf("handed over: %+v", l)
	}
	s2 := &strutsStore{}
	s2.load(file)
	if s2.s.Struts[850] != 0.57 {
		t.Errorf("file after the run: %+v", s2.s)
	}
}
