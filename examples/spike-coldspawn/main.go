//go:build windows
// +build windows

// Command spike-coldspawn measures how a NonATC departure appears on its
// stand (title "Model :: livery" or a model alone): when the object is
// assigned, whether its engines read running from the first frame, and how
// they run down once switched off (combustion, N1, gear). Modes: "stand"
// creates it on the stand and switches the engines off at once, as
// departures do; "raw" leaves it as MSFS creates it; "under" (300 ft below
// the stand) and "high" (10,000 ft above) switch them off there and place
// it on the stand 5 s later. Usage: spike-coldspawn <stand>
// <stand|raw|under|high> [title]; one CSV line per frame, 60 s.
//
// Measured live (MSFS 2024, LKPR): raw, N1 19.8 % on the first frame, 30 %
// idle by 3 s; stand, assigned in 32-48 ms (an A330 livery not loaded
// before too), combustion 0 and N1 0 from the first frame, and watched on
// the stand: no sound, no spin; under, lifted onto the stand by the sim
// within 0.9 s; high, held frozen, placed on the stand cold with the gear
// down.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type frame struct{ Comb1, Comb2, N1a, N1b, Alt, Ground, OnGround, Gear float64 }

func main() {
	standName, mode := os.Args[1], os.Args[2]
	title := "FSLTL A320 Air France SL"
	if len(os.Args) > 3 {
		title = os.Args[3]
	}
	b, err := os.ReadFile("pkg/airport/testdata/LKPR.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-coldspawn", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def, req, spawnReq = 6300, 6301, 6302
	for i, v := range []struct{ n, u string }{
		{"GENERAL ENG COMBUSTION:1", "bool"}, {"GENERAL ENG COMBUSTION:2", "bool"},
		{"TURB ENG N1:1", "percent"}, {"TURB ENG N1:2", "percent"},
		{"PLANE ALTITUDE", "feet"}, {"GROUND ALTITUDE", "feet"}, {"SIM ON GROUND", "bool"}, {"GEAR TOTAL PCT EXTENDED", "percent"},
	} {
		client.AddToDataDefinition(def, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	fleet := traffic.NewFleet(client)
	inj := traffic.NewInjector(client)
	alt := convert.MetersToFeet(l.Altitude)
	onGround := types.DWORD(1)
	if mode == "high" {
		alt += 10000
		onGround = 0
	}
	if mode == "under" {
		alt -= 300
		onGround = 0
	}
	model, livery, _ := strings.Cut(title, " :: ")
	start := time.Now()
	if err := fleet.RequestNonATC(traffic.NonATCOpts{Model: model, Livery: livery, Tail: "COLD1", Position: types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: at.Lat, Longitude: at.Lon, Altitude: alt, Heading: stand.Heading, OnGround: onGround,
	}}, spawnReq); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ms := func() int64 { return time.Since(start).Milliseconds() }
	var obj uint32
	var offAt, placedAt int64 = -1, -1
	fmt.Println("ms,event,comb1,comb2,n1a,n1b,alt,ground,on_ground,gear")
	for {
		select {
		case <-ctx.Done():
			if obj != 0 {
				_ = inj.Release(obj)
				_ = fleet.Remove(obj, spawnReq+1)
				time.Sleep(300 * time.Millisecond)
			}
			return
		case m, ok := <-client.Stream():
			if !ok {
				return
			}
			if m.SIMCONNECT_RECV == nil {
				continue
			}
			if handled, err := inj.Handle(m); handled {
				if err != nil {
					fmt.Printf("%d,injector error %v\n", ms(), err)
				}
				continue
			}
			switch types.SIMCONNECT_RECV_ID(m.DwID) {
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				a := m.AsAssignedObjectID()
				if ac, ok := fleet.Acknowledge(uint32(a.DwRequestID), uint32(a.DwObjectID)); ok {
					obj = ac.ObjectID
					fmt.Printf("%d,assigned %d\n", ms(), obj)
					client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
					if mode == "raw" {
						break // as MSFS creates it: no takeover, the engines left alone
					}
					fmt.Printf("%d,takeover %v\n", ms(), inj.Takeover(obj))
					fmt.Printf("%d,engines off %v\n", ms(), inj.SetEngines(obj, 2, false))
					offAt = ms()
				}
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := m.AsSimObjectData()
				if uint32(d.DwRequestID) != req {
					continue
				}
				f := engine.CastDataAs[frame](&d.DwData)
				fmt.Printf("%d,frame,%.0f,%.0f,%.1f,%.1f,%.0f,%.0f,%.0f,%.0f\n", ms(), f.Comb1, f.Comb2, f.N1a, f.N1b, f.Alt, f.Ground, f.OnGround, f.Gear)
				// Under the ground: on the stand once the engines are down, and 5 s
				// for what the model shows of them (fans, sound) to run down.
				if (mode == "under" || mode == "high") && placedAt < 0 && offAt >= 0 && ms()-offAt > 5000 && f.Comb1 == 0 && f.N1a < 3 {
					err := inj.Place(obj, traffic.GroundPose{Position: at, Heading: stand.Heading, Stopped: true})
					placedAt = ms()
					fmt.Printf("%d,placed on the stand %v\n", placedAt, err)
				}
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := m.AsException()
				fmt.Printf("%d,exception %d send %d\n", ms(), e.DwException, e.DwSendID)
			}
		}
	}
}
