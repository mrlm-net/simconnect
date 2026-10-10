package traffic

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// standardPlanVersion changes when the standard push planning does: plans
// saved by another version are planned again.
const standardPlanVersion = 1

// standardFile is the saved standard pushes of one airport.
type standardFile struct {
	Version     int             `json:"version"`
	ICAO        string          `json:"icao"`
	Fingerprint string          `json:"fingerprint"`
	Stands      []standardStand `json:"stands"`
}

// standardStand is one stand's standard push: the edge its pose is on and
// the way it faces (all samePose compares), or none.
type standardStand struct {
	Parking int            `json:"parking"`
	None    bool           `json:"none,omitempty"`
	From    airport.NodeID `json:"from,omitempty"`
	To      airport.NodeID `json:"to,omitempty"`
	Heading float64        `json:"heading,omitempty"`
}

// ErrStandardStale is returned by LoadStandardPushes for a file saved for
// another layout of the airport, or by another version of the planning.
var ErrStandardStale = errors.New("traffic: standard pushes saved for another layout")

// layoutFingerprint identifies g's ground layout: its stands and taxiways.
func layoutFingerprint(g *airport.Graph) string {
	h := fnv.New64a()
	l := g.Layout
	fmt.Fprintf(h, "%s %d %d %d|", l.ICAO, len(l.Parking), len(l.TaxiPoints), len(l.TaxiPaths))
	for _, p := range l.Parking {
		fmt.Fprintf(h, "%.6f,%.6f,%.0f;", p.Position.Lat, p.Position.Lon, p.Heading)
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// SaveStandardPushes writes the standard pushes planned so far at g's
// airport (PlanStandardPushes), for LoadStandardPushes on a later run: an
// airport is then planned once, not on every start.
func SaveStandardPushes(w io.Writer, g *airport.Graph) (int, error) {
	f := standardFile{Version: standardPlanVersion, ICAO: g.Layout.ICAO, Fingerprint: layoutFingerprint(g)}
	for i := range g.Layout.Parking {
		v, ok := standardPushes.Load(stdKey(g, i))
		if !ok {
			continue
		}
		s := standardStand{Parking: i, None: true}
		if p := v.(*pushPose); p != nil {
			s = standardStand{Parking: i, From: p.from, To: p.to, Heading: math.Round(p.heading*10) / 10}
		}
		f.Stands = append(f.Stands, s)
	}
	return len(f.Stands), json.NewEncoder(w).Encode(f)
}

// LoadStandardPushes reads standard pushes SaveStandardPushes wrote for g's
// airport; PlanStandardPushes then skips those stands. ErrStandardStale:
// saved for another layout or planning version (nothing is loaded).
func LoadStandardPushes(r io.Reader, g *airport.Graph) (int, error) {
	var f standardFile
	if err := json.NewDecoder(r).Decode(&f); err != nil {
		return 0, err
	}
	if f.Version != standardPlanVersion || f.ICAO != g.Layout.ICAO || f.Fingerprint != layoutFingerprint(g) {
		return 0, ErrStandardStale
	}
	n := 0
	for _, s := range f.Stands {
		if s.Parking < 0 || s.Parking >= len(g.Layout.Parking) {
			continue
		}
		var p *pushPose
		if !s.None {
			p = &pushPose{from: s.From, to: s.To, heading: s.Heading}
		}
		standardPushes.Store(stdKey(g, s.Parking), p)
		n++
	}
	return n, nil
}
