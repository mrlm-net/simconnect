package traffic

import (
	"hash/fnv"
	"slices"
	"sort"
	"strings"
)

// ModelsFor ranks the simulator's aircraft titles for a flight of airline
// (ICAO code, name) in type (ICAO designator), best first:
//
//  1. the type in the airline's livery — its ICAO code in the title before
//     only its name ("TVS-Smartwings" before "TVP-Smartwings Poland", a
//     sister airline),
//  2. another type of the same size (ICAO code letter) in the airline's
//     livery, the same maker first,
//  3. the type in any livery, airline liveries before house and white ones.
//
// Titles are matched as MSFS lists them ("FSLTL_FAIB_B738_TVS-Smartwings",
// "FSLTL A320 DLH Lufthansa", "A321 :: 01 Lufthansa"); FSLTL stubs, VIP
// and business-jet versions are left out. A cargo airline (Airline.Cargo)
// flies freighters only ("FSLTL_B738F_FDX"), any other airline none. The
// type of a title is what ProfileFor makes of it. At most max titles (0:
// all).
func ModelsFor(models []string, airline, name, typ string, max int) []string {
	return pickTitles(preferRanked(rankModels(models, airline, name, typ), airline), "", max)
}

// ModelsForFlight is ModelsFor with the best titles taken in turn by
// callsign: of equally good titles (several liveries or versions of the
// airline's type) each flight gets one of its own — the same one every time
// it is asked for — so a fleet shows its variety instead of one aircraft.
func ModelsForFlight(models []string, airline, name, typ, callsign string, max int) []string {
	return pickTitles(preferRanked(rankModels(models, airline, name, typ), airline), callsign, max)
}

// preferRanked keeps the titles used (PreferProviders), all of them when
// none is, flyable add-ons' only when there is nothing else. A reserved
// airline's (ReservedAirlines) flies its own traffic models only, its
// provider switched on or not: none installed, none.
func preferRanked(in []rankedModel, airline string) []rankedModel {
	var out []rankedModel
	if reservedAirline(airline) {
		for _, m := range in {
			if m.rank < rankOtherLivery && !titleFlyable(m.title) {
				out = append(out, m)
			}
		}
		return out
	}
	for _, m := range in {
		if TitleOn(m.title) {
			out = append(out, m)
		}
	}
	if len(out) == 0 && len(in) > 0 {
		titles := make([]string, 0, len(in))
		for _, m := range in {
			titles = append(titles, m.title)
		}
		reportFallback(titles)
		keep := notFlyable(titles)
		for _, m := range in {
			if slices.Contains(keep, m.title) {
				out = append(out, m)
			}
		}
	}
	return out
}

type rankedModel struct {
	title string
	rank  int
}

// pickTitles returns the titles best first, the best group rotated by key.
func pickTitles(out []rankedModel, key string, max int) []string {
	if key != "" && len(out) > 1 {
		n := 1
		for n < len(out) && out[n].rank == out[0].rank {
			n++
		}
		h := fnv.New32a()
		h.Write([]byte(key))
		k := int(h.Sum32() % uint32(n))
		best := append(append([]rankedModel{}, out[k:n]...), out[:k]...)
		out = append(best, out[n:]...)
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	titles := make([]string, len(out))
	for i, c := range out {
		titles[i] = c.title
	}
	return titles
}

// rankModels ranks the titles for ModelsFor, best first.
func rankModels(models []string, airline, name, typ string) []rankedModel {
	want := ProfileFor(typ)
	size := want.ICAOCode
	cargo := cargoAirline(airline)
	type cand = rankedModel
	var out []cand
	for _, t := range models {
		if skipModel(t) || freighter(t) != cargo {
			continue
		}
		p := ProfileFor(t)
		if p.Type == "" {
			continue
		}
		code := airline != "" && hasToken(t, strings.ToUpper(airline))
		ours := code || liveryOf(t, airline, name)
		// In the airline's livery by name only it may be a sister
		// airline's: after those with its code.
		byName := 0
		if !code {
			byName = 1
		}
		switch {
		case p.Type == typ && ours:
			out = append(out, cand{t, 0 + byName})
		case ours && p.ICAOCode == size && p.Category == want.Category && p.Type[:1] == typ[:1]:
			out = append(out, cand{t, 2 + byName}) // same maker first
		case ours && p.ICAOCode == size && p.Category == want.Category:
			out = append(out, cand{t, 4 + byName})
		case p.Type == typ && genericLivery(t):
			out = append(out, cand{t, 8})
		case p.Type == typ && hasToken(t, typ):
			out = append(out, cand{t, rankOtherLivery}) // named by its designator
		case p.Type == typ:
			out = append(out, cand{t, 7})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].rank != out[j].rank {
			return out[i].rank < out[j].rank
		}
		return out[i].title < out[j].title
	})
	return out
}

// rankOtherLivery: ranks from here are not in the airline's livery.
const rankOtherLivery = 6

// modelTokens splits a title into its words: "FSLTL_FAIB_B738_TVS-Smartwings"
// → FSLTL FAIB B738 TVS SMARTWINGS.
func modelTokens(t string) []string {
	return strings.FieldsFunc(strings.ToUpper(t), func(r rune) bool {
		return r == ' ' || r == '_' || r == '-' || r == ':' || r == '(' || r == ')' || r == '/'
	})
}

// liveryOf reports whether title t is in the airline's livery: its ICAO
// code is one of the words, or its name is in the title.
func liveryOf(t, airline, name string) bool {
	if airline != "" {
		for _, w := range modelTokens(t) {
			if w == strings.ToUpper(airline) {
				return true
			}
		}
	}
	if len(name) < 5 {
		return false // short names (LOT, KLM) are in other words: the code only
	}
	squash := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r == ' ' || r == '_' || r == '-' {
				return -1
			}
			return r
		}, strings.ToLower(s))
	}
	return strings.Contains(squash(t), squash(name))
}

func skipModel(t string) bool {
	if ModelNever(t) {
		return true // the model rules (#1024)
	}
	for _, w := range modelTokens(t) {
		switch w {
		case "STUB", "VIP", "BBJ", "BELUGAXL", "MILITARY":
			return true
		}
	}
	return strings.Contains(strings.ToUpper(t), "AIR FORCE") || strings.Contains(strings.ToUpper(t), "AIRFORCE")
}

// freighter reports whether title t is a freighter: a designator with an
// F (B738F, A332F, B77LF) or CARGO or FREIGHTER among its words.
func freighter(t string) bool {
	for _, w := range modelTokens(t) {
		if len(w) == 5 && w[4] == 'F' && ProfileFor(w[:4]).Type == w[:4] {
			return true
		}
		if w == "CARGO" || w == "FREIGHTER" {
			return true
		}
	}
	return false
}

// cargoAirline reports whether code is a cargo airline's (Airline.Cargo
// in the airline table).
func cargoAirline(code string) bool {
	return code != "" && slices.ContainsFunc(airlinesNow.Load(), func(a Airline) bool { return a.Cargo && strings.EqualFold(a.ICAO, code) })
}

func genericLivery(t string) bool {
	u := strings.ToUpper(t)
	for _, w := range []string{"WHITE", "HOUSE", "ADAPTIVE", "ROOT", "WORLD TRAVEL", "ORBIT", "PACIFICA", "GLOBAL FREIGHTWAYS"} {
		if strings.Contains(u, w) {
			return true
		}
	}
	_, livery, ok := strings.Cut(t, " :: ")
	return ok && livery == ""
}

func hasToken(t, w string) bool {
	for _, x := range modelTokens(t) {
		if x == w {
			return true
		}
	}
	return false
}
