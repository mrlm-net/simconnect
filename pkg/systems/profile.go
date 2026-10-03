//go:build windows
// +build windows

// Package systems reads the user aircraft's systems — power, radios,
// engines, brakes, lights, doors, transponder, flaps and gear — through a
// profile: a default set of standard SimVars, and per-model overrides as
// data (JSON) for aircraft that drive their systems with their own
// variables (the Fenix A320 family with L:vars). Local override files go on
// top: an override wins per value.
package systems

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path"
	"slices"
	"strings"
)

// The values a profile resolves. A profile gives some or all of them; the
// default profile gives every one.
const (
	Battery      = "battery"      // battery master on
	Volts        = "volts"        // bus / battery volts
	Powered      = "powered"      // buses have power
	Avionics     = "avionics"     // avionics powered
	ExtAvailable = "extAvailable" // external power available
	ExtOn        = "extOn"        // external power feeding
	COM1Power    = "com1"         // COM 1 working
	COM2Power    = "com2"         // COM 2 working
	EngineCount  = "engines"      // number of engines
	ParkingBrake = "parkingBrake"
	LightBeacon  = "lightBeacon"
	LightNav     = "lightNav"
	LightStrobe  = "lightStrobe"
	LightLanding = "lightLanding"
	LightTaxi    = "lightTaxi"
	XPDRState    = "xpdrState" // transponder state (0 off, 1 standby, 2 test, 3 on, 4 alt)
	XPDRCode     = "xpdrCode"  // the code, BCD16 (Bco16)
	FlapsPct     = "flapsPct"  // flaps handle, percent
	GearDown     = "gearDown"  // gear handle down
	// COM frequencies, MHz: active and standby of COM 1 and 2.
	COM1Active  = "com1Active"
	COM1Standby = "com1Standby"
	COM2Active  = "com2Active"
	COM2Standby = "com2Standby"
)

// Engine values: "engineRunning1"…"engineRunning4", "starter1"…"starter4";
// doors "door0"…"door3" (EXIT OPEN, open > 0).
func EngineRunning(n int) string { return fmt.Sprintf("engineRunning%d", n) }
func Starter(n int) string       { return fmt.Sprintf("starter%d", n) }
func Door(n int) string          { return fmt.Sprintf("door%d", n) }

// Value is how one value is read: one variable, or several combined.
type Value struct {
	// Vars are the SimVars or L:vars ("L:S_OH_ELEC_BAT1"), in Unit
	// ("number" when empty). With several, Combine joins them.
	Vars []string `json:"vars"`
	Unit string   `json:"unit,omitempty"`
	// Combine: "any" (true when any is true), "max", "min"; "" the first.
	Combine string `json:"combine,omitempty"`
	// TrueAt: a variable counts as true (1) only at these positions (a
	// three-position strobe switch true only at 2); else true when not 0.
	// AtLeast: true at this value or more (bus volts as powered).
	TrueAt  []float64 `json:"trueAt,omitempty"`
	AtLeast *float64  `json:"atLeast,omitempty"`
	// Scale multiplies the result (a frequency in kHz as MHz: 0.001); 0 is 1.
	Scale float64 `json:"scale,omitempty"`
	// Note says how it was found ("measured", "assumed: ...").
	Note string `json:"note,omitempty"`
}

// Match picks the aircraft a profile is for: any of its rules matching
// (all empty: none — only the default matches everything).
type Match struct {
	PackagePrefix []string `json:"packagePrefix,omitempty"` // addons.Package folder, e.g. "fnx-aircraft"
	TitleContains []string `json:"titleContains,omitempty"` // the aircraft title (TITLE), any case
	ATCType       []string `json:"atcType,omitempty"`       // ATC TYPE, e.g. "A320"
}

// Profile is a set of values for some aircraft.
type Profile struct {
	Name     string           `json:"name"`
	Match    Match            `json:"match"`
	Measured string           `json:"measured,omitempty"` // how and where it was measured
	Values   map[string]Value `json:"values"`
	// Actions are how a model is operated where the standard key events
	// do not do it (pkg/avionics), by name: "com1Swap", "com2Swap".
	Actions map[string]Action `json:"actions,omitempty"`
}

// Aircraft is what a profile is matched against.
type Aircraft struct {
	Package string // the package folder (addons.AircraftPackage), "" unknown
	Title   string
	ATCType string
}

// Matches reports whether p is for a.
func (p Profile) Matches(a Aircraft) bool {
	m := p.Match
	for _, pre := range m.PackagePrefix {
		if a.Package != "" && strings.HasPrefix(strings.ToLower(a.Package), strings.ToLower(pre)) {
			return true
		}
	}
	for _, s := range m.TitleContains {
		if a.Title != "" && strings.Contains(strings.ToLower(a.Title), strings.ToLower(s)) {
			return true
		}
	}
	for _, t := range m.ATCType {
		if strings.EqualFold(a.ATCType, t) {
			return true
		}
	}
	return false
}

// Merge is base with over's values on top: over wins per value; its name,
// match and note when it gives them.
func Merge(base, over Profile) Profile {
	out := base
	out.Values = map[string]Value{}
	for k, v := range base.Values {
		out.Values[k] = v
	}
	for k, v := range over.Values {
		out.Values[k] = v
	}
	out.Actions = map[string]Action{}
	for k, a := range base.Actions {
		out.Actions[k] = a
	}
	for k, a := range over.Actions {
		out.Actions[k] = a
	}
	if over.Name != "" {
		out.Name = over.Name
	}
	if over.Measured != "" {
		out.Measured = over.Measured
	}
	if len(over.Match.PackagePrefix)+len(over.Match.TitleContains)+len(over.Match.ATCType) > 0 {
		out.Match = over.Match
	}
	return out
}

//go:embed profiles/*.json
var shipped embed.FS

// Profiles are the shipped per-model profiles.
func Profiles() []Profile {
	var out []Profile
	ents, _ := shipped.ReadDir("profiles")
	for _, e := range ents {
		f, err := shipped.Open(path.Join("profiles", e.Name()))
		if err != nil {
			continue
		}
		if p, err := ReadProfile(f); err == nil {
			out = append(out, p)
		}
		f.Close()
	}
	return out
}

// ReadProfile reads a profile from JSON (a shipped one, or a local
// override file).
func ReadProfile(r io.Reader) (Profile, error) {
	var p Profile
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("systems: reading profile: %w", err)
	}
	for k, v := range p.Values {
		if len(v.Vars) == 0 {
			return Profile{}, fmt.Errorf("systems: profile %q value %s: no vars", p.Name, k)
		}
		switch v.Combine {
		case "", "any", "max", "min":
		default:
			return Profile{}, fmt.Errorf("systems: profile %q value %s: combine %q (any, max, min)", p.Name, k, v.Combine)
		}
	}
	return p, nil
}

// For is the profile to read a with: the default, the first matching
// shipped or given profile on top, then each matching override (local
// files, in order) on top of that.
func For(a Aircraft, overrides ...Profile) Profile {
	p := Default()
	for _, s := range Profiles() {
		if s.Matches(a) {
			p = Merge(p, s)
			break
		}
	}
	for _, o := range overrides {
		if o.Matches(a) || len(o.Match.PackagePrefix)+len(o.Match.TitleContains)+len(o.Match.ATCType) == 0 && o.Name == p.Name {
			p = Merge(p, o)
		}
	}
	return p
}

// vars are the distinct variables p reads, with their units, in a stable
// order.
func (p Profile) vars() []varUnit {
	seen := map[varUnit]bool{}
	var out []varUnit
	keys := make([]string, 0, len(p.Values))
	for k := range p.Values {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v := p.Values[k]
		for _, name := range v.Vars {
			vu := varUnit{name, v.unit()}
			if !seen[vu] {
				seen[vu] = true
				out = append(out, vu)
			}
		}
	}
	return out
}

type varUnit struct{ name, unit string }

func (v Value) unit() string {
	if v.Unit == "" {
		return "number"
	}
	return v.Unit
}

// resolve is v from the variables read (by var and unit).
func (v Value) resolve(read map[varUnit]float64) float64 {
	vals := make([]float64, 0, len(v.Vars))
	for _, name := range v.Vars {
		x := read[varUnit{name, v.unit()}]
		if len(v.TrueAt) > 0 {
			if slices.Contains(v.TrueAt, x) {
				x = 1
			} else {
				x = 0
			}
		}
		vals = append(vals, x)
	}
	var out float64
	switch v.Combine {
	case "any":
		for _, x := range vals {
			if x != 0 {
				out = 1
			}
		}
	case "max":
		out = math.Inf(-1)
		for _, x := range vals {
			out = math.Max(out, x)
		}
	case "min":
		out = math.Inf(1)
		for _, x := range vals {
			out = math.Min(out, x)
		}
	default:
		out = vals[0]
	}
	if v.Scale != 0 {
		out *= v.Scale
	}
	if v.AtLeast != nil {
		if out >= *v.AtLeast {
			return 1
		}
		return 0
	}
	return out
}

// Action is one way of operating a control: pressing a button variable
// (set to 1, then back to 0, as a click does).
type Action struct {
	Press string `json:"press"` // e.g. "L:S_PED_RMP1_XFER"
	Note  string `json:"note,omitempty"`
}

// The actions a profile may give.
const (
	COM1Swap = "com1Swap"
	COM2Swap = "com2Swap"
)
