package airport

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Frequency kinds: what a frequency is for (#416).
const (
	FreqATIS      = "atis"
	FreqClearance = "clearance"
	FreqGround    = "ground"
	FreqTower     = "tower"
	FreqApproach  = "approach"
	FreqDeparture = "departure"
	FreqCenter    = "center"
	FreqCTAF      = "ctaf" // common traffic advisory (and UNICOM, MULTICOM)
	// FreqFIS: flight information outside controlled airspace (VFRFor's FIS),
	// the last fallback of FrequencyFor.
	FreqFIS = "fis"
	FreqOther     = "other"
)

// Frequency is one of an airport's radio frequencies.
type Frequency struct {
	Kind string  `json:"kind"`
	MHz  float64 `json:"mhz"`
	Name string  `json:"name"` // as the scenery names it ("PRAHA TOWER"), may be empty
	// Service is which common frequency a FreqCTAF one is: ServiceCTAF,
	// ServiceUNICOM or ServiceMULTICOM; "" for the other kinds.
	Service string `json:"service,omitempty"`
}

// The common frequencies' services (Frequency.Service), where pilots
// announce themselves at a field without a tower (#1296).
const (
	ServiceCTAF     = "ctaf"
	ServiceUNICOM   = "unicom"
	ServiceMULTICOM = "multicom"
)

// String is the frequency as said: "118.105".
func (f Frequency) String() string {
	return FormatMHz(f.MHz)
}

// FormatMHz is a frequency in MHz as set on a radio: three decimals
// ("118.105"), two where the third is 0 ("121.90").
func FormatMHz(mhz float64) string {
	s := fmt.Sprintf("%.3f", mhz)
	if strings.HasSuffix(s, "0") {
		s = s[:len(s)-1]
	}
	return s
}

func frequenciesOf(raw []RawFrequency) []Frequency {
	var out []Frequency
	for _, r := range raw {
		if r.Hz <= 0 {
			continue
		}
		kind, service := FreqOther, ""
		switch types.SIMCONNECT_FACILITY_FREQUENCY_TYPE(r.Type) {
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_ATIS, types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_AWOS, types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_ASOS:
			kind = FreqATIS
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_CLEARANCE, types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_CPT:
			kind = FreqClearance
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_GROUND:
			kind = FreqGround
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_TOWER:
			kind = FreqTower
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_APPROACH:
			kind = FreqApproach
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_DEPARTURE:
			kind = FreqDeparture
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_CENTER:
			kind = FreqCenter
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_CTAF:
			kind, service = FreqCTAF, ServiceCTAF
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_UNICOM:
			kind, service = FreqCTAF, ServiceUNICOM
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_MULTICOM:
			kind, service = FreqCTAF, ServiceMULTICOM
		}
		out = append(out, Frequency{Kind: kind, MHz: math.Round(float64(r.Hz)/1000) / 1000, Name: r.Name, Service: service})
	}
	return out
}

// FrequencyFor is the airport's frequency of kind, falling back as ATC
// does where a position is not staffed separately: clearance to ground,
// ground to tower, departure to approach, approach to center, and tower to
// the common traffic frequency; with none of these, flight information
// (FreqFIS) for tower, ground, clearance or CTAF where the airport's VFR
// data gives it (VFRFor: Praha
// Information at LKPR). ok is false when there is none at all: a caller
// says nothing, or its own station.
func (l *Layout) FrequencyFor(kind string) (Frequency, bool) {
	fallback := map[string][]string{
		FreqClearance: {FreqGround, FreqTower, FreqCTAF},
		FreqGround:    {FreqTower, FreqCTAF},
		FreqTower:     {FreqCTAF},
		FreqDeparture: {FreqApproach, FreqCenter},
		FreqApproach:  {FreqCenter},
	}
	for _, k := range append([]string{kind}, fallback[kind]...) {
		for _, f := range l.Frequencies {
			if f.Kind == k {
				return f, true
			}
		}
	}
	switch kind { // a field without them: flight information; not for IFR positions
	case FreqTower, FreqGround, FreqClearance, FreqCTAF:
		if v, ok := VFRFor(l.ICAO); ok && v.FISFreq != "" {
			if mhz, err := strconv.ParseFloat(v.FISFreq, 64); err == nil {
				return Frequency{Kind: FreqFIS, MHz: mhz, Name: v.FIS}, true
			}
		}
	}
	return Frequency{}, false
}

// Controlled reports whether the airport has a tower: a frequency of
// kind FreqTower. Without one, pilots announce themselves on its common
// frequency (SelfAnnounceFrequency) and nobody clears them (#1296).
func (l *Layout) Controlled() bool {
	for _, f := range l.Frequencies {
		if f.Kind == FreqTower {
			return true
		}
	}
	return false
}

// SelfAnnounceFrequency is the common frequency pilots announce themselves
// on: the CTAF, else the UNICOM, else the MULTICOM. One named only for the
// runway lights (AYBK's MULTICOM "ACTIVATE LIGHT ONLY") is not a
// frequency to talk on. ok is false when there is none.
func (l *Layout) SelfAnnounceFrequency() (Frequency, bool) {
	for _, service := range []string{ServiceCTAF, ServiceUNICOM, ServiceMULTICOM, ""} {
		for _, f := range l.Frequencies {
			if f.Kind == FreqCTAF && f.Service == service && !lightsOnly(f.Name) {
				return f, true
			}
		}
	}
	return Frequency{}, false
}

// lightsOnly reports a frequency named for pilot-controlled lighting
// alone: "LIGHT" in its name and no traffic service ("CTAF", "UNICOM",
// "MULTICOM", "TRAFFIC", "ADVISORY").
func lightsOnly(name string) bool {
	n := strings.ToUpper(name)
	if !strings.Contains(n, "LIGHT") {
		return false
	}
	for _, w := range []string{"CTAF", "UNICOM", "MULTICOM", "TRAFFIC", "ADVISORY"} {
		if strings.Contains(n, w) {
			return false
		}
	}
	return true
}

// ControlledFrequency is FrequencyFor for a controller's position only,
// never the common frequency or flight information: at an airport without
// a tower (Controlled false) there is no tower, and ground or clearance
// only on a frequency of its own (KVUO: clearance by Portland), not
// falling back; approach, departure and center fall back as in
// FrequencyFor (an IFR flight's release comes from there).
func (l *Layout) ControlledFrequency(kind string) (Frequency, bool) {
	switch kind {
	case FreqCTAF, FreqFIS:
		return Frequency{}, false
	case FreqTower, FreqGround, FreqClearance:
		if l.Controlled() {
			break
		}
		for _, f := range l.Frequencies {
			if f.Kind == kind && kind != FreqTower {
				return f, true
			}
		}
		return Frequency{}, false
	}
	return l.FrequencyFor(kind)
}
