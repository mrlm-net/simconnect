package airport

import "strings"

// FitSID is the SID for a departure from runway on a route of named fixes
// (the flight plan's, in order), as ATC would replace a filed SID that
// does not serve the runway in use: the filed one when it serves runway
// and ends on the route; else the SID of runway that ends at the latest
// fix of the route (the most of the route flown on it); on a tie the one
// sharing the longest start with the filed name (LKPR: "VOZ5M" filed for
// 24 is "VOZ5D" for 06, not "VOZ4E").
// It returns the SID and its enroute transition ("" the common route);
// false when no SID of runway ends on the route.
func (p Procedures) FitSID(filed, runway string, route []string) (Procedure, string, bool) {
	filed = strings.ToUpper(strings.TrimSpace(filed))
	at := map[string]int{}
	for i, f := range route {
		at[strings.ToUpper(strings.TrimSpace(f))] = i + 1 // 0: not on the route
	}
	common := func(name string) int {
		name = strings.ToUpper(name)
		n := 0
		for n < len(name) && n < len(filed) && name[n] == filed[n] {
			n++
		}
		return n
	}
	// ends: where s (or one of its enroute transitions) leaves to the route.
	ends := func(s Procedure) (int, string) {
		rt, _ := runwayTransition(s.RunwayTransitions, runway)
		best, via := at[lastFix(append(append([]Leg(nil), rt.Legs...), s.Legs...))], ""
		for _, et := range s.EnrouteTransitions {
			if n := at[lastFix(et.Legs)]; n > best {
				best, via = n, et.Name
			}
		}
		return best, via
	}
	var pick Procedure
	pickAt, pickVia, found := 0, "", false
	for _, s := range p.SIDsFor(runway) {
		n, via := ends(s)
		if n == 0 {
			continue
		}
		if strings.EqualFold(s.Name, filed) {
			return s, via, true
		}
		if !found || n > pickAt || n == pickAt && common(s.Name) > common(pick.Name) {
			pick, pickAt, pickVia, found = s, n, via, true
		}
	}
	return pick, pickVia, found
}
