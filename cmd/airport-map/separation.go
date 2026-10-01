//go:build windows
// +build windows

package main

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

// sepMinNM is the lateral minimum watched (TerminalSeparationNM on final is
// legal; the map keeps 5 NM, EnrouteSeparationNM).
const sepMinNM = traffic.EnrouteSeparationNM

type sepMonitor struct {
	mu     sync.Mutex
	open   map[string]*sepLoss // by pair, while it lasts
	losses []sepLoss           // ended, the latest last (at most 50)
	now    []traffic.SeparationPair
}

type sepLoss struct {
	A          string    `json:"a"`
	B          string    `json:"b"`
	Since      time.Time `json:"since"`
	Until      time.Time `json:"until,omitempty"`
	ClosestNM  float64   `json:"closestNM"`
	VerticalFt float64   `json:"verticalFt"`
}

func newSepMonitor() *sepMonitor { return &sepMonitor{open: map[string]*sepLoss{}} }

func (m *sepMonitor) tick(now time.Time, aircraft []traffic.TrackedAircraft) {
	pairs := traffic.AirborneSeparation(aircraft, sepMinNM, traffic.VerticalSeparationFt)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = pairs
	seen := map[string]bool{}
	for _, p := range pairs {
		if !p.Loss {
			continue
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
		tlog.printf("separation: %s and %s %.1f NM, %.0f ft apart (under %.0f NM and 1000 ft)", p.A, p.B, p.LateralNM, p.VerticalFt, sepMinNM)
	}
	for k, l := range m.open {
		if seen[k] {
			continue
		}
		l.Until = now
		tlog.printf("separation: %s and %s separated again after %s, closest %.1f NM, %.0f ft", l.A, l.B, now.Sub(l.Since).Round(time.Second), l.ClosestNM, l.VerticalFt)
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
