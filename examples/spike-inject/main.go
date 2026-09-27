//go:build windows
// +build windows

// Command spike-inject is the #309 position-injection feasibility spike: an
// AI aircraft is taken off AI control, frozen with the FREEZE events, and
// moved by this program ~30 times a second along a taxi route with
// "Initial Position" writes, while its taxi light is switched on. Every
// second it reports tracking error, height over ground, freeze flags and
// lights.
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	defInit    uint32 = 9180
	defMon     uint32 = 9181
	reqSpawn   uint32 = 9280
	reqRelease uint32 = 9281
	reqMon     uint32 = 9282
	reqRemove  uint32 = 9283
	evtFrzLL   uint32 = 9380
	evtFrzAlt  uint32 = 9381
	evtFrzAtt  uint32 = 9382
	evtTaxi    uint32 = 9383
	evtBeacon  uint32 = 9384
)

type monitor struct {
	Lat, Lon, Alt, Ground, CG, Heading, Taxi, Beacon, OnGround, FrzLL, FrzAlt, FrzAtt float64
}

func main() {
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	stand := flag.String("stand", "N52", "stand to start from")
	runway := flag.String("runway", "06", "runway end the route goes to")
	kts := flag.Float64("kts", 12, "taxi speed")
	hz := flag.Float64("hz", 30, "position updates per second")
	secs := flag.Int("seconds", 75, "how long to drive")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := simconnect.NewClient("GO Spike - injection", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()

	sent := map[uint32]string{}
	track := func(desc string, err error) {
		if id, e := client.GetLastSentPacketID(); e == nil {
			sent[id] = desc
		}
		if err != nil {
			fmt.Printf("   %s: %v\n", desc, err)
		}
	}
	track("def Initial Position", client.AddToDataDefinition(defInit, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0))
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALTITUDE", "feet"},
		{"GROUND ALTITUDE", "feet"}, {"STATIC CG TO GROUND", "feet"}, {"PLANE HEADING DEGREES TRUE", "degrees"},
		{"LIGHT TAXI", "bool"}, {"LIGHT BEACON", "bool"}, {"SIM ON GROUND", "bool"},
		{"IS LATITUDE LONGITUDE FREEZE ON", "bool"}, {"IS ALTITUDE FREEZE ON", "bool"}, {"IS ATTITUDE FREEZE ON", "bool"},
	} {
		track("def "+v.n, client.AddToDataDefinition(defMon, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)))
	}
	for id, name := range map[uint32]string{evtFrzLL: "FREEZE_LATITUDE_LONGITUDE_SET", evtFrzAlt: "FREEZE_ALTITUDE_SET", evtFrzAtt: "FREEZE_ATTITUDE_SET", evtTaxi: "TAXI_LIGHTS_SET", evtBeacon: "BEACON_LIGHTS_SET"} {
		track("map "+name, client.MapClientEventToSimEvent(id, name))
	}
	event := func(obj, evt, data uint32, desc string) {
		track(desc, client.TransmitClientEvent(obj, evt, data, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY))
	}

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	loader.Request("LKPR")
	fleet := traffic.NewFleet(client)

	var (
		obj     uint32
		pts     []airport.LatLon
		cum     []float64
		elevFt  float64
		driving bool
		startAt time.Time
		last    monitor
		haveMon bool
		cmd     airport.LatLon
		cmdHdg  float64
		errSum  float64
		errMax  float64
		samples int
		taxiOn  int
		agls    []float64
		lastLog time.Time
	)
	// along returns the position and heading at distance s along the route.
	along := func(s float64) (airport.LatLon, float64) {
		for i := 1; i < len(pts); i++ {
			if cum[i] >= s || i == len(pts)-1 {
				a, b := pts[i-1], pts[i]
				f := 0.0
				if cum[i] > cum[i-1] {
					f = math.Min(1, math.Max(0, (s-cum[i-1])/(cum[i]-cum[i-1])))
				}
				return airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: a.Lon + (b.Lon-a.Lon)*f}, calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
			}
		}
		return pts[len(pts)-1], 0
	}
	place := func(p airport.LatLon, hdg float64) {
		alt := elevFt
		if haveMon && last.Ground != 0 {
			alt = last.Ground + last.CG
		}
		pos := types.SIMCONNECT_DATA_INITPOSITION{Latitude: p.Lat, Longitude: p.Lon, Altitude: alt, Heading: hdg, OnGround: 1, Airspeed: 0}
		client.SetDataOnSimObject(defInit, obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(pos)), unsafe.Pointer(&pos))
	}

	frame := time.NewTicker(time.Duration(float64(time.Second) / *hz))
	defer frame.Stop()
	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			return
		case <-frame.C:
			driving = !startAt.IsZero() && time.Now().After(startAt)
			if !driving {
				continue
			}
			t := time.Since(startAt).Seconds()
			if t > float64(*secs) {
				fmt.Println("\n══ Injection summary ══")
				fmt.Printf("updates %.0f Hz, speed %.0f kt, driven %.0f m\n", *hz, *kts, math.Min(t**kts*1852/3600, cum[len(cum)-1]))
				fmt.Printf("tracking error: mean %.2f m, max %.2f m over %d samples\n", errSum/float64(max(samples, 1)), errMax, samples)
				fmt.Printf("taxi light on in %d/%d samples\n", taxiOn, samples)
				if len(agls) > 0 {
					lo, hi := agls[0], agls[0]
					for _, a := range agls {
						lo, hi = math.Min(lo, a), math.Max(hi, a)
					}
					fmt.Printf("height of CG over ground: %.2f–%.2f ft (spread %.2f ft)\n", lo, hi, hi-lo)
				}
				fleet.Remove(obj, reqRemove)
				time.Sleep(time.Second)
				return
			}
			s := math.Min(t**kts*1852/3600, cum[len(cum)-1])
			cmd, cmdHdg = along(s)
			place(cmd, cmdHdg)
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil || msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				l := res.Layout
				elevFt = convert.MetersToFeet(l.Altitude)
				g, _ := cache.Graph(l.ICAO)
				k, err := l.ParkingIndex(*stand)
				if err != nil {
					fmt.Println(err)
					return
				}
				r, err := g.RouteToRunway(k, *runway, airport.RouteOptions{})
				if err != nil {
					fmt.Println(err)
					return
				}
				pts = r.Points[1:] // from the taxiway junction
				cum = make([]float64, len(pts))
				for i := 1; i < len(pts); i++ {
					cum[i] = cum[i-1] + calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
				}
				p0, h0 := along(0)
				fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: "INJ1", Position: types.SIMCONNECT_DATA_INITPOSITION{
					Latitude: p0.Lat, Longitude: p0.Lon, Altitude: elevFt, Heading: h0, OnGround: 1,
				}}, reqSpawn)
				fmt.Printf("🗺️  route %s → %s from the junction: %.0f m via %v\n", *stand, *runway, cum[len(cum)-1], r.Taxiways)
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("⚠️  exception %d on %q (parameter %d)\n", e.DwException, sent[uint32(e.DwSendID)], e.DwIndex)
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				m := msg.AsAssignedObjectID()
				if uint32(m.DwRequestID) != reqSpawn {
					continue
				}
				obj = uint32(m.DwObjectID)
				fleet.Acknowledge(reqSpawn, obj)
				track("AIReleaseControl", fleet.ReleaseControl(obj, reqRelease))
				event(obj, evtFrzLL, 1, "FREEZE_LATITUDE_LONGITUDE_SET")
				event(obj, evtFrzAlt, 1, "FREEZE_ALTITUDE_SET")
				event(obj, evtFrzAtt, 1, "FREEZE_ATTITUDE_SET")
				event(obj, evtTaxi, 1, "TAXI_LIGHTS_SET")
				event(obj, evtBeacon, 1, "BEACON_LIGHTS_SET")
				track("monitor", client.RequestDataOnSimObject(reqMon, defMon, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0))
				fmt.Printf("🆔 object %d frozen, lights on; driving in 3 s\n", obj)
				startAt = time.Now().Add(3 * time.Second)

			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				if uint32(d.DwRequestID) != reqMon {
					continue
				}
				last, haveMon = *engine.CastDataAs[monitor](&d.DwData), true
				driving = !startAt.IsZero() && time.Now().After(startAt)
				if !driving {
					continue
				}
				e := calc.HaversineMeters(cmd.Lat, cmd.Lon, last.Lat, last.Lon)
				errSum += e
				errMax = math.Max(errMax, e)
				samples++
				if last.Taxi != 0 {
					taxiOn++
				}
				agls = append(agls, last.Alt-last.Ground)
				if time.Since(lastLog) >= time.Second {
					lastLog = time.Now()
					fmt.Printf("%5.1fs  err %5.2f m  hdg %5.1f/%5.1f  CG over ground %5.2f ft (static %.2f)  on-ground %.0f  freeze LL/alt/att %.0f/%.0f/%.0f  taxi %.0f beacon %.0f\n",
						time.Since(startAt).Seconds(), e, last.Heading, cmdHdg, last.Alt-last.Ground, last.CG, last.OnGround, last.FrzLL, last.FrzAlt, last.FrzAtt, last.Taxi, last.Beacon)
				}
			}
		}
	}
}
