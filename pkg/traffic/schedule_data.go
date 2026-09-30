//go:build windows
// +build windows

package traffic

import "github.com/mrlm-net/simconnect/pkg/airport"

// DefaultScheduleConfig is the built-in schedule data: European airlines
// with their fleets and bases, the airports they fly to, and a day of
// waves (quiet night, morning and evening peaks). Save it with
// SaveScheduleConfig to edit it.
func DefaultScheduleConfig() ScheduleConfig {
	return ScheduleConfig{
		Airlines: defaultAirlines(),
		Airports: defaultScheduleAirports(),
		Waves: [24]float64{
			0.05, 0.02, 0.02, 0.02, 0.05, 0.3, // 00–05
			0.8, 1, 0.9, 0.7, 0.65, 0.7, // 06–11
			0.75, 0.7, 0.65, 0.7, 0.8, 0.95, // 12–17
			1, 0.85, 0.7, 0.5, 0.3, 0.1, // 18–23
		},
		Types: map[string]TypeLimits{
			"AT76": {MinNM: 50, MaxNM: 700, RunwayM: 1400},
			"DH8D": {MinNM: 50, MaxNM: 900, RunwayM: 1500},
			"CRJ9": {MinNM: 100, MaxNM: 1400, RunwayM: 1800},
			"E190": {MinNM: 100, MaxNM: 2000, RunwayM: 1800},
			"BCS3": {MinNM: 100, MaxNM: 3000, RunwayM: 1900},
			"A20N": {MinNM: 150, MaxNM: 3200, RunwayM: 2000},
			"A320": {MinNM: 150, MaxNM: 3000, RunwayM: 2000},
			"A321": {MinNM: 200, MaxNM: 3200, RunwayM: 2200},
			"B738": {MinNM: 150, MaxNM: 3000, RunwayM: 2100},
			"B38M": {MinNM: 150, MaxNM: 3500, RunwayM: 2100},
			"B789": {MinNM: 1200, MaxNM: 7500, RunwayM: 2800},
			"B77W": {MinNM: 1200, MaxNM: 7300, RunwayM: 3000},
		},
	}
}

func defaultAirlines() []Airline {
	eu := []string{"LK", "LO", "LZ", "LH", "LJ", "LD", "LB", "LR", "LY", "LI", "LF", "LE", "LP", "LS", "LG", "LC", "LT", "LM", "ED", "EH", "EB", "EG", "EI", "EK", "EN", "ES", "EF", "EP", "EY", "EV", "EE", "EL", "BI"}
	central := []string{"LK", "LO", "LZ", "LH", "EP", "ED", "LJ", "LD"}
	return []Airline{
		// CSA: CSA-LINES (ICAO Doc 8585 as listed publicly).
		{ICAO: "CSA", Name: "Czech Airlines", Telephony: "CSA LINES", Fleet: map[string]float64{"A320": 1, "BCS3": 1}, Bases: []string{"LKPR"}, Regions: eu, Weight: 1},
		{ICAO: "TVS", Name: "Smartwings", Telephony: "SKYTRAVEL", Fleet: map[string]float64{"B738": 3, "B38M": 2}, Bases: []string{"LKPR", "LKTB", "LKMT"}, Regions: append([]string{"HE", "GC", "DT", "OJ"}, eu...), Weight: 1.5},
		{ICAO: "DLH", Name: "Lufthansa", Telephony: "LUFTHANSA", Fleet: map[string]float64{"A20N": 3, "A320": 2, "A321": 2, "CRJ9": 1, "B789": 0.5}, Bases: []string{"EDDF", "EDDM"}, Regions: []string{"*"}, Weight: 2},
		{ICAO: "AUA", Name: "Austrian", Telephony: "AUSTRIAN", Fleet: map[string]float64{"A320": 2, "E190": 1, "DH8D": 1}, Bases: []string{"LOWW"}, Regions: eu, Weight: 1},
		{ICAO: "SWR", Name: "Swiss", Telephony: "SWISS", Fleet: map[string]float64{"A20N": 1, "A320": 2, "A321": 1}, Bases: []string{"LSZH", "LSGG"}, Regions: eu, Weight: 1},
		{ICAO: "KLM", Name: "KLM", Telephony: "KLM", Fleet: map[string]float64{"B738": 2, "E190": 2, "B789": 0.5, "B77W": 0.3}, Bases: []string{"EHAM"}, Regions: []string{"*"}, Weight: 1.5},
		{ICAO: "AFR", Name: "Air France", Telephony: "AIRFRANS", Fleet: map[string]float64{"A320": 2, "A321": 1, "B77W": 0.3}, Bases: []string{"LFPG"}, Regions: []string{"*"}, Weight: 1.5},
		{ICAO: "BAW", Name: "British Airways", Telephony: "SPEEDBIRD", Fleet: map[string]float64{"A320": 2, "A20N": 1, "A321": 1, "B77W": 0.3, "B789": 0.3}, Bases: []string{"EGLL", "EGKK"}, Regions: []string{"*"}, Weight: 1.5},
		// LOT: the ICAO designator is POLLOT, but "LOT" is what is said in practice.
		{ICAO: "LOT", Name: "LOT", Telephony: "LOT", Fleet: map[string]float64{"B38M": 2, "E190": 2, "DH8D": 1}, Bases: []string{"EPWA"}, Regions: eu, Weight: 1},
		{ICAO: "RYR", Name: "Ryanair", Telephony: "RYANAIR", Fleet: map[string]float64{"B738": 3, "B38M": 1}, Bases: []string{"EIDW", "EGSS", "LIRA", "LEPA", "LPPT"}, Regions: eu, Weight: 3},
		{ICAO: "EZY", Name: "easyJet", Telephony: "EASY", Fleet: map[string]float64{"A320": 2, "A20N": 2, "A321": 1}, Bases: []string{"EGKK", "EGGW", "LSGG", "LFPG"}, Regions: eu, Weight: 2},
		{ICAO: "WZZ", Name: "Wizz Air", Telephony: "WIZZAIR", Fleet: map[string]float64{"A321": 3, "A20N": 1}, Bases: []string{"LHBP", "EPKT", "LROP"}, Regions: eu, Weight: 2},
		{ICAO: "THY", Name: "Turkish", Telephony: "TURKISH", Fleet: map[string]float64{"A321": 2, "B38M": 1, "B77W": 0.3}, Bases: []string{"LTFM"}, Regions: []string{"*"}, Weight: 1.5},
		{ICAO: "UAE", Name: "Emirates", Telephony: "EMIRATES", Fleet: map[string]float64{"B77W": 1}, Bases: []string{"OMDB"}, Regions: []string{"*"}, Weight: 0.5},
		{ICAO: "QTR", Name: "Qatar", Telephony: "QATARI", Fleet: map[string]float64{"B789": 1, "B77W": 1}, Bases: []string{"OTHH"}, Regions: []string{"*"}, Weight: 0.5},
		{ICAO: "KAL", Name: "Korean Air", Telephony: "KOREANAIR", Fleet: map[string]float64{"B77W": 1, "B789": 1}, Bases: []string{"RKSI"}, Regions: []string{"*"}, Weight: 0.2},
		{ICAO: "ENT", Name: "Enter Air", Telephony: "ENTER", Fleet: map[string]float64{"B738": 1}, Bases: []string{"EPWA", "EPKT"}, Regions: append([]string{"HE", "GC"}, central...), Weight: 0.5},
	}
}

// scheduleAirportNames are the destinations as ATC says them in a
// clearance: the city, and the airport where the city has several.
var scheduleAirportNames = map[string]string{
	"LKPR": "Prague", "LKTB": "Brno", "LKMT": "Ostrava", "LKKV": "Karlovy Vary",
	"LOWW": "Vienna", "LOWS": "Salzburg", "LOWI": "Innsbruck", "LZIB": "Bratislava", "LZKZ": "Kosice",
	"LHBP": "Budapest", "EPWA": "Warsaw", "EPKK": "Krakow", "EPKT": "Katowice", "EPGD": "Gdansk",
	"EDDF": "Frankfurt", "EDDM": "Munich", "EDDB": "Berlin", "EDDH": "Hamburg", "EDDL": "Dusseldorf", "EDDS": "Stuttgart",
	"EDDK": "Cologne", "EDDN": "Nuremberg", "EDDP": "Leipzig", "EDDC": "Dresden",
	"LSZH": "Zurich", "LSGG": "Geneva", "LJLJ": "Ljubljana", "LDZA": "Zagreb", "LDSP": "Split", "LDDU": "Dubrovnik",
	"EHAM": "Amsterdam", "EBBR": "Brussels", "LFPG": "Paris Charles de Gaulle", "LFPO": "Paris Orly", "LFMN": "Nice",
	"EGLL": "London Heathrow", "EGKK": "London Gatwick", "EGSS": "London Stansted", "EGGW": "London Luton", "EGCC": "Manchester", "EIDW": "Dublin",
	"EKCH": "Copenhagen", "ENGM": "Oslo", "ESSA": "Stockholm Arlanda", "EFHK": "Helsinki", "EVRA": "Riga", "EYVI": "Vilnius", "EETN": "Tallinn",
	"LEMD": "Madrid", "LEBL": "Barcelona", "LEPA": "Palma", "LEMG": "Malaga", "LPPT": "Lisbon", "LPFR": "Faro",
	"LIRF": "Rome Fiumicino", "LIRA": "Rome Ciampino", "LIMC": "Milan Malpensa", "LIPZ": "Venice", "LICC": "Catania",
	"LGAV": "Athens", "LGIR": "Heraklion", "LGRP": "Rhodes", "LCLK": "Larnaca",
	"LBSF": "Sofia", "LBBG": "Burgas", "LROP": "Bucharest", "LYBE": "Belgrade", "LMML": "Malta",
	"LTFM": "Istanbul", "LTAI": "Antalya", "GCTS": "Tenerife South", "GCLP": "Gran Canaria",
	"HEGN": "Hurghada", "HESH": "Sharm el Sheikh", "OJAQ": "Aqaba", "DTTJ": "Djerba",
	"OMDB": "Dubai", "OTHH": "Doha", "RKSI": "Seoul Incheon", "KJFK": "New York Kennedy", "KORD": "Chicago O'Hare", "CYYZ": "Toronto",
}

func defaultScheduleAirports() []ScheduleAirport {
	a := func(icao string, lat, lon float64, size int, rwy float64) ScheduleAirport {
		return ScheduleAirport{ICAO: icao, Name: scheduleAirportNames[icao], Position: airport.LatLon{Lat: lat, Lon: lon}, Size: size, RunwayM: rwy}
	}
	return []ScheduleAirport{
		// Czechia and neighbours
		a("LKPR", 50.1008, 14.2600, 2, 3715), a("LKTB", 49.1513, 16.6944, 1, 2650), a("LKMT", 49.6963, 18.1111, 1, 3500), a("LKKV", 50.2030, 12.9150, 1, 2150),
		a("LOWW", 48.1103, 16.5697, 3, 3600), a("LOWS", 47.7933, 13.0043, 1, 2750), a("LOWI", 47.2602, 11.3440, 1, 2000),
		a("LZIB", 48.1702, 17.2127, 1, 3190), a("LZKZ", 48.6631, 21.2411, 1, 3100),
		a("LHBP", 47.4298, 19.2611, 2, 3707), a("EPWA", 52.1657, 20.9671, 2, 3690), a("EPKK", 50.0777, 19.7848, 1, 2550), a("EPKT", 50.4743, 19.0800, 1, 3200), a("EPGD", 54.3776, 18.4662, 1, 2800),
		a("EDDF", 50.0333, 8.5706, 3, 4000), a("EDDM", 48.3538, 11.7861, 3, 4000), a("EDDB", 52.3667, 13.5033, 2, 4000), a("EDDH", 53.6304, 9.9882, 2, 3666), a("EDDL", 51.2895, 6.7668, 2, 3000), a("EDDS", 48.6899, 9.2220, 1, 3345), a("EDDK", 50.8659, 7.1427, 1, 3815), a("EDDN", 49.4987, 11.0669, 1, 2700), a("EDDP", 51.4239, 12.2364, 1, 3600), a("EDDC", 51.1328, 13.7672, 1, 2850),
		a("LSZH", 47.4647, 8.5492, 3, 3700), a("LSGG", 46.2381, 6.1090, 2, 3900),
		a("LJLJ", 46.2237, 14.4576, 1, 3300), a("LDZA", 45.7429, 16.0688, 1, 3252), a("LDSP", 43.5389, 16.2980, 1, 2550), a("LDDU", 42.5614, 18.2682, 1, 3300),
		// Western Europe
		a("EHAM", 52.3086, 4.7639, 3, 3800), a("EBBR", 50.9014, 4.4844, 2, 3638), a("LFPG", 49.0097, 2.5479, 3, 4215), a("LFPO", 48.7233, 2.3794, 2, 3650), a("LFMN", 43.6584, 7.2159, 1, 2960),
		a("EGLL", 51.4700, -0.4543, 3, 3902), a("EGKK", 51.1481, -0.1903, 2, 3316), a("EGSS", 51.8850, 0.2350, 2, 3049), a("EGGW", 51.8747, -0.3683, 1, 2160), a("EGCC", 53.3537, -2.2750, 2, 3050), a("EIDW", 53.4213, -6.2701, 2, 3110),
		a("EKCH", 55.6180, 12.6508, 2, 3600), a("ENGM", 60.1976, 11.1004, 2, 3600), a("ESSA", 59.6519, 17.9186, 2, 3301), a("EFHK", 60.3172, 24.9633, 2, 3500),
		a("EVRA", 56.9236, 23.9711, 1, 3200), a("EYVI", 54.6341, 25.2858, 1, 2515), a("EETN", 59.4133, 24.8328, 1, 3480),
		// South
		a("LEMD", 40.4719, -3.5626, 3, 4350), a("LEBL", 41.2971, 2.0785, 3, 3352), a("LEPA", 39.5517, 2.7388, 2, 3270), a("LEMG", 36.6749, -4.4991, 2, 3200), a("LPPT", 38.7813, -9.1359, 2, 3705), a("LPFR", 37.0144, -7.9659, 1, 2490),
		a("LIRF", 41.8003, 12.2389, 3, 3900), a("LIRA", 41.7994, 12.5949, 1, 2205), a("LIMC", 45.6306, 8.7281, 2, 3920), a("LIPZ", 45.5053, 12.3519, 1, 3300), a("LICC", 37.4668, 15.0664, 1, 2436),
		a("LGAV", 37.9364, 23.9445, 2, 4000), a("LGIR", 35.3397, 25.1803, 1, 2714), a("LGRP", 36.4054, 28.0862, 1, 3305), a("LCLK", 34.8751, 33.6249, 1, 2980),
		a("LBSF", 42.6967, 23.4114, 1, 3600), a("LBBG", 42.5696, 27.5152, 1, 3200), a("LROP", 44.5711, 26.0850, 2, 3500), a("LYBE", 44.8184, 20.3091, 1, 3400), a("LMML", 35.8575, 14.4775, 1, 3544),
		a("LTFM", 41.2753, 28.7519, 3, 4100), a("LTAI", 36.8987, 30.8005, 2, 3400),
		a("GCTS", 28.0445, -16.5725, 1, 3200), a("GCLP", 27.9319, -15.3866, 1, 3100), a("HEGN", 27.1783, 33.7994, 1, 4000), a("HESH", 27.9773, 34.3950, 1, 3080), a("OJAQ", 29.6116, 35.0181, 1, 3000), a("DTTJ", 33.8750, 10.7755, 1, 3100),
		// Long haul
		a("OMDB", 25.2528, 55.3644, 3, 4447), a("OTHH", 25.2731, 51.6081, 3, 4850), a("RKSI", 37.4602, 126.4407, 3, 4000),
		a("KJFK", 40.6398, -73.7789, 3, 4423), a("KORD", 41.9786, -87.9048, 3, 3962), a("CYYZ", 43.6772, -79.6306, 3, 3389),
	}
}
