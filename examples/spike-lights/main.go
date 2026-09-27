//go:build windows
// +build windows

// Command spike-lights tests how to switch the lights of a non-ATC AI aircraft
// (#295): four aircraft on remote stands, each trying one method to turn the
// taxi light on, with every call's send ID tracked and the lights read back
// every second.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	defLights  uint32 = 9700
	defTaxi    uint32 = 9701
	defWp      uint32 = 9702
	evtTaxiSet uint32 = 9800
	evtTaxiTgl uint32 = 9801
	evtLandOn  uint32 = 9802
	reqBase    uint32 = 9900 // + 10*i: spawn, release, monitor, remove
)

type lights struct{ Landing, Taxi, Strobe, Beacon, Nav float64 }

func (l lights) String() string {
	var b strings.Builder
	for _, x := range []struct {
		v float64
		c byte
	}{{l.Landing, 'L'}, {l.Taxi, 'T'}, {l.Strobe, 'S'}, {l.Beacon, 'B'}, {l.Nav, 'N'}} {
		if x.v != 0 {
			b.WriteByte(x.c)
		} else {
			b.WriteByte('.')
		}
	}
	return b.String()
}

func main() {
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	secs := flag.Int("seconds", 60, "how long to watch")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := simconnect.NewClient("GO Spike - lights", engine.WithContext(ctx))
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
	for i, n := range []string{"LIGHT LANDING", "LIGHT TAXI", "LIGHT STROBE", "LIGHT BEACON", "LIGHT NAV"} {
		track("def "+n, client.AddToDataDefinition(defLights, n, "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)))
	}
	track("def LIGHT TAXI (write)", client.AddToDataDefinition(defTaxi, "LIGHT TAXI", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0))
	track("def waypoints", client.AddToDataDefinition(defWp, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0))
	track("map TAXI_LIGHTS_SET", client.MapClientEventToSimEvent(evtTaxiSet, "TAXI_LIGHTS_SET"))
	track("map TOGGLE_TAXI_LIGHTS", client.MapClientEventToSimEvent(evtTaxiTgl, "TOGGLE_TAXI_LIGHTS"))
	track("map LANDING_LIGHTS_ON", client.MapClientEventToSimEvent(evtLandOn, "LANDING_LIGHTS_ON"))

	methods := []struct {
		name, stand string
		apply       func(obj uint32, i int)
	}{
		{"simvar LIGHT TAXI=1", "N50", func(obj uint32, i int) {
			v := [1]float64{1}
			track("SetData LIGHT TAXI", client.SetDataOnSimObject(defTaxi, obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&v)))
		}},
		{"event TAXI_LIGHTS_SET 1", "N51", func(obj uint32, i int) {
			track("TAXI_LIGHTS_SET", client.TransmitClientEvent(obj, evtTaxiSet, 1, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY))
		}},
		{"event TOGGLE_TAXI_LIGHTS", "N52", func(obj uint32, i int) {
			track("TOGGLE_TAXI_LIGHTS", client.TransmitClientEvent(obj, evtTaxiTgl, 0, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY))
		}},
		{"event LANDING_LIGHTS_ON", "N53", func(obj uint32, i int) {
			track("LANDING_LIGHTS_ON", client.TransmitClientEvent(obj, evtLandOn, 0, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY))
		}},
	}
	objs := make([]uint32, len(methods))
	history := make([][]string, len(methods))

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	loader.Request("LKPR")
	fleet := traffic.NewFleet(client)
	var start time.Time
	stream := client.Stream()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if start.IsZero() || time.Since(start) < time.Duration(*secs)*time.Second {
				continue
			}
			fmt.Println("\n══ Light history (L T S B N per second after applying) ══")
			for i, m := range methods {
				fmt.Printf("%-26s %s\n", m.name, strings.Join(history[i], " "))
				fleet.Remove(objs[i], reqBase+uint32(10*i)+3)
			}
			time.Sleep(time.Second)
			return
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil || msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				l := res.Layout
				for i, m := range methods {
					k, err := l.ParkingIndex(m.stand)
					if err != nil {
						fmt.Println(err)
						return
					}
					p := l.Parking[k]
					fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: fmt.Sprintf("LT%d", i), Position: types.SIMCONNECT_DATA_INITPOSITION{
						Latitude: p.Position.Lat, Longitude: p.Position.Lon, Altitude: convert.MetersToFeet(l.Altitude), Heading: p.Heading, OnGround: 1,
					}}, reqBase+uint32(10*i))
				}
				fmt.Println("🅿️  4 aircraft on N50–N53")
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("⚠️  exception %d on %q (param %d)\n", e.DwException, sent[uint32(e.DwSendID)], e.DwIndex)
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				m := msg.AsAssignedObjectID()
				r := uint32(m.DwRequestID)
				if r < reqBase || (r-reqBase)%10 != 0 || int((r-reqBase)/10) >= len(methods) {
					continue
				}
				i := int((r - reqBase) / 10)
				objs[i] = uint32(m.DwObjectID)
				fleet.Acknowledge(r, objs[i])
				track(fmt.Sprintf("release %d", i), fleet.ReleaseControl(objs[i], r+1))
				methods[i].apply(objs[i], i)
				track(fmt.Sprintf("monitor %d", i), client.RequestDataOnSimObject(r+2, defLights, objs[i], types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0))
				if start.IsZero() {
					start = time.Now()
				}
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				r := uint32(d.DwRequestID)
				if r < reqBase || (r-reqBase)%10 != 2 || int((r-reqBase)/10) >= len(methods) {
					continue
				}
				i := int((r - reqBase) / 10)
				history[i] = append(history[i], engine.CastDataAs[lights](&d.DwData).String())
			}
		}
	}
}
