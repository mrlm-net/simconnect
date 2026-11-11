package world

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// gatePlaces keeps, across starts, where the simulator last parked each
// helper body on each gate (placeholderParked): the gate's measure for a
// body (gatePlaceFor) from the first dock after a start, not from the
// second (live, LKPR C24 2026-11-11: an A332 helper's nose gear 5.4 m into
// the circle, the A320 placeholders' 13.7 m; measured by the placeholders,
// our B738 stood 8.3 m behind the A332 helper's door).
var gatePlaces = &gatePlaceStore{}

type gatePlace struct {
	ICAO  string  `json:"icao"`
	Stand int     `json:"stand"`
	Title string  `json:"title"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Hdg   float64 `json:"hdg"`
}

type gatePlaceStore struct {
	mu    sync.Mutex
	file  string
	items []gatePlace
}

// load reads file ("" none: kept in memory only).
func (s *gatePlaceStore) load(file string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file, s.items = file, nil
	if file == "" {
		return
	}
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.items); err != nil {
			fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		}
	}
}

// put keeps where title's body was parked on icao's stand, replacing that
// body's last, and saves.
func (s *gatePlaceStore) put(icao string, stand int, title string, lat, lon, hdg float64) {
	body := helperType(title)
	if body == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.items[:0]
	for _, p := range s.items {
		if !(p.ICAO == icao && p.Stand == stand && strings.EqualFold(helperType(p.Title), body)) {
			kept = append(kept, p)
		}
	}
	s.items = append(kept, gatePlace{ICAO: icao, Stand: stand, Title: title, Lat: lat, Lon: lon, Hdg: hdg})
	if s.file == "" {
		return
	}
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err == nil {
		err = os.WriteFile(s.file, b, 0o644)
	}
	if err != nil {
		fmt.Fprintf(stdout, "⚠️  %s: %v\n", s.file, err)
	}
}

// get is where helper's body was last parked on icao's stand.
func (s *gatePlaceStore) get(icao string, stand int, helper string) (parkedAt, bool) {
	body := helperType(helper)
	if body == "" {
		return parkedAt{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.items {
		if p.ICAO == icao && p.Stand == stand && strings.EqualFold(helperType(p.Title), body) {
			return parkedAt{p.Title, p.Lat, p.Lon, p.Hdg}, true
		}
	}
	return parkedAt{}, false
}
