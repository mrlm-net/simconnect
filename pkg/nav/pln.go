package nav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The AppVersion a .pln file is written with (MSFS 2020 SU15, which MSFS
// 2024 reads too).
const (
	plnAppVersionMajor = 11
	plnAppVersionBuild = 282174
)

// plnDocument is an MSFS flight plan file (.pln, AceXML).
type plnDocument struct {
	XMLName    xml.Name      `xml:"SimBase.Document"`
	Type       string        `xml:"Type,attr"`
	Version    string        `xml:"version,attr"`
	Descr      string        `xml:"Descr"`
	FlightPlan plnFlightPlan `xml:"FlightPlan.FlightPlan"`
}

type plnFlightPlan struct {
	Title             string        `xml:"Title"`
	FPType            string        `xml:"FPType"`
	RouteType         string        `xml:"RouteType"`
	CruisingAlt       string        `xml:"CruisingAlt"`
	DepartureID       string        `xml:"DepartureID"`
	DepartureLLA      string        `xml:"DepartureLLA"`
	DestinationID     string        `xml:"DestinationID"`
	DestinationLLA    string        `xml:"DestinationLLA"`
	Descr             string        `xml:"Descr"`
	DeparturePosition string        `xml:"DeparturePosition,omitempty"`
	DepartureName     string        `xml:"DepartureName"`
	DestinationName   string        `xml:"DestinationName"`
	AppVersion        plnAppVersion `xml:"AppVersion"`
	// The MSFS 2024 layout (AppVersionMajor 12) names the runways and
	// procedures here instead of on the waypoints; read only.
	DepartureDetails *plnDetails   `xml:"DepartureDetails,omitempty"`
	Waypoints        []plnWaypoint `xml:"ATCWaypoint"`
	ArrivalDetails   *plnDetails   `xml:"ArrivalDetails,omitempty"`
	ApproachDetails  *plnDetails   `xml:"ApproachDetails,omitempty"`
}

// plnDetails is a DepartureDetails, ArrivalDetails or ApproachDetails
// block of the MSFS 2024 layout.
type plnDetails struct {
	RunwayNumberFP     string `xml:"RunwayNumberFP,omitempty"`
	RunwayDesignatorFP string `xml:"RunwayDesignatorFP,omitempty"`
	DepartureFP        string `xml:"DepartureFP,omitempty"`
	ArrivalFP          string `xml:"ArrivalFP,omitempty"`
	ApproachTypeFP     string `xml:"ApproachTypeFP,omitempty"`
	SuffixFP           string `xml:"SuffixFP,omitempty"`
}

type plnAppVersion struct {
	Major int `xml:"AppVersionMajor"`
	Build int `xml:"AppVersionBuild"`
}

type plnWaypoint struct {
	ID                 string   `xml:"id,attr"`
	Type               string   `xml:"ATCWaypointType"`
	WorldPosition      string   `xml:"WorldPosition"`
	SpeedMaxFP         string   `xml:"SpeedMaxFP,omitempty"`
	ATCAirway          string   `xml:"ATCAirway,omitempty"`
	DepartureFP        string   `xml:"DepartureFP,omitempty"`
	ArrivalFP          string   `xml:"ArrivalFP,omitempty"`
	ApproachTypeFP     string   `xml:"ApproachTypeFP,omitempty"`
	SuffixFP           string   `xml:"SuffixFP,omitempty"`
	RunwayNumberFP     string   `xml:"RunwayNumberFP,omitempty"`
	RunwayDesignatorFP string   `xml:"RunwayDesignatorFP,omitempty"`
	ICAO               *plnICAO `xml:"ICAO,omitempty"`
}

type plnICAO struct {
	Region string `xml:"ICAORegion,omitempty"`
	Ident  string `xml:"ICAOIdent"`
}

// PLN writes the plan as an MSFS flight plan file (.pln): the departure
// airport, the charted fixes of the SID (DepartureFP, RunwayNumberFP), the
// enroute fixes (ATCAirway when reached by an airway), the STAR
// (ArrivalFP) and approach (ApproachTypeFP, RunwayNumberFP), and the
// destination airport. Runway thresholds, computed points and TOC/TOD are
// left out: the simulator rebuilds the procedures from their names.
// Positions carry the planned altitudes.
func (fp *FlightPlan) PLN() ([]byte, error) {
	if len(fp.Waypoints) == 0 {
		return nil, fmt.Errorf("nav: empty flight plan")
	}
	dep, arr := fp.Request.Departure, fp.Request.Arrival
	depPos, depElev := dep.reference()
	arrPos, arrElev := arr.reference()
	depID, arrID := strings.ToUpper(dep.ICAO), strings.ToUpper(arr.ICAO)
	routeType := "LowAlt"
	if fp.CruiseFL >= 180 {
		routeType = "HighAlt"
	}
	doc := plnDocument{Type: "AceXML", Version: "1,1", Descr: "AceXML Document", FlightPlan: plnFlightPlan{
		Title:             depID + " to " + arrID,
		FPType:            "IFR",
		RouteType:         routeType,
		CruisingAlt:       strconv.Itoa(fp.CruiseFL * 100),
		DepartureID:       depID,
		DepartureLLA:      FormatLLA(depPos, depElev*feetPerMeter),
		DestinationID:     arrID,
		DestinationLLA:    FormatLLA(arrPos, arrElev*feetPerMeter),
		Descr:             depID + ", " + arrID,
		DeparturePosition: fp.DepartureRunway,
		DepartureName:     dep.DisplayName(),
		DestinationName:   arr.DisplayName(),
		AppVersion:        plnAppVersion{Major: plnAppVersionMajor, Build: plnAppVersionBuild},
	}}
	airportWP := func(id string, p airport.LatLon, elevM float64) plnWaypoint {
		return plnWaypoint{ID: id, Type: "Airport", WorldPosition: FormatLLA(p, elevM*feetPerMeter), ICAO: &plnICAO{Ident: id}}
	}
	wps := []plnWaypoint{airportWP(depID, depPos, depElev)}
	for _, w := range fp.Waypoints {
		if !w.Fix() {
			continue
		}
		p := plnWaypoint{ID: w.Ident, Type: plnWaypointType(w.Kind), WorldPosition: FormatLLA(w.Position, w.AltFt),
			ICAO: &plnICAO{Region: w.Region, Ident: w.Ident}}
		if w.SpeedMaxKts > 0 {
			p.SpeedMaxFP = strconv.Itoa(int(w.SpeedMaxKts))
		}
		switch w.Phase {
		case PhaseSID:
			p.DepartureFP = fp.SID
			p.RunwayNumberFP, p.RunwayDesignatorFP = plnRunway(fp.DepartureRunway)
		case PhaseEnroute:
			if w.Airway != Direct {
				p.ATCAirway = w.Airway
			}
		case PhaseSTAR:
			if w.Airway != fp.STAR && w.Airway != Direct {
				p.ATCAirway = w.Airway // the STAR's entry, reached by an airway
			}
			p.ArrivalFP = fp.STAR
		case PhaseApproach:
			if fp.ApproachType != "" {
				p.ApproachTypeFP, p.SuffixFP = fp.ApproachType, fp.ApproachSuffix
				p.RunwayNumberFP, p.RunwayDesignatorFP = plnRunway(fp.ArrivalRunway)
			}
		}
		wps = append(wps, p)
	}
	doc.FlightPlan.Waypoints = append(wps, airportWP(arrID, arrPos, arrElev))
	b, err := xml.MarshalIndent(doc, "", "    ")
	if err != nil {
		return nil, err
	}
	// encoding/xml escapes every ' and "; MSFS writes the coordinates' as
	// they are, legal in element text (attributes hold identifiers only).
	b = bytes.ReplaceAll(bytes.ReplaceAll(b, []byte("&#39;"), []byte("'")), []byte("&#34;"), []byte(`"`))
	return append([]byte(xml.Header), append(b, '\n')...), nil
}

// FormatLLA writes a position and altitude (feet) as a .pln file does:
// N50° 6' 3.13",E14° 15' 36.23",+001247.00.
func FormatLLA(p airport.LatLon, altFt float64) string {
	return dms(p.Lat, "N", "S") + "," + dms(p.Lon, "E", "W") + "," + fmt.Sprintf("%+010.2f", altFt)
}

// dms writes degrees as D° M' S.SS" with a hemisphere letter.
func dms(v float64, pos, neg string) string {
	h := pos
	if v < 0 {
		h, v = neg, -v
	}
	c := int64(math.Round(v * 360000)) // hundredths of a second
	return fmt.Sprintf("%s%d° %d' %d.%02d\"", h, c/360000, c%360000/6000, c%6000/100, c%100)
}

// plnWaypointType maps a waypoint kind to ATCWaypointType.
func plnWaypointType(kind string) string {
	switch kind {
	case PointAirport:
		return "Airport"
	case string(KindWaypoint):
		return "Intersection"
	case string(KindVOR):
		return "VOR"
	case string(KindNDB):
		return "NDB"
	}
	return "User"
}

// plnRunway splits a runway end ("06", "24L") into RunwayNumberFP ("6")
// and RunwayDesignatorFP ("LEFT"; "" for none).
func plnRunway(name string) (number, designator string) {
	name = normalizeEnd(name)
	i := 0
	for i < len(name) && isDigit(name[i]) {
		i++
	}
	if i == 0 {
		return "", ""
	}
	n, _ := strconv.Atoi(name[:i])
	switch name[i:] {
	case "L":
		designator = "LEFT"
	case "R":
		designator = "RIGHT"
	case "C":
		designator = "CENTER"
	}
	return strconv.Itoa(n), designator
}
