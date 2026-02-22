//go:build windows
// +build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Target airport — swap this to test at a different field.
const icao = "LKPR"

// Facility definition IDs — one per query shape.
const (
	defFacAirport  uint32 = 1000
	defFacParking  uint32 = 1001
	defFacTaxiPath uint32 = 1002
	defFacTaxiPt   uint32 = 1003
	defFacRunway   uint32 = 1004
)

// Facility request IDs — returned in FACILITY_DATA.UserRequestId.
const (
	reqFacAirport  uint32 = 100
	reqFacParking  uint32 = 101
	reqFacTaxiPath uint32 = 102
	reqFacTaxiPt   uint32 = 103
	reqFacRunway   uint32 = 104
)

// Waypoint data definition — shared by all AI objects.
const defWaypoints uint32 = 2000

// Monitor data definition — lat/lon/alt/hdg per-second poll.
const defMonitor uint32 = 3000

// Spawn request IDs — gate aircraft 200–204, taxi 210, takeoff 211.
const (
	reqSpawnGate0   uint32 = 200
	reqSpawnTaxi    uint32 = 210
	reqSpawnTakeoff uint32 = 211
)

// AIReleaseControl request IDs.
const (
	reqReleaseTaxi    uint32 = 220
	reqReleaseTakeoff uint32 = 221
)

// Monitor request IDs — one per monitored object.
const (
	reqMonitorTaxi    uint32 = 230
	reqMonitorTakeoff uint32 = 231
)

// Object lifecycle events.
const (
	evtAdded   uint32 = 400
	evtRemoved uint32 = 401
)

const (
	maxGates     = 5
	taxiChainLen = 8
	model        = "FSLTL A320 Air France SL"
)

// ── Facility data structs ──────────────────────────────────────────────────
// Field order must match the AddToFacilityDefinition calls exactly.

type apInfo struct {
	Latitude  float64
	Longitude float64
	Altitude  float64 // meters MSL
	ICAO      [8]byte
	Name      [32]byte
	Name64    [64]byte
}

type parkingSpot struct {
	Name             uint32
	Number           uint32
	Heading          float32
	Type             uint32
	BiasX            float32 // east offset in meters from airport reference
	BiasZ            float32 // north offset in meters from airport reference
	NumberOfAirlines uint32
}

type taxiPath struct {
	Type      uint32
	Start     uint32
	End       uint32
	NameIndex uint32
}

type taxiPoint struct {
	Type        uint32
	Orientation uint32
	BiasX       float32 // east offset in meters
	BiasZ       float32 // north offset in meters
}

type runwayData struct {
	Latitude  float64 // center lat
	Longitude float64 // center lon
	Altitude  float64 // meters MSL
	Heading   float64 // primary heading in degrees
	Length    float64 // meters
}

// parseRunway reads a runway data buffer from a FACILITY_DATA message.
// SimConnect packs the runway wire as [lat:f64][lon:f64][alt:f64][hdg:f32][len:f32]
// = 32 bytes. Both Heading and Length are float32, so Go's natural 8-byte-aligned
// struct cannot cast this directly — same root cause as SIMCONNECT_DATA_WAYPOINT.
func parseRunway(data *types.DWORD) runwayData {
	raw := (*[32]byte)(unsafe.Pointer(data))
	return runwayData{
		Latitude:  math.Float64frombits(binary.LittleEndian.Uint64(raw[0:])),
		Longitude: math.Float64frombits(binary.LittleEndian.Uint64(raw[8:])),
		Altitude:  math.Float64frombits(binary.LittleEndian.Uint64(raw[16:])),
		Heading:   float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[24:]))),
		Length:    float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[28:]))),
	}
}

// monitorData matches defMonitor field order: lat, lon, alt (ft), heading (deg).
type monitorData struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Heading   float64
}

// ── Accumulated state ──────────────────────────────────────────────────────

type trafficState struct {
	airport    apInfo
	parking    []parkingSpot
	taxiPaths  []taxiPath
	taxiPoints []taxiPoint
	runways    []runwayData
	pending    int // decrements on FACILITY_DATA_END; spawn when == 0

	selectedRunway runwayData // the runway chosen for the takeoff aircraft

	gateIDs   [maxGates]uint32
	taxiID    uint32
	takeoffID uint32
}

// ── Geometry helpers ───────────────────────────────────────────────────────

// aheadOf returns the lat/lon displaced from (lat, lon) by meters along hdgDeg.
func aheadOf(lat, lon, hdgDeg, meters float64) (float64, float64) {
	rad := hdgDeg * math.Pi / 180
	return convert.OffsetToLatLon(lat, lon,
		meters*math.Sin(rad), // east component
		meters*math.Cos(rad), // north component
	)
}

// buildTaxiChain greedy-walks the taxi-path adjacency graph from point 0,
// returning up to maxLen point indices.
func buildTaxiChain(paths []taxiPath, nPoints, maxLen int) []int {
	if len(paths) == 0 || nPoints == 0 {
		return nil
	}
	adj := make([][]int, nPoints)
	for _, p := range paths {
		s, e := int(p.Start), int(p.End)
		if s < nPoints && e < nPoints {
			adj[s] = append(adj[s], e)
			adj[e] = append(adj[e], s)
		}
	}
	visited := make([]bool, nPoints)
	chain := make([]int, 0, maxLen)
	cur := 0
	for len(chain) < maxLen {
		chain = append(chain, cur)
		visited[cur] = true
		next := -1
		for _, nb := range adj[cur] {
			if !visited[nb] {
				next = nb
				break
			}
		}
		if next == -1 {
			break
		}
		cur = next
	}
	return chain
}

// taxiWaypoints converts point-index chain to a looping ON_GROUND waypoint slice.
func taxiWaypoints(chain []int, points []taxiPoint, ap apInfo) []types.SIMCONNECT_DATA_WAYPOINT {
	altFt := convert.MetersToFeet(ap.Altitude)
	wps := make([]types.SIMCONNECT_DATA_WAYPOINT, len(chain))
	for i, idx := range chain {
		pt := points[idx]
		lat, lon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
			float64(pt.BiasX), float64(pt.BiasZ))
		flags := uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND | types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED)
		if i == len(chain)-1 {
			flags |= uint32(types.SIMCONNECT_WAYPOINT_WRAP_TO_FIRST)
		}
		wps[i] = types.SIMCONNECT_DATA_WAYPOINT{
			Latitude:  lat,
			Longitude: lon,
			Altitude:  altFt,
			Flags:     flags,
			KtsSpeed:  15,
		}
	}
	return wps
}

// takeoffWaypoints builds an ascending waypoint sequence for a departing aircraft.
// All altitudes use ALTITUDE_IS_AGL so the sim resolves them relative to local
// terrain, and COMPUTE_VERTICAL_SPEED tells the AI to calculate the VS needed to
// reach each altitude at the waypoint — this is what drives proper climb rates.
// THROTTLE_REQUESTED at 100 % keeps the engine at full power during the climb.
// No near-field anchor waypoint — that caused the aircraft to overshoot and bank
// to intercept the next waypoint, producing the observed right-turn deviation.
func takeoffWaypoints(rwy runwayData) []types.SIMCONNECT_DATA_WAYPOINT {
	hdg := rwy.Heading

	// Threshold = spawn origin — all distances measured from here.
	threshLat, threshLon := aheadOf(rwy.Latitude, rwy.Longitude, hdg+180, rwy.Length/2)

	climbFlags := uint32(
		types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED |
			types.SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED |
			types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED |
			types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL,
	)

	// Airborne waypoints measured from the departure threshold.
	// 1.5 nm: 1 500 ft AGL — COMPUTE_VERTICAL_SPEED drives proper climb rate.
	c1Lat, c1Lon := aheadOf(threshLat, threshLon, hdg, 2778)
	// 5 nm: 4 000 ft AGL.
	c2Lat, c2Lon := aheadOf(threshLat, threshLon, hdg, 9260)
	// 12 nm: 9 000 ft AGL.
	c3Lat, c3Lon := aheadOf(threshLat, threshLon, hdg, 22224)

	return []types.SIMCONNECT_DATA_WAYPOINT{
		{
			// Taxi 250 m to lineup at the threshold before the takeoff roll.
			// 15 kt matches the proven taxi speed used by the taxi aircraft.
			Latitude:  threshLat,
			Longitude: threshLon,
			Altitude:  0,
			Flags:     uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND | types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED),
			KtsSpeed:  15,
		},
		{
			Latitude:        c1Lat,
			Longitude:       c1Lon,
			Altitude:        1500,
			Flags:           climbFlags,
			KtsSpeed:        200,
			PercentThrottle: 100,
		},
		{
			Latitude:        c2Lat,
			Longitude:       c2Lon,
			Altitude:        4000,
			Flags:           climbFlags,
			KtsSpeed:        240,
			PercentThrottle: 90,
		},
		{
			Latitude:        c3Lat,
			Longitude:       c3Lon,
			Altitude:        9000,
			Flags:           climbFlags,
			KtsSpeed:        280,
			PercentThrottle: 85,
		},
	}
}

// ── SimConnect helpers ─────────────────────────────────────────────────────

func sendWaypoints(client engine.Client, objectID uint32, wps []types.SIMCONNECT_DATA_WAYPOINT) error {
	packed := engine.PackWaypoints(wps)
	return client.SetDataOnSimObject(
		defWaypoints,
		objectID,
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
		uint32(len(wps)),
		engine.WaypointWireSize,
		unsafe.Pointer(&packed[0]),
	)
}

func removeAll(client engine.Client, ids []uint32) {
	for i, id := range ids {
		if id == 0 {
			continue
		}
		fmt.Printf("🗑️  Removing object %d\n", id)
		if err := client.AIRemoveObject(id, 300+uint32(i)); err != nil {
			fmt.Fprintf(os.Stderr, "❌ AIRemoveObject id=%d: %v\n", id, err)
		}
	}
	if len(ids) > 0 {
		time.Sleep(100 * time.Millisecond)
	}
}

// ── Facility setup ─────────────────────────────────────────────────────────

func registerFacilities(client engine.Client) {
	// Airport reference position
	client.AddToFacilityDefinition(defFacAirport, "OPEN AIRPORT")
	client.AddToFacilityDefinition(defFacAirport, "LATITUDE")
	client.AddToFacilityDefinition(defFacAirport, "LONGITUDE")
	client.AddToFacilityDefinition(defFacAirport, "ALTITUDE")
	client.AddToFacilityDefinition(defFacAirport, "ICAO")
	client.AddToFacilityDefinition(defFacAirport, "NAME")
	client.AddToFacilityDefinition(defFacAirport, "NAME64")
	client.AddToFacilityDefinition(defFacAirport, "CLOSE AIRPORT")

	// Gate/apron parking positions
	client.AddToFacilityDefinition(defFacParking, "OPEN AIRPORT")
	client.AddToFacilityDefinition(defFacParking, "OPEN TAXI_PARKING")
	client.AddToFacilityDefinition(defFacParking, "NAME")
	client.AddToFacilityDefinition(defFacParking, "NUMBER")
	client.AddToFacilityDefinition(defFacParking, "HEADING")
	client.AddToFacilityDefinition(defFacParking, "TYPE")
	client.AddToFacilityDefinition(defFacParking, "BIAS_X")
	client.AddToFacilityDefinition(defFacParking, "BIAS_Z")
	client.AddToFacilityDefinition(defFacParking, "N_AIRLINES")
	client.AddToFacilityDefinition(defFacParking, "CLOSE TAXI_PARKING")
	client.AddToFacilityDefinition(defFacParking, "CLOSE AIRPORT")

	// Taxi path graph edges (start/end taxi-point indices)
	client.AddToFacilityDefinition(defFacTaxiPath, "OPEN AIRPORT")
	client.AddToFacilityDefinition(defFacTaxiPath, "OPEN TAXI_PATH")
	client.AddToFacilityDefinition(defFacTaxiPath, "TYPE")
	client.AddToFacilityDefinition(defFacTaxiPath, "START")
	client.AddToFacilityDefinition(defFacTaxiPath, "END")
	client.AddToFacilityDefinition(defFacTaxiPath, "NAME_INDEX")
	client.AddToFacilityDefinition(defFacTaxiPath, "CLOSE TAXI_PATH")
	client.AddToFacilityDefinition(defFacTaxiPath, "CLOSE AIRPORT")

	// Taxi point positions (meter offsets from airport reference)
	client.AddToFacilityDefinition(defFacTaxiPt, "OPEN AIRPORT")
	client.AddToFacilityDefinition(defFacTaxiPt, "OPEN TAXI_POINT")
	client.AddToFacilityDefinition(defFacTaxiPt, "TYPE")
	client.AddToFacilityDefinition(defFacTaxiPt, "ORIENTATION")
	client.AddToFacilityDefinition(defFacTaxiPt, "BIAS_X")
	client.AddToFacilityDefinition(defFacTaxiPt, "BIAS_Z")
	client.AddToFacilityDefinition(defFacTaxiPt, "CLOSE TAXI_POINT")
	client.AddToFacilityDefinition(defFacTaxiPt, "CLOSE AIRPORT")

	// Runway centre position, heading, and length
	client.AddToFacilityDefinition(defFacRunway, "OPEN AIRPORT")
	client.AddToFacilityDefinition(defFacRunway, "OPEN RUNWAY")
	client.AddToFacilityDefinition(defFacRunway, "LATITUDE")
	client.AddToFacilityDefinition(defFacRunway, "LONGITUDE")
	client.AddToFacilityDefinition(defFacRunway, "ALTITUDE")
	client.AddToFacilityDefinition(defFacRunway, "HEADING")
	client.AddToFacilityDefinition(defFacRunway, "LENGTH")
	client.AddToFacilityDefinition(defFacRunway, "CLOSE RUNWAY")
	client.AddToFacilityDefinition(defFacRunway, "CLOSE AIRPORT")

	// Fire all 5 requests — each produces N FACILITY_DATA + 1 FACILITY_DATA_END.
	client.RequestFacilityData(defFacAirport, reqFacAirport, icao, "")
	client.RequestFacilityData(defFacParking, reqFacParking, icao, "")
	client.RequestFacilityData(defFacTaxiPath, reqFacTaxiPath, icao, "")
	client.RequestFacilityData(defFacTaxiPt, reqFacTaxiPt, icao, "")
	client.RequestFacilityData(defFacRunway, reqFacRunway, icao, "")
}

// ── Traffic spawning ───────────────────────────────────────────────────────

// spawnTraffic is called once all 5 facility batches have been received.
func spawnTraffic(client engine.Client, st *trafficState) {
	ap := st.airport
	altFt := convert.MetersToFeet(ap.Altitude)

	// 1. Parked aircraft at real gate positions.
	gateIdx := 0
	for _, spot := range st.parking {
		if gateIdx >= maxGates {
			break
		}
		if spot.Number == 0 {
			continue // skip unnamed / non-gate spots
		}
		lat, lon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
			float64(spot.BiasX), float64(spot.BiasZ))
		tail := fmt.Sprintf("G%03d", spot.Number)
		reqID := reqSpawnGate0 + uint32(gateIdx)
		client.AICreateNonATCAircraftEX1(
			model, "", tail,
			types.SIMCONNECT_DATA_INITPOSITION{
				Latitude:  lat,
				Longitude: lon,
				Altitude:  altFt,
				Heading:   float64(spot.Heading),
				OnGround:  1,
				Airspeed:  0,
			},
			reqID,
		)
		fmt.Printf("🅿️  Gate %d: spot #%d tail=%s (reqID=%d)\n",
			gateIdx+1, spot.Number, tail, reqID)
		gateIdx++
	}
	if gateIdx == 0 {
		fmt.Println("⚠️  No usable gate spots found at", icao)
	}

	// 2. Taxiing aircraft following a connected sequence of taxi points.
	if len(st.taxiPoints) > 0 {
		chain := buildTaxiChain(st.taxiPaths, len(st.taxiPoints), taxiChainLen)
		if len(chain) >= 2 {
			startPt := st.taxiPoints[chain[0]]
			startLat, startLon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
				float64(startPt.BiasX), float64(startPt.BiasZ))
			client.AICreateNonATCAircraftEX1(
				model, "", "TAXI01",
				types.SIMCONNECT_DATA_INITPOSITION{
					Latitude:  startLat,
					Longitude: startLon,
					Altitude:  altFt,
					Heading:   0,
					OnGround:  1,
					Airspeed:  0,
				},
				reqSpawnTaxi,
			)
			fmt.Printf("🚖 Taxiing aircraft requested (chain=%d pts, reqID=%d)\n",
				len(chain), reqSpawnTaxi)
		} else {
			fmt.Println("⚠️  Taxi chain too short — skipping taxi aircraft")
		}
	}

	// 3. Departing aircraft at the primary threshold of the first usable runway.
	var chosenRwy *runwayData
	for i := range st.runways {
		if st.runways[i].Heading != 0 && st.runways[i].Length > 0 {
			chosenRwy = &st.runways[i]
			break
		}
	}
	if chosenRwy == nil && len(st.runways) > 0 {
		chosenRwy = &st.runways[0] // fall back to first if all are "zero"
	}
	if chosenRwy != nil {
		st.selectedRunway = *chosenRwy
		rwy := st.selectedRunway
		// Threshold = center displaced half-length opposite the primary heading.
		threshLat, threshLon := aheadOf(rwy.Latitude, rwy.Longitude,
			rwy.Heading+180, rwy.Length/2)
		// Spawn 250 m behind the threshold on the extended centreline so the
		// aircraft taxis into lineup before starting the takeoff roll.
		spawnLat, spawnLon := aheadOf(threshLat, threshLon, rwy.Heading+180, 250)
		client.AICreateNonATCAircraftEX1(
			model, "", "TKOF01",
			types.SIMCONNECT_DATA_INITPOSITION{
				Latitude:  spawnLat,
				Longitude: spawnLon,
				Altitude:  convert.MetersToFeet(rwy.Altitude),
				Heading:   rwy.Heading,
				OnGround:  1,
				Airspeed:  0,
			},
			reqSpawnTakeoff,
		)
		fmt.Printf("🛫 Takeoff aircraft requested (rwy hdg=%.1f°, reqID=%d)\n",
			rwy.Heading, reqSpawnTakeoff)
	}
}

// ── Connection lifecycle ───────────────────────────────────────────────────

func runConnection(ctx context.Context) error {
	client := simconnect.NewClient("GO Example - Manage Traffic",
		engine.WithContext(ctx),
	)

	fmt.Println("⏳ Waiting for simulator...")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := client.Connect(); err != nil {
				fmt.Printf("🔄 Retrying in 2s: %v\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			goto connected
		}
	}

connected:
	fmt.Println("✅ Connected to SimConnect")

	// Shared waypoint data definition used by all AI objects.
	client.AddToDataDefinition(defWaypoints, "AI Waypoint List", "number",
		types.SIMCONNECT_DATATYPE_WAYPOINT, 0, 0)

	// Monitor definition — lat/lon/alt/heading polled every second.
	client.AddToDataDefinition(defMonitor, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
	client.AddToDataDefinition(defMonitor, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 1)
	client.AddToDataDefinition(defMonitor, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2)
	client.AddToDataDefinition(defMonitor, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3)

	// Object lifecycle events for diagnostics.
	client.SubscribeToSystemEvent(evtAdded, "ObjectAdded")
	client.SubscribeToSystemEvent(evtRemoved, "ObjectRemoved")

	// Query LKPR facility data — 5 batches, each ending with FACILITY_DATA_END.
	registerFacilities(client)

	st := &trafficState{pending: 5}

	// Collect all spawned object IDs for cleanup on shutdown.
	allIDs := func() []uint32 {
		ids := make([]uint32, 0, maxGates+2)
		for _, id := range st.gateIDs {
			ids = append(ids, id)
		}
		ids = append(ids, st.taxiID, st.takeoffID)
		return ids
	}

	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔌 Shutting down...")
			removeAll(client, allIDs())
			if err := client.Disconnect(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Disconnect: %v\n", err)
			}
			return ctx.Err()

		case msg, ok := <-stream:
			if !ok {
				fmt.Println("📴 Stream closed (simulator disconnected)")
				return nil
			}
			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Stream error: %v\n", msg.Err)
				continue
			}

			switch types.SIMCONNECT_RECV_ID(msg.DwID) {

			case types.SIMCONNECT_RECV_ID_OPEN:
				o := msg.AsOpen()
				fmt.Printf("🟢 Simulator: %s v%d.%d\n",
					engine.BytesToString(o.SzApplicationName[:]),
					o.DwApplicationVersionMajor, o.DwApplicationVersionMinor)

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
				fd := msg.AsFacilityData()
				switch uint32(fd.UserRequestId) {
				case reqFacAirport:
					st.airport = *engine.CastDataAs[apInfo](&fd.Data)
					fmt.Printf("📍 Airport %s: lat=%.4f lon=%.4f alt=%.0fm\n",
						icao, st.airport.Latitude, st.airport.Longitude, st.airport.Altitude)
				case reqFacParking:
					spot := engine.CastDataAs[parkingSpot](&fd.Data)
					if spot.Number > 0 {
						st.parking = append(st.parking, *spot)
					}
				case reqFacTaxiPath:
					st.taxiPaths = append(st.taxiPaths,
						*engine.CastDataAs[taxiPath](&fd.Data))
				case reqFacTaxiPt:
					st.taxiPoints = append(st.taxiPoints,
						*engine.CastDataAs[taxiPoint](&fd.Data))
				case reqFacRunway:
					rwy := parseRunway(&fd.Data)
					fmt.Printf("  Runway: lat=%.4f lon=%.4f alt=%.0fm hdg=%.1f° len=%.0fm\n",
						rwy.Latitude, rwy.Longitude, rwy.Altitude, rwy.Heading, rwy.Length)
					st.runways = append(st.runways, rwy)
				}

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
				st.pending--
				fmt.Printf("🏁 Facility batch complete (%d remaining)\n", st.pending)
				if st.pending == 0 {
					fmt.Printf("📦 Data ready: %d gates, %d paths, %d points, %d runways\n",
						len(st.parking), len(st.taxiPaths), len(st.taxiPoints), len(st.runways))
					spawnTraffic(client, st)
				}

			case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
				assigned := msg.AsAssignedObjectID()
				reqID := uint32(assigned.DwRequestID)
				objID := uint32(assigned.DwObjectID)

				switch {
				case reqID >= reqSpawnGate0 && reqID < reqSpawnGate0+maxGates:
					idx := reqID - reqSpawnGate0
					st.gateIDs[idx] = objID
					fmt.Printf("✅ Gate %d parked: id=%d\n", idx+1, objID)

				case reqID == reqSpawnTaxi:
					st.taxiID = objID
					fmt.Printf("✅ Taxi aircraft: id=%d — releasing AI control\n", objID)
					client.AIReleaseControl(objID, reqReleaseTaxi)
					chain := buildTaxiChain(st.taxiPaths, len(st.taxiPoints), taxiChainLen)
					wps := taxiWaypoints(chain, st.taxiPoints, st.airport)
					if len(wps) > 0 {
						if err := sendWaypoints(client, objID, wps); err != nil {
							fmt.Fprintf(os.Stderr, "❌ taxi waypoints: %v\n", err)
						}
					}
					client.RequestDataOnSimObject(reqMonitorTaxi, defMonitor, objID,
						types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)

				case reqID == reqSpawnTakeoff:
					st.takeoffID = objID
					fmt.Printf("✅ Takeoff aircraft: id=%d — releasing AI control\n", objID)
					client.AIReleaseControl(objID, reqReleaseTakeoff)
					if st.selectedRunway.Length > 0 {
						wps := takeoffWaypoints(st.selectedRunway)
						if err := sendWaypoints(client, objID, wps); err != nil {
							fmt.Fprintf(os.Stderr, "❌ takeoff waypoints: %v\n", err)
						}
					}
					client.RequestDataOnSimObject(reqMonitorTakeoff, defMonitor, objID,
						types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
				}

			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				pos := engine.CastDataAs[monitorData](&d.DwData)
				label := "?"
				switch uint32(d.DwRequestID) {
				case reqMonitorTaxi:
					label = "TAXI"
				case reqMonitorTakeoff:
					label = "TKOF"
				}
				fmt.Printf("📡 [%s] id=%d lat=%.4f lon=%.4f alt=%.0fft hdg=%.1f°\n",
					label, uint32(d.DwObjectID),
					pos.Latitude, pos.Longitude, pos.Altitude, pos.Heading)

			case types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE:
				evt := msg.AsEventObjectAddRemove()
				switch uint32(evt.UEventID) {
				case evtAdded:
					fmt.Printf("➕ Object added:   id=%d type=%d\n", evt.DwData, evt.EObjType)
				case evtRemoved:
					fmt.Printf("➖ Object removed: id=%d type=%d\n", evt.DwData, evt.EObjType)
				}

			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				ex := msg.AsException()
				fmt.Printf("🚨 SimConnect exception %d at index %d\n", ex.DwException, ex.DwIndex)
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("🛑 Interrupt received, shutting down...")
		cancel()
	}()

	fmt.Println("ℹ️  Press Ctrl+C to exit")

	for {
		if err := runConnection(ctx); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return
		}
		fmt.Println("⏳ Waiting 5s before reconnecting...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			fmt.Println("🔄 Reconnecting...")
		}
	}
}
