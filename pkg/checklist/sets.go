//go:build windows

package checklist

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"path"

	"github.com/mrlm-net/simconnect/pkg/systems"
)

//go:embed sets/*.json
var shipped embed.FS

// Shipped are the built-in sets: each family's default, and the models'.
func Shipped() []Set {
	var out []Set
	ents, _ := shipped.ReadDir("sets")
	for _, e := range ents {
		f, err := shipped.Open(path.Join("sets", e.Name()))
		if err != nil {
			continue
		}
		if s, err := ReadSet(f); err == nil {
			out = append(out, s)
		}
		f.Close()
	}
	return out
}

// ReadSet reads a set from JSON (a shipped one, or a local override file)
// and validates it.
func ReadSet(r io.Reader) (Set, error) {
	var s Set
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return Set{}, fmt.Errorf("checklist: reading set: %w", err)
	}
	return s, s.Validate()
}

// For is the checklists for aircraft a: its family's default, the model's
// set on top, then each local set (overrides, in order) that matches a or
// names no aircraft on top of that. false: no family's lists match a.
func For(a systems.Aircraft, local ...Set) (Set, bool) {
	all := Shipped()
	var base, model *Set
	for i := range all {
		s := &all[i]
		if !s.Matches(a) {
			continue
		}
		if s.Base && base == nil {
			base = s
		} else if !s.Base && model == nil {
			model = s
		}
	}
	if model != nil && model.Extends != "" {
		for i := range all {
			if all[i].Name == model.Extends {
				base = &all[i]
			}
		}
	}
	if base == nil {
		return Set{}, false
	}
	out := *base
	if model != nil {
		out = Merge(out, *model)
	}
	for _, o := range local {
		if o.Matches(a) || len(o.Match.PackagePrefix)+len(o.Match.TitleContains)+len(o.Match.ATCType) == 0 {
			out = Merge(out, o)
		}
	}
	return out, true
}
