package traffic

import (
	"maps"
	"strings"
	"sync/atomic"
)

// The clearance limit as said in a departure clearance ("cleared to
// Vienna"): the destination's city, but where a city has several airline
// airports the airport's own name ("cleared to Heathrow", "Orly"); a
// per-airport name overrides both (EHAM: "Schiphol"). The built-in names
// are only those where the usage is clear; a local table merges over them
// (SetClearanceLimits), the GSX way.

// clearanceLimitNames are the built-in names by ICAO.
var clearanceLimitNames = map[string]string{
	"EHAM": "Schiphol",
	// London
	"EGLL": "Heathrow", "EGKK": "Gatwick", "EGSS": "Stansted", "EGGW": "Luton", "EGLC": "London City",
	// Paris
	"LFPG": "Charles de Gaulle", "LFPO": "Orly",
	// Milan
	"LIMC": "Malpensa", "LIML": "Linate", "LIME": "Bergamo",
	// Rome
	"LIRF": "Fiumicino", "LIRA": "Ciampino",
	// Stockholm
	"ESSA": "Arlanda", "ESSB": "Bromma",
	// Istanbul
	"LTFM": "Istanbul", "LTFJ": "Sabiha Gokcen",
	// Moscow
	"UUEE": "Sheremetyevo", "UUDD": "Domodedovo", "UUWW": "Vnukovo",
	// New York
	"KJFK": "Kennedy", "KLGA": "LaGuardia", "KEWR": "Newark",
	// Washington
	"KIAD": "Dulles", "KDCA": "Reagan National",
	// Chicago
	"KORD": "O'Hare", "KMDW": "Midway",
	// Tokyo
	"RJTT": "Haneda", "RJAA": "Narita",
	// Seoul
	"RKSI": "Incheon", "RKSS": "Gimpo",
	// Shanghai
	"ZSPD": "Pudong", "ZSSS": "Hongqiao",
	// Buenos Aires
	"SAEZ": "Ezeiza", "SABE": "Aeroparque",
	// São Paulo
	"SBGR": "Guarulhos", "SBSP": "Congonhas",
}

// clearanceLimitsNow is the table in use: the built-in names with the
// local ones over them.
var clearanceLimitsNow atomic.Pointer[map[string]string]

func init() {
	m := maps.Clone(clearanceLimitNames)
	clearanceLimitsNow.Store(&m)
}

// SetClearanceLimits merges local names (ICAO → name) over the built-in
// ones, replacing any set before; an empty name removes the built-in one
// (the city is said). nil goes back to the built-in names.
func SetClearanceLimits(local map[string]string) {
	m := maps.Clone(clearanceLimitNames)
	for icao, name := range local {
		icao = strings.ToUpper(strings.TrimSpace(icao))
		if name = strings.TrimSpace(name); name == "" {
			delete(m, icao)
		} else {
			m[icao] = name
		}
	}
	clearanceLimitsNow.Store(&m)
}

// ClearanceLimits is the table in use (a copy).
func ClearanceLimits() map[string]string { return maps.Clone(*clearanceLimitsNow.Load()) }

// ClearanceLimit is the destination icao as said in a departure clearance:
// its name in the table, else city (the flight plan's city or airport
// name), else icao.
func ClearanceLimit(icao, city string) string {
	if name, ok := (*clearanceLimitsNow.Load())[strings.ToUpper(strings.TrimSpace(icao))]; ok {
		return name
	}
	if city = strings.TrimSpace(city); city != "" {
		return city
	}
	return strings.ToUpper(strings.TrimSpace(icao))
}
