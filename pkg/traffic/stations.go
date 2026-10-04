package traffic

import "github.com/mrlm-net/simconnect/pkg/airport"

// StationFor is position pos at the airport of l as said on the radio
// ("Ruzyne Tower") and its frequency ("134.56"; "" none), as the airport
// map says them: the AIP's unit call sign where known (AIPUnitName), else
// the scenery's frequency name for what the frequency is (a departure
// handed to the approach frequency talks to approach: there is no
// "Ruzyne Departure", #462). With no layout or no frequency for pos, the
// position's own name (PositionName) and "".
func StationFor(l *airport.Layout, pos Position) (name, freq string) {
	if l == nil {
		return PositionName(pos), ""
	}
	f, ok := l.FrequencyFor(FreqKind(pos))
	if !ok {
		return PositionName(pos), ""
	}
	if name := AIPUnitName(l.ICAO, f.String()); name != "" {
		return name, f.String()
	}
	return StationName(f.Name, KindPosition(f.Kind, pos)), f.String()
}

// KindPosition is the position a frequency of kind (airport.FreqTower, ...)
// is; pos when kind is none of them.
func KindPosition(kind string, pos Position) Position {
	switch kind {
	case airport.FreqClearance:
		return PosDelivery
	case airport.FreqGround:
		return PosGround
	case airport.FreqTower:
		return PosTower
	case airport.FreqApproach:
		return PosApproach
	case airport.FreqDeparture:
		return PosDeparture
	case airport.FreqCenter:
		return PosCenter
	}
	return pos
}

// FreqKind is the airport frequency kind a position talks on ("" none).
func FreqKind(pos Position) string {
	switch pos {
	case PosDelivery:
		return airport.FreqClearance
	case PosGround:
		return airport.FreqGround
	case PosTower:
		return airport.FreqTower
	case PosApproach:
		return airport.FreqApproach
	case PosDeparture:
		return airport.FreqDeparture
	case PosCenter:
		return airport.FreqCenter
	case PosATIS:
		return airport.FreqATIS
	}
	return ""
}

// aipUnitNames are unit call signs from the AIP where the scenery's
// frequency names do not give the unit (LKPR AD 2.18: approach and
// departure are "Ruzyně Radar"; the scenery names them "RUZYNE"). By
// airport, then frequency as the radio writes it.
var aipUnitNames = map[string]map[string]string{
	"LKPR": {
		"120.06": "Ruzyne Delivery", "121.91": "Ruzyne Ground", "134.56": "Ruzyne Tower",
		"118.31": "Ruzyne Radar", "119.01": "Ruzyne Radar", "120.53": "Praha Radar", "127.58": "Praha Radar",
	},
}

// AIPUnitName is the AIP's call sign of the unit on freq ("134.56") at
// icao, "" when none is known.
func AIPUnitName(icao, freq string) string {
	return aipUnitNames[icao][freq]
}
