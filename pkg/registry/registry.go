// Package registry provides a typed metadata store for Microsoft Flight
// Simulator SimVar names, accepted unit strings, data types, and
// writability attributes.
//
// The registry has no build tag requirements and no imports from
// pkg/types, pkg/datasets, or any Windows-gated package. It is safe
// to import on all platforms.
//
// All exported functions are safe for concurrent use. The package-level
// state is initialised once in init() and never mutated thereafter.
package registry

import (
	"errors"
	"strings"
)

// SimVarMeta holds static metadata for a single MSFS SimVar.
//
// Name is the canonical SimConnect variable name, upper-case with spaces,
// without any :N index suffix (e.g., "ENG RPM" not "ENG RPM:1").
//
// Units lists every unit string accepted by SimConnect for this variable,
// in lowercase canonical form.
//
// DefaultUnit is the primary unit recommended for new data definitions.
// It is always an element of Units.
//
// Type is a local string alias describing the Go numeric type appropriate
// for reading this variable. Valid values: "float64", "float32", "int32",
// "int64", "bool", "string", "enum".
//
// Category groups SimVars by domain. Valid values in this release:
// "aircraft", "environment", "simulator", "navigation", "autopilot".
//
// Writable reports whether SimConnect accepts SetDataOnSimObject writes.
//
// Indexed reports whether this variable accepts an :N suffix
// (e.g., "ENG RPM:1", "ENG RPM:2").
//
// Description is an optional human-readable summary.
type SimVarMeta struct {
	Name        string
	Units       []string
	DefaultUnit string
	Type        string
	Category    string
	Writable    bool
	Indexed     bool
	Description string
}

// simvarMap is the pre-normalised lookup index, keyed by strings.ToUpper(sv.Name).
// Built once in init(); never mutated after package initialisation.
var simvarMap map[string]SimVarMeta

// simvarList is a snapshot of all entries in declaration order.
// Built once in init(); never mutated after package initialisation.
var simvarList []SimVarMeta

func init() {
	simvarMap = make(map[string]SimVarMeta, len(simvars))
	simvarList = make([]SimVarMeta, len(simvars))
	copy(simvarList, simvars)
	for _, sv := range simvars {
		key := strings.ToUpper(sv.Name)
		if _, exists := simvarMap[key]; exists {
			panic("registry: duplicate SimVar name: " + sv.Name)
		}
		simvarMap[key] = sv
	}
}

// stripIndexSuffix removes a trailing :N suffix (N = one or more digits)
// from name. Returns name unchanged if no such suffix is present.
// Uses no regex — strings.LastIndexByte plus a digit loop.
func stripIndexSuffix(name string) string {
	i := strings.LastIndexByte(name, ':')
	if i < 0 {
		return name
	}
	suffix := name[i+1:]
	if len(suffix) == 0 {
		return name
	}
	for j := 0; j < len(suffix); j++ {
		if suffix[j] < '0' || suffix[j] > '9' {
			return name
		}
	}
	return name[:i]
}

// Lookup returns the SimVarMeta for the given SimVar name and true.
// Returns the zero SimVarMeta and false if no entry exists.
//
// The lookup is case-insensitive: "plane latitude", "PLANE LATITUDE",
// and "Plane Latitude" all resolve to the same entry.
//
// A :N index suffix (e.g., ":1", ":4") is stripped before lookup.
// The returned SimVarMeta.Indexed field is true for indexed variables.
func Lookup(name string) (SimVarMeta, bool) {
	key := strings.ToUpper(stripIndexSuffix(name))
	sv, ok := simvarMap[key]
	return sv, ok
}

// All returns a copy of all SimVar metadata entries in declaration order.
// The returned slice is independent of package state.
func All() []SimVarMeta {
	return append([]SimVarMeta{}, simvarList...)
}

// Validate returns nil if unit is a valid unit string for the SimVar
// identified by name. Returns an error otherwise.
//
// Both name and unit comparisons are case-insensitive.
// A :N index suffix is stripped from name before lookup.
func Validate(name, unit string) error {
	sv, ok := Lookup(name)
	if !ok {
		return errors.New("registry: unknown SimVar: " + name)
	}
	u := strings.ToLower(unit)
	for _, valid := range sv.Units {
		if valid == u {
			return nil
		}
	}
	return errors.New("registry: unit \"" + unit + "\" not valid for " + sv.Name +
		"; valid units: " + strings.Join(sv.Units, ", "))
}

// ByUnit returns all SimVar entries whose Units slice contains unit.
// The comparison is case-insensitive. Returns nil if no entries match.
func ByUnit(unit string) []SimVarMeta {
	u := strings.ToLower(unit)
	var result []SimVarMeta
	for _, sv := range simvarList {
		for _, v := range sv.Units {
			if v == u {
				result = append(result, sv)
				break
			}
		}
	}
	return result
}

// ByCategory returns all SimVar entries whose Category equals category.
// The comparison is case-insensitive. Returns nil if no entries match.
func ByCategory(category string) []SimVarMeta {
	c := strings.ToLower(category)
	var result []SimVarMeta
	for _, sv := range simvarList {
		if strings.ToLower(sv.Category) == c {
			result = append(result, sv)
		}
	}
	return result
}
