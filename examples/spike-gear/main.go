//go:build windows
// +build windows

// Command spike-gear is the #301 gear matrix test: five non-ATC aircraft on
// final for one runway, each lowering its gear a different way, to find which
// call is needed and which one raises an exception. Every call's send ID is
// recorded with GetLastSentPacketID, so exceptions name the exact call.
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sort"
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
	defWaypoints uint32 = 9100
	defMonitor   uint32 = 9101
	defHandle    uint32 = 9102
	defPos       uint32 = 9103
	evtGearDown  uint32 = 9300
	reqBase      uint32 = 9200 // + 10*i: spawn, release, monitor, remove
)

type variant struct {
	name               string
	handle, pos, event bool
	nm                 float64 // spawn distance on final
}

type monitor struct {
	Lat, Lon, AGL, GS, OnGround, Handle, Center, Total float64
}

type plane struct {
	variant
	idx        int
	objectID   uint32
	airborne   bool
	touchdown  string
	gearDownAt string
	done       bool
	last       monitor
}

func main() {
	icao := flag.String("icao", "LKPR", "airport")
	runway := flag.String("runway", "24", "runway end")
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	maxRun := flag.Duration("max", 9*time.Minute, "remove all aircraft and exit after this long")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, cancelMax := context.WithTimeout(ctx, *maxRun)
	defer cancelMax()

	client := simconnect.NewClient("GO Spike - gear matrix", engine.WithContext(ctx))
	for client.Connect() != nil {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	defer client.Disconnect()

	// sent records what each send ID was, for attributing exceptions.
	sent := map[uint32]string{}
	track := func(desc string, err error) {
		id, idErr := client.GetLastSentPacketID()
		if idErr != nil {
			fmt.Printf("   GetLastSentPacketID: %v\n", idErr)
		}
		sent[id] = desc
		if err != nil {
			fmt.Printf("   %s → immediate error %v\n", desc, err)
		}
	}

	track("AddToDataDefinition AI Waypoint List", client.AddToDataDefinition(defWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0))
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"GROUND VELOCITY", "knots"}, {"SIM ON GROUND", "bool"}, {"GEAR HANDLE POSITION", "bool"},
		{"GEAR CENTER POSITION", "percent over 100"}, {"GEAR TOTAL PCT EXTENDED", "percent over 100"},
	} {
		track("AddToDataDefinition monitor "+v.n, client.AddToDataDefinition(defMonitor, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)))
	}
	track("AddToDataDefinition GEAR HANDLE POSITION", client.AddToDataDefinition(defHandle, "GEAR HANDLE POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0))
	for i, n := range []string{"GEAR CENTER POSITION", "GEAR LEFT POSITION", "GEAR RIGHT POSITION"} {
		track("AddToDataDefinition "+n, client.AddToDataDefinition(defPos, n, "percent over 100", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)))
	}
	track("MapClientEventToSimEvent GEAR_DOWN", client.MapClientEventToSimEvent(evtGearDown, "GEAR_DOWN"))

	planes := []*plane{
		{variant: variant{"all", true, true, true, 5}},
		{variant: variant{"event", false, false, true, 8}},
		{variant: variant{"handle", true, false, false, 11}},
		{variant: variant{"position", false, true, false, 14}},
		{variant: variant{"control", false, false, false, 17}},
	}
	for i, p := range planes {
		p.idx = i
	}
	byReq := func(req uint32) (*plane, uint32) {
		if req < reqBase || req >= reqBase+uint32(10*len(planes)) {
			return nil, 0
		}
		return planes[(req-reqBase)/10], (req - reqBase) % 10
	}

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	loader.Request(*icao)
	fleet := traffic.NewFleet(client)

	var (
		layout *airport.Layout
		rwy    airport.Runway
		end    airport.RunwayEnd
		elev   float64
		start  = time.Now()
	)
	stamp := func() string { return fmt.Sprintf("%3.0fs", time.Since(start).Seconds()) }

	approach := func(p *plane) []types.SIMCONNECT_DATA_WAYPOINT {
		var wps []types.SIMCONNECT_DATA_WAYPOINT
		agl := uint32(types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED | types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL | types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED)
		for nm := math.Floor(p.nm) - 1; nm >= 1; nm-- {
			lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, math.Mod(end.Heading+180, 360), nm*1852)
			wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: 318 * nm, Flags: agl, KtsSpeed: 135})
		}
		wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: end.Threshold.Lat, Longitude: end.Threshold.Lon, Altitude: 50, Flags: agl, KtsSpeed: 130})
		for _, s := range []struct{ m, kts float64 }{{300, 120}, {1000, 60}, {1600, 30}, {2000, 15}} {
			lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, s.m)
			wps = append(wps, traffic.TaxiWaypoint(lat, lon, elev, s.kts))
		}
		return wps
	}

	remove := func(p *plane, why string) {
		if p.done || p.objectID == 0 {
			return
		}
		p.done = true
		err := fleet.Remove(p.objectID, reqBase+uint32(10*p.idx)+3)
		track(fmt.Sprintf("AIRemoveObject [%s]", p.name), err)
		fmt.Printf("%s 🗑️  [%s] removed: %s\n", stamp(), p.name, why)
	}

	summary := func() {
		fmt.Println("\n══ Summary ══")
		for _, p := range planes {
			fmt.Printf("%-9s handle=%-5v pos=%-5v event=%-5v │ gear down: %-28s │ touchdown: %s\n",
				p.name, p.handle, p.pos, p.event, orDash(p.gearDownAt), orDash(p.touchdown))
		}
		ids := make([]int, 0, len(sent))
		for id := range sent {
			ids = append(ids, int(id))
		}
		sort.Ints(ids)
		fmt.Printf("(%d calls tracked, send IDs %d–%d)\n", len(ids), ids[0], ids[len(ids)-1])
	}

	stream := client.Stream()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			for _, p := range planes {
				remove(p, "time limit")
			}
			summary()
			time.Sleep(time.Second)
			return
		case <-tick.C:
			if layout == nil {
				continue
			}
			var b strings.Builder
			fmt.Fprintf(&b, "%s", stamp())
			for _, p := range planes {
				m := p.last
				fmt.Fprintf(&b, " │ %-8s agl%5.0f gs%4.0f gnd%.0f h%.0f c%.2f t%.2f", p.name, m.AGL, m.GS, m.OnGround, m.Handle, m.Center, m.Total)
			}
			fmt.Println(b.String())
			allDone := true
			for _, p := range planes {
				allDone = allDone && p.done
			}
			if allDone {
				summary()
				return
			}
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
				layout = res.Layout
				rwy, end, _ = layout.RunwayEnd(*runway)
				elev = convert.MetersToFeet(layout.Altitude)
				for _, p := range planes {
					lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, math.Mod(end.Heading+180, 360), p.nm*1852)
					err := fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: "G" + strings.ToUpper(p.name[:3]), Position: types.SIMCONNECT_DATA_INITPOSITION{
						Latitude: lat, Longitude: lon, Altitude: elev + 318*p.nm, Heading: end.Heading, Airspeed: 150,
					}}, reqBase+uint32(10*p.idx))
					track(fmt.Sprintf("AICreateNonATCAircraft [%s]", p.name), err)
				}
				fmt.Printf("🛬 5 aircraft on final %s at 5/8/11/14/17 nm (%s)\n", end.Name, rwy.Name())
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				desc, ok := sent[uint32(e.DwSendID)]
				if !ok {
					desc = "(untracked call)"
				}
				fmt.Printf("%s ⚠️  exception %d on send %d, index %d: %s\n", stamp(), e.DwException, e.DwSendID, e.DwIndex, desc)
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				m := msg.AsAssignedObjectID()
				p, off := byReq(uint32(m.DwRequestID))
				if p == nil || off != 0 {
					continue
				}
				p.objectID = uint32(m.DwObjectID)
				fleet.Acknowledge(uint32(m.DwRequestID), p.objectID)
				base := reqBase + uint32(10*p.idx)
				track(fmt.Sprintf("AIReleaseControl [%s]", p.name), fleet.ReleaseControl(p.objectID, base+1))
				if p.handle {
					v := [1]float64{1}
					track(fmt.Sprintf("SetDataOnSimObject GEAR HANDLE POSITION=1 [%s]", p.name),
						client.SetDataOnSimObject(defHandle, p.objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(v)), unsafe.Pointer(&v)))
				}
				if p.pos {
					v := [3]float64{1, 1, 1}
					track(fmt.Sprintf("SetDataOnSimObject GEAR CENTER/LEFT/RIGHT POSITION=1 [%s]", p.name),
						client.SetDataOnSimObject(defPos, p.objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(v)), unsafe.Pointer(&v)))
				}
				if p.event {
					track(fmt.Sprintf("TransmitClientEvent GEAR_DOWN [%s]", p.name),
						client.TransmitClientEvent(p.objectID, evtGearDown, 0, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY))
				}
				track(fmt.Sprintf("SetWaypoints approach [%s]", p.name), fleet.SetWaypoints(p.objectID, defWaypoints, approach(p)))
				track(fmt.Sprintf("RequestDataOnSimObject monitor [%s]", p.name),
					client.RequestDataOnSimObject(base+2, defMonitor, p.objectID, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0))
				fmt.Printf("%s 🆔 [%s] object %d\n", stamp(), p.name, p.objectID)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				p, off := byReq(uint32(d.DwRequestID))
				if p == nil || off != 2 || p.done {
					continue
				}
				m := *engine.CastDataAs[monitor](&d.DwData)
				p.last = m
				if m.OnGround == 0 {
					p.airborne = true
				}
				if p.gearDownAt == "" && m.Total > 0.99 {
					p.gearDownAt = fmt.Sprintf("%s (handle %.0f)", stamp(), m.Handle)
					fmt.Printf("%s ⚙️  [%s] gear fully extended\n", stamp(), p.name)
				}
				thr := calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, m.Lat, m.Lon)
				if p.airborne && m.OnGround != 0 && p.touchdown == "" {
					p.touchdown = fmt.Sprintf("%.0f m past threshold at %.0f kt", thr, m.GS)
					fmt.Printf("%s 🛬 [%s] touchdown %s\n", stamp(), p.name, p.touchdown)
				}
				past := calc.AlongTrackMeters(end.Threshold.Lat, end.Threshold.Lon, rwy.Center.Lat, rwy.Center.Lon, m.Lat, m.Lon)
				switch {
				case p.touchdown != "" && m.GS < 30:
					remove(p, "landed and slowed below 30 kt")
				case p.touchdown == "" && past > rwy.Length+300:
					p.touchdown = "none — passed the runway end airborne"
					remove(p, "floated past the runway end")
				}
			}
		}
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
