package traffic

import (
	"sort"
	"strings"
)

// gearGeom is where a model's gear really is: its nose gear NoseM ahead of
// the simulator's reference point, its main gear MainM behind it (metres).
type gearGeom struct{ NoseM, MainM float64 }

// gearKeys: gearGeometry's keys, longest first (a title token "B738F"
// takes "B738F" before "B738").
var gearKeys = func() []string {
	k := make([]string, 0, len(gearGeometry))
	for t := range gearGeometry {
		k = append(k, t)
	}
	sort.Slice(k, func(i, j int) bool { return len(k[i]) > len(k[j]) || len(k[i]) == len(k[j]) && k[i] < k[j] })
	return k
}()

// gearFor is the gear geometry of model (a title): from the type designator
// a token of the title starts with (FSLTL_B738_KLM, AIB_B738_BAW,
// FSLTL_A332F_MAS → A332), else from the type ProfileFor resolves.
//
// The motion profiles keep their own figures (RefAheadMeters 1 m for every
// type), which stand and push planning were tuned with: with the real 2.5 m
// on the B738, EHAM U26 found no push pose. Only the tug takes these.
func gearFor(model string) (gearGeom, bool) {
	tokens := strings.FieldsFunc(strings.ToUpper(model), func(r rune) bool {
		return (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
	for _, t := range tokens {
		for _, k := range gearKeys {
			if strings.HasPrefix(t, k) {
				return gearGeometry[k], true
			}
		}
	}
	g, ok := gearGeometry[ProfileFor(model).Type]
	return g, ok
}

// fitTug puts a SimObjectTug on the real nose gear of model (gearFor): its
// copy of the aircraft's motion profile, used for the nose gear and its way
// round the aircraft, gets the model's wheelbase and reference point, the
// tail where the profile had it. With the profiles' 1 m the tug sat up to
// 8 m off the nose wheel (B738 1.6 m ahead: user screenshot, KLM B738,
// LKPR 22A, 2026-11-06; B772 6.1 m and MD11 7.9 m behind; C208 8.3 m
// ahead, its type unknown to the profiles).
func fitTug(t PushbackTug, model string) {
	s, ok := t.(*SimObjectTug)
	if !ok {
		return
	}
	g, ok := gearFor(model)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := &s.prof
	tail := p.TailMeters + p.RefAheadMeters // tail end behind the reference point
	p.WheelbaseMeters, p.RefAheadMeters = g.NoseM+g.MainM, g.MainM
	if tail > g.MainM {
		p.TailMeters = tail - g.MainM
	}
}
