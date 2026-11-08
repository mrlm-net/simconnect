package world

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// strutsLessons keeps what the injector learnt of the models' struts
// (traffic.StrutsLearnt) in DataDir/struts.json across runs.
var strutsLessons = &strutsStore{}

// strutsSaveEvery: how often what was learnt is written, when it changed.
const strutsSaveEvery = time.Minute

type strutsStore struct {
	mu   sync.Mutex
	file string
	s    traffic.StrutsLearnt
}

func (s *strutsStore) load(file string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file, s.s = file, traffic.StrutsLearnt{}
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.s); err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
}

// attach hands inj what was learnt before and saves what it learns until
// the returned stop is called (saving once more then).
func (s *strutsStore) attach(inj *traffic.Injector) (stop func()) {
	s.mu.Lock()
	file, before := s.file, s.s
	s.mu.Unlock()
	if inj == nil || file == "" {
		return func() {}
	}
	inj.SetStrutsLearnt(before)
	_, saved := inj.StrutsLearnt()
	save := func() {
		l, gen := inj.StrutsLearnt()
		if gen == saved {
			return
		}
		saved = gen
		s.mu.Lock()
		s.s = l
		s.mu.Unlock()
		b, err := json.MarshalIndent(l, "", "  ")
		if err == nil {
			err = os.WriteFile(file, b, 0o644)
		}
		if err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		t := time.NewTicker(strutsSaveEvery)
		defer t.Stop()
		for {
			select {
			case <-done:
				save()
				return
			case <-t.C:
				save()
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done); <-finished }) }
}
