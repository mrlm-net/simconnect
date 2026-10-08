//go:build windows
// +build windows

// Command spike-tcas puts an intruder head-on with one of the map's own
// aircraft: a non-ATC aircraft at the same level, nm ahead on the
// reciprocal track, flown by waypoints. It then polls the map's /api/tcas
// for two minutes and removes the intruder (#450: TCAS seen live). Usage:
// spike-tcas TAIL [nm] [title].
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const mapURL = "http://127.0.0.1:8080"

type seen struct {
	Tail     string  `json:"tail"`
	Ours     bool    `json:"ours"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Alt      float64 `json:"alt"`
	Heading  float64 `json:"heading"`
	GS       float64 `json:"groundKts"`
	ObjectID uint32  `json:"objectId"`
}

func traffic0() ([]seen, error) {
	r, err := http.Get(mapURL + "/api/traffic")
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	var out []seen
	return out, json.NewDecoder(r.Body).Decode(&out)
}

func main() {
	tail := os.Args[1]
	nm, title := 12.0, "FSLTL_FAIB_B738_TVS-Skytravel_TSOC"
	if len(os.Args) > 2 {
		nm, _ = strconv.ParseFloat(os.Args[2], 64)
	}
	if len(os.Args) > 3 {
		title = os.Args[3]
	}
	all, err := traffic0()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var t seen
	for _, a := range all {
		if a.Tail == tail {
			t = a
		}
	}
	if t.Tail == "" {
		fmt.Fprintln(os.Stderr, "no", tail)
		os.Exit(1)
	}
	lat, lon := calc.DisplaceByHeading(t.Lat, t.Lon, t.Heading, nm*1852)
	back := math.Mod(t.Heading+180, 360)
	endLat, endLon := calc.DisplaceByHeading(lat, lon, back, 80*1852)
	spawn, wps, err := traffic.EnrouteStart([]traffic.RoutePoint{
		{Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: t.Alt, Kts: 450},
		{Position: airport.LatLon{Lat: endLat, Lon: endLon}, AltFt: t.Alt, Kts: 450},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-tcas", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const req, def = 6800, 6800
	client.AddToDataDefinition(def, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0)
	fleet := traffic.NewFleet(client)
	if err := fleet.RequestNonATC(traffic.NonATCOpts{Model: title, Tail: "TCAS1", Position: spawn}, req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("intruder %.1f NM ahead of %s at %.0f ft, heading %.0f; %s at %.0f°\n", nm, tail, t.Alt, back, tail, t.Heading)
	var obj uint32
	poll := time.NewTicker(2 * time.Second)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			if obj != 0 {
				_ = fleet.Remove(obj, req+1)
				time.Sleep(300 * time.Millisecond)
			}
			return
		case m := <-client.Stream():
			if m.SIMCONNECT_RECV == nil {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(m.DwID) {
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				a := m.AsAssignedObjectID()
				if a, ok := fleet.Acknowledge(uint32(a.DwRequestID), uint32(a.DwObjectID)); ok {
					obj = a.ObjectID
					fmt.Println("intruder object", obj, fleet.SetWaypoints(obj, def, wps))
				}
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := m.AsException()
				fmt.Println("exception", e.DwException, "send", e.DwSendID)
			}
		case <-poll.C:
			r, err := http.Get(mapURL + "/api/tcas")
			if err != nil {
				continue
			}
			var v struct {
				Events []map[string]any `json:"events"`
				RA, TA int
			}
			_ = json.NewDecoder(r.Body).Decode(&v)
			r.Body.Close()
			var gap string
			if all, err := traffic0(); err == nil {
				var me, it seen
				for _, a := range all {
					switch {
					case a.Tail == tail:
						me = a
					case a.ObjectID == obj && obj != 0:
						it = a
					}
				}
				if it.ObjectID != 0 {
					gap = fmt.Sprintf("%.1f NM, %.0f ft", calc.HaversineNM(me.Lat, me.Lon, it.Lat, it.Lon), me.Alt-it.Alt)
				}
			}
			fmt.Printf("%s TA %d RA %d %s events %v\n", time.Now().Format("15:04:05"), v.TA, v.RA, gap, v.Events)
		}
	}
}
