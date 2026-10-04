package airport

import (
	"fmt"
	"math"
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
	FreqOther     = "other"
)

// Frequency is one of an airport's radio frequencies.
type Frequency struct {
	Kind string  `json:"kind"`
	MHz  float64 `json:"mhz"`
	Name string  `json:"name"` // as the scenery names it ("PRAHA TOWER"), may be empty
}

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
		kind := FreqOther
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
		case types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_CTAF, types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_UNICOM, types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_MULTICOM:
			kind = FreqCTAF
		}
		out = append(out, Frequency{Kind: kind, MHz: math.Round(float64(r.Hz)/1000) / 1000, Name: r.Name})
	}
	return out
}

// FrequencyFor is the airport's frequency of kind, falling back as ATC
// does where a position is not staffed separately: clearance to ground,
// ground to tower, departure to approach, approach to center, and tower to
// the common traffic frequency. ok is false when there is none.
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
	return Frequency{}, false
}
