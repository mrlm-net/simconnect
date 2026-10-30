package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Spots (camera.Spot) at any airport, from its data: where a spotter stands
// by each runway end and apron. DataDir/spots.json changes them by airport,
// local winning per value: {"LKPR": {"touchdown:24": {"up": 12}, "mine":
// {"kind": "custom", "lat": 50.1, "lon": 14.2, "up": 4}}}.
//
// Heights are above the runway's elevation (the airport's, off a runway):
// SimConnect has no call for the ground at a point, so the ground under an
// approach spot is not known. Where it falls or rises, the table sets it
// ("up", or "altM" above sea level).

// CameraSpot is one spot: on a runway (Runway, Along meters from its threshold,
// Right of its centre line) or at Lat, Lon; Up above the ground or AltM above
// the sea. The rail's other end, if any, likewise.
type CameraSpot struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"` // threshold, touchdown, approach, holding, apron, custom
	Runway string  `json:"runway,omitempty"`
	Along  float64 `json:"along,omitempty"`
	Right  float64 `json:"right,omitempty"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Up     float64 `json:"up,omitempty"`
	AltM   float64 `json:"altM"`
	// The rail: RailAlong and RailRight on Runway, or RailLat and RailLon.
	RailAlong float64 `json:"railAlong,omitempty"`
	RailRight float64 `json:"railRight,omitempty"`
	RailLat   float64 `json:"railLat,omitempty"`
	RailLon   float64 `json:"railLon,omitempty"`
	RailSpeed float64 `json:"railSpeed,omitempty"`
	// The lens, and the share of the frame the aircraft fills (0: the
	// camera package's defaults).
	FovMin float64 `json:"fovMin,omitempty"`
	FovMax float64 `json:"fovMax,omitempty"`
	Share  float64 `json:"share,omitempty"`
	// Off leaves a generated spot out.
	Off bool `json:"off,omitempty"`
}

// The spots' geometry, meters.
const (
	// spotRunwaySides: how far from the centre line, first that is clear.
	spotThresholdUp = 6.0
	spotTouchdownUp = 3.0
	spotApproachUp  = 5.0
	spotHoldingUp   = 6.0
	// spotApronUp clears a terminal's roof (clearAboveM).
	spotApronUp = clearAboveM + 2

	spotTouchdownFrom = 150.0
	spotTouchdownTo   = 650.0
	spotApproachAlong = -500.0
	spotHoldingAway   = 70.0
	spotApronBack     = 150.0
	// spotStandClear, spotTaxiClear: a low camera keeps this far from a stand
	// (the buildings by it) and from a taxiway (what taxies by).
	spotStandClear = 80.0
	spotTaxiClear  = 30.0
	// spotApronGrid groups the stands of an apron (degrees of latitude).
	spotApronGrid = 0.004
	spotApronMin  = 3
)

// spotSides are the distances from the centre line a runway spot tries.
var spotSides = []float64{120, 90, 160, 200}

// spotClear reports whether a low camera can stand at p: off the runways,
// away from the stands and the taxiways.
func spotClear(g *airport.Graph, p airport.LatLon) bool {
	if g.RunwayAt(p, 30) >= 0 {
		return false
	}
	if _, near := g.NearestNode(p, spotTaxiClear); near {
		return false
	}
	for _, s := range g.Layout.Parking {
		if calc.HaversineMeters(p.Lat, p.Lon, s.Position.Lat, s.Position.Lon) < spotStandClear {
			return false
		}
	}
	return true
}

// runwayPoint is along and right of runway end.
func runwayPoint(end airport.RunwayEnd, along, right float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, along)
	lat, lon = calc.DisplaceByHeading(lat, lon, math.Mod(end.Heading+90, 360), right)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// runwaySideClear is the first side of end, along it, that is clear.
func runwaySideClear(g *airport.Graph, end airport.RunwayEnd, along float64, sides []float64) (float64, bool) {
	for _, d := range sides {
		for _, r := range []float64{d, -d} {
			if spotClear(g, runwayPoint(end, along, r)) {
				return r, true
			}
		}
	}
	return 0, false
}

// generatedSpots are g's airport's spots, without the local table.
func generatedSpots(g *airport.Graph) []CameraSpot {
	var out []CameraSpot
	l := g.Layout
	for _, r := range l.Runways {
		for _, end := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			if right, ok := runwaySideClear(g, end, 0, spotSides); ok {
				out = append(out, CameraSpot{Name: "threshold:" + end.Name, Kind: "threshold", Runway: end.Name, Right: right, Up: spotThresholdUp})
			}
			if right, ok := runwaySideClear(g, end, spotTouchdownFrom, spotSides); ok && spotClear(g, runwayPoint(end, spotTouchdownTo, right)) {
				out = append(out, CameraSpot{Name: "touchdown:" + end.Name, Kind: "touchdown", Runway: end.Name, Along: spotTouchdownFrom, Right: right, Up: spotTouchdownUp,
					RailAlong: spotTouchdownTo, RailRight: right})
			}
			if right, ok := runwaySideClear(g, end, spotApproachAlong, []float64{60, 120}); ok {
				out = append(out, CameraSpot{Name: "approach:" + end.Name, Kind: "approach", Runway: end.Name, Along: spotApproachAlong, Right: right, Up: spotApproachUp})
			}
			if p, ok := holdingSpot(g, end); ok {
				out = append(out, CameraSpot{Name: "holding:" + end.Name, Kind: "holding", Runway: end.Name, Lat: p.Lat, Lon: p.Lon, Up: spotHoldingUp})
			}
		}
	}
	return append(out, apronSpots(g)...)
}

// holdingSpot is beside end's full-length holding point, clear of it.
func holdingSpot(g *airport.Graph, end airport.RunwayEnd) (airport.LatLon, bool) {
	entries, err := g.RunwayEntries(end.Name)
	if err != nil {
		return airport.LatLon{}, false
	}
	for _, e := range entries {
		if e.HoldShort < 0 {
			continue
		}
		h := g.Nodes[e.HoldShort].Position
		for _, turn := range []float64{-90, 90, 180, -135, 135, -45, 45, 0} {
			lat, lon := calc.DisplaceByHeading(h.Lat, h.Lon, math.Mod(end.Heading+turn+360, 360), spotHoldingAway)
			if p := (airport.LatLon{Lat: lat, Lon: lon}); spotClear(g, p) {
				return p, true
			}
		}
		return airport.LatLon{}, false
	}
	return airport.LatLon{}, false
}

// apronSpots look over each group of stands from the pavement before them,
// above the roofs. A stand's heading is the way its aircraft points: the
// pavement is behind it.
func apronSpots(g *airport.Graph) []CameraSpot {
	gates := map[[2]int][]airport.Parking{}
	any := false
	for _, s := range g.Layout.Parking {
		any = any || s.IsGate()
	}
	for _, s := range g.Layout.Parking {
		if any && !s.IsGate() {
			continue
		}
		k := [2]int{int(math.Floor(s.Position.Lat / spotApronGrid)), int(math.Floor(s.Position.Lon / (spotApronGrid / math.Cos(s.Position.Lat*math.Pi/180))))}
		gates[k] = append(gates[k], s)
	}
	var out []CameraSpot
	for _, list := range gates {
		if len(list) < spotApronMin {
			continue
		}
		var lat, lon, x, y float64
		first := list[0].Label()
		for _, s := range list {
			lat += s.Position.Lat / float64(len(list))
			lon += s.Position.Lon / float64(len(list))
			x += math.Sin(s.Heading * math.Pi / 180)
			y += math.Cos(s.Heading * math.Pi / 180)
			if s.Label() < first {
				first = s.Label()
			}
		}
		back := math.Mod(math.Atan2(x, y)*180/math.Pi+540, 360)
		elat, elon := calc.DisplaceByHeading(lat, lon, back, spotApronBack)
		p := airport.LatLon{Lat: elat, Lon: elon}
		if _, over := g.NearestNode(p, 2*clearNodeM); !over {
			continue // not over the pavement
		}
		out = append(out, CameraSpot{Name: "apron:" + first, Kind: "apron", Lat: elat, Lon: elon, Up: spotApronUp})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// spotTable is DataDir/spots.json: by airport, by spot, the values to change.
func (cc *controlCenter) spotTable(icao string) map[string]json.RawMessage {
	b, err := os.ReadFile(filepath.Join(cc.core.hooks.DataDir, "spots.json"))
	if err != nil {
		return nil
	}
	var all map[string]map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		tlog.printf("spots.json: %v", err)
		return nil
	}
	return all[strings.ToUpper(icao)]
}

// spots are g's airport's spots, the local table applied, each placed.
func (cc *controlCenter) spots(g *airport.Graph) []CameraSpot {
	list := generatedSpots(g)
	table := cc.spotTable(g.Layout.ICAO)
	for i := range list {
		if raw, ok := table[list[i].Name]; ok {
			name := list[i].Name
			if err := json.Unmarshal(raw, &list[i]); err != nil {
				tlog.printf("spots.json: %s: %v", name, err)
			}
			list[i].Name = name
			delete(table, name)
		}
	}
	names := make([]string, 0, len(table))
	for k := range table {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		s := CameraSpot{Kind: "custom"}
		if err := json.Unmarshal(table[k], &s); err != nil {
			tlog.printf("spots.json: %s: %v", k, err)
			continue
		}
		s.Name = k
		list = append(list, s)
	}
	out := list[:0]
	for _, s := range list {
		if !s.Off && s.place(g.Layout) {
			out = append(out, s)
		}
	}
	return out
}

// place works out Lat, Lon and AltM (false: a runway it does not know, or
// nowhere). A Lat and Lon given win over Along and Right.
func (s *CameraSpot) place(l *airport.Layout) bool {
	ground := l.Altitude
	var end airport.RunwayEnd
	if s.Runway != "" {
		r, e, ok := l.RunwayEnd(s.Runway)
		if !ok {
			return false
		}
		end, ground = e, r.Altitude
		if s.Lat == 0 && s.Lon == 0 {
			p := runwayPoint(end, s.Along, s.Right)
			s.Lat, s.Lon = p.Lat, p.Lon
		}
		if s.RailLat == 0 && s.RailLon == 0 && (s.RailAlong != 0 || s.RailRight != 0) {
			p := runwayPoint(end, s.RailAlong, s.RailRight)
			s.RailLat, s.RailLon = p.Lat, p.Lon
		}
	}
	if s.AltM == 0 {
		s.AltM = ground + s.Up
	}
	return s.Lat != 0 || s.Lon != 0
}

// camera is s as the camera plays it.
func (s CameraSpot) camera() camera.Spot {
	out := camera.Spot{Name: s.Name, Eye: camera.At(s.Lat, s.Lon, s.AltM), RailSpeed: s.RailSpeed, FovMin: s.FovMin, FovMax: s.FovMax, Share: s.Share}
	if s.RailLat != 0 || s.RailLon != 0 {
		end := camera.At(s.RailLat, s.RailLon, s.AltM)
		out.RailEnd = &end
	}
	return out
}

// scanned is the scan as the camera reads it, on its goroutine: no lock.
type scanned struct {
	at   time.Time
	list []Traffic
}

// spotFeed is the samples of the aircraft with objectID (user: the player's)
// from the scan.
func (cc *controlCenter) spotFeed(objectID uint32, user bool) func() (camera.SpotSample, bool) {
	var seen *scanned
	var sample camera.SpotSample
	var have bool
	return func() (camera.SpotSample, bool) {
		sc := cc.scanNow.Load()
		if sc == nil || sc == seen {
			return sample, have
		}
		seen = sc
		for i := range sc.list {
			t := &sc.list[i]
			if user && !t.User || !user && t.ObjectID != objectID {
				continue
			}
			sample = camera.SpotSample{Pose: camera.ObjectPose{Lat: t.Latitude, Lon: t.Longitude, AltM: t.Alt * 0.3048, Heading: t.Heading}, At: sc.at}
			// AI on the ground report no speed; the vertical speed is left to
			// the samples (wrong sign for some AI on final).
			if t.GroundKts > 0 {
				mps := t.GroundKts * 0.514444
				sample.MpsEast = mps * math.Sin(t.Heading*math.Pi/180)
				sample.MpsNorth = mps * math.Cos(t.Heading*math.Pi/180)
			}
			have = true
			break
		}
		return sample, have
	}
}

// DefaultSpotSec is how long a spot shot runs, not told.
const DefaultSpotSec = 45.0

// spotOptions are a shot's changes to its spot.
type spotOptions struct {
	Share float64
	Lock  bool
}

// spotShot plays spot name on an aircraft, by the camera's id for it
// (user: the player's) and its largest dimension in meters.
func (cc *controlCenter) spotShot(g *airport.Graph, name string, object uint32, user bool, size, sec float64, o spotOptions) (camera.Shot, error) {
	for _, s := range cc.spots(g) {
		if s.Name != name {
			continue
		}
		spot := s.camera()
		if o.Share > 0 {
			spot.Share = o.Share
		}
		spot.LockAim = o.Lock
		if sec <= 0 {
			sec = DefaultSpotSec
		}
		id := object
		if user {
			id = 0
		}
		return spot.Track(id, size, time.Duration(sec*float64(time.Second)), cc.spotFeed(object, user), traffic.Now), nil
	}
	return nil, fmt.Errorf("no spot %q at %s", name, g.Layout.ICAO)
}

// SpotRequest plays a spot on any aircraft: ID its object id (-1 the
// player's) or Tail; Sec how long (0: DefaultSpotSec, at most
// camera.MaxStretch), Share and Lock as on a scene's spot shot.
type SpotRequest struct {
	ICAO  string  `json:"icao"`
	Spot  string  `json:"spot"`
	ID    int     `json:"id"`
	Tail  string  `json:"tail"`
	Sec   float64 `json:"sec"`
	Share float64 `json:"share"`
	Lock  bool    `json:"lock"`
}

// playSpot plays a spot (mode shot, as playShot).
func (m *cameraMan) playSpot(req SpotRequest) (string, error) {
	if m.dir == nil {
		return "", errors.New("no camera on this connection")
	}
	m.mu.Lock()
	scene := m.mode == "scene"
	m.mu.Unlock()
	if scene {
		return "", errors.New("a scene is playing")
	}
	g, err := m.cc.graph(strings.ToUpper(req.ICAO))
	if err != nil {
		return "", err
	}
	sc := m.cc.scanNow.Load()
	if sc == nil {
		return "", errors.New("no aircraft seen yet")
	}
	var found *Traffic
	for i := range sc.list {
		t := &sc.list[i]
		if req.Tail != "" && strings.EqualFold(t.Tail, req.Tail) || req.Tail == "" && (req.ID == -1 && t.User || req.ID > 0 && t.ObjectID == uint32(req.ID)) {
			found = t
			break
		}
	}
	if found == nil {
		return "", fmt.Errorf("no aircraft %q %d", req.Tail, req.ID)
	}
	size := found.Span * 1.05
	if size <= 0 {
		size = nominalSize.Length
	}
	shot, err := m.cc.spotShot(g, req.Spot, found.ObjectID, found.User, size, req.Sec, spotOptions{req.Share, req.Lock})
	if err != nil {
		return "", err
	}
	if err := m.setMode("shot", 0); err != nil {
		return "", err
	}
	m.playAlone(found.Tail, shot)
	return fmt.Sprintf("%s, %.0f s", shot.Name(), shot.Length().Seconds()), nil
}

// Spots are the spots at icao's airport, placed.
func (w *World) Spots(icao string) ([]CameraSpot, error) {
	var out []CameraSpot
	err := w.Get("/api/camera/spots?icao="+strings.ToUpper(icao), &out)
	return out, err
}

// PlaySpot plays a spot on an aircraft. Refused with no camera on this
// connection, with a scene playing, or without that spot or aircraft.
func (w *World) PlaySpot(req SpotRequest) error {
	_, err := w.Do("POST", "/api/camera/spot", req)
	return err
}
