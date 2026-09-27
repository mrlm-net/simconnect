//go:build windows
// +build windows

// Command spike-flare is the #295 flare matrix: four non-ATC aircraft on
// final for one runway, each with a different flare profile, sampled several
// times a second near the ground to measure touchdown point, vertical speed
// and pitch. Each aircraft is removed once it slows below 40 kt.
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
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	defWaypoints uint32 = 9500
	defMonitor   uint32 = 9501
	defGear      uint32 = 9502
	reqBase      uint32 = 9600 // + 10*i: spawn, release, monitor, remove
)

// profile is threshold height (ft), airborne flare points {meters past the
// threshold, ft} and the ground touchdown waypoint distance.
type profile struct {
	name    string
	thrFt   float64
	flare   [][2]float64
	groundM float64
	nm      float64
}

type monitor struct {
	Lat, Lon, AGL, VS, Pitch, GS, OnGround float64
}

type plane struct {
	profile
	idx      int
	objectID uint32
	airborne bool
	done     bool
	prev     monitor
	minVS    float64 // most negative VS below 50 ft
	maxPitch float64 // highest nose-up pitch below 50 ft (MSFS pitch: negative = nose up)
	result   string
}

func main() {
	runway := flag.String("runway", "24", "runway end")
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	maxRun := flag.Duration("max", 9*time.Minute, "time limit")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, cancelMax := context.WithTimeout(ctx, *maxRun)
	defer cancelMax()

	client := simconnect.NewClient("GO Spike - flare matrix", engine.WithContext(ctx))
	for client.Connect() != nil {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	defer client.Disconnect()

	client.AddToDataDefinition(defWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0)
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"VERTICAL SPEED", "feet per minute"}, {"PLANE PITCH DEGREES", "degrees"}, {"GROUND VELOCITY", "knots"}, {"SIM ON GROUND", "bool"},
	} {
		client.AddToDataDefinition(defMonitor, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.AddToDataDefinition(defGear, "GEAR HANDLE POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)

	planes := []*plane{
		{profile: profile{"V0 direct", 50, nil, 300, 5}},
		{profile: profile{"V1 one-flare", 50, [][2]float64{{200, 20}}, 400, 8}},
		{profile: profile{"V2 low-thr", 40, nil, 350, 11}},
		{profile: profile{"V3 two-flare", 50, [][2]float64{{150, 25}, {300, 10}}, 600, 14}},
	}
	for i, p := range planes {
		p.idx = i
	}

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	loader.Request("LKPR")
	fleet := traffic.NewFleet(client)
	var (
		rwy   airport.Runway
		end   airport.RunwayEnd
		elev  float64
		start = time.Now()
	)

	waypoints := func(p *plane) []types.SIMCONNECT_DATA_WAYPOINT {
		t, back := end.Threshold, math.Mod(end.Heading+180, 360)
		air := uint32(types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED | types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL | types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED)
		var wps []types.SIMCONNECT_DATA_WAYPOINT
		for nm := math.Ceil(p.nm) - 1; nm >= 1; nm-- {
			lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, back, nm*1852)
			wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: 318 * nm, Flags: air, KtsSpeed: 135})
		}
		wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: t.Lat, Longitude: t.Lon, Altitude: p.thrFt, Flags: air, KtsSpeed: 130})
		for _, f := range p.flare {
			lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, end.Heading, f[0])
			wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: f[1], Flags: air, KtsSpeed: 125})
		}
		for _, s := range []struct{ m, kts float64 }{{p.groundM, 120}, {p.groundM + 700, 60}, {p.groundM + 1300, 25}, {p.groundM + 1700, 15}} {
			lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, end.Heading, s.m)
			wps = append(wps, traffic.TaxiWaypoint(lat, lon, elev, s.kts))
		}
		return wps
	}
	remove := func(p *plane, why string) {
		if p.done || p.objectID == 0 {
			return
		}
		p.done = true
		fleet.Remove(p.objectID, reqBase+uint32(10*p.idx)+3)
		fmt.Printf("%4.0fs 🗑️  [%s] %s\n", time.Since(start).Seconds(), p.name, why)
	}
	summary := func() {
		fmt.Println("\n══ Flare matrix ══")
		for _, p := range planes {
			fmt.Printf("%-13s thr %2.0f ft, flare %v, ground wp %4.0f m │ %s\n", p.name, p.thrFt, p.flare, p.groundM, p.result)
		}
	}

	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			for _, p := range planes {
				remove(p, "time limit")
			}
			summary()
			time.Sleep(time.Second)
			return
		case msg, ok := <-stream:
			if !ok {
				return
			}
			if msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				if res.Err != nil {
					fmt.Println(res.Err)
					return
				}
				rwy, end, _ = res.Layout.RunwayEnd(*runway)
				elev = convert.MetersToFeet(res.Layout.Altitude)
				for _, p := range planes {
					lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, math.Mod(end.Heading+180, 360), p.nm*1852)
					fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: "F" + strings.Fields(p.name)[0], Position: types.SIMCONNECT_DATA_INITPOSITION{
						Latitude: lat, Longitude: lon, Altitude: elev + 318*p.nm, Heading: end.Heading, Airspeed: 150,
					}}, reqBase+uint32(10*p.idx))
				}
				fmt.Printf("🛬 4 aircraft on final %s (%s)\n", end.Name, rwy.Name())
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("⚠️  exception %d send %d\n", e.DwException, e.DwSendID)
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				m := msg.AsAssignedObjectID()
				req := uint32(m.DwRequestID)
				if req < reqBase || req >= reqBase+40 || (req-reqBase)%10 != 0 {
					continue
				}
				p := planes[(req-reqBase)/10]
				p.objectID = uint32(m.DwObjectID)
				fleet.Acknowledge(req, p.objectID)
				base := reqBase + uint32(10*p.idx)
				fleet.ReleaseControl(p.objectID, base+1)
				gear := [1]float64{1}
				client.SetDataOnSimObject(defGear, p.objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&gear))
				fleet.SetWaypoints(p.objectID, defWaypoints, waypoints(p))
				// About 6 samples a second: every 3rd sim frame.
				client.RequestDataOnSimObject(base+2, defMonitor, p.objectID, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 3, 0)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				req := uint32(d.DwRequestID)
				if req < reqBase || req >= reqBase+40 || (req-reqBase)%10 != 2 {
					continue
				}
				p := planes[(req-reqBase)/10]
				if p.done {
					continue
				}
				m := *engine.CastDataAs[monitor](&d.DwData)
				if m.OnGround == 0 {
					p.airborne = true
					if m.AGL < 50 {
						p.minVS = math.Min(p.minVS, m.VS)
						p.maxPitch = math.Min(p.maxPitch, m.Pitch)
					}
				}
				past := calc.AlongTrackMeters(end.Threshold.Lat, end.Threshold.Lon, rwy.Center.Lat, rwy.Center.Lon, m.Lat, m.Lon)
				if p.airborne && m.OnGround != 0 && p.result == "" {
					p.result = fmt.Sprintf("touchdown %4.0f m, %3.0f kt, VS just before %5.0f fpm (worst <50 ft %5.0f), nose-up %.1f°", past, m.GS, p.prev.VS, p.minVS, -p.maxPitch)
					fmt.Printf("%4.0fs 🛬 [%s] %s\n", time.Since(start).Seconds(), p.name, p.result)
				}
				if p.result != "" && m.GS < 40 {
					remove(p, "slowed below 40 kt")
				}
				if p.result == "" && past > rwy.Length+300 {
					p.result = "no touchdown (floated past the runway end)"
					remove(p, p.result)
				}
				p.prev = m
				all := true
				for _, q := range planes {
					all = all && q.done
				}
				if all {
					summary()
					time.Sleep(time.Second)
					return
				}
			}
		}
	}
}
