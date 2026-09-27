//go:build windows
// +build windows

// Command spike-lighttiming measures when MSFS AI switches the lights of a
// taxiing non-ATC aircraft (#295). Two aircraft taxi from remote stands to a
// runway; every sim frame their lights, AI state and waypoint progress are
// sampled and every change is logged with a timestamp.
//
//	A: lights written once after the waypoints are sent (measurement)
//	B: lights written again each time a waypoint is passed (candidate fix)
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	defWp     uint32 = 9160
	defMon    uint32 = 9161
	defLights uint32 = 9162
	defState  uint32 = 9163
	reqBase   uint32 = 9260 // + 10*i: spawn, release, monitor, remove, state
)

type monitor struct {
	Lat, Lon, GS                       float64
	Landing, Taxi, Strobe, Beacon, Nav float64
}

func (m monitor) lights() string {
	var b strings.Builder
	for _, x := range []struct {
		v float64
		c byte
	}{{m.Landing, 'L'}, {m.Taxi, 'T'}, {m.Strobe, 'S'}, {m.Beacon, 'B'}, {m.Nav, 'N'}} {
		if x.v != 0 {
			b.WriteByte(x.c)
		} else {
			b.WriteByte('.')
		}
	}
	return b.String()
}

type plane struct {
	name, stand  string
	rewrite      bool
	obj          uint32
	wps          []types.SIMCONNECT_DATA_WAYPOINT
	next         int // next waypoint not yet passed
	lights       string
	state        string
	offSince     time.Time
	offTotal     time.Duration
	offCount     int
	lastPassedAt time.Time
	writes       int
}

func main() {
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	runway := flag.String("runway", "06", "runway end to taxi to")
	secs := flag.Int("seconds", 240, "how long to watch")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, cancelMax := context.WithTimeout(ctx, time.Duration(*secs)*time.Second)
	defer cancelMax()

	client := simconnect.NewClient("GO Spike - light timing", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()

	client.AddToDataDefinition(defWp, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0)
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"GROUND VELOCITY", "knots"},
		{"LIGHT LANDING", "bool"}, {"LIGHT TAXI", "bool"}, {"LIGHT STROBE", "bool"}, {"LIGHT BEACON", "bool"}, {"LIGHT NAV", "bool"},
	} {
		client.AddToDataDefinition(defMon, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	for i, n := range []string{"LIGHT LANDING", "LIGHT TAXI", "LIGHT STROBE", "LIGHT BEACON", "LIGHT NAV"} {
		client.AddToDataDefinition(defLights, n, "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.AddToDataDefinition(defState, "AI TRAFFIC STATE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)

	planes := []*plane{
		{name: "A measure", stand: "N50"},
		{name: "B rewrite@wp", stand: "N51", rewrite: true},
	}
	start := time.Now()
	logf := func(p *plane, format string, a ...any) {
		fmt.Printf("%7.2fs [%s] %s\n", time.Since(start).Seconds(), p.name, fmt.Sprintf(format, a...))
	}
	writeLights := func(p *plane, why string) {
		l := [5]float64{0, 1, 0, 1, 1} // taxi, beacon, nav
		client.SetDataOnSimObject(defLights, p.obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(l)), unsafe.Pointer(&l))
		p.writes++
		logf(p, "write lights .TBN (%s)", why)
	}

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	loader.Request("LKPR")
	fleet := traffic.NewFleet(client)
	var g *airport.Graph

	summary := func() {
		fmt.Println("\n══ Light timing summary ══")
		for _, p := range planes {
			if !p.offSince.IsZero() {
				p.offTotal += time.Since(p.offSince)
			}
			fmt.Printf("%-13s taxi light switched off by AI %d times, off %.1fs in total, %d writes, reached waypoint %d/%d\n",
				p.name, p.offCount, p.offTotal.Seconds(), p.writes, p.next, len(p.wps))
			fleet.Remove(p.obj, reqBase+3)
		}
		time.Sleep(time.Second)
	}

	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			summary()
			return
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil || msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				g, _ = cache.Graph(res.Layout.ICAO)
				for i, p := range planes {
					k, err := res.Layout.ParkingIndex(p.stand)
					if err != nil {
						fmt.Println(err)
						return
					}
					r, err := g.RouteToRunway(k, *runway, airport.RouteOptions{})
					if err != nil {
						fmt.Println(err)
						return
					}
					p.wps, _ = traffic.TaxiWaypoints(g, r)
					st := res.Layout.Parking[k]
					fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: fmt.Sprintf("LTM%d", i), Position: types.SIMCONNECT_DATA_INITPOSITION{
						Latitude: st.Position.Lat, Longitude: st.Position.Lon, Altitude: res.Layout.Altitude * 3.28084, Heading: st.Heading, OnGround: 1,
					}}, reqBase+uint32(10*i))
					fmt.Printf("🗺️  %s: %s → %s, %d waypoints via %v\n", p.name, p.stand, *runway, len(p.wps), r.Taxiways)
				}
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				m := msg.AsAssignedObjectID()
				r := uint32(m.DwRequestID)
				if r < reqBase || (r-reqBase)%10 != 0 || int((r-reqBase)/10) >= len(planes) {
					continue
				}
				p := planes[(r-reqBase)/10]
				p.obj = uint32(m.DwObjectID)
				fleet.Acknowledge(r, p.obj)
				fleet.ReleaseControl(p.obj, r+1)
				fleet.SetWaypoints(p.obj, defWp, p.wps)
				writeLights(p, "after waypoints sent")
				// Every sim frame.
				client.RequestDataOnSimObject(r+2, defMon, p.obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)
				client.RequestDataOnSimObject(r+4, defState, p.obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, 0, 0)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				r := uint32(d.DwRequestID)
				if r < reqBase || int((r-reqBase)/10) >= len(planes) {
					continue
				}
				p := planes[(r-reqBase)/10]
				switch (r - reqBase) % 10 {
				case 4:
					s := engine.BytesToString(engine.CastDataAs[[256]byte](&d.DwData)[:])
					if s != p.state {
						logf(p, "AI state %s → %s (lights %s, next waypoint %d)", p.state, s, p.lights, p.next)
						p.state = s
					}
				case 2:
					m := *engine.CastDataAs[monitor](&d.DwData)
					// Waypoint passed: within 6 m of the next waypoint, or closer to the one after it.
					for p.next < len(p.wps) {
						w := p.wps[p.next]
						dw := calc.HaversineMeters(m.Lat, m.Lon, w.Latitude, w.Longitude)
						passed := dw < 6
						if !passed && p.next+1 < len(p.wps) {
							w2 := p.wps[p.next+1]
							passed = calc.HaversineMeters(m.Lat, m.Lon, w2.Latitude, w2.Longitude) < calc.HaversineMeters(w.Latitude, w.Longitude, w2.Latitude, w2.Longitude)-2
						}
						if !passed {
							break
						}
						p.next++
						p.lastPassedAt = time.Now()
						logf(p, "passed waypoint %d/%d (%.1f kt, lights %s)", p.next, len(p.wps), m.GS, m.lights())
						if p.rewrite {
							writeLights(p, fmt.Sprintf("after waypoint %d", p.next))
						}
					}
					l := m.lights()
					if l != p.lights {
						since := "—"
						if !p.lastPassedAt.IsZero() {
							since = fmt.Sprintf("%.2fs after waypoint %d", time.Since(p.lastPassedAt).Seconds(), p.next)
						}
						logf(p, "lights %s → %s (%s, %.1f kt, AI %s)", p.lights, l, since, m.GS, p.state)
						if p.lights != "" && m.Taxi == 0 && strings.Contains(p.lights, "T") {
							p.offCount++
							p.offSince = time.Now()
						}
						if m.Taxi != 0 && !p.offSince.IsZero() {
							p.offTotal += time.Since(p.offSince)
							p.offSince = time.Time{}
						}
						p.lights = l
					}
				}
			}
		}
	}
}

var _ = math.Abs
