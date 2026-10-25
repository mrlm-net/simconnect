package traffic

import (
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // the zones on machines without a Go install

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Airline timetables follow the clock on the airport's wall, not the sun:
// Madrid's morning bank leaves at 07:00 CEST, 05:00 UTC, while the sun
// there says a quarter to five. The waves are read in the airport's civil
// time, summer time included; a region the table below does not know falls
// back to solar time from the longitude.

// zoneByPrefix is the time zone of a single-zone country's ICAO prefix
// (two letters first, then one).
var zoneByPrefix = map[string]string{
	"LK": "Europe/Prague", "LO": "Europe/Vienna", "LZ": "Europe/Bratislava", "EP": "Europe/Warsaw",
	"ED": "Europe/Berlin", "ET": "Europe/Berlin", "LH": "Europe/Budapest", "LJ": "Europe/Ljubljana",
	"LD": "Europe/Zagreb", "LY": "Europe/Belgrade", "LQ": "Europe/Sarajevo", "LW": "Europe/Skopje",
	"LA": "Europe/Tirane", "LR": "Europe/Bucharest", "LB": "Europe/Sofia", "LG": "Europe/Athens",
	"LT": "Europe/Istanbul", "LC": "Asia/Nicosia", "LM": "Europe/Malta", "LI": "Europe/Rome",
	"LS": "Europe/Zurich", "LF": "Europe/Paris", "LN": "Europe/Monaco", "EB": "Europe/Brussels",
	"EH": "Europe/Amsterdam", "EL": "Europe/Luxembourg", "EG": "Europe/London", "EI": "Europe/Dublin",
	"EK": "Europe/Copenhagen", "EN": "Europe/Oslo", "ES": "Europe/Stockholm", "EF": "Europe/Helsinki",
	"EE": "Europe/Tallinn", "EV": "Europe/Riga", "EY": "Europe/Vilnius", "UM": "Europe/Minsk",
	"UK": "Europe/Kyiv", "LU": "Europe/Chisinau", "LP": "Europe/Lisbon", "LE": "Europe/Madrid",
	"GC": "Atlantic/Canary", "LX": "Europe/Gibraltar", "BI": "Atlantic/Reykjavik", "UU": "Europe/Moscow",
	"UL": "Europe/Moscow", "UG": "Asia/Tbilisi", "UD": "Asia/Yerevan", "UB": "Asia/Baku",
	"LL": "Asia/Jerusalem", "OJ": "Asia/Amman", "OL": "Asia/Beirut", "OS": "Asia/Damascus",
	"OR": "Asia/Baghdad", "OI": "Asia/Tehran", "OK": "Asia/Kuwait", "OB": "Asia/Bahrain",
	"OT": "Asia/Qatar", "OM": "Asia/Dubai", "OO": "Asia/Muscat", "OE": "Asia/Riyadh",
	"HE": "Africa/Cairo", "GM": "Africa/Casablanca", "DT": "Africa/Tunis", "DA": "Africa/Algiers",
	"HL": "Africa/Tripoli", "HK": "Africa/Nairobi", "DN": "Africa/Lagos", "FA": "Africa/Johannesburg",
	"OP": "Asia/Karachi", "VI": "Asia/Kolkata", "VA": "Asia/Kolkata", "VO": "Asia/Kolkata",
	"VE": "Asia/Kolkata", "VT": "Asia/Bangkok", "VV": "Asia/Ho_Chi_Minh", "WS": "Asia/Singapore",
	"WM": "Asia/Kuala_Lumpur", "VH": "Asia/Hong_Kong", "RC": "Asia/Taipei", "RP": "Asia/Manila",
	"RK": "Asia/Seoul", "RJ": "Asia/Tokyo", "RO": "Asia/Tokyo", "NZ": "Pacific/Auckland",
	"PH": "Pacific/Honolulu", "PA": "America/Anchorage", "Z": "Asia/Shanghai",
}

// The Azores keep their own hour, an hour behind Lisbon.
var azores = map[string]bool{"LPAZ": true, "LPPD": true, "LPLA": true, "LPHR": true, "LPFL": true,
	"LPPI": true, "LPSJ": true, "LPGR": true, "LPCR": true}

var zones sync.Map // name → *time.Location

func loadZone(name string) *time.Location {
	if z, ok := zones.Load(name); ok {
		return z.(*time.Location)
	}
	z, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}
	zones.Store(name, z)
	return z
}

// AirportZone is the civil time zone of the airport icao at p, or nil when
// it is not known (LocalHour then uses solar time).
func AirportZone(icao string, p airport.LatLon) *time.Location {
	icao = strings.ToUpper(icao)
	if azores[icao] {
		return loadZone("Atlantic/Azores")
	}
	if len(icao) == 4 && icao[0] == 'K' {
		// The contiguous US by longitude; Arizona keeps standard time.
		switch {
		case p.Lon > -87.5:
			return loadZone("America/New_York")
		case p.Lon > -101:
			return loadZone("America/Chicago")
		case p.Lon > -114.5 && p.Lat < 37 && p.Lon < -109:
			return loadZone("America/Phoenix")
		case p.Lon > -114.5:
			return loadZone("America/Denver")
		}
		return loadZone("America/Los_Angeles")
	}
	for _, n := range []int{2, 1} {
		if len(icao) >= n {
			if z, ok := zoneByPrefix[icao[:n]]; ok {
				return loadZone(z)
			}
		}
	}
	return nil
}

// LocalHour is the hour (0–23) on the airport's clock at t: its civil time
// (AirportZone), or solar time from the longitude where that is unknown.
func LocalHour(icao string, p airport.LatLon, t time.Time) int {
	if z := AirportZone(icao, p); z != nil {
		return t.In(z).Hour()
	}
	return t.Add(time.Duration(p.Lon / 15 * float64(time.Hour))).UTC().Hour()
}
