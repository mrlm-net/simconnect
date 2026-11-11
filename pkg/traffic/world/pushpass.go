package world

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Ground holds a pushback while one of ours taxies through what the push
// will sweep: pushed first, the departure stood on that taxiway facing
// the traffic coming along it, and neither could pass (live, LKPR
// 2026-11-11: CSA1889 pushed from C26 onto H1 nose to nose with TVS1832
// taxiing in to C27; TVS1832 had given way to the push 245 m out). The
// push waits on its stand, told once ("standby, traffic passing behind"),
// and is answered once the traffic is by. Generic: any airport, any two.
const (
	// pushPassAheadM: traffic with the push area within this much of the
	// way ahead of it holds the push (a minute or two of taxiing).
	pushPassAheadM = 400.0
	// pushPassStepM: the way ahead is looked at in steps of this.
	pushPassStepM = 5.0
	// pushPassMaxWait: held this long, the push is answered anyway (the
	// traffic stopped for good, or for something else).
	pushPassMaxWait = 3 * time.Minute
	// pushPassRetry: a held push is asked about again after this.
	pushPassRetry = 10 * time.Second
)

// passingBehind is one of ours at its airport taxiing with what its push
// will sweep on the next pushPassAheadM of its way, nil none.
func (it *controlled) passingBehind() *controlled {
	if it.dep == nil {
		return nil
	}
	area, ok := it.dep.PushArea()
	if !ok || len(area.Points) == 0 {
		return nil
	}
	for _, o := range it.cc.snapshotItems() {
		if o == it || o.ICAO != it.ICAO {
			continue
		}
		_, way, movable, ok := o.groundWay()
		if !ok || !movable {
			continue
		}
		o.mu.Lock()
		half := traffic.MotionProfileFor(o.view.Model).SpanMeters / 2
		o.mu.Unlock()
		if wayMeets(way, area, half+area.HalfSpan+traffic.HeadOnMarginMeters, pushPassAheadM) {
			return o
		}
	}
	return nil
}

// wayMeets reports whether the first ahead metres of way come within reach
// of any point of area.
func wayMeets(way []airport.LatLon, area airport.Occupied, reach, ahead float64) bool {
	near := func(p airport.LatLon) bool {
		for _, q := range area.Points {
			if calc.HaversineMeters(p.Lat, p.Lon, q.Lat, q.Lon) < reach {
				return true
			}
		}
		return false
	}
	gone := 0.0
	for i := 0; i+1 < len(way) && gone <= ahead; i++ {
		a, b := way[i], way[i+1]
		seg := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
		steps := int(math.Max(1, math.Ceil(seg/pushPassStepM)))
		for k := 0; k < steps && gone+float64(k)*seg/float64(steps) <= ahead; k++ {
			f := float64(k) / float64(steps)
			if near(airport.LatLon{Lat: a.Lat + f*(b.Lat-a.Lat), Lon: a.Lon + f*(b.Lon-a.Lon)}) {
				return true
			}
		}
		gone += seg
	}
	if len(way) > 0 && gone <= ahead {
		return near(way[len(way)-1])
	}
	return false
}

// holdPushForPassing is whether ground holds its push for traffic passing
// behind: told once, asked again after pushPassRetry; never longer than
// pushPassMaxWait in all.
func (it *controlled) holdPushForPassing(req string) bool {
	o := it.passingBehind()
	now := it.cc.clock.Now()
	it.mu.Lock()
	if o == nil || it.request != req || !it.pushPassFrom.IsZero() && now.Sub(it.pushPassFrom) > pushPassMaxWait {
		it.mu.Unlock()
		return false
	}
	if it.pushPassFrom.IsZero() {
		it.pushPassFrom = now
	}
	first := !it.spoken["standby passing"]
	it.spoken["standby passing"] = true
	it.mu.Unlock()
	if first {
		it.cc.log.printf("%-6s ground: pushback held — %s taxiing through the push", it.Tail, o.Tail)
		it.say(traffic.Transmission{Position: traffic.PosGround, Callsign: it.Tail, Intent: traffic.IntentStandby,
			Text: it.Tail + ", standby, traffic passing behind"})
	}
	it.cc.pending.later(now.Add(pushPassRetry), func() { it.askGround(req) })
	return true
}
