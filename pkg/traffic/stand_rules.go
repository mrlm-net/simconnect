package traffic

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Stand rules: where our aircraft stop on a stand, corrected per stand the
// GSX way (an airport profile's stop position). The simulator's stand data
// gives a parking point, a heading and a circle; the stop is worked out
// from the circle's size (StandPoint), right on most stands. On a big
// stand a smaller aircraft stopped as far forward as a widebody would
// (live, LKPR B14 and C22, 35 m circles: an A320 9 m past its stop
// mark; on the 22 m stands next to them GSX and ours agree). A table like
// the others, replaceable at runtime (traffic.standRules, pkg/dict): the
// shipped rules, and a host's local file over them by stand.

// StandRuleItem moves the stop on one stand, "ICAO LABEL" ("LKPR B14"):
// ForwardM along the stand's line (negative back from the stop worked
// out), RightM across it, for aircraft of a wingspan up to MaxSpanM (0:
// any). Reason says why (shown, not used).
type StandRuleItem struct {
	Stand    string  `json:"stand"`
	ForwardM float64 `json:"forwardM,omitempty"`
	RightM   float64 `json:"rightM,omitempty"`
	MaxSpanM float64 `json:"maxSpanM,omitempty"`
	Reason   string  `json:"reason,omitempty"`
}

// narrowbodySpanM: up to an A321's or a 737-900's span, with a margin.
const narrowbodySpanM = 36.5

var shippedStandRules = []StandRuleItem{
	{Stand: "LKPR B14", ForwardM: -9, MaxSpanM: narrowbodySpanM, Reason: "35 m circle: narrowbodies stop 9 m back from the widebody stop (the user, live: CSA278 on C22, on the stop marks)"},
	{Stand: "LKPR C22", ForwardM: -9, MaxSpanM: narrowbodySpanM, Reason: "35 m circle: narrowbodies stop 9 m back from the widebody stop (the user, live: CSA278 on C22, on the stop marks)"},
}

var standRulesNow dict.Value[map[string]StandRuleItem]

func standKey(s string) string { return strings.ToUpper(strings.Join(strings.Fields(s), " ")) }

func init() {
	dict.Register(dict.Keyed("traffic.standRules", "stand", "", "",
		func() []StandRuleItem { return slices.Clone(shippedStandRules) },
		func(i StandRuleItem) string { return standKey(i.Stand) },
		func(items []StandRuleItem) {
			m := map[string]StandRuleItem{}
			for _, i := range items {
				m[standKey(i.Stand)] = i
			}
			standRulesNow.Store(m)
		}))
	_ = dict.Reset("traffic.standRules") // the shipped copy in use
}

// StandRuleFor is the rule for stand label at icao for an aircraft of
// wingspan spanM (0 unknown: any rule applies), if there is one.
func StandRuleFor(icao, label string, spanM float64) (StandRuleItem, bool) {
	r, ok := standRulesNow.Load()[standKey(icao+" "+label)]
	if !ok || r.MaxSpanM > 0 && spanM > r.MaxSpanM {
		return StandRuleItem{}, false
	}
	return r, true
}

// StandRules are the rules in use, by stand.
func StandRules() []StandRuleItem {
	m := standRulesNow.Load()
	out := make([]StandRuleItem, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b StandRuleItem) int { return strings.Compare(a.Stand, b.Stand) })
	return out
}
