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

	"github.com/mrlm-net/simconnect/pkg/dict"
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
	// Ground equipment (#667): wheel chocks in place, the aircraft's own
	// ground power unit connected (a model that has them; not in the
	// default).
	Chocks = "chocks"
	GPU    = "gpu"
	// The sim's pushback for the user aircraft (Services Variables):
	// a tug attached, a pushback possible here, waiting to push (#666).
	PushbackAttached  = "pushbackAttached"
	PushbackAvailable = "pushbackAvailable"
	PushbackWait      = "pushbackWait"
	// Cabin (#759): the seat belt sign on, the no smoking sign (0 off, 1
	// auto, 2 on; a two-way switch 0 or 1), external power on (feeding,
	// the switch's state).
	Seatbelts = "seatbelts"
	NoSmoking = "noSmoking"
	ExtPower  = "extPower"
)

// CabinCall is the action calling the cabin crew (a press, no state;
// Controls.Press): the Fenix's CALLS ALL button (#759).
const CabinCall = "cabinCall"

// The sim's own ground services for the user aircraft, requested by name
// with Controls.Request (#666): the standard key events by default (MSFS
// 2024 SDK Key Events), a model's own way where its profile gives one.
const (
	Jetway      = "jetway"      // TOGGLE_JETWAY: a jetway at a parking spot, or sent away
	Stairs      = "stairs"      // TOGGLE_RAMPTRUCK: a boarding ramp, or sent away
	Baggage     = "baggage"     // REQUEST_LUGGAGE: the baggage loader
	Catering    = "catering"    // REQUEST_CATERING: the catering truck
	PowerSupply = "powerSupply" // REQUEST_POWER_SUPPLY: a ground power unit
	FuelTruck   = "fuelTruck"   // REQUEST_FUEL_KEY: a fuel truck (at a parking spot)
	Pushback    = "pushback"    // TOGGLE_PUSHBACK: the pushback tug
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
	// EFB is the aircraft's tablet when it serves one over HTTP (the Fenix
	// EFB on port 8083); nil none (#667).
	EFB *EFB `json:"efb,omitempty"`
	// Doors are the aircraft's exits by name, in the order of EXIT OPEN
	// and TOGGLE_AIRCRAFT_EXIT (Door(0) is exit 1); their number is how many
	// it has. The default: "Door 1"…"Door 4"; the Fenix: its 8 (#700). A
	// door without its own value and action reads EXIT OPEN:n and toggles
	// TOGGLE_AIRCRAFT_EXIT n+1. In JSON, "doors" is these names, or
	// objects {"name": "L2", "exit": 4} naming each door's exit (Exits),
	// with "efb" when the door is moved through the tablet (DoorEFB).
	Doors []string `json:"doors,omitempty"`
	// Exits are the doors' exits (TOGGLE_AIRCRAFT_EXIT k toggles EXIT
	// OPEN:k-1), by door; none, or 0: door n is exit n+1. With exits, the
	// doors' values and actions are those exits' (the Fenix A319: L1 1, L2
	// 4, R1 5, R2 8).
	Exits []int `json:"exits,omitempty"`
	// DoorEFB are the doors' tablet data refs, by door: "" the exit's
	// event; the Fenix's cargo doors ("doors.cargo.forward", #759).
	DoorEFB []string `json:"doorEFB,omitempty"`
	// Base: a profile for an aircraft type (the A320 family on the
	// standard SimVars), used when no model's profile matches; Extends: a
	// model's profile goes on top of that base (#759).
	Base    bool   `json:"base,omitempty"`
	Extends string `json:"extends,omitempty"`
}

// UnmarshalJSON reads a profile whose "doors" are names or {name, exit}
// objects.
func (p *Profile) UnmarshalJSON(b []byte) error {
	type plain Profile
	var raw struct {
		plain
		Doors json.RawMessage `json:"doors"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*p = Profile(raw.plain)
	if len(raw.Doors) == 0 || string(raw.Doors) == "null" {
		return nil
	}
	var names []string
	if json.Unmarshal(raw.Doors, &names) == nil {
		p.Doors = names
		return nil
	}
	var doors []struct {
		Name string `json:"name"`
		Exit int    `json:"exit"`
		EFB  string `json:"efb"`
	}
	if err := json.Unmarshal(raw.Doors, &doors); err != nil {
		return fmt.Errorf("doors: names, or {name, exit} objects: %w", err)
	}
	p.Doors, p.Exits, p.DoorEFB = nil, nil, nil
	for _, d := range doors {
		if d.Exit < 0 {
			return fmt.Errorf("door %q: exit %d", d.Name, d.Exit)
		}
		p.Doors, p.Exits, p.DoorEFB = append(p.Doors, d.Name), append(p.Exits, d.Exit), append(p.DoorEFB, d.EFB)
	}
	return nil
}

// exit is door n's exit (1-based, TOGGLE_AIRCRAFT_EXIT).
func (p Profile) exit(n int) int {
	if n < len(p.Exits) && p.Exits[n] > 0 {
		return p.Exits[n]
	}
	return n + 1
}

// EFB is where an aircraft's tablet is served: http://<sim host>:Port+Path.
type EFB struct {
	Port int    `json:"port"`
	Path string `json:"path,omitempty"` // "" is "/"
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
	if over.EFB != nil {
		out.EFB = over.EFB
	}
	if len(over.Doors) > 0 {
		out.Doors, out.Exits, out.DoorEFB = over.Doors, over.Exits, over.DoorEFB
	}
	if len(over.Match.PackagePrefix)+len(over.Match.TitleContains)+len(over.Match.ATCType) > 0 {
		out.Match = over.Match
	}
	return out
}

//go:embed profiles/*.json
var shipped embed.FS

// Profiles are the per-model and type profiles in use: the shipped ones,
// with a host's merged over them by name (pkg/dict "systems.profiles",
// #768).
func Profiles() []Profile {
	return slices.Clone(profilesNow.Load())
}

var profilesNow dict.Value[[]Profile]

func init() {
	dict.Register(dict.Keyed("systems.profiles", "name", "", "", shippedProfiles,
		func(p Profile) string { return p.Name }, func(items []Profile) { profilesNow.Store(items) }))
	_ = dict.Reset("systems.profiles")
}

// shippedProfiles are the embedded profiles.
func shippedProfiles() []Profile {
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
	// The model's profile on its type's base (#759); with no model's, the
	// type's base alone.
	var model, base *Profile
	all := Profiles()
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
	if base != nil {
		p = Merge(p, *base)
	}
	if model != nil {
		p = Merge(p, *model)
	}
	for _, o := range overrides {
		if o.Matches(a) || len(o.Match.PackagePrefix)+len(o.Match.TitleContains)+len(o.Match.ATCType) == 0 && o.Name == p.Name {
			p = Merge(p, o)
		}
	}
	return withDoors(p)
}

// withDoors gives every door of p.Doors its value and action (the
// standard EXIT OPEN:n and TOGGLE_AIRCRAFT_EXIT n+1) where p has none, and
// drops the default's doors beyond its number.
func withDoors(p Profile) Profile {
	if len(p.Doors) == 0 {
		return withCounters(p)
	}
	values, actions := map[string]Value{}, map[string]Action{}
	for k, v := range p.Values {
		values[k] = v
	}
	for k, a := range p.Actions {
		actions[k] = a
	}
	mapped := len(p.Exits) > 0
	for n := 0; n < max(10, len(p.Doors)); n++ {
		if n >= len(p.Doors) {
			delete(values, Door(n))
			delete(actions, Door(n))
			continue
		}
		k := p.exit(n)
		if _, ok := values[Door(n)]; !ok || mapped {
			values[Door(n)] = Value{Vars: []string{fmt.Sprintf("EXIT OPEN:%d", k-1)}, Unit: "percent"}
		}
		if _, ok := actions[Door(n)]; !ok || mapped {
			exit := uint32(k)
			actions[Door(n)] = Action{Event: "TOGGLE_AIRCRAFT_EXIT", Toggle: true, Data: &exit}
			if n < len(p.DoorEFB) && p.DoorEFB[n] != "" {
				actions[Door(n)] = Action{EFB: p.DoorEFB[n]}
			}
		}
	}
	p.Values, p.Actions = values, actions
	return withCounters(p)
}

// withCounters reads each counted button's counter (Action.Counter) as
// the value "<action>Counter": Controls presses it from where it stands.
func withCounters(p Profile) Profile {
	for name, a := range p.Actions {
		if a.Counter != "" {
			if p.Values == nil {
				p.Values = map[string]Value{}
			}
			p.Values[name+"Counter"] = Value{Vars: []string{a.Counter}}
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

// Action is one way of operating a control, one of:
//   - Press: a button variable clicked (set to 1, then back to 0);
//   - Set: a variable set to the state wanted (1 on or open, 0 off or
//     closed), e.g. the Fenix's L:B_CONFIG_CHOCKS;
//   - Event: a key event; with Toggle it is sent only when the state
//     differs from the one wanted (TOGGLE_AIRCRAFT_EXIT), else with the
//     state as its data (PARKING_BRAKE_SET 1 or 0); Data is sent instead
//     when given (the exit's index);
//   - EFB: a boolean data ref written through the aircraft's tablet API
//     (Profile.EFB, GraphQL writeBool): the Fenix's chocks and GPU, which
//     take no L:var write (measured).
type Action struct {
	Press  string  `json:"press,omitempty"` // e.g. "L:S_PED_RMP1_XFER"
	Set    string  `json:"set,omitempty"`
	Event  string  `json:"event,omitempty"`
	Toggle bool    `json:"toggle,omitempty"`
	Data   *uint32 `json:"data,omitempty"`
	EFB    string  `json:"efb,omitempty"` // e.g. "fenix.efb.chocks"
	// Counter: a push button counted up (the Fenix's EXT PWR, CALLS ALL):
	// pressed from an even count to +1, released to +2 (as FSUIPC's
	// presets), #759.
	Counter string `json:"counter,omitempty"`
	Note    string `json:"note,omitempty"`
}

// The actions a profile may give: the radios' swap (pkg/avionics), and
// the ground controls Controls operates by the value they change: Door(n),
// Chocks, GPU, ParkingBrake (#667).
const (
	COM1Swap = "com1Swap"
	COM2Swap = "com2Swap"
)
