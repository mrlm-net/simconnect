package nav

import "sort"

// MergeAirwayGraphs is one graph of gs (nil ones skipped): the airways
// read around several airports (#799). A fix keeps the first position
// known for it; an airway gathers the segments of all, each once (the
// higher minimum altitude kept). Center and RadiusNM are the first's.
func MergeAirwayGraphs(gs ...*AirwayGraph) *AirwayGraph {
	out := &AirwayGraph{Airways: []Airway{}}
	seenFix := map[FixKey]bool{}
	type segKey struct {
		airway   string
		from, to FixKey
	}
	byName := map[string]*Airway{}
	seg := map[segKey]int{}
	first := true
	for _, g := range gs {
		if g == nil {
			continue
		}
		if first {
			out.Center, out.RadiusNM, first = g.Center, g.RadiusNM, false
		}
		for _, f := range g.Fixes {
			if k := f.Key(); !seenFix[k] {
				seenFix[k] = true
				out.Fixes = append(out.Fixes, f)
			}
		}
		for _, a := range g.Airways {
			m := byName[a.Name]
			if m == nil {
				m = &Airway{Name: a.Name, Type: a.Type}
				byName[a.Name] = m
			}
			for _, s := range a.Segments {
				k := segKey{a.Name, s.From, s.To}
				if i, ok := seg[k]; ok {
					m.Segments[i].MinAltM = max(m.Segments[i].MinAltM, s.MinAltM)
					continue
				}
				if i, ok := seg[segKey{a.Name, s.To, s.From}]; ok {
					m.Segments[i].MinAltM = max(m.Segments[i].MinAltM, s.MinAltM)
					continue
				}
				seg[k] = len(m.Segments)
				m.Segments = append(m.Segments, s)
			}
		}
	}
	for _, a := range byName {
		out.Airways = append(out.Airways, *a)
	}
	sort.Slice(out.Airways, func(i, j int) bool { return out.Airways[i].Name < out.Airways[j].Name })
	out.link()
	return out
}
