package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// ScheduleTuning is what a user may change of the scheduled traffic's
// timing (#1303): the local override, DataDir/schedule-tuning.json, the
// GSX way: a value there wins, one left out stays the default. Times are
// in minutes.
type ScheduleTuning struct {
	// Waves are the movements by local hour against the peak, each 0–1
	// (ScheduleConfig.Waves).
	Waves *[24]float64 `json:"waves,omitempty"`
	// NightWave: an hour with at most this many movements against the
	// peak is night (0.1, by default 23–04 local).
	NightWave *float64 `json:"nightWave,omitempty"`
	// MorningWaveMin: how long the morning wave is, which night stops and
	// overnight arrivals leave in (120).
	MorningWaveMin *float64 `json:"morningWaveMin,omitempty"`
	// NightStops: departures parked per airport through the night (0 half
	// the per-airport limit; negative none).
	NightStops *int `json:"nightStops,omitempty"`
	// MinTurnMin, MaxTurnMin: an arrival turns around this long after its
	// STA (40, 180; a negative MinTurnMin: never).
	MinTurnMin *float64 `json:"minTurnMin,omitempty"`
	MaxTurnMin *float64 `json:"maxTurnMin,omitempty"`
	// HorizonMin: how far ahead the timetable is planned (120).
	HorizonMin *float64 `json:"horizonMin,omitempty"`
	// DepartureLeadMin, ArrivalLeadMin, VFRLeadMin: how long before its
	// time a flight appears (10, 25, 8); a night stop calls delivery
	// DepartureLeadMin before its STD.
	DepartureLeadMin *float64 `json:"departureLeadMin,omitempty"`
	ArrivalLeadMin   *float64 `json:"arrivalLeadMin,omitempty"`
	VFRLeadMin       *float64 `json:"vfrLeadMin,omitempty"`
	// DepartureLateMin, ArrivalLateMin: a flight not started this long
	// after that is cancelled (15, 10).
	DepartureLateMin *float64 `json:"departureLateMin,omitempty"`
	ArrivalLateMin   *float64 `json:"arrivalLateMin,omitempty"`
	// OvernightRemote: aircraft parked overnight take stands without a
	// jetway first (true, #1298).
	OvernightRemote *bool `json:"overnightRemote,omitempty"`
	// SelfAnnounce: our crews announce themselves at a field without a
	// tower (true, #1296); false: its tower and ground named after the
	// common frequency clear them.
	SelfAnnounce *bool `json:"selfAnnounce,omitempty"`
	// CargoShare scales the cargo airlines' weight (1, #1300; 0 none, up
	// to 10).
	CargoShare *float64 `json:"cargoShare,omitempty"`
}

// Default night threshold and cargo share (ScheduleTuning).
const (
	defaultNightWave  = 0.1
	defaultCargoShare = 1.0
)

// baseTiming is the manager's timing before any tuning: Options.Schedule
// over the library's defaults.
func baseTiming(t ScheduleTiming) traffic.ManagerTiming {
	o := traffic.ManagerOptions{Horizon: t.Horizon, DepartureLead: t.DepartureLead, ArrivalLead: t.ArrivalLead, VFRLead: t.VFRLead,
		DepartureLate: t.DepartureLate, ArrivalLate: t.ArrivalLate}
	return o.Timing()
}

// timing is base with t's values over it.
func (t ScheduleTuning) timing(base traffic.ManagerTiming) traffic.ManagerTiming {
	set := func(d *time.Duration, min *float64) {
		if min != nil {
			*d = time.Duration(*min * float64(time.Minute))
		}
	}
	set(&base.MorningWave, t.MorningWaveMin)
	set(&base.MinTurn, t.MinTurnMin)
	set(&base.MaxTurn, t.MaxTurnMin)
	set(&base.Horizon, t.HorizonMin)
	set(&base.DepartureLead, t.DepartureLeadMin)
	set(&base.ArrivalLead, t.ArrivalLeadMin)
	set(&base.VFRLead, t.VFRLeadMin)
	set(&base.DepartureLate, t.DepartureLateMin)
	set(&base.ArrivalLate, t.ArrivalLateMin)
	if t.NightStops != nil {
		base.NightStops = *t.NightStops
	}
	return base
}

// validate refuses values the triggers cannot work with, on base's values
// for those left out; every problem at once.
func (t ScheduleTuning) validate(base traffic.ManagerTiming) error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if t.Waves != nil && slices.ContainsFunc(t.Waves[:], func(v float64) bool { return !(v >= 0 && v <= 1) }) {
		bad("waves: each hour 0 to 1")
	}
	if t.NightWave != nil && !(*t.NightWave >= 0 && *t.NightWave <= 1) {
		bad("nightWave %v: 0 to 1", *t.NightWave)
	}
	if t.CargoShare != nil && !(*t.CargoShare >= 0 && *t.CargoShare <= 10) {
		bad("cargoShare %v: 0 to 10", *t.CargoShare)
	}
	for _, m := range []struct {
		name string
		v    *float64
		min  float64
	}{{"morningWaveMin", t.MorningWaveMin, 1}, {"maxTurnMin", t.MaxTurnMin, 1}, {"horizonMin", t.HorizonMin, 30},
		{"departureLeadMin", t.DepartureLeadMin, 1}, {"arrivalLeadMin", t.ArrivalLeadMin, 1}, {"vfrLeadMin", t.VFRLeadMin, 1},
		{"departureLateMin", t.DepartureLateMin, 1}, {"arrivalLateMin", t.ArrivalLateMin, 1}} {
		if m.v != nil && !(*m.v >= m.min && *m.v <= 24*60) {
			bad("%s %v: %v to 1440", m.name, *m.v, m.min)
		}
	}
	if t.MinTurnMin != nil && !(*t.MinTurnMin <= 24*60) {
		bad("minTurnMin %v: at most 1440", *t.MinTurnMin)
	}
	r := t.timing(base)
	if r.MinTurn > r.MaxTurn {
		bad("minTurn %v is after maxTurn %v", r.MinTurn, r.MaxTurn)
	}
	for _, l := range []struct {
		name string
		d    time.Duration
	}{{"departureLead", r.DepartureLead}, {"arrivalLead", r.ArrivalLead}, {"vfrLead", r.VFRLead}} {
		if l.d > r.Horizon {
			bad("%s %v is beyond the horizon %v: the flight would be planned too late", l.name, l.d, r.Horizon)
		}
	}
	return errors.Join(errs...)
}

// tunedSchedule is the tuning as the scheduler works with it.
type tunedSchedule struct {
	cfg             traffic.ScheduleConfig
	nightWave       float64
	overnightRemote bool
}

// tuned is base with t over it. The airlines dropped by a CargoShare of 0
// stay known to the scheduler by code (scheduler.airlines).
func (t ScheduleTuning) tuned(base traffic.ScheduleConfig) *tunedSchedule {
	out := &tunedSchedule{cfg: base, nightWave: defaultNightWave, overnightRemote: true}
	if t.Waves != nil {
		out.cfg.Waves = *t.Waves
	}
	if t.NightWave != nil {
		out.nightWave = *t.NightWave
	}
	if t.OvernightRemote != nil {
		out.overnightRemote = *t.OvernightRemote
	}
	if t.CargoShare != nil {
		out.cfg.Airlines = nil
		for _, a := range base.Airlines {
			if a.Cargo {
				if *t.CargoShare == 0 {
					continue
				}
				a.Weight *= *t.CargoShare
			}
			out.cfg.Airlines = append(out.cfg.Airlines, a)
		}
	}
	return out
}

// tuningStore keeps the local override.
type tuningStore struct {
	mu    sync.Mutex
	file  string
	local ScheduleTuning
}

// loadTuning reads file (missing: nothing tuned). A file with values the
// triggers cannot work with is left out whole, and said so.
func loadTuning(file string, base traffic.ManagerTiming) *tuningStore {
	s := &tuningStore{file: file}
	b, err := os.ReadFile(file)
	if err != nil {
		return s
	}
	var t ScheduleTuning
	if err := json.Unmarshal(b, &t); err != nil {
		fmt.Fprintf(stdout, "⚠️  %s: %v\n", file, err)
		return s
	}
	if err := t.validate(base); err != nil {
		fmt.Fprintf(stdout, "⚠️  %s left out: %v\n", file, err)
		return s
	}
	s.local = t
	return s
}

// get is the local override; none on a nil store.
func (s *tuningStore) get() ScheduleTuning {
	if s == nil {
		return ScheduleTuning{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.local
}

// set replaces the local override and saves it.
func (s *tuningStore) set(t ScheduleTuning) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(s.file, b, 0o644); err != nil {
		return err
	}
	s.local = t
	return nil
}

// tuningView is GET /api/schedule/tuning: the local override, and what is
// in use with the defaults under it.
type tuningView struct {
	Local     ScheduleTuning `json:"local"`
	Effective ScheduleTuning `json:"effective"`
}

// effectiveTuning is every value in use.
func effectiveTuning(local ScheduleTuning, base traffic.ManagerTiming) ScheduleTuning {
	r := local.timing(base)
	mins := func(d time.Duration) *float64 { v := d.Minutes(); return &v }
	on := func(v *bool) *bool {
		out := v == nil || *v
		return &out
	}
	val := func(v *float64, def float64) *float64 {
		if v != nil {
			return v
		}
		return &def
	}
	waves := traffic.DefaultScheduleConfig().Waves
	if local.Waves != nil {
		waves = *local.Waves
	}
	return ScheduleTuning{Waves: &waves, NightWave: val(local.NightWave, defaultNightWave), MorningWaveMin: mins(r.MorningWave), NightStops: &r.NightStops,
		MinTurnMin: mins(r.MinTurn), MaxTurnMin: mins(r.MaxTurn), HorizonMin: mins(r.Horizon), DepartureLeadMin: mins(r.DepartureLead),
		ArrivalLeadMin: mins(r.ArrivalLead), VFRLeadMin: mins(r.VFRLead), DepartureLateMin: mins(r.DepartureLate), ArrivalLateMin: mins(r.ArrivalLate),
		OvernightRemote: on(local.OvernightRemote), SelfAnnounce: on(local.SelfAnnounce), CargoShare: val(local.CargoShare, defaultCargoShare)}
}

// applyTuning puts t in use: from now on, and for the hours ahead asked
// again. Flights already paired or parked stay as they are.
func (s *scheduler) applyTuning(t ScheduleTuning) {
	s.tuned.Store(t.tuned(s.base))
	s.cc.selfAnnounceOff.Store(t.SelfAnnounce != nil && !*t.SelfAnnounce)
	s.mgr.SetTiming(t.timing(s.timing))
	s.mgr.Replan()
}

// ScheduleTuning is the local override of the traffic's timing
// (DataDir/schedule-tuning.json).
func (w *World) ScheduleTuning() ScheduleTuning { return w.st.tuning.get() }

// SetScheduleTuning replaces the local override, saves it and puts it in
// use; ScheduleTuning{} takes every value back to its default. Values the
// triggers cannot work with are refused, with nothing changed.
func (w *World) SetScheduleTuning(t ScheduleTuning) error {
	_, err := w.Do(http.MethodPost, "/api/schedule/tuning", t)
	return err
}

// registerScheduleTuning serves GET and POST /api/schedule/tuning.
func registerScheduleTuning(mux *http.ServeMux, st *state) {
	base := func() traffic.ManagerTiming { return baseTiming(st.core.hooks.Schedule) }
	mux.HandleFunc("GET /api/schedule/tuning", func(w http.ResponseWriter, r *http.Request) {
		local := st.tuning.get()
		writeJSON(w, tuningView{Local: local, Effective: effectiveTuning(local, base())})
	})
	mux.HandleFunc("POST /api/schedule/tuning", func(w http.ResponseWriter, r *http.Request) {
		var t ScheduleTuning
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := t.validate(base()); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := st.tuning.set(t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		st.mu.Lock()
		s := st.schedule
		st.mu.Unlock()
		if s != nil {
			s.applyTuning(t)
		}
		writeJSON(w, tuningView{Local: t, Effective: effectiveTuning(t, base())})
	})
}
