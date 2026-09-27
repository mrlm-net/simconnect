//go:build windows
// +build windows

// Command spike-landing is the #289 spike: it compares two ways of landing an
// AI aircraft at LKPR and then tries to take it over on the ground.
//
//	-mode waypoint  non-ATC aircraft spawned on final, landed with waypoints
//	-mode atc       ATC arrival on a flight plan, landed by the simulator
//
// In both modes, once the aircraft is on the ground below -takeover-kts, the
// spike releases AI control and sends taxi-in waypoints to -stand. Every
// second a line is logged to -csv for analysis.
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
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
	defWaypoints uint32 = 8100
	defMonitor   uint32 = 8101
	defState     uint32 = 8102
	defGear      uint32 = 8103
	evtGearDown  uint32 = 8300
	reqSpawn     uint32 = 8200
	reqRelease   uint32 = 8201
	reqMonitor   uint32 = 8202
	reqState     uint32 = 8203
	reqRemove    uint32 = 8204
)

type monitor struct {
	Lat, Lon, AGL, Heading, GS, IAS, VS, OnGround, Pitch, AltMSL float64
}

func main() {
	mode := flag.String("mode", "waypoint", "waypoint | atc")
	icao := flag.String("icao", "LKPR", "airport")
	runway := flag.String("runway", "24", "runway end to land on (waypoint mode)")
	stand := flag.String("stand", "C22", "stand to taxi to after takeover")
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	from := flag.String("from", "LKHK", "departure airport of the ATC flight plan (atc mode)")
	phase := flag.Float64("phase", 0.85, "flight plan position 0..1 at spawn (atc mode)")
	takeoverKts := flag.Float64("takeover-kts", 35, "take over below this ground speed on the ground")
	maxRun := flag.Duration("max", 10*time.Minute, "remove the aircraft and exit after this long")
	variant := flag.String("variant", "a1", "waypoint landing variant: a1 (ground waypoints) | a2 (touchdown at 0 ft AGL, throttle 0)")
	scan := flag.Int("scan", 0, "list every aircraft within this many meters of the user aircraft, then exit")
	remove := flag.String("remove", "", "comma-separated object IDs to remove, then exit")
	afterRollout := flag.Bool("after-rollout", false, "take over only once the AI state has left STATE_LANDING/STATE_LANDING_ROLLOUT")
	csvPath := flag.String("csv", "", "CSV log path (default dumps/spike-<mode>.csv)")
	flag.Parse()
	if *csvPath == "" {
		*csvPath = filepath.Join(`C:\msfs-development\dumps`, "spike-"+*mode+".csv")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	ctx, cancelMax := context.WithTimeout(ctx, *maxRun)
	defer cancelMax()
	defer cancel()

	client := simconnect.NewClient("GO Spike - AI landing", engine.WithContext(ctx))
	for client.Connect() != nil {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	defer client.Disconnect()
	fmt.Println("✅ Connected")

	if *scan > 0 {
		// Every aircraft within scan meters of the user aircraft.
		const defScan, reqScan = 8110, 8210
		client.AddToDataDefinition(defScan, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
		client.AddToDataDefinition(defScan, "AI TRAFFIC STATE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 1)
		for i, v := range []struct{ n, u string }{{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"},
			{"PLANE ALT ABOVE GROUND", "feet"}, {"GROUND VELOCITY", "knots"}, {"PLANE HEADING DEGREES TRUE", "degrees"}} {
			client.AddToDataDefinition(defScan, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i+2))
		}
		type scanData struct {
			Title, State               [256]byte
			Lat, Lon, AGL, GS, Heading float64
		}
		client.RequestDataOnSimObjectType(reqScan, defScan, uint32(*scan), types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
		deadline := time.After(5 * time.Second)
		for {
			select {
			case msg := <-client.Stream():
				if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE {
					continue
				}
				d := msg.AsSimObjectDataBType()
				s := engine.CastDataAs[scanData](&d.DwData)
				fmt.Printf("✈️  obj %-10d %-40.40s agl %6.0f ft  gs %5.1f kt  hdg %3.0f  %.5f,%.5f  [%s]\n", d.DwObjectID,
					engine.BytesToString(s.Title[:]), s.AGL, s.GS, s.Heading, s.Lat, s.Lon, engine.BytesToString(s.State[:]))
			case <-deadline:
				return
			}
		}
	}

	if *remove != "" {
		for i, s := range strings.Split(*remove, ",") {
			var id uint32
			fmt.Sscan(strings.TrimSpace(s), &id)
			err := client.AIRemoveObject(id, reqRemove+uint32(i)+100)
			fmt.Printf("🗑️  AIRemoveObject(%d): %v\n", id, err)
		}
		deadline := time.After(3 * time.Second)
		for {
			select {
			case msg := <-client.Stream():
				if msg.SIMCONNECT_RECV != nil && types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EXCEPTION {
					e := msg.AsException()
					fmt.Printf("⚠️  exception %d sendID=%d\n", e.DwException, e.DwSendID)
				}
			case <-deadline:
				return
			}
		}
	}

	logf, err := os.Create(*csvPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer logf.Close()
	fmt.Fprintln(logf, "t,phase,lat,lon,agl_ft,alt_msl_ft,hdg,gs_kt,ias_kt,vs_fpm,on_ground,pitch,ai_state,dist_thr_m,cross_m")

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	fleet := traffic.NewFleet(client)
	if err := loader.Request(*icao); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	client.AddToDataDefinition(defWaypoints, "AI Waypoint List", "number", types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0)
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"PLANE HEADING DEGREES TRUE", "degrees"}, {"GROUND VELOCITY", "knots"}, {"AIRSPEED INDICATED", "knots"},
		{"VERTICAL SPEED", "feet per minute"}, {"SIM ON GROUND", "bool"}, {"PLANE PITCH DEGREES", "degrees"},
		{"PLANE ALTITUDE", "feet"},
	} {
		client.AddToDataDefinition(defMonitor, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.AddToDataDefinition(defState, "AI TRAFFIC STATE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
	for i, v := range []struct{ n, u string }{{"GEAR HANDLE POSITION", "bool"}, {"GEAR CENTER POSITION", "percent over 100"},
		{"GEAR LEFT POSITION", "percent over 100"}, {"GEAR RIGHT POSITION", "percent over 100"}} {
		client.AddToDataDefinition(defGear, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.MapClientEventToSimEvent(evtGearDown, "GEAR_DOWN")

	var (
		layout      *airport.Layout
		graph       *airport.Graph
		rwy         airport.Runway
		end         airport.RunwayEnd
		objectID    uint32
		phaseName   = "spawning"
		aiState     string
		start       = time.Now()
		lastPrint   time.Time
		touchdown   *airport.LatLon
		airborne    bool
		lastHeading float64
	)

	spawn := func() error {
		var ok bool
		rwy, end, ok = layout.RunwayEnd(*runway)
		if !ok {
			return fmt.Errorf("no runway %s", *runway)
		}
		elev := convert.MetersToFeet(layout.Altitude)
		switch *mode {
		case "waypoint":
			lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, math.Mod(end.Heading+180, 360), 5*1852)
			fmt.Printf("🛬 Spawning on final %s: 5 nm, %.0f ft MSL, 140 kt\n", end.Name, elev+1590)
			return fleet.RequestNonATC(traffic.NonATCOpts{Model: *model, Tail: "SPK1", Position: types.SIMCONNECT_DATA_INITPOSITION{
				Latitude: lat, Longitude: lon, Altitude: elev + 1590, Heading: end.Heading, OnGround: 0, Airspeed: 140,
			}}, reqSpawn)
		case "atc":
			plan, err := writePlan(*from, layout)
			if err != nil {
				return err
			}
			fmt.Printf("🛬 Spawning ATC arrival %s → %s at phase %.2f (plan %s)\n", *from, layout.ICAO, *phase, plan)
			return fleet.RequestEnroute(traffic.EnrouteOpts{Model: *model, Tail: "SPK2", FlightNumber: 289, FlightPlan: plan, Phase: *phase}, reqSpawn)
		}
		return fmt.Errorf("unknown mode %q", *mode)
	}

	approach := func() error {
		// 3° glide path: 318 ft per nm to the threshold, then flare to touchdown.
		var wps []types.SIMCONNECT_DATA_WAYPOINT
		agl := uint32(types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED | types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL | types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED)
		for _, nm := range []float64{4, 3, 2, 1} {
			lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, math.Mod(end.Heading+180, 360), nm*1852)
			wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: 318 * nm, Flags: agl, KtsSpeed: 130 + 3*nm})
		}
		wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: end.Threshold.Lat, Longitude: end.Threshold.Lon, Altitude: 50, Flags: agl, KtsSpeed: 130})
		elev := convert.MetersToFeet(layout.Altitude)
		for i, p := range []struct{ m, kts float64 }{{400, 120}, {1000, 60}, {1600, 30}, {2000, 15}} {
			lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, p.m)
			if *variant == "a3" && i == 0 {
				lat, lon = calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, 300) // touchdown zone
			}
			if *variant == "a2" && i == 0 {
				// Touchdown: 0 ft AGL, idle thrust.
				wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{Latitude: lat, Longitude: lon, Altitude: 0, KtsSpeed: p.kts,
					Flags: uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND | types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED | types.SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED | types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL)})
				continue
			}
			wps = append(wps, traffic.TaxiWaypoint(lat, lon, elev, p.kts))
		}
		return fleet.SetWaypoints(objectID, defWaypoints, wps)
	}

	takeover := func(pos airport.LatLon) error {
		parking, err := layout.ParkingIndex(*stand)
		if err != nil {
			return err
		}
		from := nearestTaxiNodeAhead(graph, pos, lastHeading)
		route, err := graph.RouteToParking(from, parking, airport.RouteOptions{UseRunwayPaths: true})
		if err != nil {
			return err
		}
		fmt.Printf("🔁 Takeover at %.5f,%.5f: node %d → %s, %.0f m via %s\n", pos.Lat, pos.Lon, from, *stand, route.Length, strings.Join(route.Taxiways, " → "))
		if *mode == "atc" {
			if err := fleet.ReleaseControl(objectID, reqRelease); err != nil {
				return err
			}
		}
		elev := convert.MetersToFeet(layout.Altitude)
		var wps []types.SIMCONNECT_DATA_WAYPOINT
		for i, p := range route.Points {
			kts := 10.0
			if i == len(route.Points)-1 {
				kts = 3
			}
			wps = append(wps, traffic.TaxiWaypoint(p.Lat, p.Lon, elev, kts))
		}
		return fleet.SetWaypoints(objectID, defWaypoints, wps)
	}

	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			if objectID != 0 {
				fleet.Remove(objectID, reqRemove)
			}
			fmt.Println("🛑 Removed, exiting")
			return
		case msg, ok := <-stream:
			if !ok {
				return
			}
			if msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				if res.Err != nil {
					fmt.Fprintln(os.Stderr, res.Err)
					return
				}
				layout = res.Layout
				graph, _ = cache.Graph(layout.ICAO)
				if err := spawn(); err != nil {
					fmt.Fprintln(os.Stderr, "spawn:", err)
					return
				}
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Fprintf(os.Stderr, "⚠️  exception %d sendID=%d index=%d\n", e.DwException, e.DwSendID, e.DwIndex)
			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				m := msg.AsAssignedObjectID()
				if uint32(m.DwRequestID) != reqSpawn {
					continue
				}
				objectID = uint32(m.DwObjectID)
				fleet.Acknowledge(reqSpawn, objectID)
				fmt.Printf("🆔 Object %d\n", objectID)
				client.RequestDataOnSimObject(reqMonitor, defMonitor, objectID, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
				client.RequestDataOnSimObject(reqState, defState, objectID, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
				if *mode == "waypoint" {
					if err := fleet.ReleaseControl(objectID, reqRelease); err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
					if *variant == "a3" {
						// Known AI bug: with gear up the aircraft is never "on ground"
						// in STATE_LANDING. Set the gear SimVars, then send GEAR_DOWN.
						gear := [4]float64{1, 1, 1, 1}
						err1 := client.SetDataOnSimObject(defGear, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(gear)), unsafe.Pointer(&gear))
						err2 := client.TransmitClientEvent(objectID, evtGearDown, 0, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
						fmt.Printf("⚙️  Gear down: set=%v event=%v\n", err1, err2)
					}
					if err := approach(); err != nil {
						fmt.Fprintln(os.Stderr, "approach:", err)
					}
					phaseName = "approach"
				} else {
					phaseName = "atc"
				}
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				switch uint32(d.DwRequestID) {
				case reqState:
					aiState = engine.BytesToString(engine.CastDataAs[[256]byte](&d.DwData)[:])
				case reqMonitor:
					m := *engine.CastDataAs[monitor](&d.DwData)
					pos := airport.LatLon{Lat: m.Lat, Lon: m.Lon}
					distThr, cross := math.NaN(), math.NaN()
					if layout != nil {
						distThr = calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, m.Lat, m.Lon)
						cross = calc.CrossTrackMeters(rwy.Primary.Threshold.Lat, rwy.Primary.Threshold.Lon, rwy.Secondary.Threshold.Lat, rwy.Secondary.Threshold.Lon, m.Lat, m.Lon)
					}
					lastHeading = m.Heading
					if m.OnGround == 0 {
						airborne = true
					}
					if m.OnGround != 0 && airborne && touchdown == nil && phaseName != "spawning" {
						touchdown = &pos
						phaseName = "rollout"
						fmt.Printf("🛬 Touchdown: %.0f m from threshold %s, %.0f m off centreline, %.0f kt, VS %.0f fpm\n", distThr, end.Name, cross, m.GS, m.VS)
					}
					rolling := aiState == "STATE_LANDING" || aiState == "STATE_LANDING_ROLLOUT"
					if phaseName == "rollout" && m.GS < *takeoverKts && (!*afterRollout || !rolling) {
						phaseName = "taxi-in"
						if err := takeover(pos); err != nil {
							fmt.Fprintln(os.Stderr, "takeover:", err)
						}
					}
					t := time.Since(start).Seconds()
					fmt.Fprintf(logf, "%.0f,%s,%.6f,%.6f,%.0f,%.0f,%.0f,%.1f,%.1f,%.0f,%.0f,%.1f,%q,%.0f,%.0f\n",
						t, phaseName, m.Lat, m.Lon, m.AGL, m.AltMSL, m.Heading, m.GS, m.IAS, m.VS, m.OnGround, m.Pitch, aiState, distThr, cross)
					if time.Since(lastPrint) >= 3*time.Second {
						lastPrint = time.Now()
						fmt.Printf("%4.0fs %-9s agl %5.0f ft  gs %5.1f  ias %5.1f  vs %6.0f  hdg %3.0f  gnd %.0f  thr %6.0f m  xtrk %5.0f  [%s]\n",
							t, phaseName, m.AGL, m.GS, m.IAS, m.VS, m.Heading, m.OnGround, distThr, cross, aiState)
					}
				}
			}
		}
	}
}

// nearestTaxiNodeAhead returns the closest routable taxi point at least 30 m
// away within 60° of heading, falling back to the closest one overall.
func nearestTaxiNodeAhead(g *airport.Graph, p airport.LatLon, heading float64) airport.NodeID {
	best, bestD := airport.NodeID(-1), math.Inf(1)
	for _, n := range g.Nodes {
		if n.Kind == airport.NodeParking || len(g.Adj[n.ID]) == 0 {
			continue
		}
		d := calc.HaversineMeters(p.Lat, p.Lon, n.Position.Lat, n.Position.Lon)
		b := calc.BearingDegrees(p.Lat, p.Lon, n.Position.Lat, n.Position.Lon)
		if d >= 30 && math.Abs(math.Mod(b-heading+540, 360)-180) <= 60 && d < bestD {
			best, bestD = n.ID, d
		}
	}
	if best < 0 {
		return nearestTaxiNode(g, p)
	}
	return best
}

// nearestTaxiNode returns the taxi point node closest to p.
func nearestTaxiNode(g *airport.Graph, p airport.LatLon) airport.NodeID {
	best, bestD := airport.NodeID(0), math.Inf(1)
	for _, n := range g.Nodes {
		if n.Kind == airport.NodeParking || len(g.Adj[n.ID]) == 0 {
			continue
		}
		if d := calc.HaversineMeters(p.Lat, p.Lon, n.Position.Lat, n.Position.Lon); d < bestD {
			best, bestD = n.ID, d
		}
	}
	return best
}

// enroute returns user waypoints every ~10 km between the departure and the
// destination, so a flight plan position (phase) has legs to interpolate on.
func enroute(d [3]float64, l *airport.Layout, lla func(lat, lon, altFt float64) string) string {
	dist := calc.HaversineMeters(d[0], d[1], l.Latitude, l.Longitude)
	brg := calc.BearingDegrees(d[0], d[1], l.Latitude, l.Longitude)
	n := int(dist / 10000)
	var b strings.Builder
	for i := 1; i < n; i++ {
		lat, lon := calc.DisplaceByHeading(d[0], d[1], brg, dist*float64(i)/float64(n))
		fmt.Fprintf(&b, "        <ATCWaypoint id=\"WP%02d\"><ATCWaypointType>User</ATCWaypointType><WorldPosition>%s</WorldPosition></ATCWaypoint>\n", i, lla(lat, lon, 6000))
	}
	return b.String()
}

// writePlan writes a direct IFR flight plan from dep to the layout's airport
// and returns its path without the .pln extension, as SimConnect expects.
func writePlan(dep string, l *airport.Layout) (string, error) {
	deps := map[string][3]float64{"LKHK": {50.2533, 15.8453, 791}, "LKKV": {50.2029, 12.9150, 1989}, "LKPD": {50.0134, 15.7386, 741}, "LKVO": {50.2166, 14.3958, 919}, "LKKB": {50.1214, 14.5436, 939}}
	d, ok := deps[dep]
	if !ok {
		return "", fmt.Errorf("unknown departure %s", dep)
	}
	lla := func(lat, lon, altFt float64) string {
		f := func(v float64, pos, neg string) string {
			h := pos
			if v < 0 {
				h, v = neg, -v
			}
			deg := math.Floor(v)
			min := math.Floor((v - deg) * 60)
			sec := ((v-deg)*60 - min) * 60
			return fmt.Sprintf("%s%.0f° %.0f' %.2f\"", h, deg, min, sec)
		}
		return fmt.Sprintf("%s,%s,%+010.2f", f(lat, "N", "S"), f(lon, "E", "W"), altFt)
	}
	destAlt := convert.MetersToFeet(l.Altitude)
	plan := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SimBase.Document Type="AceXML" version="1,0">
    <Descr>AceXML Document</Descr>
    <FlightPlan.FlightPlan>
        <Title>%[1]s to %[2]s</Title>
        <FPType>IFR</FPType>
        <RouteType>LowAlt</RouteType>
        <CruisingAlt>6000</CruisingAlt>
        <DepartureID>%[1]s</DepartureID>
        <DepartureLLA>%[3]s</DepartureLLA>
        <DestinationID>%[2]s</DestinationID>
        <DestinationLLA>%[4]s</DestinationLLA>
        <Descr>%[1]s, %[2]s</Descr>
        <DepartureName>%[1]s</DepartureName>
        <DestinationName>%[2]s</DestinationName>
        <AppVersion><AppVersionMajor>11</AppVersionMajor><AppVersionBuild>282174</AppVersionBuild></AppVersion>
        <ATCWaypoint id="%[1]s"><ATCWaypointType>Airport</ATCWaypointType><WorldPosition>%[3]s</WorldPosition><ICAO><ICAOIdent>%[1]s</ICAOIdent></ICAO></ATCWaypoint>
%[5]s        <ATCWaypoint id="%[2]s"><ATCWaypointType>Airport</ATCWaypointType><WorldPosition>%[4]s</WorldPosition><ICAO><ICAOIdent>%[2]s</ICAOIdent></ICAO></ATCWaypoint>
    </FlightPlan.FlightPlan>
</SimBase.Document>
`, dep, l.ICAO, lla(d[0], d[1], d[2]), lla(l.Latitude, l.Longitude, destAlt), enroute(d, l, lla))
	path := filepath.Join(`C:\msfs-development\dumps`, dep+"-"+l.ICAO)
	if err := os.WriteFile(path+".pln", []byte(plan), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
