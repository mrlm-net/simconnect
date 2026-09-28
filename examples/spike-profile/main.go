//go:build windows
// +build windows

// Command spike-profile reads the type SimVars traffic.ProfileReader asks
// for (#325) from the user aircraft and from AI aircraft around it, and
// prints them with the profile ProfileFor resolves from the title and what
// Refine makes of it: which of these do AI objects report reliably? It
// only reads.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type titleData struct {
	Title    [256]byte
	OnGround float64
}

func main() {
	radius := flag.Uint("radius", 10000, "meters around the user aircraft")
	n := flag.Int("n", 1, "AI aircraft to read")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	client := simconnect.NewClient("GO Spike - profile", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()

	const titleDef, titleReq = 1, 1
	client.AddToDataDefinition(titleDef, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
	client.AddToDataDefinition(titleDef, "SIM ON GROUND", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 1)
	// The user first, then the AI around it (the scan includes the user).
	client.RequestDataOnSimObjectType(titleReq+1, titleDef, 0, types.SIMCONNECT_SIMOBJECT_TYPE_USER)

	pr := traffic.NewProfileReader(client, 2, 3)
	titles := map[uint32]string{}
	requested := map[uint32]bool{}
	ai := 0
	userID := uint32(0)
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-client.Stream():
			if msg.SIMCONNECT_RECV == nil {
				continue
			}
			if v, ok := pr.Handle(msg); ok {
				report(titles[v.ObjectID], v)
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("exception %d (index %d)\n", e.DwException, e.DwIndex)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
				d := msg.AsSimObjectDataBType()
				obj := uint32(d.DwObjectID)
				t := *engine.CastDataAs[titleData](&d.DwData)
				titles[obj] = engine.BytesToString(t.Title[:])
				user := uint32(d.DwRequestID) == titleReq+1
				if user {
					userID = obj
					fmt.Printf("user aircraft is object %d %q\n", obj, titles[obj])
					client.RequestDataOnSimObjectType(titleReq, titleDef, uint32(*radius), types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
					obj = 0 // SIMCONNECT_OBJECT_ID_USER
				} else if obj == userID || ai >= *n || requested[obj] {
					continue
				} else {
					ai++
				}
				requested[obj] = true
				if err := pr.Request(obj); err != nil {
					fmt.Println("request:", err)
				}
			}
		}
	}
}

func report(title string, v traffic.SimVarData) {
	fmt.Printf("\nobject %d  %q\n", v.ObjectID, title)
	fmt.Printf("  ATC MODEL %q  ATC TYPE %q  CATEGORY %q\n", v.ATCModel, v.ATCType, v.Category)
	fmt.Printf("  span %.1f m  CG %.2f m  VS0 %.0f  VS1 %.0f  TO %.0f  climb %.0f  VC %.0f kt\n",
		v.WingspanM, v.CGHeightM, v.VS0Kts, v.VS1Kts, v.TakeoffKts, v.ClimbKts, v.CruiseKts)
	fmt.Printf("  engines %d type %d  max gross %.0f kg  total %.0f kg  flap positions %d\n",
		v.Engines, v.EngineType, v.MaxGrossKg, v.TotalWeightKg, v.FlapPositions)
	p := traffic.ProfileFor(title)
	if p.Type == "" {
		p = traffic.ProfileFor(v.ATCModel)
	}
	r := traffic.Refine(p, v)
	fmt.Printf("  profile %q (code %c): Vapp %.0f Vr %.0f flaps %.1f/%.1f/%.1f -> refined %q Vapp %.0f Vr %.0f accel %.2f flaps %.1f/%.1f/%.1f CG %.2f\n",
		p.Type, p.ICAOCode, p.Approach.ApproachKts, p.Takeoff.RotateKts, p.Flaps.TakeoffPct, p.Flaps.ApproachPct, p.Flaps.LandingPct,
		r.Type, r.Approach.ApproachKts, r.Takeoff.RotateKts, r.Takeoff.RollAccel, r.Flaps.TakeoffPct, r.Flaps.ApproachPct, r.Flaps.LandingPct, r.CGHeightM)
}
