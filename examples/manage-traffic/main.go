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
	defFacTaxiName uint32 = 1005
)

// Facility request IDs — returned in FACILITY_DATA.UserRequestId.
const (
	reqFacAirport  uint32 = 100
	reqFacParking  uint32 = 101
	reqFacTaxiPath uint32 = 102
	reqFacTaxiPt   uint32 = 103
	reqFacRunway   uint32 = 104
	reqFacTaxiName uint32 = 105
)

// Waypoint data definition — shared by all AI objects.
const defWaypoints uint32 = 2000

// Monitor data definition — lat/lon/alt/hdg per-second poll.
const defMonitor uint32 = 3000

// Spawn request IDs — gate aircraft 200–204, departing aircraft 210.
const (
	reqSpawnGate0 uint32 = 200
	reqSpawnDept  uint32 = 210
)

// AIReleaseControl request ID for the departing aircraft.
const reqReleaseDept uint32 = 220

// Monitor request ID for the departing aircraft.
const reqMonitorDept uint32 = 230

// Object lifecycle events.
const (
	evtAdded   uint32 = 400
	evtRemoved uint32 = 401
)

const (
	maxGates = 5
	model    = "FSLTL A320 Air France SL"
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

type taxiName struct {
	Name [32]byte
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
	taxiNames  []taxiName
	taxiPoints []taxiPoint
	runways    []runwayData
	pending    int // decrements on FACILITY_DATA_END; spawn when == 0

	selectedRunway runwayData  // the runway chosen for the departing aircraft
	deptGate       parkingSpot // the gate the departing aircraft departs from

	gateIDs [maxGates]uint32
	deptID  uint32
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

// latLonToOffset is the inverse of convert.OffsetToLatLon: given an airport
// reference (latRef, lonRef) and a target (lat, lon), it returns the east (X)
// and north (Z) meter offsets of the target from the reference point.
func latLonToOffset(latRef, lonRef, lat, lon float64) (xEast, zNorth float64) {
	const a = 6378137.0
	const b = 6356752.314245
	e2 := (a*a - b*b) / (a * a)

	latRefRad := latRef * math.Pi / 180.0
	sinLat := math.Sin(latRefRad)
	w := math.Sqrt(1 - e2*sinLat*sinLat)

	M := a * (1 - e2) / (w * w * w) // meridian radius of curvature
	N := a / w                        // prime vertical radius of curvature

	zNorth = (lat - latRef) * (math.Pi / 180.0) * M
	if math.Abs(latRef) < 90.0 {
		xEast = (lon - lonRef) * (math.Pi / 180.0) * N * math.Cos(latRefRad)
	}
	return
}

// maxSegmentMeters is the maximum allowed Euclidean distance between the two
// endpoints of a taxi-path segment included in the routing graph.
// MSFS taxi networks contain long "phantom" PATH(4) connections (sometimes
// > 2 km) spanning the entire airport — useful for the simulator's internal
// AI but catastrophic when used as direct waypoints (the aircraft flies straight
// through grass / roads / buildings). Segments beyond this threshold are dropped.
const maxSegmentMeters = 500.0

// buildAdjacency returns an undirected adjacency list for the taxi-point graph.
// Only pure taxiway path types are included:
//   Type 1 = TAXI — main taxiway surface paths
//   Type 4 = PATH — general apron / connector paths
// Excluded:
//   Type 2 = RUNWAY       — runway surface
//   Type 3 = PARKING      — gate-to-taxiway spur connectors.
//                           TAXI_PATH.Start/End index a shared 0-3999 space that
//                           includes both taxi points AND parking spaces. A type-3
//                           edge whose endpoint is a parking-space index that
//                           happens to fall inside [0, len(taxiPoints)) would
//                           create a spurious edge to an unrelated taxi node.
//                           Gate entry is handled geometrically via taxiExitNode.
//   Type 5 = CLOSED       — closed path
//   Type 6 = VEHICLE      — ground-vehicle service roads
//   Type 7 = ROAD         — public roads
//   Type 8 = PAINTEDLINE  — visual marking only
// Additionally, any segment longer than maxSegmentMeters is excluded.
func buildAdjacency(paths []taxiPath, points []taxiPoint) [][]int {
	nPoints := len(points)
	adj := make([][]int, nPoints)
	for _, p := range paths {
		switch p.Type {
		case 1, 4: // TAXI, PATH — pure taxiway surfaces
		default:
			continue
		}
		s, e := int(p.Start), int(p.End)
		if s >= nPoints || e >= nPoints {
			continue
		}
		dx := float64(points[s].BiasX) - float64(points[e].BiasX)
		dz := float64(points[s].BiasZ) - float64(points[e].BiasZ)
		if math.Sqrt(dx*dx+dz*dz) > maxSegmentMeters {
			continue
		}
		adj[s] = append(adj[s], e)
		adj[e] = append(adj[e], s)
	}
	return adj
}

// logPathTypes prints a summary of taxi path type distribution for diagnostics.
func logPathTypes(paths []taxiPath) {
	counts := make(map[uint32]int)
	for _, p := range paths {
		counts[p.Type]++
	}
	names := map[uint32]string{
		0: "NONE", 1: "TAXI", 2: "RUNWAY", 3: "PARKING",
		4: "PATH", 5: "CLOSED", 6: "VEHICLE", 7: "ROAD", 8: "PAINTEDLINE",
	}
	routed := map[uint32]bool{1: true, 4: true}
	fmt.Print("🛣️  Path types: ")
	for t := uint32(0); t <= 8; t++ {
		if n := counts[t]; n > 0 {
			name := names[t]
			if name == "" {
				name = "?"
			}
			tag := ""
			if routed[t] {
				tag = "*"
			}
			fmt.Printf("%s(%d)=%d%s ", name, t, n, tag)
		}
	}
	fmt.Println("  (* = used in routing graph)")
}

// findNearestTaxiPoint returns the index of the taxi point closest (Euclidean
// in BiasX/BiasZ space) to the given meter offsets from the airport reference.
func findNearestTaxiPoint(biasX, biasZ float64, points []taxiPoint) int {
	best, bestDist := 0, math.MaxFloat64
	for i, pt := range points {
		dx := float64(pt.BiasX) - biasX
		dz := float64(pt.BiasZ) - biasZ
		if d := dx*dx + dz*dz; d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

// findNearestReachableNode flood-fills the adjacency graph from `from` and
// returns the index of the reachable node geographically nearest to
// (targetX, targetZ). Falls back to `from` when the graph has no edges.
func findNearestReachableNode(from int, adj [][]int, targetX, targetZ float64, points []taxiPoint) int {
	visited := make([]bool, len(adj))
	visited[from] = true
	queue := []int{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range adj[cur] {
			if !visited[nb] {
				visited[nb] = true
				queue = append(queue, nb)
			}
		}
	}
	best, bestDist := from, math.MaxFloat64
	for i, pt := range points {
		if !visited[i] {
			continue
		}
		dx := float64(pt.BiasX) - targetX
		dz := float64(pt.BiasZ) - targetZ
		if d := dx*dx + dz*dz; d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

// findNearestTaxiPointInDir returns the index of the closest taxi point that
// lies in the half-space defined by direction (dirX, dirZ) from (biasX, biasZ).
// Falls back to findNearestTaxiPoint if no point satisfies the direction constraint.
func findNearestTaxiPointInDir(biasX, biasZ, dirX, dirZ float64, points []taxiPoint) int {
	best, bestDist := -1, math.MaxFloat64
	for i, pt := range points {
		dx := float64(pt.BiasX) - biasX
		dz := float64(pt.BiasZ) - biasZ
		if dx*dirX+dz*dirZ > 0 {
			if d := dx*dx + dz*dz; d < bestDist {
				bestDist = d
				best = i
			}
		}
	}
	if best == -1 {
		return findNearestTaxiPoint(biasX, biasZ, points)
	}
	return best
}

// findHoldingShortNode returns the taxiway node that sits at the holding-short
// position nearest to (threshX, threshZ). It works by:
//  1. Collecting all nodes that appear in any type=2 (RUNWAY surface) path.
//  2. Scanning type=3 (PARKING connector) paths: the non-runway endpoint of a
//     path that connects to a runway node is a holding-short node.
//  3. Returning whichever holding-short node is geographically closest to the
//     runway threshold in BiasX/BiasZ space.
//
// Returns -1 if no holding-short nodes are found (caller should fall back to
// findNearestReachableNode).
func findHoldingShortNode(paths []taxiPath, points []taxiPoint, threshX, threshZ float64) int {
	// Step 1: mark every node that appears in a type=2 (runway surface) path.
	runwayNodes := make(map[int]bool)
	for _, p := range paths {
		if p.Type == 2 {
			runwayNodes[int(p.Start)] = true
			runwayNodes[int(p.End)] = true
		}
	}

	// Step 2: find the nearest holding-short node.
	// A holding-short node is the non-runway endpoint of a type=3 path whose
	// other endpoint is a runway node.
	best, bestDist := -1, math.MaxFloat64
	for _, p := range paths {
		if p.Type != 3 {
			continue
		}
		s, e := int(p.Start), int(p.End)
		var candidate int
		switch {
		case runwayNodes[s] && !runwayNodes[e]:
			candidate = e
		case runwayNodes[e] && !runwayNodes[s]:
			candidate = s
		default:
			continue
		}
		if candidate >= len(points) {
			continue
		}
		dx := float64(points[candidate].BiasX) - threshX
		dz := float64(points[candidate].BiasZ) - threshZ
		if d := dx*dx + dz*dz; d < bestDist {
			bestDist = d
			best = candidate
		}
	}
	return best
}

// nextNodeInDir returns the neighbor of `from` in the adjacency list whose
// direction from `from` is most aligned with (dirX, dirZ) — but ONLY if
// that alignment is positive (the neighbor is genuinely ahead in the given
// direction). Returns `from` unchanged when no neighbor lies in the correct
// half-space, so the caller can detect that no useful hop exists.
func nextNodeInDir(from int, adj [][]int, dirX, dirZ float64, points []taxiPoint) int {
	best, bestDot := from, 0.0 // threshold = 0: only take a step if dot > 0
	fromPt := points[from]
	for _, nb := range adj[from] {
		dx := float64(points[nb].BiasX) - float64(fromPt.BiasX)
		dz := float64(points[nb].BiasZ) - float64(fromPt.BiasZ)
		dot := dx*dirX + dz*dirZ
		if dot > bestDot {
			bestDot = dot
			best = nb
		}
	}
	return best
}

// bfsTaxiPath returns the shortest hop-count path from start to end through
// the taxi-point graph, or nil if the two nodes are not connected.
func bfsTaxiPath(adj [][]int, start, end int) []int {
	if start == end {
		return []int{start}
	}
	n := len(adj)
	prev := make([]int, n)
	for i := range prev {
		prev[i] = -1
	}
	prev[start] = start
	queue := []int{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == end {
			// Reconstruct path by walking prev back to start then reversing.
			var path []int
			for cur != start {
				path = append(path, cur)
				cur = prev[cur]
			}
			path = append(path, start)
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			return path
		}
		for _, nb := range adj[cur] {
			if prev[nb] == -1 {
				prev[nb] = cur
				queue = append(queue, nb)
			}
		}
	}
	return nil // disconnected
}

// weightedEdge is an adjacency-list entry carrying the Euclidean distance
// between two taxi-point nodes. Used by dijkstraTaxiPath.
type weightedEdge struct {
	to   int
	dist float64
}

// buildWeightedAdjacency mirrors buildAdjacency but returns edge weights
// (Euclidean distance in meters). Same path-type and segment-length filters.
func buildWeightedAdjacency(paths []taxiPath, points []taxiPoint) [][]weightedEdge {
	nPoints := len(points)
	adj := make([][]weightedEdge, nPoints)
	for _, p := range paths {
		switch p.Type {
		case 1, 3, 4: // TAXI, PARKING spur connectors, PATH
		default:
			continue
		}
		s, e := int(p.Start), int(p.End)
		if s >= nPoints || e >= nPoints {
			continue
		}
		dx := float64(points[s].BiasX) - float64(points[e].BiasX)
		dz := float64(points[s].BiasZ) - float64(points[e].BiasZ)
		dist := math.Sqrt(dx*dx + dz*dz)
		if dist > maxSegmentMeters {
			continue
		}
		adj[s] = append(adj[s], weightedEdge{e, dist})
		adj[e] = append(adj[e], weightedEdge{s, dist})
	}
	return adj
}

// dijkstraTaxiPath returns the shortest-DISTANCE path from start to end
// through the taxi-point graph. Edge weights are the Euclidean distance
// between the two endpoint nodes. Uses a simple O(V²) scan which is fast
// enough for the <2000-node taxi graph.
// Returns nil if start and end are not connected.
func dijkstraTaxiPath(adj [][]weightedEdge, start, end int) []int {
	if start == end {
		return []int{start}
	}
	n := len(adj)
	dist := make([]float64, n)
	prev := make([]int, n)
	visited := make([]bool, n)
	for i := range dist {
		dist[i] = math.MaxFloat64
		prev[i] = -1
	}
	dist[start] = 0
	prev[start] = start
	for {
		// Pick the unvisited node with the smallest tentative distance.
		u := -1
		for i, d := range dist {
			if !visited[i] && d < math.MaxFloat64 {
				if u == -1 || d < dist[u] {
					u = i
				}
			}
		}
		if u == -1 || u == end {
			break
		}
		visited[u] = true
		for _, e := range adj[u] {
			if nd := dist[u] + e.dist; nd < dist[e.to] {
				dist[e.to] = nd
				prev[e.to] = u
			}
		}
	}
	if prev[end] == -1 {
		return nil // disconnected
	}
	// Reconstruct path.
	var path []int
	for cur := end; cur != start; cur = prev[cur] {
		path = append(path, cur)
	}
	path = append(path, start)
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// taxiExitNode returns the taxi-network node the aircraft should be pushed
// back to from the given gate.  It finds the gate's spur-entry node (the
// nearest taxi point in the pushback half-space) and then steps one hop
// further along the taxiway centreline so the aircraft lands squarely on
// the main taxiway, not just at the spur connection point.
// Returns (entryNode, startNode, pushDirX, pushDirZ).
func taxiExitNode(gate parkingSpot, adj [][]int, points []taxiPoint) (entryNode, startNode int, pushDirX, pushDirZ float64) {
	pushDirX = math.Sin((float64(gate.Heading) + 180) * math.Pi / 180)
	pushDirZ = math.Cos((float64(gate.Heading) + 180) * math.Pi / 180)
	entryNode = findNearestTaxiPointInDir(
		float64(gate.BiasX), float64(gate.BiasZ),
		pushDirX, pushDirZ, points,
	)
	startNode = nextNodeInDir(entryNode, adj, pushDirX, pushDirZ, points)
	return
}

// gateSpawnHeading computes the heading the aircraft should face when spawned
// at the gate so that it is perfectly aligned with the taxiway it will enter
// after pushback.
//
// The direction from entryNode → startNode is the taxiway heading in the
// pushback direction.  The aircraft spawns facing 180° from this (nose
// pointing toward the terminal / away from the taxiway), which means after
// the REVERSE waypoint the nose will face toward the runway.
func gateSpawnHeading(gate parkingSpot, taxiPaths []taxiPath, taxiPoints []taxiPoint) float64 {
	if len(taxiPaths) == 0 || len(taxiPoints) == 0 {
		return float64(gate.Heading)
	}
	adj := buildAdjacency(taxiPaths, taxiPoints)
	entryNode, startNode, _, _ := taxiExitNode(gate, adj, taxiPoints)
	if startNode == entryNode {
		return float64(gate.Heading) // no usable hop found, keep original
	}
	entryPt := taxiPoints[entryNode]
	startPt := taxiPoints[startNode]
	dx := float64(startPt.BiasX) - float64(entryPt.BiasX)
	dz := float64(startPt.BiasZ) - float64(entryPt.BiasZ)
	// Bearing in degrees: atan2(east, north) — X is east, Z is north in SimConnect.
	bearing := math.Atan2(dx, dz) * 180 / math.Pi
	// Spawn heading = opposite (nose faces terminal, tail toward taxiway).
	return math.Mod(bearing+180+360, 360)
}

// departureWaypoints builds the full gate → pushback → taxi → lineup → climb
// waypoint chain for a single departing aircraft.
//
// Flag usage:
//   - WP[0]:     ON_GROUND | REVERSE | SPEED_REQUESTED
//                The AI backs the aircraft from the spawn (gate) TO this point.
//                Speed 3 kts mimics a realistic pushback truck pace.
//   - WP[1..N]:  ON_GROUND | SPEED_REQUESTED  (15 kts taxi speed)
//                Dijkstra-routed path through the taxi network (shortest distance).
//   - WP[N+1]:   ON_GROUND | SPEED_REQUESTED  (5 kts for precise lineup)
//                Runway threshold — last ground waypoint before takeoff roll.
//   - WP[N+2..]: SPEED_REQUESTED | THROTTLE_REQUESTED |
//                COMPUTE_VERTICAL_SPEED | ALTITUDE_IS_AGL
//                First non-ON_GROUND WP triggers takeoff roll + rotation.
//                COMPUTE_VERTICAL_SPEED lets the sim derive the required VS.
func departureWaypoints(gate parkingSpot, taxiPaths []taxiPath, taxiNames []taxiName, taxiPoints []taxiPoint, rwy runwayData, ap apInfo) []types.SIMCONNECT_DATA_WAYPOINT {
	altFt := convert.MetersToFeet(ap.Altitude)

	// Runway threshold: primary end = center displaced half-length opposite the
	// primary heading.
	threshLat, threshLon := aheadOf(rwy.Latitude, rwy.Longitude, rwy.Heading+180, rwy.Length/2)

	var wps []types.SIMCONNECT_DATA_WAYPOINT

	// WP[0]: pushback endpoint — two-hop approach:
	//   1. Find nearest taxi node in the pushback half-space (gate spur entry).
	//   2. Step one more hop in the pushback direction to the taxiway centreline.
	// This ensures the aircraft ends up on the main taxiway, not at the spur stub.
	if len(taxiPoints) > 0 && len(taxiPaths) > 0 {
		// Build both unweighted (for flood-fill) and weighted (for Dijkstra) graphs.
		adj := buildAdjacency(taxiPaths, taxiPoints)
		wAdj := buildWeightedAdjacency(taxiPaths, taxiPoints)

		entryNode, startNode, _, _ := taxiExitNode(gate, adj, taxiPoints)

		pushPt := taxiPoints[startNode]
		pushLat, pushLon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
			float64(pushPt.BiasX), float64(pushPt.BiasZ))
		fmt.Printf("📍 Pushback: entry node %d → start node %d (BiasX=%.1f BiasZ=%.1f)\n",
			entryNode, startNode, pushPt.BiasX, pushPt.BiasZ)

		// WP[0]: REVERSE to the taxiway centreline node.
		wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{
			Latitude:  pushLat,
			Longitude: pushLon,
			Altitude:  altFt,
			Flags: uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND |
				types.SIMCONNECT_WAYPOINT_REVERSE |
				types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED),
			KtsSpeed: 3,
		})

		// WP[1..N]: Dijkstra from the taxiway centreline node to the holding-short
		// node (the taxiway node that connects to the runway via a type=3 path),
		// or the nearest reachable node if no holding-short node is found.
		// Distance-weighted routing produces the most direct geographic route.
		threshX, threshZ := latLonToOffset(ap.Latitude, ap.Longitude, threshLat, threshLon)
		endNode := findHoldingShortNode(taxiPaths, taxiPoints, threshX, threshZ)
		if endNode == -1 {
			endNode = findNearestReachableNode(startNode, adj, threshX, threshZ, taxiPoints)
		}

		path := dijkstraTaxiPath(wAdj, startNode, endNode)
		endPt := taxiPoints[endNode]
		endLat, endLon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
			float64(endPt.BiasX), float64(endPt.BiasZ))
		fmt.Printf("🗺️  Taxi path: %d steps/Dijkstra (node %d → node %d) end=%.4f,%.4f\n",
			len(path), startNode, endNode, endLat, endLon)
		fmt.Printf("   Thresh: %.4f,%.4f  EndNode BiasX=%.1f BiasZ=%.1f\n",
			threshLat, threshLon, endPt.BiasX, endPt.BiasZ)

		// Build edge→taxiway-name lookup from paths (undirected).
		type edgeKey struct{ a, b int }
		edgeNames := make(map[edgeKey]string)
		for _, p := range taxiPaths {
			ni := int(p.NameIndex)
			name := ""
			if ni < len(taxiNames) {
				name = engine.BytesToString(taxiNames[ni].Name[:])
			}
			s, e := int(p.Start), int(p.End)
			edgeNames[edgeKey{s, e}] = name
			edgeNames[edgeKey{e, s}] = name
		}

		// Print named-taxiway route summary, deduplicating consecutive identical names.
		if len(path) > 1 {
			fmt.Print("🛣️  Route via: ")
			prev := ""
			for i := 1; i < len(path); i++ {
				name := edgeNames[edgeKey{path[i-1], path[i]}]
				if name == "" {
					name = "?"
				}
				if name != prev {
					if prev != "" {
						fmt.Print(" → ")
					}
					fmt.Print(name)
					prev = name
				}
			}
			fmt.Println(" → threshold")
		}

		for i, idx := range path {
			pt := taxiPoints[idx]
			ptLat, ptLon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
				float64(pt.BiasX), float64(pt.BiasZ))
			fmt.Printf("   WP[%02d] node=%d lat=%.4f lon=%.4f (BiasX=%.1f BiasZ=%.1f)\n",
				i+1, idx, ptLat, ptLon, pt.BiasX, pt.BiasZ)
			wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{
				Latitude:  ptLat,
				Longitude: ptLon,
				Altitude:  altFt,
				Flags: uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND |
					types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED),
				KtsSpeed: 15,
			})
		}
	}

	// WP[N+1]: runway threshold lineup — slow to 5 kts for precise positioning.
	wps = append(wps, types.SIMCONNECT_DATA_WAYPOINT{
		Latitude:  threshLat,
		Longitude: threshLon,
		Altitude:  altFt,
		Flags: uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND |
			types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED),
		KtsSpeed: 5,
	})

	// WP[N+2..]: airborne climb sequence.
	// The transition from ON_GROUND → no ON_GROUND is what triggers the takeoff roll.
	// COMPUTE_VERTICAL_SPEED lets the sim derive the required VS to reach each
	// altitude at the waypoint. ALTITUDE_IS_AGL makes altitudes terrain-relative.
	climbFlags := uint32(
		types.SIMCONNECT_WAYPOINT_SPEED_REQUESTED |
			types.SIMCONNECT_WAYPOINT_THROTTLE_REQUESTED |
			types.SIMCONNECT_WAYPOINT_COMPUTE_VERTICAL_SPEED |
			types.SIMCONNECT_WAYPOINT_ALTITUDE_IS_AGL,
	)
	c1Lat, c1Lon := aheadOf(threshLat, threshLon, rwy.Heading, 2778)  // 1.5 nm
	c2Lat, c2Lon := aheadOf(threshLat, threshLon, rwy.Heading, 9260)  // 5 nm
	c3Lat, c3Lon := aheadOf(threshLat, threshLon, rwy.Heading, 22224) // 12 nm
	wps = append(wps,
		types.SIMCONNECT_DATA_WAYPOINT{
			Latitude: c1Lat, Longitude: c1Lon, Altitude: 1500,
			Flags: climbFlags, KtsSpeed: 200, PercentThrottle: 100,
		},
		types.SIMCONNECT_DATA_WAYPOINT{
			Latitude: c2Lat, Longitude: c2Lon, Altitude: 4000,
			Flags: climbFlags, KtsSpeed: 240, PercentThrottle: 90,
		},
		types.SIMCONNECT_DATA_WAYPOINT{
			Latitude: c3Lat, Longitude: c3Lon, Altitude: 9000,
			Flags: climbFlags, KtsSpeed: 280, PercentThrottle: 85,
		},
	)
	return wps
}

// printAllTaxiways prints a summary of every unique taxiway name, writes one
// GPX file per name, and also prints which taxiways are adjacent to gate.
// This replaces the single-name printTaxiwayPoints used during investigation.
func printAllTaxiways(paths []taxiPath, names []taxiName, points []taxiPoint, ap apInfo, gate parkingSpot) {
	// Print the index→name table so we can verify the NameIndex mapping.
	fmt.Printf("\n📋 TAXI_NAME index table (%d entries):\n", len(names))
	for i, n := range names {
		fmt.Printf("  [%d] = %q\n", i, engine.BytesToString(n.Name[:]))
	}
	fmt.Println()

	// pathTypeLabel is used in GPX waypoint names.
	pathTypeLabel := map[uint32]string{
		0: "NONE", 1: "TAXI", 2: "RUNWAY", 3: "PARKING",
		4: "PATH", 5: "CLOSED", 6: "VEHICLE", 7: "ROAD", 8: "PAINTEDLINE",
	}

	// Build map: taxiway name → node index → set of path types referencing it.
	// Storing path types lets the GPX label each point with what kind of path
	// caused it to appear (TAXI, PARKING spur, VEHICLE road, etc.).
	type nodeTypes = map[int]map[uint32]bool
	taxiwayNodes := make(map[string]nodeTypes)

	addNode := func(twName string, nodeIdx int, pathType uint32) {
		if taxiwayNodes[twName] == nil {
			taxiwayNodes[twName] = make(nodeTypes)
		}
		if taxiwayNodes[twName][nodeIdx] == nil {
			taxiwayNodes[twName][nodeIdx] = make(map[uint32]bool)
		}
		taxiwayNodes[twName][nodeIdx][pathType] = true
	}

	for _, p := range paths {
		idx := int(p.NameIndex)
		if idx >= len(names) {
			continue
		}
		n := engine.BytesToString(names[idx].Name[:])
		if n == "" {
			n = "(unnamed)"
		}
		if s := int(p.Start); s < len(points) {
			addNode(n, s, p.Type)
		}
		if e := int(p.End); e < len(points) {
			addNode(n, e, p.Type)
		}
	}

	fmt.Printf("\n🛤️  Taxiway network at %s (%d names):\n", icao, len(taxiwayNodes))
	fmt.Println("name,nodes,biasX_min,biasX_max,biasZ_min,biasZ_max")

	// For each taxiway: print summary line and write GPX.
	for twName, ns := range taxiwayNodes {
		minX, maxX := math.MaxFloat64, -math.MaxFloat64
		minZ, maxZ := math.MaxFloat64, -math.MaxFloat64
		for i := range ns {
			x := float64(points[i].BiasX)
			z := float64(points[i].BiasZ)
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if z < minZ {
				minZ = z
			}
			if z > maxZ {
				maxZ = z
			}
		}
		fmt.Printf("%s,%d,%.0f,%.0f,%.0f,%.0f\n",
			twName, len(ns), minX, maxX, minZ, maxZ)

		// Write one GPX per taxiway name (skip "(unnamed)").
		if twName == "(unnamed)" {
			continue
		}
		gpxFile := fmt.Sprintf("taxiway-%s.gpx", twName)
		if f, err := os.Create(gpxFile); err == nil {
			fmt.Fprintln(f, `<?xml version="1.0" encoding="UTF-8"?>`)
			fmt.Fprintln(f, `<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1" creator="manage-traffic">`)
			for i, ptTypes := range ns {
				pt := points[i]
				lat, lon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
					float64(pt.BiasX), float64(pt.BiasZ))
				// Build a label: node index + all path types that reference this node.
				// e.g. "123:PATH" or "456:TAXI+PARKING"
				label := fmt.Sprintf("%d:", i)
				sep := ""
				for pt32 := uint32(0); pt32 <= 8; pt32++ {
					if ptTypes[pt32] {
						lbl := pathTypeLabel[pt32]
						if lbl == "" {
							lbl = fmt.Sprintf("T%d", pt32)
						}
						label += sep + lbl
						sep = "+"
					}
				}
				fmt.Fprintf(f, "  <wpt lat=\"%.6f\" lon=\"%.6f\"><name>%s</name></wpt>\n", lat, lon, label)
			}
			fmt.Fprintln(f, "</gpx>")
			f.Close()
		}
	}
	fmt.Println()

	// Show which taxiways have nodes near gate (within 500 m).
	gx, gz := float64(gate.BiasX), float64(gate.BiasZ)
	fmt.Printf("🅿️  Gate #%d (BiasX=%.0f BiasZ=%.0f) — nearby taxiways (≤500 m):\n",
		gate.Number, gx, gz)
	for twName, ns := range taxiwayNodes {
		closest := math.MaxFloat64
		for i := range ns {
			dx := float64(points[i].BiasX) - gx
			dz := float64(points[i].BiasZ) - gz
			if d := math.Sqrt(dx*dx + dz*dz); d < closest {
				closest = d
			}
		}
		if closest <= 500 {
			fmt.Printf("  %-12s  nearest node %.0f m away\n", twName, closest)
		}
	}
	fmt.Println()
}

// writeTaxiCSV dumps taxi points and taxi paths as CSV files for inspection.
// taxi_points.csv: index, lat, lon, biasX, biasZ, type, orientation
// taxi_paths.csv:  index, start, end, type, typeName, nameIndex, taxiwayName, distM, inGraph
func writeTaxiCSV(points []taxiPoint, paths []taxiPath, names []taxiName, ap apInfo) {
	pathTypeNames := map[uint32]string{
		0: "NONE", 1: "TAXI", 2: "RUNWAY", 3: "PARKING",
		4: "PATH", 5: "CLOSED", 6: "VEHICLE", 7: "ROAD", 8: "PAINTEDLINE",
	}

	// taxi_points.csv
	if fp, err := os.Create("taxi_points.csv"); err == nil {
		defer fp.Close()
		fmt.Fprintln(fp, "index,lat,lon,biasX_m,biasZ_m,type,orientation")
		for i, pt := range points {
			lat, lon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
				float64(pt.BiasX), float64(pt.BiasZ))
			fmt.Fprintf(fp, "%d,%.6f,%.6f,%.2f,%.2f,%d,%d\n",
				i, lat, lon, pt.BiasX, pt.BiasZ, pt.Type, pt.Orientation)
		}
		fmt.Printf("📄 taxi_points.csv written (%d rows)\n", len(points))
	}

	// taxi_paths.csv
	nPoints := len(points)
	if fp, err := os.Create("taxi_paths.csv"); err == nil {
		defer fp.Close()
		fmt.Fprintln(fp, "index,start,end,type,typeName,nameIndex,taxiwayName,dist_m,inGraph")
		for i, p := range paths {
			typeName := pathTypeNames[p.Type]
			dist := 0.0
			inGraph := ""
			s, e := int(p.Start), int(p.End)
			if s < nPoints && e < nPoints {
				dx := float64(points[s].BiasX) - float64(points[e].BiasX)
				dz := float64(points[s].BiasZ) - float64(points[e].BiasZ)
				dist = math.Sqrt(dx*dx + dz*dz)
				switch p.Type {
				case 1, 4:
					if dist <= maxSegmentMeters {
						inGraph = "unweighted+weighted"
					}
				case 3:
					if dist <= maxSegmentMeters {
						inGraph = "weighted"
					}
				}
			} else {
				inGraph = "out-of-range"
			}
			twName := ""
			if ni := int(p.NameIndex); ni < len(names) {
				twName = engine.BytesToString(names[ni].Name[:])
			}
			fmt.Fprintf(fp, "%d,%d,%d,%d,%s,%d,%s,%.2f,%s\n",
				i, p.Start, p.End, p.Type, typeName, p.NameIndex, twName, dist, inGraph)
		}
		fmt.Printf("📄 taxi_paths.csv written (%d rows)\n", len(paths))
	}
}

// writeGPX writes the ON_GROUND waypoints from wps as a GPX track file.
// Open the file in Google Earth, Google My Maps (import), or any GPS tool.
func writeGPX(wps []types.SIMCONNECT_DATA_WAYPOINT, filename string) {
	f, err := os.Create(filename)
	if err != nil {
		fmt.Printf("⚠️  GPX write failed: %v\n", err)
		return
	}
	defer f.Close()
	fmt.Fprintln(f, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(f, `<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1" creator="manage-traffic">`)
	fmt.Fprintln(f, `  <trk><name>Taxi Route</name><trkseg>`)
	count := 0
	for _, wp := range wps {
		if wp.Flags&uint32(types.SIMCONNECT_WAYPOINT_ON_GROUND) == 0 {
			continue // skip airborne climb waypoints
		}
		fmt.Fprintf(f, "    <trkpt lat=\"%.6f\" lon=\"%.6f\"/>\n", wp.Latitude, wp.Longitude)
		count++
	}
	fmt.Fprintln(f, `  </trkseg></trk>`)
	fmt.Fprintln(f, `</gpx>`)
	fmt.Printf("🗺️  GPX written: %s (%d ground waypoints)\n", filename, count)
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

	// Taxiway names — maps NameIndex → name string (e.g. "A", "R", "C")
	client.AddToFacilityDefinition(defFacTaxiName, "OPEN AIRPORT")
	client.AddToFacilityDefinition(defFacTaxiName, "OPEN TAXI_NAME")
	client.AddToFacilityDefinition(defFacTaxiName, "NAME")
	client.AddToFacilityDefinition(defFacTaxiName, "CLOSE TAXI_NAME")
	client.AddToFacilityDefinition(defFacTaxiName, "CLOSE AIRPORT")

	// Fire all 6 requests — each produces N FACILITY_DATA + 1 FACILITY_DATA_END.
	client.RequestFacilityData(defFacAirport, reqFacAirport, icao, "")
	client.RequestFacilityData(defFacParking, reqFacParking, icao, "")
	client.RequestFacilityData(defFacTaxiPath, reqFacTaxiPath, icao, "")
	client.RequestFacilityData(defFacTaxiPt, reqFacTaxiPt, icao, "")
	client.RequestFacilityData(defFacRunway, reqFacRunway, icao, "")
	client.RequestFacilityData(defFacTaxiName, reqFacTaxiName, icao, "")
}

// ── Traffic spawning ───────────────────────────────────────────────────────

// spawnTraffic is called once all 5 facility batches have been received.
func spawnTraffic(client engine.Client, st *trafficState) {
	ap := st.airport
	altFt := convert.MetersToFeet(ap.Altitude)

	// Collect valid parking spots (those with a gate number assigned).
	var validSpots []parkingSpot
	for _, spot := range st.parking {
		if spot.Number > 0 {
			validSpots = append(validSpots, spot)
		}
	}
	if len(validSpots) == 0 {
		fmt.Println("⚠️  No usable gate spots found at", icao)
	}

	// 1. Static parked aircraft at the first maxGates valid gate spots.
	for i := 0; i < maxGates && i < len(validSpots); i++ {
		spot := validSpots[i]
		lat, lon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
			float64(spot.BiasX), float64(spot.BiasZ))
		tail := fmt.Sprintf("G%03d", spot.Number)
		reqID := reqSpawnGate0 + uint32(i)
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
			i+1, spot.Number, tail, reqID)
	}

	// 2. Departing aircraft — pick the first valid spot past the static ones
	//    (falls back to spot 0 if there are not enough spots).
	var chosenRwy *runwayData
	for i := range st.runways {
		if st.runways[i].Heading != 0 && st.runways[i].Length > 0 {
			chosenRwy = &st.runways[i]
			break
		}
	}
	if chosenRwy == nil && len(st.runways) > 0 {
		chosenRwy = &st.runways[0]
	}

	const deptGateNumber = 10 // change this to test different gates
	deptIdx := maxGates       // fallback: first spot past the static ones
	for i, spot := range validSpots {
		if spot.Number == deptGateNumber {
			deptIdx = i
			break
		}
	}
	if deptIdx >= len(validSpots) {
		deptIdx = 0
	}

	if chosenRwy != nil && len(validSpots) > 0 {
		st.selectedRunway = *chosenRwy
		st.deptGate = validSpots[deptIdx]
		gate := st.deptGate

		// Compute spawn heading aligned with the actual taxiway: the direction
		// from the gate spur entry node to the taxiway centreline node, reversed
		// so the aircraft nose faces the terminal (tail toward the taxiway).
		// After the REVERSE waypoint the nose will face toward the runway.
		spawnHdg := gateSpawnHeading(gate, st.taxiPaths, st.taxiPoints)

		gateLat, gateLon := convert.OffsetToLatLon(ap.Latitude, ap.Longitude,
			float64(gate.BiasX), float64(gate.BiasZ))
		client.AICreateNonATCAircraftEX1(
			model, "", "DEPT01",
			types.SIMCONNECT_DATA_INITPOSITION{
				Latitude:  gateLat,
				Longitude: gateLon,
				Altitude:  altFt,
				Heading:   spawnHdg,
				OnGround:  1,
				Airspeed:  0,
			},
			reqSpawnDept,
		)
		fmt.Printf("🛫 Departure aircraft at gate #%d gate-hdg=%.1f° spawn-hdg=%.1f° rwy hdg=%.1f° (reqID=%d)\n",
			gate.Number, gate.Heading, spawnHdg, chosenRwy.Heading, reqSpawnDept)
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

	st := &trafficState{pending: 6}

	// Collect all spawned object IDs for cleanup on shutdown.
	allIDs := func() []uint32 {
		ids := make([]uint32, 0, maxGates+1)
		for _, id := range st.gateIDs {
			ids = append(ids, id)
		}
		ids = append(ids, st.deptID)
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
				case reqFacTaxiName:
					st.taxiNames = append(st.taxiNames,
						*engine.CastDataAs[taxiName](&fd.Data))
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
					logPathTypes(st.taxiPaths)

					// Resolve departure gate so we can show nearby taxiways.
					var validSpotsDiag []parkingSpot
					for _, spot := range st.parking {
						if spot.Number > 0 {
							validSpotsDiag = append(validSpotsDiag, spot)
						}
					}
					const deptGateNumberDiag = 10
					deptGateDiag := validSpotsDiag[0] // fallback
					for _, spot := range validSpotsDiag {
						if spot.Number == deptGateNumberDiag {
							deptGateDiag = spot
							break
						}
					}
					printAllTaxiways(st.taxiPaths, st.taxiNames, st.taxiPoints, st.airport, deptGateDiag)
					writeTaxiCSV(st.taxiPoints, st.taxiPaths, st.taxiNames, st.airport)
					// Connectivity sanity check: count isolated nodes in the routed graph.
					adj0 := buildAdjacency(st.taxiPaths, st.taxiPoints)
					isolated := 0
					for _, nb := range adj0 {
						if len(nb) == 0 {
							isolated++
						}
					}
					fmt.Printf("🔗 Graph: %d nodes, %d isolated (degree 0 in type-1/4 graph)\n", len(adj0), isolated)
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

				case reqID == reqSpawnDept:
					st.deptID = objID
					fmt.Printf("✅ Departure aircraft: id=%d — releasing AI control\n", objID)
					client.AIReleaseControl(objID, reqReleaseDept)
					wps := departureWaypoints(st.deptGate, st.taxiPaths, st.taxiNames, st.taxiPoints,
						st.selectedRunway, st.airport)
					writeGPX(wps, fmt.Sprintf("route-gate%d.gpx", st.deptGate.Number))
					fmt.Printf("📋 Sending %d waypoints (pushback + %d taxi + lineup + 3 climb)\n",
						len(wps), len(wps)-5)
					if err := sendWaypoints(client, objID, wps); err != nil {
						fmt.Fprintf(os.Stderr, "❌ departure waypoints: %v\n", err)
					}
					client.RequestDataOnSimObject(reqMonitorDept, defMonitor, objID,
						types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
				}

			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				if uint32(d.DwRequestID) != reqMonitorDept {
					continue
				}
				pos := engine.CastDataAs[monitorData](&d.DwData)
				fmt.Printf("📡 [DEPT] id=%d lat=%.4f lon=%.4f alt=%.0fft hdg=%.1f°\n",
					uint32(d.DwObjectID),
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
	if wd, err := os.Getwd(); err == nil {
		fmt.Println("📁 Output dir:", wd)
	}

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
