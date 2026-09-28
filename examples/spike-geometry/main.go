//go:build windows
// +build windows

// Command spike-geometry prints what the simulator reports about each
// aircraft's geometry: the gear contact points relative to the reference
// point, wing span and the CG height, to derive nose gear position,
// wheelbase and span per aircraft instead of assuming an A320 (#304, #324).
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
	"github.com/mrlm-net/simconnect/pkg/types"
)

type geometry struct {
	Title        [256]byte
	Contact      [4]types.SIMCONNECT_DATA_XYZ // feet: x right, y up, z forward
	WingSpanFt   float64
	CGToGroundFt float64
	NContacts    float64
}

func main() {
	radius := flag.Uint("radius", 5000, "meters around the user aircraft")
	probe := flag.Bool("probe", false, "probe simvar names on the user aircraft")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	client := simconnect.NewClient("GO Spike - geometry", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()
	if *probe {
		probeNames(ctx, client, []string{
			"CONTACT POINT POSITION:0", "CONTACT POINT POSITION:1", "CONTACT POINT POSITION:2", "XYZ CONTACT POINT POSITION:3",
			"VISUAL MODEL RADIUS", "DESIGN SPAWN ALTITUDE DESCENT", "GEAR CENTER STEER ANGLE",
			"WING SPAN", "WING AREA", "STATIC CG TO GROUND", "CG PERCENT",
		})
		return
	}
	const def = 1
	id := uint32(0)
	add := func(name, unit string, t types.SIMCONNECT_DATATYPE) {
		client.AddToDataDefinition(def, name, unit, t, 0, id)
		id++
	}
	add("TITLE", "", types.SIMCONNECT_DATATYPE_STRING256)
	for i := 0; i < 4; i++ {
		add(fmt.Sprintf("CONTACT POINT POSITION:%d", i), "feet", types.SIMCONNECT_DATATYPE_XYZ)
	}
	add("WING SPAN", "feet", types.SIMCONNECT_DATATYPE_FLOAT64)
	add("STATIC CG TO GROUND", "feet", types.SIMCONNECT_DATATYPE_FLOAT64)
	add("NUMBER OF CONTACT POINTS", "number", types.SIMCONNECT_DATATYPE_FLOAT64)
	client.RequestDataOnSimObjectType(1, def, uint32(*radius), types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
	client.RequestDataOnSimObjectType(2, def, 0, types.SIMCONNECT_SIMOBJECT_TYPE_USER)
	seen := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-client.Stream():
			if msg.SIMCONNECT_RECV == nil {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("exception %d (index %d)\n", e.DwException, e.DwIndex)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
				d := msg.AsSimObjectDataBType()
				g := *engine.CastDataAs[geometry](&d.DwData)
				title := engine.BytesToString(g.Title[:])
				if seen[title] {
					continue
				}
				seen[title] = true
				fmt.Printf("%-45s span %6.1f ft, CG %5.1f ft, %2.0f contacts:", title, g.WingSpanFt, g.CGToGroundFt, g.NContacts)
				for i, c := range g.Contact {
					fmt.Printf("  [%d] x %6.1f y %6.1f z %6.1f", i, c.X, c.Y, c.Z)
				}
				fmt.Println()
			}
		}
	}
}
