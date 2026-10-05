package traffic

import (
	"maps"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// The package's tables, replaceable at runtime (pkg/dict, #768): the
// shipped copies below stay the defaults; a host's data is merged over
// them by id.

// TelephonyItem is an airline's radio call sign (traffic.telephony).
type TelephonyItem struct {
	ICAO      string `json:"icao"`
	Telephony string `json:"telephony"`
	Name      string `json:"name"`
}

// InitialismItem is a word said letter by letter (traffic.initialisms):
// "CSA" is "C-S-A", not a word.
type InitialismItem struct {
	Word string `json:"word"`
}

// AircraftTypeItem is a known aircraft type (traffic.aircraftTypes): what
// it matches in a title, its airframe, approach and ground figures, its
// take-off profile and flap schedule.
type AircraftTypeItem struct {
	Type       string           `json:"type"`
	Match      []string         `json:"match"`
	Category   AircraftCategory `json:"category"`
	SpanM      float64          `json:"spanM"`
	LengthM    float64          `json:"lengthM"`
	WheelbaseM float64          `json:"wheelbaseM"`
	CGM        float64          `json:"cgM"`
	TakeoffM   float64          `json:"takeoffDistanceM"`
	VappKts    float64          `json:"vappKts"`
	Pitch      float64          `json:"pitchDeg"`
	FlarePitch float64          `json:"flarePitchDeg"`
	FlareFt    float64          `json:"flareFt"`
	TouchFpm   float64          `json:"touchdownFpm"`
	Takeoff    TakeoffProfile   `json:"takeoff"`
	Brake      float64          `json:"brakeMS2"`
	TaxiKts    float64          `json:"taxiKts"`
	Flaps      FlapSchedule     `json:"flaps"`
	Heavy      bool             `json:"heavy,omitempty"`
}

// WakeItem is a type's wake categories (traffic.wake).
type WakeItem struct {
	Type string `json:"type"`
	Wake
}

// AirportNameItem is an airport as ATC names it (traffic.airportNames).
type AirportNameItem struct {
	ICAO string `json:"icao"`
	Name string `json:"name"`
}

// AIPUnitItem is an airport's ATC unit call signs by frequency
// (traffic.aipUnits).
type AIPUnitItem struct {
	ICAO  string            `json:"icao"`
	Units map[string]string `json:"units"` // "121.91": "Ruzyne Ground"
}

// GATypesItem is the types a kind of GA operator flies, by weight
// (traffic.gaTypes).
type GATypesItem struct {
	Kind  GAKind   `json:"kind"`
	Types []gaType `json:"types"`
}

type gaType struct {
	Type   string  `json:"type"`
	Weight float64 `json:"weight"`
}

var (
	telephonyNow    dict.Value[map[string]telephonyEntry]
	initialismsNow  dict.Value[map[string]bool]
	knownTypesNow   dict.Value[[]typeSpec]
	wakeNow         dict.Value[map[string]Wake]
	airportNamesNow dict.Value[map[string]string]
	aipUnitsNow     dict.Value[map[string]map[string]string]
	airlinesNow     dict.Value[[]Airline]
	gaTypesNow      dict.Value[map[GAKind][]gaType]
)

func init() {
	const wiki = "Wikipedia, \"List of airline codes\", retrieved 2026-10-02: https://en.wikipedia.org/wiki/List_of_airline_codes"
	reg := func(t dict.Table) { dict.Register(t) }

	reg(dict.Keyed("traffic.telephony", "icao", wiki, "CC BY-SA 4.0 (https://creativecommons.org/licenses/by-sa/4.0/)",
		shippedTelephony, func(i TelephonyItem) string { return strings.ToUpper(i.ICAO) },
		func(items []TelephonyItem) {
			m := make(map[string]telephonyEntry, len(items))
			for _, i := range items {
				m[strings.ToUpper(i.ICAO)] = telephonyEntry{tel: i.Telephony, name: i.Name}
			}
			telephonyNow.Store(m)
		}))
	reg(dict.Keyed("traffic.initialisms", "word", "", "",
		func() []InitialismItem {
			var out []InitialismItem
			for _, w := range slices.Sorted(maps.Keys(initialisms)) {
				out = append(out, InitialismItem{w})
			}
			return out
		}, func(i InitialismItem) string { return strings.ToUpper(i.Word) },
		func(items []InitialismItem) {
			m := map[string]bool{}
			for _, i := range items {
				m[strings.ToUpper(i.Word)] = true
			}
			initialismsNow.Store(m)
		}))
	reg(dict.Keyed("traffic.aircraftTypes", "type", "", "",
		func() []AircraftTypeItem {
			out := make([]AircraftTypeItem, len(knownTypes))
			for i, k := range knownTypes {
				out[i] = k.item()
			}
			return out
		}, func(i AircraftTypeItem) string { return i.Type },
		func(items []AircraftTypeItem) {
			out := make([]typeSpec, len(items))
			for i, it := range items {
				out[i] = it.spec()
			}
			knownTypesNow.Store(out)
		}))
	reg(dict.Keyed("traffic.wake", "type", "", "",
		func() []WakeItem {
			var out []WakeItem
			for _, t := range slices.Sorted(maps.Keys(wakeTypes)) {
				out = append(out, WakeItem{t, wakeTypes[t]})
			}
			return out
		}, func(i WakeItem) string { return strings.ToUpper(i.Type) },
		func(items []WakeItem) {
			m := map[string]Wake{}
			for _, i := range items {
				m[strings.ToUpper(i.Type)] = i.Wake
			}
			wakeNow.Store(m)
		}))
	reg(dict.Keyed("traffic.airportNames", "icao", "", "",
		func() []AirportNameItem {
			var out []AirportNameItem
			for _, k := range slices.Sorted(maps.Keys(scheduleAirportNames)) {
				out = append(out, AirportNameItem{k, scheduleAirportNames[k]})
			}
			return out
		}, func(i AirportNameItem) string { return strings.ToUpper(i.ICAO) },
		func(items []AirportNameItem) {
			m := map[string]string{}
			for _, i := range items {
				m[strings.ToUpper(i.ICAO)] = i.Name
			}
			airportNamesNow.Store(m)
		}))
	reg(dict.Keyed("traffic.aipUnits", "icao", "", "",
		func() []AIPUnitItem {
			var out []AIPUnitItem
			for _, k := range slices.Sorted(maps.Keys(aipUnitNames)) {
				out = append(out, AIPUnitItem{k, aipUnitNames[k]})
			}
			return out
		}, func(i AIPUnitItem) string { return strings.ToUpper(i.ICAO) },
		func(items []AIPUnitItem) {
			m := map[string]map[string]string{}
			for _, i := range items {
				m[strings.ToUpper(i.ICAO)] = i.Units
			}
			aipUnitsNow.Store(m)
		}))
	reg(dict.Keyed("traffic.airlines", "icao", "", "",
		defaultAirlines, func(a Airline) string { return strings.ToUpper(a.ICAO) },
		func(items []Airline) { airlinesNow.Store(items) }))
	reg(dict.Keyed("traffic.gaTypes", "kind", "", "",
		func() []GATypesItem {
			var out []GATypesItem
			for _, k := range slices.Sorted(maps.Keys(gaTypes)) {
				out = append(out, GATypesItem{k, gaTypes[k]})
			}
			return out
		}, func(i GATypesItem) string { return string(i.Kind) },
		func(items []GATypesItem) {
			m := map[GAKind][]gaType{}
			for _, i := range items {
				m[i.Kind] = i.Types
			}
			gaTypesNow.Store(m)
		}))
	for _, n := range []string{"traffic.telephony", "traffic.initialisms", "traffic.aircraftTypes", "traffic.wake",
		"traffic.airportNames", "traffic.aipUnits", "traffic.airlines", "traffic.gaTypes"} {
		_ = dict.Reset(n) // the shipped copies in use
	}
}

// shippedTelephony is the embedded telephony list.
func shippedTelephony() []TelephonyItem {
	var out []TelephonyItem
	for _, line := range strings.Split(telephonyTSV, "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		f := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 3)
		if len(f) == 3 {
			out = append(out, TelephonyItem{ICAO: f[0], Telephony: f[1], Name: f[2]})
		}
	}
	return out
}

func (s typeSpec) item() AircraftTypeItem {
	return AircraftTypeItem{Type: s.Type, Match: s.match, Category: s.Category, SpanM: s.span, LengthM: s.length, WheelbaseM: s.wheelbase,
		CGM: s.cg, TakeoffM: s.tod, VappKts: s.vapp, Pitch: s.pitch, FlarePitch: s.flarePitch, FlareFt: s.flareFt, TouchFpm: s.tdFpm,
		Takeoff: s.takeoff, Brake: s.brake, TaxiKts: s.taxi, Flaps: s.flaps, Heavy: s.heavy}
}

func (i AircraftTypeItem) spec() typeSpec {
	match := i.Match
	if len(match) == 0 {
		match = []string{strings.ToUpper(i.Type)}
	}
	return typeSpec{match: match, Type: i.Type, Category: i.Category, span: i.SpanM, length: i.LengthM, wheelbase: i.WheelbaseM,
		cg: i.CGM, tod: i.TakeoffM, vapp: i.VappKts, pitch: i.Pitch, flarePitch: i.FlarePitch, flareFt: i.FlareFt, tdFpm: i.TouchFpm,
		takeoff: i.Takeoff, brake: i.Brake, taxi: i.TaxiKts, flaps: i.Flaps, heavy: i.Heavy}
}
