//go:build windows
// +build windows

// Command spike-tugheight checks whether the pushback tug at the nose lifts
// the aircraft (the shake as a tug disconnects): it creates an aircraft on
// a stand and a tug (traffic.DefaultTugTitle) in front of its nose gear,
// facing it, then moves the tug a meter closer every 4 s, from 8 m to 1 m,
// and takes it away; printing the aircraft's height above the ground and
// pitch every frame. Usage: spike-tugheight <stand> [title].
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type frame struct{ Alt, Ground, Pitch float64 }

func main() {
	standName := os.Args[1]
	title := "FSLTL A320 Air France SL"
	if len(os.Args) > 2 {
		title = os.Args[2]
	}
	b, _ := os.ReadFile("pkg/airport/testdata/LKPR.json")
	var raw airport.RawAirport
	_ = json.Unmarshal(b, &raw)
	l, err := airport.BuildLayout(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	si, err := l.ParkingIndex(standName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	stand := l.Parking[si]
	at := traffic.StandPoint(stand, 0)
	model, livery, _ := strings.Cut(title, " :: ")
	nose := traffic.NoseGear(at, stand.Heading, traffic.ProfileFor(model).Motion)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-tugheight", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def, req, spawnReq, tugReq = 6700, 6701, 6702, 6703
	for i, v := range []struct{ n, u string }{{"PLANE ALTITUDE", "feet"}, {"GROUND ALTITUDE", "feet"}, {"PLANE PITCH DEGREES", "degrees"}} {
		client.AddToDataDefinition(def, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	fleet := traffic.NewFleet(client)
	inj := traffic.NewInjector(client)
	if err := fleet.RequestNonATC(traffic.NonATCOpts{Model: model, Livery: livery, Tail: "TUGH1", Position: types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: at.Lat, Longitude: at.Lon, Altitude: convert.MetersToFeet(l.Altitude), Heading: stand.Heading, OnGround: 1,
	}}, spawnReq); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tugAt := func(ahead float64) traffic.GroundPose {
		lat, lon := calc.DisplaceByHeading(nose.Lat, nose.Lon, stand.Heading, ahead)
		return traffic.GroundPose{Position: airport.LatLon{Lat: lat, Lon: lon}, Heading: math.Mod(stand.Heading+180, 360), Stopped: true}
	}
	start := time.Now()
	var obj, tug uint32
	tugAsked := false
	fmt.Println("t,tug_ahead_m,above_ground,pitch")
	for {
		select {
		case <-ctx.Done():
			for _, o := range []uint32{tug, obj} {
				if o != 0 {
					_ = inj.Release(o)
					_ = client.AIRemoveObject(o, 6790+o%10)
				}
			}
			time.Sleep(300 * time.Millisecond)
			return
		case m, ok := <-client.Stream():
			if !ok || m.SIMCONNECT_RECV == nil {
				continue
			}
			if handled, _ := inj.Handle(m); handled {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(m.DwID) {
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				a := m.AsAssignedObjectID()
				switch uint32(a.DwRequestID) {
				case tugReq:
					tug = uint32(a.DwObjectID)
					_ = inj.Takeover(tug)
				default:
					if ac, ok := fleet.Acknowledge(uint32(a.DwRequestID), uint32(a.DwObjectID)); ok {
						obj = ac.ObjectID
						_ = inj.Takeover(obj)
						client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
					}
				}
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := m.AsSimObjectData()
				if uint32(d.DwRequestID) != req {
					continue
				}
				f := engine.CastDataAs[frame](&d.DwData)
				t := time.Since(start).Seconds()
				ahead := -1.0 // no tug
				switch {
				case t >= 4 && t < 36:
					ahead = 8 - math.Floor((t-4)/4) // 8, 7, … 1 m
				case t >= 36 && t < 40:
					ahead = 20 // taken away
				}
				if ahead > 0 && !tugAsked {
					tugAsked = true
					p := tugAt(ahead).Position
					_ = client.AICreateSimulatedObject(traffic.DefaultTugTitle, types.SIMCONNECT_DATA_INITPOSITION{
						Latitude: p.Lat, Longitude: p.Lon, Heading: math.Mod(stand.Heading+180, 360), OnGround: 1}, tugReq)
				}
				if ahead > 0 && tug != 0 {
					_ = inj.PlaceMoving(tug, tugAt(ahead))
				}
				// PLACE_AC=1: the aircraft placed where it stands every frame too,
				// as a departure is from its push on.
				if os.Getenv("PLACE_AC") == "1" && obj != 0 && t >= 2 {
					_ = inj.Place(obj, traffic.GroundPose{Position: at, Heading: stand.Heading, Stopped: true})
				}
				fmt.Printf("%.3f,%.0f,%.2f,%.2f\n", t, ahead, f.Alt-f.Ground, f.Pitch)
			}
		}
	}
}
