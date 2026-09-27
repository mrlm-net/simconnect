//go:build windows
// +build windows

// Command spike-speed is the #295 approach speed matrix: four non-ATC
// aircraft on final, each trying a different way to fly the approach slower
// (MSFS AI ignores requested airspeed and flies about 164 kt), reading back
// flaps, airspeed and touchdown.
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
	defWp      uint32 = 9150
	defMon     uint32 = 9151
	defGear    uint32 = 9152
	defFlaps   uint32 = 9153
	reqBase    uint32 = 9250
	throttlePc        = 35.0
)

type variant struct {
	name     string
	flaps    bool
	throttle bool
	nm       float64
}

type monitor struct {
	Lat, Lon, AGL, IAS, GS, VS, OnGround, FlapsIdx, FlapsPct, FlapsPositions float64
}

type plane struct {
	variant
	idx       int
	obj       uint32
	airborne  bool
	done      bool
	ias       []string // IAS at 4, 3, 2, 1 nm
	nextNm    float64
	flapsSeen string
	result    string
}

func main() {
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	runway := flag.String("runway", "24", "runway end")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, cancelMax := context.WithTimeout(ctx, 9*time.Minute)
	defer cancelMax()

	client := simconnect.NewClient("GO Spike - approach speed", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()

	sent := map[uint32]string{}
	track := func(desc string, err error) {
		if id, e := client.GetLastSentPacketID(); e == nil {
			sent[id] = desc
		}
	}
	client.AddToDataDefinition(defWp, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0)
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"AIRSPEED INDICATED", "knots"}, {"GROUND VELOCITY", "knots"}, {"VERTICAL SPEED", "feet per minute"},
		{"SIM ON GROUND", "bool"}, {"FLAPS HANDLE INDEX", "number"}, {"TRAILING EDGE FLAPS LEFT PERCENT", "percent"},
		{"FLAPS NUM HANDLE POSITIONS", "number"},
	} {
		client.AddToDataDefinition(defMon, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.AddToDataDefinition(defGear, "GEAR HANDLE POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
	track("def flaps", client.AddToDataDefinition(defFlaps, "FLAPS HANDLE INDEX", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0))

	planes := []*plane{
		{variant: variant{"control", false, false, 5}},
		{variant: variant{"flaps full", true, false, 8}},
		{variant: variant{"throttle 35%", false, true, 11}},
		{variant: variant{"flaps+throttle", true, true, 14}},
	}
	for i, p := range planes {
		p.idx, p.nextNm = i, 4
	}

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	loader.Request("LKPR")
	fleet := traffic.NewFleet(client)
	var (
		rwy  airport.Runway
		end  airport.RunwayEnd
		elev float64
	)
	wps := func(p *plane) []types.SIMCONNECT_DATA_WAYPOINT {
		t, back := end.Threshold, math.Mod(end.Heading+180, 360)
		air := uint32(types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED | types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL | types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED)
		thr := 0.0
		if p.throttle {
			air |= uint32(types.SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED)
			thr = throttlePc
		}
		var out []types.SIMCONNECT_DATA_WAYPOINT
		for nm := math.Ceil(p.nm) - 1; nm >= 1; nm-- {
			lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, back, nm*1852)
			out = append(out, types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: 318 * nm, Flags: air, KtsSpeed: 135, PercentThrottle: thr})
		}
		out = append(out, types.SIMCONNECT_DATA_WAYPOINT{Latitude: t.Lat, Longitude: t.Lon, Altitude: 50, Flags: air, KtsSpeed: 130, PercentThrottle: thr})
		for _, s := range []struct{ m, kts float64 }{{300, 120}, {1000, 60}, {1600, 30}, {2000, 15}} {
			lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, end.Heading, s.m)
			out = append(out, traffic.TaxiWaypoint(lat, lon, elev, s.kts))
		}
		return out
	}
	remove := func(p *plane, why string) {
		if !p.done && p.obj != 0 {
			p.done = true
			fleet.Remove(p.obj, reqBase+uint32(10*p.idx)+3)
			if p.result == "" {
				p.result = why
			}
		}
	}
	summary := func() {
		fmt.Println("\n══ Approach speed matrix (IAS at 4/3/2/1 nm) ══")
		for _, p := range planes {
			fmt.Printf("%-15s IAS %-24s flaps %-18s │ %s\n", p.name, strings.Join(p.ias, " "), p.flapsSeen, p.result)
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
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil || msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				rwy, end, _ = res.Layout.RunwayEnd(*runway)
				elev = convert.MetersToFeet(res.Layout.Altitude)
				for _, p := range planes {
					lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, math.Mod(end.Heading+180, 360), p.nm*1852)
					fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: fmt.Sprintf("SP%d", p.idx), Position: types.SIMCONNECT_DATA_INITPOSITION{
						Latitude: lat, Longitude: lon, Altitude: elev + 318*p.nm, Heading: end.Heading, Airspeed: 150,
					}}, reqBase+uint32(10*p.idx))
				}
				fmt.Printf("🛬 4 aircraft on final %s\n", end.Name)
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				if d, ok := sent[uint32(e.DwSendID)]; ok {
					fmt.Printf("⚠️  exception %d on %s\n", e.DwException, d)
				}
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
				g := [1]float64{1}
				client.SetDataOnSimObject(defGear, p.obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&g))
				if p.flaps {
					f := [1]float64{8} // clamped to the last handle position by the sim, if it accepts the write
					track("SetData FLAPS HANDLE INDEX ["+p.name+"]", client.SetDataOnSimObject(defFlaps, p.obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&f)))
				}
				fleet.SetWaypoints(p.obj, defWp, wps(p))
				client.RequestDataOnSimObject(r+2, defMon, p.obj, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				r := uint32(d.DwRequestID)
				if r < reqBase || (r-reqBase)%10 != 2 || int((r-reqBase)/10) >= len(planes) {
					continue
				}
				p := planes[(r-reqBase)/10]
				if p.done {
					continue
				}
				m := *engine.CastDataAs[monitor](&d.DwData)
				nm := calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, m.Lat, m.Lon) / 1852
				if m.OnGround == 0 {
					p.airborne = true
				}
				if p.nextNm >= 1 && nm <= p.nextNm && m.OnGround == 0 {
					p.ias = append(p.ias, fmt.Sprintf("%.0f", m.IAS))
					p.flapsSeen = fmt.Sprintf("idx %.0f/%.0f %3.0f%%", m.FlapsIdx, m.FlapsPositions, m.FlapsPct)
					p.nextNm--
				}
				past := calc.AlongTrackMeters(end.Threshold.Lat, end.Threshold.Lon, rwy.Center.Lat, rwy.Center.Lon, m.Lat, m.Lon)
				if p.airborne && m.OnGround != 0 && p.result == "" {
					p.result = fmt.Sprintf("touchdown %4.0f m at %3.0f kt", past, m.GS)
					fmt.Printf("🛬 [%s] %s, IAS %v, flaps %s\n", p.name, p.result, p.ias, p.flapsSeen)
				}
				if (p.result != "" && m.GS < 40) || (p.result == "" && past > rwy.Length+300) {
					remove(p, "no touchdown")
				}
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
