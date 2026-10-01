//go:build windows
// +build windows

package airport

import (
	"errors"
	"math"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ErrNoData is returned when facility data contains no airport, e.g. for an
// unknown ICAO code.
var ErrNoData = errors.New("airport: no facility data")

// RawAirport holds undecoded facility records for one airport, in SimConnect
// list order. Its JSON form matches the dump written by cmd/airport-map.
type RawAirport struct {
	ICAO       string         `json:"icao"`
	Name       string         `json:"name"`
	Latitude   float64        `json:"lat"`
	Longitude  float64        `json:"lon"`
	Altitude   float64        `json:"alt"`
	Runways    []RawRunway    `json:"runways"`
	Parking    []RawParking   `json:"parking"`
	TaxiPoints []RawTaxiPoint `json:"taxiPoints"`
	TaxiPaths  []RawTaxiPath  `json:"taxiPaths"`
	TaxiNames  []string       `json:"taxiNames"`
	// ParkingAirlines are the airline codes assigned to parking spots, by
	// parking index (TAXI_PARKING_AIRLINE child records).
	ParkingAirlines map[int][]string `json:"parkingAirlines,omitempty"`
	// Frequencies are the FREQUENCY records (#416).
	Frequencies []RawFrequency `json:"frequencies,omitempty"`
	// Tower is the airport's tower, nil when it has none (or the data was
	// captured without it).
	Tower *RawTower `json:"tower,omitempty"`
}

// RawFrequency is a FREQUENCY record: TYPE
// (SIMCONNECT_FACILITY_FREQUENCY_TYPE), FREQUENCY in Hz, NAME.
// RawTower is the AIRPORT record's tower (TOWER_LATITUDE, _LONGITUDE,
// _ALTITUDE).
type RawTower struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
	Altitude  float64 `json:"alt"`
}

type RawFrequency struct {
	Type int32  `json:"type"`
	Hz   int32  `json:"hz"`
	Name string `json:"name"`
}

// RawRunway is a RUNWAY record: LATITUDE, LONGITUDE, ALTITUDE, HEADING,
// LENGTH, WIDTH, PRIMARY_NUMBER, PRIMARY_DESIGNATOR, SECONDARY_NUMBER,
// SECONDARY_DESIGNATOR.
type RawRunway struct {
	Latitude            float64 `json:"lat"`
	Longitude           float64 `json:"lon"`
	Altitude            float64 `json:"alt"`
	Heading             float32 `json:"heading"`
	Length              float32 `json:"length"`
	Width               float32 `json:"width"`
	PrimaryNumber       int32   `json:"primaryNumber"`
	PrimaryDesignator   int32   `json:"primaryDesignator"`
	SecondaryNumber     int32   `json:"secondaryNumber"`
	SecondaryDesignator int32   `json:"secondaryDesignator"`
}

// RawParking is a TAXI_PARKING record: NAME, SUFFIX, NUMBER, TYPE, HEADING,
// RADIUS, BIAS_X, BIAS_Z. All fields are 4 bytes, so the struct matches the
// wire layout.
type RawParking struct {
	Name    int32   `json:"name"`
	Suffix  int32   `json:"suffix"`
	Number  uint32  `json:"number"`
	Type    int32   `json:"type"`
	Heading float32 `json:"heading"`
	Radius  float32 `json:"radius"`
	BiasX   float32 `json:"biasX"`
	BiasZ   float32 `json:"biasZ"`
}

// RawTaxiPoint is a TAXI_POINT record: TYPE, ORIENTATION, BIAS_X, BIAS_Z.
type RawTaxiPoint struct {
	Type        int32   `json:"type"`
	Orientation int32   `json:"orientation"`
	BiasX       float32 `json:"biasX"`
	BiasZ       float32 `json:"biasZ"`
}

// RawTaxiPath is a TAXI_PATH record: TYPE, WIDTH, RUNWAY_NUMBER,
// RUNWAY_DESIGNATOR, START, END, NAME_INDEX.
type RawTaxiPath struct {
	Type             int32   `json:"type"`
	Width            float32 `json:"width"`
	RunwayNumber     int32   `json:"runwayNumber"`
	RunwayDesignator int32   `json:"runwayDesignator"`
	Start            int32   `json:"start"`
	End              int32   `json:"end"`
	NameIndex        uint32  `json:"nameIndex"`
}

// BuildLayout decodes raw facility records into a Layout, resolving BIAS_X and
// BIAS_Z offsets to positions and computing runway thresholds. It returns
// ErrNoData when raw holds no airport.
func BuildLayout(raw RawAirport) (*Layout, error) {
	if raw.Latitude == 0 && raw.Longitude == 0 && len(raw.Runways) == 0 && len(raw.TaxiPoints) == 0 {
		return nil, ErrNoData
	}
	l := &Layout{
		ICAO:        raw.ICAO,
		Name:        raw.Name,
		Latitude:    raw.Latitude,
		Longitude:   raw.Longitude,
		Altitude:    raw.Altitude,
		TaxiNames:   append([]string(nil), raw.TaxiNames...),
		Frequencies: frequenciesOf(raw.Frequencies),
	}
	if t := raw.Tower; t != nil {
		l.Tower, l.TowerAltitude, l.HasTower = LatLon{Lat: t.Latitude, Lon: t.Longitude}, t.Altitude, true
	}
	offset := func(x, z float32) LatLon {
		lat, lon := convert.OffsetToLatLon(raw.Latitude, raw.Longitude, float64(x), float64(z))
		return LatLon{Lat: lat, Lon: lon}
	}

	for i, r := range raw.Runways {
		l.Runways = append(l.Runways, buildRunway(i, r))
	}
	for i, p := range raw.Parking {
		l.Parking = append(l.Parking, Parking{
			Index:    i,
			Name:     types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME(p.Name),
			Suffix:   types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME(p.Suffix),
			Number:   p.Number,
			Type:     types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE(p.Type),
			Heading:  float64(p.Heading),
			Radius:   float64(p.Radius),
			BiasX:    float64(p.BiasX),
			BiasZ:    float64(p.BiasZ),
			Position: offset(p.BiasX, p.BiasZ),
			Airlines: raw.ParkingAirlines[i],
		})
	}
	for i, t := range raw.TaxiPoints {
		l.TaxiPoints = append(l.TaxiPoints, TaxiPoint{
			Index:       i,
			Type:        types.SIMCONNECT_FACILITY_TAXI_POINT_TYPE(t.Type),
			Orientation: t.Orientation,
			BiasX:       float64(t.BiasX),
			BiasZ:       float64(t.BiasZ),
			Position:    offset(t.BiasX, t.BiasZ),
		})
	}
	for i, p := range raw.TaxiPaths {
		// Only runway paths carry a runway; MSFS leaves the fields
		// uninitialised on the others.
		if types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE(p.Type) != types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY {
			p.RunwayNumber, p.RunwayDesignator = 0, 0
		}
		l.TaxiPaths = append(l.TaxiPaths, TaxiPath{
			Index:            i,
			Type:             types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE(p.Type),
			Width:            float64(p.Width),
			RunwayNumber:     p.RunwayNumber,
			RunwayDesignator: types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR(p.RunwayDesignator),
			Start:            p.Start,
			End:              p.End,
			NameIndex:        p.NameIndex,
		})
	}
	return l, nil
}

// buildRunway computes both runway ends. HEADING is the primary direction, so
// the primary threshold lies half a length behind the centre and the secondary
// threshold half a length ahead of it.
func buildRunway(i int, r RawRunway) Runway {
	hdg := float64(r.Heading)
	half := float64(r.Length) / 2
	pLat, pLon := calc.DisplaceByHeading(r.Latitude, r.Longitude, math.Mod(hdg+180, 360), half)
	sLat, sLon := calc.DisplaceByHeading(r.Latitude, r.Longitude, hdg, half)
	pDes := types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR(r.PrimaryDesignator)
	sDes := types.SIMCONNECT_FACILITY_RUNWAY_DESIGNATOR(r.SecondaryDesignator)
	return Runway{
		Index:    i,
		Center:   LatLon{Lat: r.Latitude, Lon: r.Longitude},
		Altitude: r.Altitude,
		Heading:  hdg,
		Length:   float64(r.Length),
		Width:    float64(r.Width),
		Primary: RunwayEnd{
			Number: r.PrimaryNumber, Designator: pDes, Name: runwayEndName(r.PrimaryNumber, pDes),
			Heading: hdg, Threshold: LatLon{Lat: pLat, Lon: pLon},
		},
		Secondary: RunwayEnd{
			Number: r.SecondaryNumber, Designator: sDes, Name: runwayEndName(r.SecondaryNumber, sDes),
			Heading: math.Mod(hdg+180, 360), Threshold: LatLon{Lat: sLat, Lon: sLon},
		},
	}
}
