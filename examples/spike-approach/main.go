//go:build windows
// +build windows

// Command spike-approach is the #318 spike: fly the final approach, flare,
// touchdown, de-rotation and rollout of an AI aircraft by position
// injection instead of MSFS AI. Every frame it places the aircraft on a
// 3° glide path with a speed schedule, pitches it for approach and flare and
// reads back what the sim shows (pitch, height, gear, flaps, on-ground).
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
	defInit  uint32 = 9190
	defMon   uint32 = 9191
	defGear  uint32 = 9192
	defFlaps uint32 = 9193
	defSurf  uint32 = 9194
	reqSpawn uint32 = 9290
	reqRel   uint32 = 9291
	reqMon   uint32 = 9292
	reqRem   uint32 = 9293
	evtBase  uint32 = 9390
)

type monitor struct {
	Lat, Lon, Alt, Ground, CG, AGL, Pitch, Bank, Heading, OnGround, GearPct, GearHandle, FlapsPct, FlapsIdx, Landing float64
}

var events = []string{"FREEZE_LATITUDE_LONGITUDE_SET", "FREEZE_ALTITUDE_SET", "FREEZE_ATTITUDE_SET", "GEAR_DOWN", "FLAPS_DOWN", "FLAPS_SET", "FLAPS_INCR", "LANDING_LIGHTS_SET", "STROBES_SET", "NAV_LIGHTS_SET", "BEACON_LIGHTS_SET"}

func main() {
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	rwyEnd := flag.String("runway", "24", "runway end")
	startNm := flag.Float64("nm", 4, "distance out to start")
	vapp := flag.Float64("vapp", 135, "approach speed at the threshold, kt")
	vstart := flag.Float64("vstart", 150, "speed at the start, kt")
	pitchSign := flag.Float64("pitch-sign", -1, "sign SimConnect uses for nose-up pitch in Initial Position")
	hz := flag.Float64("hz", 60, "updates per second")
	flapsMode := flag.String("flaps", "down", "flaps method: down | set | incr | surface")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := simconnect.NewClient("GO Spike - injected approach", engine.WithContext(ctx))
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
	track("def init", client.AddToDataDefinition(defInit, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0))
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALTITUDE", "feet"},
		{"GROUND ALTITUDE", "feet"}, {"STATIC CG TO GROUND", "feet"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"PLANE PITCH DEGREES", "degrees"}, {"PLANE BANK DEGREES", "degrees"}, {"PLANE HEADING DEGREES TRUE", "degrees"},
		{"SIM ON GROUND", "bool"}, {"GEAR CENTER POSITION", "percent"}, {"GEAR HANDLE POSITION", "bool"},
		{"TRAILING EDGE FLAPS LEFT PERCENT", "percent"}, {"FLAPS HANDLE INDEX", "number"}, {"LIGHT LANDING", "bool"},
	} {
		track("def "+v.n, client.AddToDataDefinition(defMon, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)))
	}
	track("def gear", client.AddToDataDefinition(defGear, "GEAR HANDLE POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0))
	track("def flaps", client.AddToDataDefinition(defFlaps, "FLAPS HANDLE INDEX", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0))
	for i, n := range []string{"TRAILING EDGE FLAPS LEFT PERCENT", "TRAILING EDGE FLAPS RIGHT PERCENT", "LEADING EDGE FLAPS LEFT PERCENT", "LEADING EDGE FLAPS RIGHT PERCENT"} {
		track("def "+n, client.AddToDataDefinition(defSurf, n, "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)))
	}
	for i, n := range events {
		track("map "+n, client.MapClientEventToSimEvent(evtBase+uint32(i), n))
	}
	event := func(obj uint32, name string, data uint32) {
		for i, n := range events {
			if n == name {
				track(name, client.TransmitClientEvent(obj, evtBase+uint32(i), data, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY))
			}
		}
	}

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	loader.Request("LKPR")
	fleet := traffic.NewFleet(client)

	var (
		obj            uint32
		end            airport.RunwayEnd
		rwy            airport.Runway
		elevFt         float64
		mon            monitor
		haveMon        bool
		flying         bool
		x              float64 // meters past the threshold (negative before)
		h              float64 // main wheels above the runway, feet
		v              float64 // ground speed, m/s
		pitch          float64 // nose up, degrees
		phase          = "approach"
		touchX, touchV float64
		touchAt        time.Time
		lastTick       time.Time
		lastLog        time.Time
		start          time.Time
	)
	const (
		gs       = 3.0  // glide path, degrees
		tch      = 50.0 // threshold crossing height, ft
		flareFt  = 30.0
		tdFpm    = -120.0
		appPitch = 2.5
		flrPitch = 5.5
		derotS   = 4.0
		brake    = 1.5 // m/s²
		kt       = 0.514444
	)
	pos := func(x float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, x)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	place := func() {
		ground, cg := elevFt, 12.25
		if haveMon && mon.Ground != 0 {
			ground, cg = mon.Ground, mon.CG
		}
		p := pos(x)
		onGround := types.DWORD(0)
		if h <= 0 {
			onGround = 1
		}
		ip := types.SIMCONNECT_DATA_INITPOSITION{
			Latitude: p.Lat, Longitude: p.Lon, Altitude: ground + cg + math.Max(h, 0),
			Pitch: *pitchSign * pitch, Bank: 0, Heading: end.Heading, OnGround: onGround,
			Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(v / kt),
		}
		client.SetDataOnSimObject(defInit, obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(ip)), unsafe.Pointer(&ip))
	}
	// speed schedule: vstart at the start, vapp from 1 nm, 5 kt less at touchdown.
	speedAt := func(x float64) float64 {
		d := -x / 1852
		switch {
		case d > 1:
			f := math.Min(1, (d-1)/math.Max(*startNm-1, 0.1))
			return (*vapp + (*vstart-*vapp)*f) * kt
		case x < 0:
			return *vapp * kt
		}
		return (*vapp - 5) * kt
	}

	frame := time.NewTicker(time.Duration(float64(time.Second) / *hz))
	defer frame.Stop()
	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-frame.C:
			if !flying {
				continue
			}
			dt := math.Min(now.Sub(lastTick).Seconds(), 0.1)
			lastTick = now
			switch phase {
			case "approach", "flare":
				v += (speedAt(x) - v) * math.Min(1, dt/3)
				vsGS := -v * math.Tan(gs*math.Pi/180) / 0.3048 * 60 // fpm on the glide path
				vs := vsGS
				if h <= flareFt {
					if phase == "approach" {
						phase = "flare"
						fmt.Printf("🛬 flare at %.0f ft, x %.0f m, %.0f kt\n", h, x, v/kt)
					}
					vs = tdFpm + (vsGS-tdFpm)*(h/flareFt)
					pitch = flrPitch + (appPitch-flrPitch)*(h/flareFt)
				} else {
					pitch = appPitch
				}
				h += vs / 60 * dt
				x += v * dt
				if h <= 0 {
					h, phase, touchX, touchV, touchAt = 0, "derotate", x, vs, now
					fmt.Printf("🛬 touchdown %.0f m past the threshold at %.0f kt, %.0f fpm\n", x, v/kt, vs)
				}
			case "derotate":
				f := math.Min(1, now.Sub(touchAt).Seconds()/derotS)
				pitch = flrPitch * (1 - f*f*(3-2*f)) // smoothstep down
				v = math.Max(0, v-0.5*dt)
				x += v * dt
				if f >= 1 {
					phase = "rollout"
				}
			case "rollout":
				v = math.Max(0, v-brake*dt)
				x += v * dt
				if v < 20*kt {
					fmt.Printf("\n══ touchdown %.0f m, %.0f fpm; rollout to 20 kt at %.0f m past the threshold (runway %.0f m)\n", touchX, touchV, x, rwy.Length)
					fleet.Remove(obj, reqRem)
					time.Sleep(time.Second)
					return
				}
			}
			place()
			if now.Sub(lastLog) >= 500*time.Millisecond && haveMon {
				lastLog = now
				fmt.Printf("%5.1fs %-8s x %6.0f m  h %5.1f ft (AGL read %5.1f)  %5.1f kt  pitch cmd %4.1f read %5.1f  gear %3.0f%% (handle %.0f)  flaps %3.0f%% (idx %.0f)  ground %.0f  L %.0f\n",
					now.Sub(start).Seconds(), phase, x, h, mon.AGL, v/kt, pitch, mon.Pitch, mon.GearPct, mon.GearHandle, mon.FlapsPct, mon.FlapsIdx, mon.OnGround, mon.Landing)
			}
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil || msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				l := res.Layout
				elevFt = convert.MetersToFeet(l.Altitude)
				var ok bool
				rwy, end, ok = l.RunwayEnd(*rwyEnd)
				if !ok {
					fmt.Println("unknown runway")
					return
				}
				x = -*startNm * 1852
				h = tch + (-x)*math.Tan(gs*math.Pi/180)/0.3048
				v = *vstart * kt
				p := pos(x)
				fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: "INJ318", Position: types.SIMCONNECT_DATA_INITPOSITION{
					Latitude: p.Lat, Longitude: p.Lon, Altitude: elevFt + 12 + h, Heading: end.Heading, Airspeed: types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(*vstart),
				}}, reqSpawn)
				fmt.Printf("🗺️  %s: start %.1f nm, %.0f ft, %.0f kt; runway %.0f m, elevation %.0f ft\n", end.Name, *startNm, h, *vstart, rwy.Length, elevFt)
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("⚠️  exception %d on %q\n", e.DwException, sent[uint32(e.DwSendID)])
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				m := msg.AsAssignedObjectID()
				if uint32(m.DwRequestID) != reqSpawn {
					continue
				}
				obj = uint32(m.DwObjectID)
				fleet.Acknowledge(reqSpawn, obj)
				track("release", fleet.ReleaseControl(obj, reqRel))
				for _, n := range []string{"FREEZE_LATITUDE_LONGITUDE_SET", "FREEZE_ALTITUDE_SET", "FREEZE_ATTITUDE_SET"} {
					event(obj, n, 1)
				}
				g := [1]float64{1}
				track("gear handle", client.SetDataOnSimObject(defGear, obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&g)))
				event(obj, "GEAR_DOWN", 0)
				f := [1]float64{4}
				track("flaps handle", client.SetDataOnSimObject(defFlaps, obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&f)))
				switch *flapsMode {
				case "down":
					event(obj, "FLAPS_DOWN", 0)
				case "set":
					event(obj, "FLAPS_SET", 16383)
				case "incr":
					for range 5 {
						event(obj, "FLAPS_INCR", 0)
					}
				case "surface":
					s := [4]float64{100, 100, 100, 100}
					track("flap surfaces", client.SetDataOnSimObject(defSurf, obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 32, unsafe.Pointer(&s)))
				}
				for _, n := range []string{"LANDING_LIGHTS_SET", "STROBES_SET", "NAV_LIGHTS_SET", "BEACON_LIGHTS_SET"} {
					event(obj, n, 1)
				}
				track("monitor", client.RequestDataOnSimObject(reqMon, defMon, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0))
				flying, start, lastTick = true, time.Now(), time.Now()
				fmt.Printf("🆔 object %d taken over on final\n", obj)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				if uint32(d.DwRequestID) == reqMon {
					mon, haveMon = *engine.CastDataAs[monitor](&d.DwData), true
				}
			}
		}
	}
}
