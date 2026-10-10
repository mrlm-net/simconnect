package world

import (
	"encoding/json"
	"net/http"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Holding the World's traffic for a replay (#1014): a replay shows the
// traffic recorded with the flight (flight.GhostFleet), so the live
// traffic must not be there with it. Hold(true) stops the schedule and
// removes every aircraft of ours from the simulator (each flight done);
// Hold(false) starts the schedule again as it was, and it fills the scene
// from its timetable as at any start. The aircraft removed are not
// brought back where they were: a replay can last any time, and their
// flights have moved on meanwhile.

// HoldResult is what Hold did.
type HoldResult struct {
	Held    bool `json:"held"`
	Removed int  `json:"removed"` // aircraft taken out of the simulator
}

// Hold holds the World's traffic for a replay (true) or lets it go on
// (false); holding again, or letting go when not held, does nothing.
func (w *World) Hold(on bool) (HoldResult, error) {
	var res HoldResult
	body, err := w.Do(http.MethodPost, "/api/hold", map[string]bool{"on": on})
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(body, &res)
	return res, err
}

func registerHold(mux *http.ServeMux, st *state) {
	mux.HandleFunc("POST /api/hold", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			On bool `json:"on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st.mu.Lock()
		s := st.schedule
		st.mu.Unlock()
		if s == nil {
			http.Error(w, "simulator not connected", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, s.hold(req.On))
	})
}

// hold is World.Hold on the scheduler.
func (s *scheduler) hold(on bool) HoldResult {
	s.mu.Lock()
	if s.held == on {
		s.mu.Unlock()
		return HoldResult{Held: on}
	}
	s.held = on
	if on {
		s.heldEnabled = s.mgr.Enabled()
	}
	was := s.heldEnabled
	s.mu.Unlock()
	if !on {
		s.mgr.SetEnabled(was)
		s.cc.log.printf("traffic held no more: the schedule %s", map[bool]string{true: "goes on", false: "stays off"}[was])
		return HoldResult{}
	}
	s.mgr.SetEnabled(false)
	now := s.cc.clock.Now()
	removed := 0
	for _, f := range s.mgr.Flights() {
		if f.Status != traffic.FlightScheduled && f.Status != traffic.FlightDone && f.Status != traffic.FlightCancelled {
			s.mgr.Remove(f.Callsign, now)
			removed++
		}
	}
	// The aircraft not of the schedule (spawned by hand, the host's).
	for _, it := range s.cc.snapshotItems() {
		it.mu.Lock()
		managed := it.managed != nil
		it.mu.Unlock()
		if !managed {
			_ = s.cc.do(func() error { return s.cc.remove(it) })
			removed++
		}
	}
	s.cc.log.printf("traffic held for a replay: the schedule stopped, %d aircraft removed", removed)
	return HoldResult{Held: true, Removed: removed}
}
