package traffic

import (
	"fmt"
	"hash/fnv"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Station is one ATC station of an airport (#722): a position worked on a
// frequency, by a controller, over a sector. A big airport has more than
// one per position (Ground North and South, Apron, a tower per runway); a
// quiet one has one controller on several frequencies (ground and tower
// combined), the same person and voice on each.
type Station struct {
	Position Position `json:"position"`
	// Name as said on the radio: "Ruzyne Ground", "Ruzyne Apron".
	Name string `json:"name"`
	// Freq as the radio writes it: "121.91".
	Freq string `json:"freq"`
	// Controller is who works it: stations with the same Controller are one
	// person (one voice, one thing said at a time). "" in an override: the
	// airport and frequency, so positions sharing a frequency share one.
	Controller string `json:"controller,omitempty"`
	// The sector: Taxiways (ground: by taxiway name, "JO"), Area (a
	// polygon, an apron) and Runways (tower: runway names "06/24" or ends
	// "24"). None: the rest of the position's traffic.
	Taxiways []string         `json:"taxiways,omitempty"`
	Area     []airport.LatLon `json:"area,omitempty"`
	Runways  []string         `json:"runways,omitempty"`
}

// sectored is whether s works only part of its position's traffic.
func (s Station) sectored() bool {
	return len(s.Taxiways) > 0 || len(s.Area) > 2 || len(s.Runways) > 0
}

// Where is what picks an aircraft's station of a position: the taxiways it
// is on, its runway, where it is.
type Where struct {
	Taxiways []string
	Runway   string
	At       *airport.LatLon
}

// covers is whether s's sector holds w.
func (s Station) covers(w Where) bool {
	for _, t := range s.Taxiways {
		if slices.ContainsFunc(w.Taxiways, func(x string) bool { return strings.EqualFold(x, t) }) {
			return true
		}
	}
	if w.At != nil && len(s.Area) > 2 && insidePolygon(*w.At, s.Area) {
		return true
	}
	if w.Runway != "" {
		for _, r := range s.Runways {
			if strings.EqualFold(r, w.Runway) || slices.ContainsFunc(strings.Split(r, "/"), func(e string) bool { return strings.EqualFold(e, w.Runway) }) {
				return true
			}
		}
	}
	return false
}

// insidePolygon is whether p is inside poly (even-odd rule, lat/lon as
// plane coordinates: an airport is small).
func insidePolygon(p airport.LatLon, poly []airport.LatLon) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a.Lat > p.Lat) != (b.Lat > p.Lat) && p.Lon < (b.Lon-a.Lon)*(p.Lat-a.Lat)/(b.Lat-a.Lat)+a.Lon {
			in = !in
		}
	}
	return in
}

// stationPositions are the positions an airport's stations are made for.
var stationPositions = []Position{PosDelivery, PosGround, PosTower, PosApproach, PosDeparture, PosCenter}

// DefaultStations are the stations of the airport of l from its scenery
// frequencies, one per position as StationFor says it; a position with no
// frequency of its own works on the one it falls back to (ground on the
// tower's), with that frequency's controller: one person.
func DefaultStations(l *airport.Layout) []Station {
	if l == nil {
		return nil
	}
	var out []Station
	for _, pos := range stationPositions {
		name, freq := StationFor(l, pos)
		if freq == "" {
			continue
		}
		out = append(out, Station{Position: pos, Name: name, Freq: freq, Controller: l.ICAO + " " + freq})
	}
	return out
}

// StationsWith are defaults with override (a local configuration): a
// position the override has stations for takes them instead of its
// default; an override station without a Controller gets the airport and
// frequency (icao "LKPR").
func StationsWith(icao string, defaults, override []Station) []Station {
	if len(override) == 0 {
		return defaults
	}
	mine := map[Position]bool{}
	for _, s := range override {
		mine[s.Position] = true
	}
	var out []Station
	for _, s := range defaults {
		if !mine[s.Position] {
			out = append(out, s)
		}
	}
	for _, s := range override {
		if s.Controller == "" {
			s.Controller = icao + " " + s.Freq
		}
		out = append(out, s)
	}
	return out
}

// PickStation is the station of position pos that works an aircraft at w:
// the first whose sector holds it, else the first without a sector, else
// the position's first. false: no station for pos.
func PickStation(stations []Station, pos Position, w Where) (Station, bool) {
	var open, first *Station
	for i := range stations {
		s := &stations[i]
		if s.Position != pos {
			continue
		}
		if first == nil {
			first = s
		}
		if !s.sectored() {
			if open == nil {
				open = s
			}
			continue
		}
		if s.covers(w) {
			return *s, true
		}
	}
	if open != nil {
		return *open, true
	}
	if first != nil {
		return *first, true
	}
	return Station{}, false
}

// ControllerOn is who works freq among stations ("" none).
func ControllerOn(stations []Station, freq string) string {
	for _, s := range stations {
		if s.Freq == freq {
			return s.Controller
		}
	}
	return ""
}

// BandBoxed are stations with ground and delivery worked by the tower's
// controller (#722): at night one person works them all, the same voice on
// each frequency; the frequencies and names stay.
func BandBoxed(stations []Station) []Station {
	tower, ok := PickStation(stations, PosTower, Where{})
	if !ok {
		return stations
	}
	out := append([]Station(nil), stations...)
	for i := range out {
		if out[i].Position == PosGround || out[i].Position == PosDelivery {
			out[i].Controller = tower.Controller
		}
	}
	return out
}

// DefaultStationsAt are DefaultStations with, where the airport has
// several frequencies for one unit (LKPR: Ruzyne Radar 118.31 and 119.01,
// Ruzyne Ground 121.91 and 131.95), the one in use picked for block: one
// at a time, another in another block (the World's three-hour blocks of
// traffic time, #772). A unit is its name as said (AIP or scenery); a
// unit's positions (approach, departure) share its pick.
func DefaultStationsAt(l *airport.Layout, block uint64) []Station {
	out := DefaultStations(l)
	if l == nil {
		return out
	}
	for i, s := range out {
		var kind string
		for _, f := range l.Frequencies {
			if f.String() == s.Freq {
				kind = f.Kind
				break
			}
		}
		var group []string
		for _, f := range l.Frequencies {
			if f.Kind != kind {
				continue
			}
			name := AIPUnitName(l.ICAO, f.String())
			if name == "" {
				name = StationName(f.Name, KindPosition(f.Kind, s.Position))
			}
			if name == s.Name && !slices.Contains(group, f.String()) {
				group = append(group, f.String())
			}
		}
		if len(group) < 2 {
			continue
		}
		h := fnv.New64a()
		fmt.Fprintf(h, "%s %s %d", l.ICAO, s.Name, block)
		pick := group[h.Sum64()%uint64(len(group))]
		out[i].Freq, out[i].Controller = pick, l.ICAO+" "+pick
	}
	return out
}
