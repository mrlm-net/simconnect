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
	evtNav     uint32 = 9385
	evtLogo    uint32 = 9386
)

type monitor struct {
	Lat, Lon, Alt, Ground, CG, Heading, Taxi, Beacon, OnGround, FrzLL, FrzAlt, FrzAtt, Nav, Logo float64
}

func main() {
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	stand := flag.String("stand", "N52", "stand to start from")
	runway := flag.String("runway", "06", "runway end the route goes to")
	kts := flag.Float64("kts", 12, "taxi speed")
	hz := flag.Float64("hz", 30, "position updates per second")
	secs := flag.Int("seconds", 75, "how long to drive")
	length := flag.Float64("length", 0, "stop after this many metres of route (0 = whole route)")
	wheelbase := flag.Float64("wheelbase", 12.6, "nose gear to main gear, metres (A320 12.6)")
	cgAhead := flag.Float64("cg-ahead", 1.0, "sim reference point ahead of the main gear, metres")
	smooth := flag.Int("smooth", 4, "corner-rounding passes (Chaikin)")
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
		{"LIGHT NAV", "bool"}, {"LIGHT LOGO", "bool"},
	} {
		track("def "+v.n, client.AddToDataDefinition(defMon, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)))
	}
	for id, name := range map[uint32]string{evtFrzLL: "FREEZE_LATITUDE_LONGITUDE_SET", evtFrzAlt: "FREEZE_ALTITUDE_SET", evtFrzAtt: "FREEZE_ATTITUDE_SET", evtTaxi: "TAXI_LIGHTS_SET", evtBeacon: "BEACON_LIGHTS_SET", evtNav: "NAV_LIGHTS_SET", evtLogo: "LOGO_LIGHTS_SET"} {
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
		obj                 uint32
		pts                 []airport.LatLon
		cum                 []float64
		elevFt              float64
		driving             bool
		startAt             time.Time
		last                monitor
		haveMon             bool
		cmd                 airport.LatLon
		cmdHdg              float64
		errSum              float64
		errMax              float64
		samples             int
		taxiOn              int
		agls                []float64
		lastLog             time.Time
		limits              []float64
		pos                 float64 // metres along the route
		vel                 float64 // m/s
		lastTick, stoppedAt time.Time
		acc                 float64        // m/s², current
		gear                airport.LatLon // main gear
	)
	const (
		accel = 0.35 // m/s², pulling away
		decel = 0.5  // m/s², planned braking
		jerk  = 0.2  // m/s³, how fast acceleration may change
	)
	limitAt := func(s float64) float64 {
		for i := 1; i < len(cum); i++ {
			if cum[i] >= s {
				f := (s - cum[i-1]) / math.Max(cum[i]-cum[i-1], 1e-6)
				return limits[i-1] + (limits[i]-limits[i-1])*f
			}
		}
		return 0
	}
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
			dt := 0.0
			if !lastTick.IsZero() {
				dt = time.Since(lastTick).Seconds()
			}
			lastTick = time.Now()
			// Chase the allowed speed at the current point and a bit ahead
			// (the nose must already be slow entering the turn), reaching it
			// in about 2 s; near the end, brake exactly onto the stop point.
			// The acceleration itself changes at most jerk m/s³, so every
			// speed change starts and ends softly.
			end := cum[len(cum)-1]
			target := math.Min(limitAt(pos), limitAt(pos+10))
			want := (target - vel) / 2
			if rem := end - pos; rem < 40 && rem > 0.05 {
				want = math.Min(want, -vel*vel/(2*rem))
			}
			want = math.Max(-1.5*decel, math.Min(accel, want))
			if want > acc {
				acc = math.Min(want, acc+jerk*dt)
			} else {
				acc = math.Max(want, acc-jerk*dt)
			}
			vel = math.Max(0, vel+acc*dt)
			pos = math.Min(pos+vel*dt, end)
			if end-pos < 0.3 && vel < 0.1 {
				vel, acc, pos = 0, 0, end
			}
			if pos >= end-0.05 && vel == 0 && stoppedAt.IsZero() {
				stoppedAt = time.Now()
				fmt.Printf("🛑 stopped at the end of the route after %.0f s\n", t)
			}
			if t > float64(*secs) || (!stoppedAt.IsZero() && time.Since(stoppedAt) > 5*time.Second) {
				fmt.Println("\n══ Injection summary ══")
				fmt.Printf("updates %.0f Hz, cruise %.0f kt, driven %.0f m in %.0f s\n", *hz, *kts, pos, t)
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
			// The nose wheel follows the path; the main gear trails it on the
			// wheelbase like a towed trailer (it cuts inside turns), and the
			// fuselage points from main gear to nose — so the heading eases
			// into and out of every turn by itself.
			// Done in local metres around the nose: repeated bearing/displace
			// round trips are not exact and drift the gear sideways (sliding).
			nose, _ := along(pos)
			const mPerDeg = 111319.49
			kx := mPerDeg * math.Cos(nose.Lat*math.Pi/180)
			dx, dy := (gear.Lon-nose.Lon)*kx, (gear.Lat-nose.Lat)*mPerDeg // nose → gear
			if d := math.Hypot(dx, dy); d > 1e-6 {
				dx, dy = dx/d**wheelbase, dy/d**wheelbase
			}
			gear = airport.LatLon{Lat: nose.Lat + dy/mPerDeg, Lon: nose.Lon + dx/kx}
			cmdHdg = math.Mod(math.Atan2(-dx, -dy)*180/math.Pi+360, 360)
			f := 1 - *cgAhead / *wheelbase // reference point, from the nose
			cmd = airport.LatLon{Lat: nose.Lat + dy*f/mPerDeg, Lon: nose.Lon + dx*f/kx}
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
				pts = chaikin(r.Points[1:], *smooth) // from the taxiway junction, corners rounded
				cum = make([]float64, len(pts))
				for i := 1; i < len(pts); i++ {
					cum[i] = cum[i-1] + calc.HaversineMeters(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
				}
				if *length > 0 && *length < cum[len(cum)-1] {
					n := 1
					for n < len(cum) && cum[n] < *length {
						n++
					}
					pts, cum = pts[:n], cum[:n]
				}
				limits = speedLimits(pts, cum, *kts*0.5144, 0.6, 3*0.5144, decel)
				var turns []string
				for i := 1; i < len(limits); i++ {
					if limits[i] < limits[i-1] && (i+1 == len(limits) || limits[i] <= limits[i+1]) && limits[i] < *kts*0.5144-0.5 && i+1 < len(limits) {
						turns = append(turns, fmt.Sprintf("%.0fm:%.0fkt", cum[i], limits[i]/0.5144))
					}
				}
				fmt.Printf("🐢 turn speeds (distance:kt): %v\n", turns)
				// Main gear at the route start, nose one wheelbase ahead.
				gear, _ = along(0)
				nose, _ := along(*wheelbase)
				pos = *wheelbase
				h0 := calc.BearingDegrees(gear.Lat, gear.Lon, nose.Lat, nose.Lon)
				c0lat, c0lon := calc.DisplaceByHeading(gear.Lat, gear.Lon, h0, *cgAhead)
				fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: "INJ1", Position: types.SIMCONNECT_DATA_INITPOSITION{
					Latitude: c0lat, Longitude: c0lon, Altitude: elevFt, Heading: h0, OnGround: 1,
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
				event(obj, evtNav, 1, "NAV_LIGHTS_SET")
				event(obj, evtLogo, 1, "LOGO_LIGHTS_SET")
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
				if !driving || cmd.Lat == 0 {
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
					fmt.Printf("%5.1fs  %4.1f kt  err %5.2f m  hdg %5.1f/%5.1f  CG over ground %5.2f ft (static %.2f)  on-ground %.0f  freeze LL/alt/att %.0f/%.0f/%.0f  taxi %.0f beacon %.0f nav %.0f logo %.0f\n",
						time.Since(startAt).Seconds(), vel/0.5144, e, last.Heading, cmdHdg, last.Alt-last.Ground, last.CG, last.OnGround, last.FrzLL, last.FrzAlt, last.FrzAtt, last.Taxi, last.Beacon, last.Nav, last.Logo)
				}
			}
		}
	}
}

// chaikin rounds the corners of a polyline: each pass replaces every
// segment by its 1/4 and 3/4 points, keeping the end points.
func chaikin(p []airport.LatLon, passes int) []airport.LatLon {
	for ; passes > 0 && len(p) > 2; passes-- {
		out := []airport.LatLon{p[0]}
		for i := 0; i+1 < len(p); i++ {
			a, b := p[i], p[i+1]
			out = append(out,
				airport.LatLon{Lat: a.Lat*0.75 + b.Lat*0.25, Lon: a.Lon*0.75 + b.Lon*0.25},
				airport.LatLon{Lat: a.Lat*0.25 + b.Lat*0.75, Lon: a.Lon*0.25 + b.Lon*0.75})
		}
		p = append(out, p[len(p)-1])
	}
	return p
}

// speedLimits returns the allowed speed (m/s) at each route point: cruise,
// slower where the path curves (lateral acceleration latG on the turn radius),
// and a braking curve (decel m/s²) down to a stop at the end.
func speedLimits(pts []airport.LatLon, cum []float64, cruise, latG, minTurn, decel float64) []float64 {
	n := len(pts)
	v := make([]float64, n)
	for i := range v {
		v[i] = cruise
	}
	// Turn radius from the heading change over ±8 m around each point.
	const win = 8.0
	j0, j1 := 0, 0
	for i := 0; i < n; i++ {
		for j0 < i && cum[i]-cum[j0] > win {
			j0++
		}
		for j1 < n-1 && cum[j1]-cum[i] < win {
			j1++
		}
		if j0 == i || j1 == i {
			continue
		}
		h1 := calc.BearingDegrees(pts[j0].Lat, pts[j0].Lon, pts[i].Lat, pts[i].Lon)
		h2 := calc.BearingDegrees(pts[i].Lat, pts[i].Lon, pts[j1].Lat, pts[j1].Lon)
		d := math.Abs(math.Mod(h2-h1+540, 360)-180) * math.Pi / 180
		if d > 0.01 {
			r := (cum[j1] - cum[j0]) / d
			v[i] = math.Min(v[i], math.Max(minTurn, math.Sqrt(latG*r)))
		}
	}
	v[n-1] = 0
	for i := n - 2; i >= 0; i-- {
		v[i] = math.Min(v[i], math.Sqrt(v[i+1]*v[i+1]+2*decel*(cum[i+1]-cum[i])))
	}
	return v
}
