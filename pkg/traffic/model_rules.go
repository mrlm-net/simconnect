package traffic

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Model rules (#1024): simulator models our traffic never uses, aircraft
// or ground vehicles that do not work as traffic (a model that does not
// show attached, one that looks wrong). A table like the others, replaceable
// at runtime (traffic.modelRules, pkg/dict): the shipped rules, and a host's
// local file over them by title ({"title": "…", "never": false} takes a
// shipped one back). A model never used is passed over for the next choice:
// ModelsFor's next title, TugTitleFor's next tug, the next stairs or GPU.

// ModelRuleItem is a rule for one simulator title, or for every title
// beginning with a prefix when it ends in "*" ("MyCrew *"), in
// traffic.modelRules; the exact title wins over a prefix, a longer prefix
// over a shorter:
// Kind what it is ("aircraft", or a VehicleKind: "tug", "gpu", …), Never
// that our traffic never uses it, Reason why (shown, not used).
type ModelRuleItem struct {
	Title  string `json:"title"`
	Kind   string `json:"kind,omitempty"`
	Never  bool   `json:"never"`
	Reason string `json:"reason,omitempty"`
}

var shippedModelRules = []ModelRuleItem{
	{Title: "MyCrew *", Never: true, Reason: "our own helper and tool SimObjects (the invisible jetway helpers) are never traffic"},
	{Title: TugSmallTitle, Kind: string(VehicleTug), Never: true, Reason: "the user, live: the small robot does not show attached to the nose gear"},
	{Title: "Car Ground Power Unit", Kind: string(VehicleGPU), Never: true, Reason: "the user, live: MSFS's GPU van does not suit a stand"},
}

var modelRulesNow dict.Value[map[string]ModelRuleItem]

func init() {
	dict.Register(dict.Keyed("traffic.modelRules", "title", "", "",
		func() []ModelRuleItem { return slices.Clone(shippedModelRules) },
		func(i ModelRuleItem) string { return strings.ToLower(i.Title) },
		func(items []ModelRuleItem) {
			m := map[string]ModelRuleItem{}
			for _, i := range items {
				m[strings.ToLower(i.Title)] = i
			}
			modelRulesNow.Store(m)
		}))
	_ = dict.Reset("traffic.modelRules") // the shipped copy in use
}

// ModelNever reports that our traffic never uses the simulator model title.
func ModelNever(title string) bool {
	rules := modelRulesNow.Load()
	t := strings.ToLower(title)
	if r, ok := rules[t]; ok {
		return r.Never
	}
	best, never := -1, false
	for k, r := range rules {
		if p, ok := strings.CutSuffix(k, "*"); ok && len(p) > best && strings.HasPrefix(t, p) {
			best, never = len(p), r.Never
		}
	}
	return never
}

// ModelRules are the rules in use, by title.
func ModelRules() []ModelRuleItem {
	m := modelRulesNow.Load()
	out := make([]ModelRuleItem, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b ModelRuleItem) int { return strings.Compare(a.Title, b.Title) })
	return out
}

// UsableModels are titles without those never used, in order.
func UsableModels(titles []string) []string {
	out := make([]string, 0, len(titles))
	for _, t := range titles {
		if !ModelNever(t) {
			out = append(out, t)
		}
	}
	return out
}
