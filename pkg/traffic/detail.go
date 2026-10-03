//go:build windows
// +build windows

package traffic

import (
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Level of detail (#370): an injected aircraft is driven — its monitor
// answers, the mover steps, the aircraft is placed — on every sim frame
// near the viewer and on the runway, on fewer frames when taxiing farther
// away, and a couple of times a second while it stands still. Share one
// Detail between the controllers (TaxiWithDetail, ArrivalWithDetail) and
// keep its viewer current (SetViewer, e.g. the user aircraft).
type Detail struct {
	// NearMeters: closer to the viewer, every frame; MidMeters: closer,
	// every MidInterval+1 frames; farther, every FarInterval+1 frames.
	NearMeters, MidMeters    float64
	MidInterval, FarInterval uint32
	// StillInterval: standing still, every StillInterval+1 frames.
	StillInterval uint32
	// SlowerAfter: an aircraft goes to fewer frames only once it asked for
	// them this long (it goes back to more at once).
	SlowerAfter time.Duration

	mu        sync.Mutex
	viewer    airport.LatLon
	hasViewer bool
	intervals map[uint32]uint32 // by object: what each aircraft runs at
}

// Level of detail defaults: every frame within 8 km (the whole airport and
// its short finals: at 3 km the far end of LKPR's 24 ran at half rate and
// stuttered from the tower), every 2nd within 20 km, every 4th beyond
// (15 Hz at 60 fps), every 30th standing still.
const (
	DefaultDetailNearMeters     = 8000
	DefaultDetailMidMeters      = 20000
	DefaultDetailMidInterval    = 1
	DefaultDetailFarInterval    = 3
	DefaultDetailStillInterval  = 29
	DefaultDetailSlowerAfter    = 2 * time.Second
	detailAssumedFramesPerSecon = 60
)

// NewDetail creates a Detail with the defaults.
func NewDetail() *Detail {
	return &Detail{NearMeters: DefaultDetailNearMeters, MidMeters: DefaultDetailMidMeters,
		MidInterval: DefaultDetailMidInterval, FarInterval: DefaultDetailFarInterval,
		StillInterval: DefaultDetailStillInterval, SlowerAfter: DefaultDetailSlowerAfter,
		intervals: map[uint32]uint32{}}
}

// SetViewer sets where the detail is highest (the user aircraft).
func (d *Detail) SetViewer(p airport.LatLon) {
	d.mu.Lock()
	d.viewer, d.hasViewer = p, true
	d.mu.Unlock()
}

// Interval is how many frames an aircraft at p skips between updates:
// 0 every frame. Aircraft on the runway (full) are always 0.
func (d *Detail) Interval(p airport.LatLon, moving, full bool) uint32 {
	if d == nil || full {
		return 0
	}
	if !moving {
		return d.StillInterval
	}
	d.mu.Lock()
	v, ok := d.viewer, d.hasViewer
	d.mu.Unlock()
	if !ok {
		return 0
	}
	switch m := calc.HaversineMeters(v.Lat, v.Lon, p.Lat, p.Lon); {
	case m < d.NearMeters:
		return 0
	case m < d.MidMeters:
		return d.MidInterval
	}
	return d.FarInterval
}

// report records what an aircraft runs at; forget drops it.
func (d *Detail) report(objectID, interval uint32) {
	d.mu.Lock()
	d.intervals[objectID] = interval
	d.mu.Unlock()
}

func (d *Detail) forget(objectID uint32) {
	if d == nil {
		return
	}
	d.mu.Lock()
	delete(d.intervals, objectID)
	d.mu.Unlock()
}

// Load is the aircraft driven and the updates a second they take together
// at 60 frames a second, and how many run at every frame.
func (d *Detail) Load() (aircraft int, updatesPerSecond float64, everyFrame int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, n := range d.intervals {
		aircraft++
		updatesPerSecond += detailAssumedFramesPerSecon / float64(n+1)
		if n == 0 {
			everyFrame++
		}
	}
	return aircraft, updatesPerSecond, everyFrame
}

// detailState is a controller's level of detail: the interval its monitor
// runs at and since when a slower one was asked for.
type detailState struct {
	interval  uint32
	set       bool
	slowerFor time.Time
}

// want decides the monitor interval for now; it returns the interval to
// request and true when it changed.
func (s *detailState) want(d *Detail, now time.Time, p airport.LatLon, moving, full bool) (uint32, bool) {
	if d == nil {
		return 0, false
	}
	n := d.Interval(p, moving, full)
	switch {
	case !s.set:
		// The monitor starts at every frame.
		s.set = true
		s.interval = 0
		if n == 0 {
			return 0, false
		}
		s.slowerFor = now
		return 0, false
	case n < s.interval:
		s.interval, s.slowerFor = n, time.Time{}
		return n, true
	case n > s.interval:
		if s.slowerFor.IsZero() {
			s.slowerFor = now
		}
		if now.Sub(s.slowerFor) >= d.SlowerAfter {
			s.interval, s.slowerFor = n, time.Time{}
			return n, true
		}
	default:
		s.slowerFor = time.Time{}
	}
	return s.interval, false
}

// requestFrames re-requests a monitor at every interval+1 sim frames.
func requestFrames(client interface {
	RequestDataOnSimObject(uint32, uint32, uint32, types.SIMCONNECT_PERIOD, types.SIMCONNECT_DATA_REQUEST_FLAG, uint32, uint32, uint32) error
}, req, def, objectID, interval uint32) error {
	return client.RequestDataOnSimObject(req, def, objectID, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, interval, 0)
}
