//go:build windows
// +build windows

package nav

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// PLNPlan is an MSFS flight plan file (.pln) as read: what the file says,
// nothing worked out. Fields the file leaves out stay empty.
type PLNPlan struct {
	Title     string
	Rules     string // FPType: "IFR" or "VFR"
	RouteType string // e.g. "HighAlt", "LowAlt", "VOR", "Direct"
	// CruiseFt is CruisingAlt in feet (0 when missing).
	CruiseFt    float64
	Departure   string // DepartureID, e.g. "LKPR"
	Destination string // DestinationID
	// DepartureRunway and ArrivalRunway, SID, STAR and Approach come from
	// the waypoints' DepartureFP / ArrivalFP / ApproachTypeFP and
	// RunwayNumberFP / RunwayDesignatorFP, as MSFS writes them.
	DepartureRunway string
	SID             string
	STAR            string
	Approach        string // e.g. "ILS", "RNAV"
	ArrivalRunway   string
	// DeparturePosition is the gate or runway the flight starts at, when set.
	DeparturePosition string
	Waypoints         []PLNWaypoint
}

// PLNWaypoint is one ATCWaypoint of a .pln.
type PLNWaypoint struct {
	ID       string // the id attribute
	Type     string // ATCWaypointType: Airport, Intersection, VOR, NDB, User, ...
	Ident    string // ICAOIdent ("" for user points)
	Region   string // ICAORegion
	Position airport.LatLon
	AltFt    float64
	Airway   string // ATCAirway: the airway it is reached by
	// SID, STAR and Approach: the procedure the point belongs to, if any.
	SID, STAR, Approach string
}

// ReadPLNFile reads a .pln file; see ReadPLN.
func ReadPLNFile(path string) (*PLNPlan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadPLN(f)
}

// ReadPLN reads an MSFS flight plan (.pln, AceXML) as MSFS 2020/2024 and
// FlightPlan.PLN write it: the departure and destination, rules, cruise
// altitude, the waypoints with their airways, and the SID, STAR and
// approach with their runways named on the waypoints. A .pln has no
// alternate airport field; none is read.
func ReadPLN(r io.Reader) (*PLNPlan, error) {
	var doc plnDocument
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("nav: reading .pln: %w", err)
	}
	fp := doc.FlightPlan
	p := &PLNPlan{
		Title: fp.Title, Rules: strings.ToUpper(strings.TrimSpace(fp.FPType)), RouteType: fp.RouteType,
		Departure: strings.TrimSpace(fp.DepartureID), Destination: strings.TrimSpace(fp.DestinationID),
		DeparturePosition: fp.DeparturePosition,
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(fp.CruisingAlt), 64); err == nil {
		p.CruiseFt = v
	}
	for _, w := range fp.Waypoints {
		pw := PLNWaypoint{ID: w.ID, Type: w.Type, Airway: w.ATCAirway, SID: w.DepartureFP, STAR: w.ArrivalFP, Approach: w.ApproachTypeFP}
		if w.ICAO != nil {
			pw.Ident, pw.Region = w.ICAO.Ident, w.ICAO.Region
		}
		if pos, alt, ok := ParseLLA(w.WorldPosition); ok {
			pw.Position, pw.AltFt = pos, alt
		}
		rwy := strings.TrimSpace(w.RunwayNumberFP) + plnDesignator(w.RunwayDesignatorFP)
		if n, err := strconv.Atoi(strings.TrimSpace(w.RunwayNumberFP)); err == nil && n < 10 {
			rwy = "0" + rwy // "6" → "06"
		}
		switch {
		case w.DepartureFP != "":
			if p.SID == "" {
				p.SID = w.DepartureFP
			}
			if p.DepartureRunway == "" && w.RunwayNumberFP != "" {
				p.DepartureRunway = rwy
			}
		case w.ApproachTypeFP != "":
			if p.Approach == "" {
				p.Approach = w.ApproachTypeFP
			}
			if w.RunwayNumberFP != "" {
				p.ArrivalRunway = rwy
			}
		case w.ArrivalFP != "":
			if p.STAR == "" {
				p.STAR = w.ArrivalFP
			}
			if p.ArrivalRunway == "" && w.RunwayNumberFP != "" {
				p.ArrivalRunway = rwy
			}
		}
		p.Waypoints = append(p.Waypoints, pw)
	}
	return p, nil
}

// plnDesignator is a RunwayDesignatorFP as a runway name's suffix: "LEFT"
// → "L", "RIGHT" → "R", "CENTER" → "C"; anything else nothing.
func plnDesignator(d string) string {
	switch strings.ToUpper(strings.TrimSpace(d)) {
	case "LEFT", "L":
		return "L"
	case "RIGHT", "R":
		return "R"
	case "CENTER", "CENTRE", "C":
		return "C"
	}
	return ""
}

var llaPart = regexp.MustCompile(`^([NSEW])\s*(\d+)°\s*(\d+)'\s*([\d.]+)"$`)

// ParseLLA reads a .pln position, `N50° 6' 3.00",E14° 15' 36.00",+001194.62`
// (FormatLLA's form): the position and the altitude in feet.
func ParseLLA(s string) (airport.LatLon, float64, bool) {
	parts := strings.Split(s, ",")
	if len(parts) < 2 {
		return airport.LatLon{}, 0, false
	}
	var v [2]float64
	for i := range 2 {
		m := llaPart.FindStringSubmatch(strings.TrimSpace(parts[i]))
		if m == nil {
			return airport.LatLon{}, 0, false
		}
		deg, _ := strconv.ParseFloat(m[2], 64)
		mins, _ := strconv.ParseFloat(m[3], 64)
		secs, _ := strconv.ParseFloat(m[4], 64)
		v[i] = deg + mins/60 + secs/3600
		if m[1] == "S" || m[1] == "W" {
			v[i] = -v[i]
		}
	}
	alt := 0.0
	if len(parts) > 2 {
		alt, _ = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	}
	return airport.LatLon{Lat: v[0], Lon: v[1]}, alt, true
}
