//go:build windows
// +build windows

// Command spike-placeheight checks what decides the height of an aircraft
// placed on the ground (SetDataOnSimObject with an INITPOSITION): it
// creates one on a stand, takes it over and places it every frame: at its
// rest height and pitch standing (3-8 s), moving back 1 m/s (8-13 s), the
// same 0.05° nose down (13-18 s), stopped (18-19 s), with its engines
// started (19-25 s), then lets it go; printing the height shown above the
// ground, pitch, SIM ON GROUND and the gear every frame. WAIT=60 lets it
// stand that long first. Usage: spike-placeheight <stand> [title].
//
// Measured live (MSFS 2024, LKPR C24, A320 and B738): only the pitch
// changes the height: 0.05° off its rest pitch, a B738 shows 8.48 → 9.05 ft
// (extended struts), 1° 9.75; moving, stopped, engines on, OnGround 0 or a
// foot lower asked: 8.48 (MovingPitchDeg, #892).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
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

type frame struct{ Alt, Ground, StaticCG, Pitch, OnGround, Gear float64 }

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
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-placeheight", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def, req, spawnReq, posDef = 6600, 6601, 6602, 6603
	for i, v := range []struct{ n, u string }{
		{"PLANE ALTITUDE", "feet"}, {"GROUND ALTITUDE", "feet"}, {"STATIC CG TO GROUND", "feet"},
		{"PLANE PITCH DEGREES", "degrees"}, {"SIM ON GROUND", "bool"}, {"GEAR TOTAL PCT EXTENDED", "percent over 100"},
	} {
		client.AddToDataDefinition(def, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.AddToDataDefinition(posDef, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0)
	fleet := traffic.NewFleet(client)
	inj := traffic.NewInjector(client)
	model, livery, _ := strings.Cut(title, " :: ")
	if err := fleet.RequestNonATC(traffic.NonATCOpts{Model: model, Livery: livery, Tail: "PLH1", Position: types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: at.Lat, Longitude: at.Lon, Altitude: convert.MetersToFeet(l.Altitude), Heading: stand.Heading, OnGround: 1,
	}}, spawnReq); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	start := time.Now()
	var obj uint32
	rest, pitch := 0.0, 0.0
	enginesOn := false
	wait := 0.0
	if w := os.Getenv("WAIT"); w != "" {
		fmt.Sscanf(w, "%g", &wait)
	}
	fmt.Println("t,phase,shown_above_ground,static_cg,pitch,on_ground,gear")
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
			if !ok || m.SIMCONNECT_RECV == nil {
				continue
			}
			if handled, _ := inj.Handle(m); handled {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(m.DwID) {
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				a := m.AsAssignedObjectID()
				if ac, ok := fleet.Acknowledge(uint32(a.DwRequestID), uint32(a.DwObjectID)); ok {
					obj = ac.ObjectID
					_ = inj.Takeover(obj)
					client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
				}
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := m.AsSimObjectData()
				if uint32(d.DwRequestID) != req {
					continue
				}
				f := engine.CastDataAs[frame](&d.DwData)
				t := time.Since(start).Seconds() - wait
				if rest == 0 {
					rest, pitch = f.Alt-f.Ground, f.Pitch
				}
				phase, place, onGround, off, dPitch := "standing", false, types.DWORD(1), 0.0, 0.0
				switch {
				case t >= 3 && t < 8:
					phase, place = "og1-rest", true
				case t >= 8 && t < 13:
					phase, place, off = "og1-moving", true, 0
				case t >= 13 && t < 18:
					phase, place, dPitch = "og1-moving-pitch+tiny", true, 0.05
				case t >= 18 && t < 19:
					phase, place = "og1-stop", true
					if !enginesOn {
						enginesOn = true
						_ = inj.SetEngines(obj, 2, true)
					}
				case t >= 19 && t < 25:
					phase, place = "og1-engines-on", true
				case t >= 25:
					phase = "let go"
				}
				if place {
					pos := at
					if t >= 8 { // backwards along the stand, 1 m/s, then stopped
						pos.Lat, pos.Lon = calc.DisplaceByHeading(at.Lat, at.Lon, stand.Heading+180, math.Min(t, 18)-8)
					}
					p := types.SIMCONNECT_DATA_INITPOSITION{Latitude: pos.Lat, Longitude: pos.Lon, Altitude: f.Ground + rest + off,
						Pitch: pitch + dPitch, Heading: stand.Heading, OnGround: onGround}
					client.SetDataOnSimObject(posDef, obj, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
				}
				fmt.Printf("%.3f,%s,%.2f,%.2f,%.2f,%.0f,%.2f\n", t, phase, f.Alt-f.Ground, f.StaticCG, f.Pitch, f.OnGround, f.Gear)
			}
		}
	}
}
