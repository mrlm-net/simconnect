package traffic

import (
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Types that board by their own airstairs (the user: a CRJ does not use a
// jetway, and has its stairs built in): regional jets with a low door and
// airliner turboprops, business jets. They prefer stands without a jetway
// (StandRequirements.NoJetway), get no stairs vehicle anywhere, and parked
// at a gate get a GPU (no jetway power). A table like the others
// (traffic.noJetway, pkg/dict): the shipped types, a host's local ones
// over them by type ({"type": "E145", "noJetway": false} takes one back).

// NoJetwayItem is a type that does not use a jetway (traffic.noJetway).
type NoJetwayItem struct {
	Type     string `json:"type"`
	NoJetway bool   `json:"noJetway"`
}

var shippedNoJetway = []string{
	"CRJ1", "CRJ2", "CRJ7", "CRJ9", "CRJX", // Bombardier CRJ
	"E135", "E145", "E35L", // Embraer ERJ
	"AT43", "AT45", "AT72", "AT75", "AT76", // ATR
	"DH8A", "DH8B", "DH8C", "DH8D", // Dash 8
	"SF34", "SB20", "D328", "JS41", "B190", "E120", "F50",
	// Business jets and turboprops (the user: "same as Citation")
	"C25A", "C25B", "C25C", "C510", "C525", "C550", "C560", "C56X", "C650", "C680", "C68A", "C700", "C750",
	"CL30", "CL35", "CL60", "GLEX", "GL5T", "GL7T", "GLF4", "GLF5", "GLF6", "G280",
	"E50P", "E55P", "E545", "E550", "LJ25", "LJ35", "LJ45", "LJ60", "LJ75", "H25B", "HDJT", "SF50",
	"F2TH", "FA7X", "FA8X", "F900", "PC12", "PC24", "BE20", "BE9L",
}

var noJetwayNow dict.Value[map[string]bool]

func init() {
	dict.Register(dict.Keyed("traffic.noJetway", "type", "", "",
		func() []NoJetwayItem {
			out := make([]NoJetwayItem, len(shippedNoJetway))
			for i, t := range shippedNoJetway {
				out[i] = NoJetwayItem{Type: t, NoJetway: true}
			}
			return out
		}, func(i NoJetwayItem) string { return strings.ToUpper(i.Type) },
		func(items []NoJetwayItem) {
			m := map[string]bool{}
			for _, i := range items {
				if i.NoJetway {
					m[strings.ToUpper(i.Type)] = true
				}
			}
			noJetwayNow.Store(m)
		}))
	_ = dict.Reset("traffic.noJetway") // the shipped copy in use
}

// NoJetway reports that model (a title or a type) boards by its own
// airstairs: no jetway, no stairs vehicle.
func NoJetway(model string) bool {
	t := ProfileFor(model).Type
	if t == "" {
		t = strings.ToUpper(strings.TrimSpace(model))
	}
	return noJetwayNow.Load()[strings.ToUpper(t)]
}

// NoJetwayTypes are the types in the table now, sorted.
func NoJetwayTypes() []string {
	m := noJetwayNow.Load()
	out := make([]string, 0, len(m))
	for t := range m {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}
