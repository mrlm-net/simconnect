//go:build windows
// +build windows

package world

import (
	"net/http"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The separation monitor (#395): every second, every pair of airborne
// aircraft (ours and other traffic) closer than 5 NM and 1000 ft at once
// is logged when it starts and when it ends, with its closest distance.
//
//	GET /api/separation — the closest pairs now, the losses so far, and the
//	predicted conflicts with the resolutions given (conflicts.go)

// sepMinNM is the lateral minimum watched en route (EnrouteSeparationNM);
// sepTerminalNM in the terminal area, both aircraft at an airport below
// 10000 ft (TerminalSeparationNM, Doc 4444 8.7.3). The landing sequence
// keeps sepMinNM in trail.
const (
	sepMinNM      = traffic.EnrouteSeparationNM
	sepTerminalNM = traffic.TerminalSeparationNM
)

type sepMonitor struct {
	log    *trafficLog
	mu     sync.Mutex
	open   map[string]*sepLoss // by pair, while it lasts
	losses []sepLoss           // ended, the latest last (at most 50)
	now    []traffic.SeparationPair
	// needed: whether two call signs need separation where they are
	// (airspace classes, #570); nil: always.
	needed func(a, b string) bool
}

type sepLoss struct {
	A          string    `json:"a"`
	B          string    `json:"b"`
	Since      time.Time `json:"since"`
	Until      time.Time `json:"until,omitempty"`
	ClosestNM  float64   `json:"closestNM"`
	VerticalFt float64   `json:"verticalFt"`
}

func newSepMonitor(log *trafficLog) *sepMonitor {
	return &sepMonitor{log: log, open: map[string]*sepLoss{}}
}

func (m *sepMonitor) tick(now time.Time, aircraft []traffic.TrackedAircraft) {
	pairs := traffic.AirborneSeparationFor(aircraft, conflictOpts)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = pairs
	seen := map[string]bool{}
	for _, p := range pairs {
		if !p.Loss || m.needed != nil && !m.needed(p.A, p.B) {
			continue // no separation required here (VFR in D, E, G; #570): no loss
		}
		k := p.A + "/" + p.B
		seen[k] = true
		if l := m.open[k]; l != nil {
			if p.LateralNM < l.ClosestNM {
				l.ClosestNM, l.VerticalFt = p.LateralNM, p.VerticalFt
			}
			continue
		}
		m.open[k] = &sepLoss{A: p.A, B: p.B, Since: now, ClosestNM: p.LateralNM, VerticalFt: p.VerticalFt}
		m.log.printf("separation: %s and %s %.1f NM, %.0f ft apart (under %.0f NM and 1000 ft)", p.A, p.B, p.LateralNM, p.VerticalFt, p.MinNM)
	}
	for k, l := range m.open {
		if seen[k] {
			continue
		}
		l.Until = now
		m.log.printf("separation: %s and %s separated again after %s, closest %.1f NM, %.0f ft", l.A, l.B, now.Sub(l.Since).Round(time.Second), l.ClosestNM, l.VerticalFt)
		m.losses = append(m.losses, *l)
		if len(m.losses) > 50 {
			m.losses = m.losses[len(m.losses)-50:]
		}
		delete(m.open, k)
	}
}

func registerSeparation(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/separation", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		m, cw := st.separation, st.conflicts
		st.mu.Unlock()
		type view struct {
			MinNM   float64                  `json:"minNM"`
			Closest []traffic.SeparationPair `json:"closest"`
			Open    []sepLoss                `json:"open"`
			Losses  []sepLoss                `json:"losses"`
			// Predicted conflicts and the resolutions given (#395).
			Conflicts   []traffic.Conflict `json:"conflicts"`
			Resolutions []resolutionView   `json:"resolutions"`
		}
		v := view{MinNM: sepMinNM, Closest: []traffic.SeparationPair{}, Open: []sepLoss{}, Losses: []sepLoss{}}
		v.Conflicts, v.Resolutions = []traffic.Conflict{}, []resolutionView{}
		if cw != nil {
			v.Conflicts, v.Resolutions = cw.view()
		}
		if m != nil {
			m.mu.Lock()
			if len(m.now) > 10 {
				v.Closest = append(v.Closest, m.now[:10]...)
			} else {
				v.Closest = append(v.Closest, m.now...)
			}
			for _, l := range m.open {
				v.Open = append(v.Open, *l)
			}
			v.Losses = append(v.Losses, m.losses...)
			m.mu.Unlock()
		}
		writeJSON(w, v)
	})
}
