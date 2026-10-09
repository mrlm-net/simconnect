package airport

import (
	"maps"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/dict"
)

// VFR data per airport (the MyCrew app's VFR flow: traffic.ReportAt,
// AtPoint, ContactFIS): its visual reporting points — the entry and exit
// points of its control zone and the route points — and the flight
// information station outside it. Shipped for a few airports from their
// published VFR manual, replaceable at runtime as the dict table
// "airport.vfr" (a host's own data or a local override file, merged by
// ICAO over the shipped copy).

// VFRPoint is a visual reporting point.
type VFRPoint struct {
	Name     string `json:"name"` // as said: "NOVEMBER"
	Position LatLon `json:"position"`
	// Entry: an entry and exit point of the control zone (else a route
	// point inside it).
	Entry bool `json:"entry,omitempty"`
	// Where is the landmark: "Velvary (silo)".
	Where string `json:"where,omitempty"`
}

// VFRAirport is an airport's VFR data.
type VFRAirport struct {
	ICAO   string     `json:"icao"`
	Points []VFRPoint `json:"points"`
	// FIS and FISFreq: the flight information station a VFR flight leaving
	// the zone is handed to, as said ("Praha Information") and its
	// frequency as written ("126.100"); "" none.
	FIS     string `json:"fis,omitempty"`
	FISFreq string `json:"fisFreq,omitempty"`
	// Source: where it is from and as of when.
	Source string `json:"source,omitempty"`
}

// dms is degrees, minutes and seconds as degrees.
func dms(d, m, s float64) float64 { return d + m/60 + s/3600 }

func vfrPoint(name, where string, entry bool, latD, latM, latS, lonD, lonM, lonS float64) VFRPoint {
	return VFRPoint{Name: name, Where: where, Entry: entry, Position: LatLon{Lat: dms(latD, latM, latS), Lon: dms(lonD, lonM, lonS)}}
}

// czFIS: the Czech FIR's flight information service (AIP ČR ENR 2.1:
// PRAHA INFORMATION 126.100 MHz, H24).
const czFIS, czFISFreq = "Praha Information", "126.100"

// KnownVFR is the shipped VFR data.
var KnownVFR = map[string]VFRAirport{
	"LKPR": {Source: "Czech VFR Manual, LKPR, WEF 01 OCT 26 (aim.rlp.cz)", FIS: czFIS, FISFreq: czFISFreq, Points: []VFRPoint{
		vfrPoint("SIERRA", "Beroun (motorway bridge)", true, 49, 57, 42, 14, 4, 58),
		vfrPoint("NOVEMBER", "Velvary (silo)", true, 50, 16, 6, 14, 14, 21),
		vfrPoint("WHISKEY", "Kačice (motorway flyover)", true, 50, 9, 10, 13, 58, 59),
		vfrPoint("ECHO", "Radotín (railway station)", true, 49, 59, 10, 14, 21, 41),
		vfrPoint("TANGO", "motorway interchange", false, 50, 2, 59, 14, 16, 22),
		vfrPoint("ALFA", "petrol station", false, 50, 8, 17, 14, 14, 38),
		vfrPoint("BRAVO", "railway flyover", false, 50, 11, 16, 14, 11, 9),
		vfrPoint("CHARLIE", "castle", false, 50, 11, 18, 14, 2, 28),
	}},
	"LKPD": {Source: "Czech VFR Manual, LKPD, 06 AUG 26 (aim.rlp.cz); a military MCTR", FIS: czFIS, FISFreq: czFISFreq, Points: []VFRPoint{
		vfrPoint("ECHO", "Zámrsk train station", true, 49, 59, 42, 16, 6, 33),
		vfrPoint("LIMA", "Lhota u Skutče, 0.3 NM west", true, 49, 51, 33, 16, 2, 5),
		vfrPoint("NOVEMBER", "Opatovice traffic roundabout", true, 50, 8, 21, 15, 47, 19),
		vfrPoint("OSKAR", "Tůmovka pond", true, 50, 7, 50, 16, 4, 0),
		vfrPoint("SIERRA", "Chrast church", true, 49, 54, 5, 15, 56, 9),
		vfrPoint("WHISKEY", "Chýšť collective farm", true, 50, 7, 40, 15, 32, 12),
		vfrPoint("X-RAY", "Prachovice cement plant", true, 49, 53, 50, 15, 38, 26),
		vfrPoint("ALFA", "Svinčany municipal office", false, 49, 58, 31, 15, 38, 35),
		vfrPoint("BRAVO", "Úhřetice industry hall", false, 49, 58, 27, 15, 52, 13),
		vfrPoint("CHARLIE", "Křičeň collective farm", false, 50, 6, 56, 15, 39, 10),
		vfrPoint("DELTA", "Bohumileč, 1 NM east of golf course", false, 50, 6, 9, 15, 51, 25),
	}},
	"LKTB": {Source: "Czech VFR Manual, LKTB, WEF 01 OCT 26 (aim.rlp.cz)", FIS: czFIS, FISFreq: czFISFreq, Points: []VFRPoint{
		vfrPoint("NOVEMBER", "Kuřim (railway crossing)", true, 49, 17, 32, 16, 33, 37),
		vfrPoint("ROMEO", "Rousínov (church)", true, 49, 12, 13, 16, 53, 10),
		vfrPoint("WHISKEY", "Ořechov (church)", true, 49, 6, 39, 16, 31, 15),
		vfrPoint("VICTOR", "Velké Němčice (highway intersection)", true, 48, 59, 47, 16, 41, 20),
		vfrPoint("ALFA", "Sokolnice (railway crossing)", false, 49, 7, 3, 16, 42, 12),
		vfrPoint("BRAVO", "Podolí (highway overbridge)", false, 49, 10, 54, 16, 42, 45),
	}},
	"LKKV": {Source: "Czech VFR Manual, LKKV, WEF 01 OCT 26 (aim.rlp.cz)", FIS: czFIS, FISFreq: czFISFreq, Points: []VFRPoint{
		vfrPoint("NOVEMBER", "Velká Nejda pond", true, 50, 16, 54, 12, 56, 19),
		vfrPoint("ECHO", "Žlutice (reservoir dam)", true, 50, 5, 3, 13, 7, 36),
		vfrPoint("SIERRA", "Bečov", true, 50, 5, 2, 12, 50, 24),
		vfrPoint("WHISKEY", "Loket", true, 50, 11, 22, 12, 45, 29),
		vfrPoint("ALFA", "Hotel Hubertus parking lot", false, 50, 14, 16, 12, 55, 40),
		vfrPoint("BRAVO", "Stanovice (north bank of the dam)", false, 50, 10, 15, 12, 53, 30),
	}},
	"LKMT": {Source: "Czech VFR Manual, LKMT, WEF 01 OCT 26 (aim.rlp.cz)", FIS: czFIS, FISFreq: czFISFreq, Points: []VFRPoint{
		vfrPoint("NOVEMBER", "Hrabyně", true, 49, 52, 59, 18, 3, 17),
		vfrPoint("WHISKEY", "Vrchy (church)", true, 49, 44, 57, 17, 52, 19),
		vfrPoint("TANGO", "Bělotín", true, 49, 35, 6, 17, 47, 59),
		vfrPoint("SIERRA", "Hodslavice", true, 49, 32, 20, 18, 1, 25),
		vfrPoint("ECHO", "Frýdek-Místek (reservoir dam)", true, 49, 39, 48, 18, 19, 13),
		vfrPoint("FOXTROT", "Šenov (church)", true, 49, 47, 10, 18, 22, 29),
		vfrPoint("ALFA", "Příbor", false, 49, 39, 0, 18, 8, 28),
		vfrPoint("BRAVO", "Studénka (railway crossing)", false, 49, 42, 17, 18, 3, 4),
	}},
}

var knownVFRNow dict.Value[map[string]VFRAirport]

func init() {
	dict.Register(dict.Keyed("airport.vfr", "icao", "Czech VFR Manual and AIP ČR (aim.rlp.cz)", "",
		func() []VFRAirport {
			var out []VFRAirport
			for _, k := range slices.Sorted(maps.Keys(KnownVFR)) {
				v := KnownVFR[k]
				v.ICAO = k
				out = append(out, v)
			}
			return out
		}, func(v VFRAirport) string { return strings.ToUpper(v.ICAO) },
		func(items []VFRAirport) {
			m := map[string]VFRAirport{}
			for _, v := range items {
				m[strings.ToUpper(v.ICAO)] = v
			}
			knownVFRNow.Store(m)
		}))
}

// VFRFor is icao's VFR data (the dict table's, else the shipped); false
// when there is none.
func VFRFor(icao string) (VFRAirport, bool) {
	icao = strings.ToUpper(icao)
	if m := knownVFRNow.Load(); m != nil {
		v, ok := m[icao]
		if ok {
			v.ICAO = icao
		}
		return v, ok
	}
	v, ok := KnownVFR[icao]
	v.ICAO = icao
	return v, ok
}

// Point is the reporting point named name (any case).
func (v VFRAirport) Point(name string) (VFRPoint, bool) {
	for _, p := range v.Points {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return VFRPoint{}, false
}

// EntryNearest is the entry and exit point nearest to pos: where a VFR
// arrival from there comes in, or a departure towards there leaves.
func (v VFRAirport) EntryNearest(pos LatLon) (VFRPoint, bool) {
	best, bestD, ok := VFRPoint{}, 0.0, false
	for _, p := range v.Points {
		if !p.Entry {
			continue
		}
		d := calc.HaversineNM(pos.Lat, pos.Lon, p.Position.Lat, p.Position.Lon)
		if !ok || d < bestD {
			best, bestD, ok = p, d, true
		}
	}
	return best, ok
}
