package world

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Capturing a composition: the user frames a shot with the simulator's own
// camera (the drone), and the capture reads it (CameraGet answers without
// the camera taken) and says where it is: in the world, from the aircraft
// of ours it looks at, and on the airport (stand, taxiway, runway). A
// scene's shot is then built to match it.

// CompositionCapture is the simulator's camera as captured.
type CompositionCapture struct {
	Name string    `json:"name"`
	At   time.Time `json:"at"`
	// Camera: where it is (meters MSL), heading (degrees true), pitch
	// (degrees, positive looking down), field of view (degrees).
	Camera struct {
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
		AltM       float64 `json:"altM"`
		AboveGndM  float64 `json:"aboveGroundM"` // above the airport's elevation
		HeadingDeg float64 `json:"headingDeg"`
		PitchDeg   float64 `json:"pitchDeg"`
		FovDeg     float64 `json:"fovDeg"`
	} `json:"camera"`
	// Subject: the aircraft of ours filmed (asked for, or the one the
	// camera looks most nearly at within 2 km), and the camera from it.
	Subject *CaptureSubject `json:"subject,omitempty"`
	// Airport: the camera on the airport.
	Airport CaptureAirport `json:"airport"`
}

// CaptureSubject is the aircraft a capture films and the camera from it:
// Eye and Target are offsets from it (meters to its right, above the
// ground under it, ahead of it), as a scene shot's.
type CaptureSubject struct {
	Tail     string        `json:"tail"`
	Model    string        `json:"model"`
	State    string        `json:"state"`
	Stand    string        `json:"stand,omitempty"`
	OnGround bool          `json:"onGround"`
	Heading  float64       `json:"headingDeg"`
	Distance float64       `json:"distanceM"`
	Eye      camera.Offset `json:"eye"`
	Target   camera.Offset `json:"target"`
	// AimMissM: how far the camera's line of sight passes from its middle
	// (0 dead centre).
	AimMissM float64 `json:"aimMissM"`
	// Length: its length (m), the scale the moves are written in.
	Length float64 `json:"lengthM"`
}

// CaptureAirport is where a capture's camera is on the airport.
type CaptureAirport struct {
	ICAO         string  `json:"icao"`
	Stand        string  `json:"nearestStand,omitempty"`
	StandM       float64 `json:"nearestStandM,omitempty"`
	Taxiway      string  `json:"nearestTaxiway,omitempty"`
	TaxiwayM     float64 `json:"nearestTaxiwayM,omitempty"`
	RunwayEnd    string  `json:"runwayEnd,omitempty"`
	AlongRunwayM float64 `json:"alongRunwayM,omitempty"` // from the threshold, negative before it
	RightRunwayM float64 `json:"rightOfRunwayM,omitempty"`
}

// cameraGetter reads the simulator's camera (*engine.Engine).
type cameraGetter interface {
	CameraGet(referential types.SIMCONNECT_POSITION_REFERENTIAL) error
}

// capture reads the simulator's camera and places it.
func (m *cameraMan) capture(icao, name, tail string) (CompositionCapture, error) {
	var out CompositionCapture
	get, ok := m.sim.client.(cameraGetter)
	if !ok {
		return out, errors.New("no camera on this connection")
	}
	g, err := m.cc.graph(strings.ToUpper(icao))
	if err != nil {
		return out, err
	}
	for len(m.camData) > 0 {
		<-m.camData // an old answer
	}
	if err := m.cc.do(func() error { return get.CameraGet(types.SIMCONNECT_POSITION_REFERENTIAL_WORLD) }); err != nil {
		return out, err
	}
	var d types.SIMCONNECT_DATA_CAMERA
	select {
	case d = <-m.camData:
	case <-time.After(3 * time.Second):
		return out, errors.New("the simulator did not give its camera")
	}
	l := g.Layout
	out.Name, out.At = name, time.Now()
	c := &out.Camera
	c.Lat, c.Lon, c.AltM = d.Position.X, d.Position.Y, d.Position.Z
	c.AboveGndM = c.AltM - l.Altitude
	// The members are not in their names' order (live, 2026-10-03): Bank is
	// the heading, Pitch the pitch (down positive).
	c.HeadingDeg, c.PitchDeg, c.FovDeg = math.Mod(float64(d.Pbh.Bank)+360, 360), float64(d.Pbh.Pitch), d.Fov*180/math.Pi
	out.Airport = placeOnAirport(g, c.Lat, c.Lon)
	out.Subject = m.captureSubject(g, out, tail)
	return out, nil
}

// captureSubject is the aircraft the camera films, and the camera from it.
func (m *cameraMan) captureSubject(g *airport.Graph, cp CompositionCapture, tail string) *CaptureSubject {
	c := cp.Camera
	h, p := c.HeadingDeg*math.Pi/180, c.PitchDeg*math.Pi/180
	var best *CaptureSubject
	bestMiss := math.Inf(1)
	m.cc.mu.Lock()
	items := make([]*controlled, 0, len(m.cc.items))
	for _, it := range m.cc.items {
		items = append(items, it)
	}
	m.cc.mu.Unlock()
	for _, it := range items {
		if it.finished.Load() || tail != "" && !strings.EqualFold(it.Tail, tail) {
			continue
		}
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		if v.Position.Lat == 0 && v.Position.Lon == 0 {
			continue
		}
		dist := calc.HaversineMeters(v.Position.Lat, v.Position.Lon, c.Lat, c.Lon)
		if tail == "" && dist > 2000 {
			continue
		}
		rel := (calc.BearingDegrees(v.Position.Lat, v.Position.Lon, c.Lat, c.Lon) - v.Heading) * math.Pi / 180
		up := c.AboveGndM
		eye := camera.Offset{Right: dist * math.Sin(rel), Up: up, Forward: dist * math.Cos(rel)}
		// The line of sight in its frame, and the point on it nearest its
		// middle (2 m up).
		dh := h - v.Heading*math.Pi/180
		dir := camera.Offset{Right: math.Sin(dh) * math.Cos(p), Up: -math.Sin(p), Forward: math.Cos(dh) * math.Cos(p)}
		ex, ey, ez := -eye.Right, 2-eye.Up, -eye.Forward
		t := max(1, ex*dir.Right+ey*dir.Up+ez*dir.Forward)
		tgt := camera.Offset{Right: eye.Right + dir.Right*t, Up: eye.Up + dir.Up*t, Forward: eye.Forward + dir.Forward*t}
		miss := math.Sqrt(tgt.Right*tgt.Right + (tgt.Up-2)*(tgt.Up-2) + tgt.Forward*tgt.Forward)
		if miss >= bestMiss {
			continue
		}
		bestMiss = miss
		best = &CaptureSubject{Tail: it.Tail, Model: v.Model, State: v.State, Stand: v.Stand, OnGround: v.OnGround,
			Heading: v.Heading, Distance: math.Round(dist*10) / 10, Eye: roundOffset(eye), Target: roundOffset(tgt),
			AimMissM: math.Round(miss*10) / 10, Length: math.Round(lengthOf(it)*10) / 10}
	}
	return best
}

// placeOnAirport is a point on the airport: the stand and the taxiway
// nearest it, and where it is from the runway end it is nearest beside.
func placeOnAirport(g *airport.Graph, lat, lon float64) CaptureAirport {
	l := g.Layout
	out := CaptureAirport{ICAO: l.ICAO}
	out.StandM = math.Inf(1)
	for _, p := range l.Parking {
		if d := calc.HaversineMeters(lat, lon, p.Position.Lat, p.Position.Lon); d < out.StandM {
			out.Stand, out.StandM = p.Label(), math.Round(d)
		}
	}
	if math.IsInf(out.StandM, 1) {
		out.StandM = 0
	}
	out.TaxiwayM = math.Inf(1)
	for id, es := range g.Adj {
		for _, e := range es {
			if e.Name == "" {
				continue
			}
			n := g.Nodes[id].Position
			if d := calc.HaversineMeters(lat, lon, n.Lat, n.Lon); d < out.TaxiwayM {
				out.Taxiway, out.TaxiwayM = e.Name, math.Round(d)
			}
		}
	}
	if math.IsInf(out.TaxiwayM, 1) {
		out.TaxiwayM = 0
	}
	best := math.Inf(1)
	for _, r := range l.Runways {
		for _, end := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			d := calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, lat, lon)
			rel := (calc.BearingDegrees(end.Threshold.Lat, end.Threshold.Lon, lat, lon) - end.Heading) * math.Pi / 180
			along, right := d*math.Cos(rel), d*math.Sin(rel)
			// Beside the runway in its length first, then nearest.
			score := math.Abs(right)
			if along < 0 || along > r.Length {
				score += math.Min(math.Abs(along), math.Abs(along-r.Length))
			}
			if along < 0 {
				score += 1 // the end it is before is the other's far one
			}
			if score < best {
				best = score
				out.RunwayEnd, out.AlongRunwayM, out.RightRunwayM = end.Name, math.Round(along), math.Round(right)
			}
		}
	}
	return out
}

func roundOffset(o camera.Offset) camera.Offset {
	r := func(x float64) float64 { return math.Round(x*10) / 10 }
	return camera.Offset{Right: r(o.Right), Up: r(o.Up), Forward: r(o.Forward)}
}
