package traffic

// StrutsLearnt is what an Injector has learnt of the models' struts, by
// model (static CG height in hundredths of a foot): how far they extend
// (Struts) and how far above its static height the simulator holds one at
// its static pitch (Reach). Kept by a host across runs, the first take-off
// of a model after a restart is not its lesson (live LKPR 2026-11-08:
// THY1039 B738 at 8.50 ft and OKLOZ, unseeded, popped 0.58 and 0.53 ft
// as the nose came up).
type StrutsLearnt struct {
	Struts map[int]float64 `json:"struts,omitempty"`
	Reach  map[int]float64 `json:"reach,omitempty"`
}

// StrutsLearnt returns a copy of what i has learnt and a counter that
// changes whenever it learns something new.
func (i *Injector) StrutsLearnt() (StrutsLearnt, uint64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	s := StrutsLearnt{Struts: make(map[int]float64, len(i.struts)), Reach: make(map[int]float64, len(i.reach))}
	for k, v := range i.struts {
		s.Struts[k] = v
	}
	for k, v := range i.reach {
		s.Reach[k] = v
	}
	return s, i.strutsGen
}

// SetStrutsLearnt adds what was learnt before (a run before): the struts
// where not learnt in this run, the reach where shorter.
func (i *Injector) SetStrutsLearnt(s StrutsLearnt) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for k, v := range s.Struts {
		if _, ok := i.struts[k]; !ok && k > 0 && v >= strutsMinFt && v <= strutsMaxFt {
			i.struts[k] = v
		}
	}
	for k, v := range s.Reach {
		if old, ok := i.reach[k]; k > 0 && v >= 0 && (!ok || v < old) {
			i.reach[k] = v
		}
	}
}
