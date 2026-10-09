package airport

import (
	"encoding/json"
	"math"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// GeoJSON types (RFC 7946). Coordinates are [longitude, latitude].
type (
	// FeatureCollection is a GeoJSON FeatureCollection.
	FeatureCollection struct {
		Type     string    `json:"type"`
		Features []Feature `json:"features"`
	}
	// Feature is a GeoJSON Feature.
	Feature struct {
		Type       string         `json:"type"`
		Geometry   Geometry       `json:"geometry"`
		Properties map[string]any `json:"properties"`
	}
	// Geometry is a GeoJSON Point, LineString or Polygon.
	Geometry struct {
		Type        string `json:"type"`
		Coordinates any    `json:"coordinates"`
	}
)

func coord(p LatLon) [2]float64 { return [2]float64{p.Lon, p.Lat} }

func point(p LatLon, props map[string]any) Feature {
	return Feature{Type: "Feature", Geometry: Geometry{Type: "Point", Coordinates: coord(p)}, Properties: props}
}

func line(pts []LatLon, props map[string]any) Feature {
	c := make([][2]float64, len(pts))
	for i, p := range pts {
		c[i] = coord(p)
	}
	return Feature{Type: "Feature", Geometry: Geometry{Type: "LineString", Coordinates: c}, Properties: props}
}

// FeatureCollection returns the layout as GeoJSON features, each with a
// "kind" property: "runway" (Polygon), "taxiPath" (LineString), "parking" and
// "taxiPoint" (Point). Raw facility fields are kept in the properties. Paths
// whose endpoints do not resolve are left out.
func (l *Layout) FeatureCollection() FeatureCollection {
	fc := FeatureCollection{Type: "FeatureCollection", Features: []Feature{}}
	for _, r := range l.Runways {
		fc.Features = append(fc.Features, Feature{
			Type:     "Feature",
			Geometry: Geometry{Type: "Polygon", Coordinates: [][][2]float64{runwayOutline(r)}},
			Properties: map[string]any{
				"kind": "runway", "index": r.Index, "name": r.Name(), "heading": r.Heading,
				"length": r.Length, "width": r.Width, "primary": r.Primary.Name, "secondary": r.Secondary.Name,
			},
		})
	}
	for _, p := range l.TaxiPaths {
		a, b, ok := l.PathEndpoints(p)
		if !ok {
			continue
		}
		fc.Features = append(fc.Features, line([]LatLon{a, b}, map[string]any{
			"kind": "taxiPath", "index": p.Index, "type": p.Type, "name": l.PathName(p), "width": p.Width,
			"start": p.Start, "end": p.End, "endsAtParking": p.EndsAtParking(),
		}))
	}
	// The taxiway gaps the taxi graph joins (TaxiwayBridges), as taxi paths
	// of their taxiway marked "bridge".
	for _, br := range l.TaxiwayBridges() {
		fc.Features = append(fc.Features, line([]LatLon{br.From, br.To}, map[string]any{
			"kind": "taxiPath", "index": -1, "type": 1, "name": br.Name, "start": br.FromPoint, "end": br.ToPoint, "bridge": true,
		}))
	}
	for _, p := range l.Parking {
		fc.Features = append(fc.Features, point(p.Position, map[string]any{
			"kind": "parking", "index": p.Index, "label": p.Label(), "name": p.Name, "suffix": p.Suffix,
			"number": p.Number, "type": p.Type, "heading": p.Heading, "radius": p.Radius, "gate": p.IsGate(),
		}))
	}
	for _, t := range l.TaxiPoints {
		fc.Features = append(fc.Features, point(t.Position, map[string]any{
			"kind": "taxiPoint", "index": t.Index, "type": t.Type, "orientation": t.Orientation, "holdShort": t.IsHoldShort(),
		}))
	}
	return fc
}

// GeoJSON returns the layout as a GeoJSON FeatureCollection.
func (l *Layout) GeoJSON() ([]byte, error) { return json.Marshal(l.FeatureCollection()) }

// Feature returns the route as a GeoJSON LineString feature.
func (r *Route) Feature() Feature {
	return line(r.Points, map[string]any{
		"kind": "route", "length": r.Length, "taxiways": r.Taxiways, "runwayCrossings": r.RunwayCrossings,
		"runway": r.Runway, "runwayEnd": r.RunwayEnd,
	})
}

// runwayOutline returns the closed ring of a runway rectangle.
func runwayOutline(r Runway) [][2]float64 {
	hw := r.Width / 2
	corner := func(end RunwayEnd, side float64) [2]float64 {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, math.Mod(r.Heading+90*side+360, 360), hw)
		return [2]float64{lon, lat}
	}
	c0, c1 := corner(r.Primary, -1), corner(r.Primary, 1)
	c2, c3 := corner(r.Secondary, 1), corner(r.Secondary, -1)
	return [][2]float64{c0, c1, c2, c3, c0}
}
