//go:build windows

package checklist

import (
	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/systems"
)

// StateFromSample is what a recorded sample shows as systems.State: the
// gear, flaps, spoilers armed, lights, parking brake, engines (N1 over
// 15%) and the autopilot. A recording has no signs or doors: their items
// cannot be checked on it.
func StateFromSample(s flight.Sample) systems.State {
	b := func(on bool) float64 {
		if on {
			return 1
		}
		return 0
	}
	st := systems.State{Engines: s.EngineCount, GearDown: s.GearHandle, ParkingBrake: s.ParkingBrake,
		Beacon: s.Lights&flight.LightBeacon != 0, Nav: s.Lights&flight.LightNav != 0, Strobe: s.Lights&flight.LightStrobe != 0,
		Landing: s.Lights&flight.LightLanding != 0, Taxi: s.Lights&flight.LightTaxi != 0, SpoilersArmed: s.SpoilersArmed}
	for i := range min(s.EngineCount, len(st.Running), len(s.N1)) {
		st.Running[i] = s.N1[i] > 15
	}
	st.Values = map[string]float64{
		systems.GearDown: b(s.GearHandle), systems.FlapsIndex: float64(s.FlapsIndex), systems.SpoilersArmed: b(s.SpoilersArmed),
		systems.ParkingBrake: b(s.ParkingBrake), systems.LightBeacon: b(st.Beacon), systems.LightNav: b(st.Nav),
		systems.LightStrobe: b(st.Strobe), systems.LightLanding: b(st.Landing), systems.LightTaxi: b(st.Taxi),
		systems.APMaster: b(s.AP.Master), systems.FD: b(s.AP.FD), systems.ATHR: b(s.AP.Autothrottle),
	}
	return st
}

// CompletedAt is when on the samples from..to (indexes, to excluded) list
// l was first complete (every item a recording can check done); false
// never, or nothing of it can be checked on a recording.
func CompletedAt(l List, ss []flight.Sample, from, to int) (float64, bool) {
	for i := max(from, 0); i < min(to, len(ss)); i++ {
		if l.Complete(StateFromSample(ss[i])) {
			return ss[i].T, true
		}
	}
	return 0, false
}

// ForAssess is when set's take-off and landing checklists were complete on
// track, for flight.AssessOptions.Checklists: "before-takeoff" before the
// lift-off, "landing" before the last touchdown (from the last time below
// 10,000 ft). A list nothing of which a recording can check is left out.
func ForAssess(set Set, track *flight.Track) []flight.ChecklistDone {
	ss := track.Samples
	lift, touch := -1, -1
	for i := 1; i < len(ss); i++ {
		if lift < 0 && ss[i-1].OnGround && !ss[i].OnGround && ss[i].IAS > 40 {
			lift = i
		}
		if lift >= 0 && !ss[i-1].OnGround && ss[i].OnGround {
			touch = i
		}
	}
	var out []flight.ChecklistDone
	check := func(name string, from, to int) {
		l, ok := set.List(name)
		if !ok || !checkable(l) || to <= from {
			return
		}
		t, done := CompletedAt(l, ss, from, to)
		out = append(out, flight.ChecklistDone{Name: name, Done: done, T: t})
	}
	if lift > 0 {
		check("before-takeoff", 0, lift)
	}
	if touch > 0 {
		from := touch - 1
		for from > lift && ss[from].AltFt-ss[from].GroundFt < 10000 {
			from--
		}
		check("landing", from, touch)
	}
	return out
}

// checkable reports whether a recording can check anything of l.
func checkable(l List) bool {
	probe := StateFromSample(flight.Sample{EngineCount: 1})
	for _, it := range l.Items {
		if it.Verify(probe) != CantCheck {
			return true
		}
	}
	return false
}
