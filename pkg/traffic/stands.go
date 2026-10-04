package traffic

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var (
	// ErrStandTaken is returned by Occupy for a stand that is occupied, or
	// blocked by an aircraft on an overlapping stand.
	ErrStandTaken = errors.New("traffic: stand is taken")
	// ErrNoStand is returned by Assign when no free stand fits.
	ErrNoStand = errors.New("traffic: no free suitable stand")
)

// Occupant is who holds a stand: a reservation made through the allocator
// (Owner set) or an aircraft found standing on it by Scan (Detected).
type Occupant struct {
	Owner    string  `json:"owner,omitempty"`
	HalfSpan float64 `json:"halfSpan"` // meters
	Detected bool    `json:"detected"`
	ObjectID uint32  `json:"objectId,omitempty"` // detected aircraft
	// OffBlock is when the reserved aircraft is due off the stand (zero:
	// not known).
	OffBlock time.Time `json:"offBlock,omitempty"`
}

// StandRequirements describe the aircraft a stand is wanted for.
type StandRequirements struct {
	// Owner names who takes the stand, e.g. the tail number.
	Owner string
	// HalfSpan is half the wing span in meters; 0 uses DefaultHalfSpanMeters.
	HalfSpan float64
	// Airline prefers the stands assigned to this airline code; when none of
	// them is free, any stand that serves every airline is taken.
	Airline string
	// Types limits the parking TYPEs, e.g. gates only; empty allows all.
	Types []types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE
	// Runway is the arrival runway end: the stand with the shortest taxi-in
	// from its best exit wins. Empty ranks by stand index.
	Runway string
	// OffBlock is when the aircraft is due to leave the stand: a departure's
	// STD, an arrival's turnaround. Stands next to one whose aircraft is
	// due off at nearly the same time rank lower (StandPushConflictWindow),
	// so neighbours do not push into each other. Zero: not known.
	OffBlock time.Time
}

// StandAllocator assigns stands at one airport and keeps track of who is on
// them (#292): reservations made with Occupy or Assign, and aircraft that
// Scan finds standing on stands (the user and MSFS AI traffic). An
// aircraft on a stand blocks the stands that overlap it
// (airport.Layout.ParkingConflicts) when the two aircraft's half spans and
// StandWingtipClearanceMeters do not fit between the stand centres.
//
// Feed every message to Handle from the message loop; call Scan
// periodically (e.g. every 10 s) to refresh the detected aircraft.
type StandAllocator struct {
	client           engine.Client
	g                *airport.Graph
	defBase, reqBase uint32
	radius           uint32 // scan radius, meters
	registered       bool

	mu       sync.Mutex
	reserved map[int]Occupant
	detected map[int]Occupant
	partial  map[uint32][]scanned // per scan request, until its last entry
	lastScan map[uint32][]scanned
	routes   map[string][]airport.NodeID
	// spread: stands within this share of the best taxi-in (plus
	// StandSpreadMeters) are picked at random (rng), so a schedule does not
	// fill the same gates every time; 0 picks the best.
	spread float64
	rng    *rand.Rand
}

// StandWithSpread sets how far from the best taxi-in a stand may be and
// still be picked at random (default StandSpread); 0 always picks the
// stand with the shortest taxi-in.
func StandWithSpread(share float64) StandOption {
	return func(a *StandAllocator) { a.spread = math.Max(0, share) }
}

type scanned struct {
	object uint32
	data   standScanData
}

// standScanData is one aircraft of a scan, in definition order.
type standScanData struct {
	Lat, Lon      float64 // degrees
	OnGround      float64
	GroundSpeedKt float64
	WingSpanFt    float64
}

// StandOption configures a StandAllocator.
type StandOption func(*StandAllocator)

// StandWithIDs sets the SimConnect definition and request ID bases (one
// definition ID and two request IDs are used).
func StandWithIDs(defBase, reqBase uint32) StandOption {
	return func(a *StandAllocator) { a.defBase, a.reqBase = defBase, reqBase }
}

const (
	standReqAircraft = iota
	standReqUser
)

// NewStandAllocator creates an allocator for the airport of g. client may be
// nil when only reservations are used (no Scan).
func NewStandAllocator(client engine.Client, g *airport.Graph, opts ...StandOption) *StandAllocator {
	a := &StandAllocator{
		client: client, g: g,
		defBase: DefaultStandDefinitionBase, reqBase: DefaultStandRequestBase,
		reserved: map[int]Occupant{}, detected: map[int]Occupant{},
		partial: map[uint32][]scanned{}, lastScan: map[uint32][]scanned{},
		routes: map[string][]airport.NodeID{},
		spread: StandSpread,
		rng:    rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x5354414e44)),
	}
	for _, o := range opts {
		o(a)
	}
	// Scan out to the farthest stand from the airport reference, plus margin.
	far := 0.0
	ref := airport.LatLon{Lat: g.Layout.Latitude, Lon: g.Layout.Longitude}
	for _, p := range g.Layout.Parking {
		far = math.Max(far, localDist(ref, p.Position))
	}
	a.radius = uint32(far + 500)
	return a
}

func (a *StandAllocator) valid(stand int) bool {
	return stand >= 0 && stand < len(a.g.Layout.Parking)
}

// Occupant returns who holds the stand: a reservation first, else a
// detected aircraft.
func (a *StandAllocator) Occupant(stand int) (Occupant, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.occupant(stand)
}

// occupant merges a reservation with an aircraft detected on the same
// stand: the owner stays, the object ID and the measured span come from the
// scan.
func (a *StandAllocator) occupant(stand int) (Occupant, bool) {
	o, reserved := a.reserved[stand]
	d, detected := a.detected[stand]
	switch {
	case reserved && detected:
		o.Detected, o.ObjectID, o.HalfSpan = true, d.ObjectID, d.HalfSpan
		return o, true
	case reserved:
		return o, true
	}
	return d, detected
}

// Occupancy is a snapshot of every held stand, by parking index.
func (a *StandAllocator) Occupancy() map[int]Occupant {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[int]Occupant, len(a.reserved)+len(a.detected))
	for i := range a.detected {
		out[i], _ = a.occupant(i)
	}
	for i := range a.reserved {
		out[i], _ = a.occupant(i)
	}
	return out
}

// Free reports whether an aircraft of halfSpan meters can use the stand:
// nobody holds it and no aircraft on an overlapping stand is in the way.
func (a *StandAllocator) Free(stand int, halfSpan float64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.blockedBy(stand, halfSpan, "") == ""
}

// blockedBy names what keeps an aircraft of halfSpan off the stand ("" if
// nothing): its occupant, or an aircraft on an overlapping stand. A
// reservation by owner itself does not block.
func (a *StandAllocator) blockedBy(stand int, halfSpan float64, owner string) string {
	if halfSpan <= 0 {
		halfSpan = DefaultHalfSpanMeters
	}
	l := a.g.Layout
	if o, ok := a.occupant(stand); ok && (owner == "" || o.Owner != owner) {
		return fmt.Sprintf("%s holds %s", o.describe(), l.Parking[stand].Label())
	}
	for _, c := range l.ParkingConflicts(stand) {
		o, ok := a.occupant(c)
		if !ok || (owner != "" && o.Owner == owner) {
			continue
		}
		if halfSpan+o.HalfSpan+StandWingtipClearanceMeters > localDist(l.Parking[stand].Position, l.Parking[c].Position) {
			return fmt.Sprintf("%s on %s is in the way", o.describe(), l.Parking[c].Label())
		}
	}
	return ""
}

func (o Occupant) describe() string {
	switch {
	case o.Owner != "":
		return o.Owner
	case o.ObjectID != 0:
		return fmt.Sprintf("aircraft %d", o.ObjectID)
	}
	return "an aircraft"
}

// Occupy reserves the stand for owner, whose aircraft has halfSpan meters
// (0: DefaultHalfSpanMeters). It fails with ErrStandTaken if the stand is
// held by someone else or an aircraft on an overlapping stand is in the way,
// and with airport.ErrUnknownParking for an unknown stand. Occupying again
// for the same owner updates the reservation.
func (a *StandAllocator) Occupy(stand int, owner string, halfSpan float64) error {
	if !a.valid(stand) {
		return fmt.Errorf("%w: index %d", airport.ErrUnknownParking, stand)
	}
	if halfSpan <= 0 {
		halfSpan = DefaultHalfSpanMeters
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if why := a.blockedBy(stand, halfSpan, owner); why != "" {
		return fmt.Errorf("%w: %s", ErrStandTaken, why)
	}
	a.reserved[stand] = Occupant{Owner: owner, HalfSpan: halfSpan}
	return nil
}

// Release frees a reservation (detected aircraft stay until the next scan
// no longer finds them).
func (a *StandAllocator) Release(stand int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.reserved, stand)
}

// ReleaseOwner frees every stand and taxi route reserved by owner.
func (a *StandAllocator) ReleaseOwner(owner string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, o := range a.reserved {
		if o.Owner == owner {
			delete(a.reserved, i)
		}
	}
	delete(a.routes, owner)
}

// Assign reserves the best free stand for the requirements and returns its
// parking index: suitable for the span (airport.Layout.SuitableStands) and
// the TYPEs, the airline's own stands before stands open to any airline,
// then the shortest taxi-in from the arrival runway. ErrNoStand if none.
func (a *StandAllocator) Assign(req StandRequirements) (int, error) {
	half := req.HalfSpan
	if half <= 0 {
		half = DefaultHalfSpanMeters
	}
	l := a.g.Layout
	a.mu.Lock()
	var own, open []int
	for _, i := range l.SuitableStands(half, req.Types...) {
		if a.blockedBy(i, half, req.Owner) != "" {
			continue
		}
		p := l.Parking[i]
		switch {
		case req.Airline != "" && len(p.Airlines) > 0 && p.ServesAirline(req.Airline):
			own = append(own, i)
		case len(p.Airlines) == 0 || req.Airline == "":
			open = append(open, i)
		}
	}
	a.mu.Unlock()
	for _, group := range [][]int{own, open} {
		if len(group) == 0 {
			continue
		}
		for _, i := range a.rank(group, req.Runway, req.OffBlock) {
			if err := a.Occupy(i, req.Owner, half); err == nil {
				a.SetOffBlock(req.Owner, req.OffBlock)
				return i, nil
			}
		}
	}
	return -1, ErrNoStand
}

// rank orders stands by taxi-in length from the runway's best exit. Only
// the standRankCandidates nearest the runway are routed; the rest follow by
// distance. Without a runway the order is random (by index without the
// spread). Either way a stand next to one whose aircraft is due off near
// offBlock pays pushConflict.
func (a *StandAllocator) rank(stands []int, runwayEnd string, offBlock time.Time) []int {
	out := slices.Clone(stands)
	pen := map[int]float64{}
	a.mu.Lock()
	for _, i := range out {
		pen[i] = a.pushConflict(i, offBlock)
	}
	a.mu.Unlock()
	if runwayEnd == "" {
		// No runway to rank by: any suitable stand (the first by index was
		// always LKPR A1).
		if a.spread > 0 {
			a.mu.Lock()
			a.rng.Shuffle(len(out), func(x, y int) { out[x], out[y] = out[y], out[x] })
			a.mu.Unlock()
		}
		sort.SliceStable(out, func(x, y int) bool { return pen[out[x]] < pen[out[y]] })
		return out
	}
	l := a.g.Layout
	rwy, _, ok := l.RunwayEnd(runwayEnd)
	if !ok {
		return out
	}
	mid := airport.LatLon{Lat: (rwy.Primary.Threshold.Lat + rwy.Secondary.Threshold.Lat) / 2, Lon: (rwy.Primary.Threshold.Lon + rwy.Secondary.Threshold.Lon) / 2}
	cost := map[int]float64{}
	for _, i := range out {
		cost[i] = 1e7 + localDist(mid, l.Parking[i].Position)
	}
	sort.SliceStable(out, func(x, y int) bool { return cost[out[x]] < cost[out[y]] })
	for _, i := range out[:min(len(out), standRankCandidates)] {
		if _, r, err := bestExit(a.g, runwayEnd, i, airport.RouteOptions{}, exitReach{}); err == nil && r != nil {
			cost[i] = r.Length
		}
	}
	for _, i := range out {
		cost[i] += pen[i]
	}
	sort.SliceStable(out, func(x, y int) bool { return cost[out[x]] < cost[out[y]] })
	// Not always the very best: the stands nearly as close (as a stand
	// controller allocates, or a schedule's gates) in random order first.
	if a.spread > 0 && len(out) > 1 && cost[out[0]] < 1e7 {
		limit := cost[out[0]]*(1+a.spread) + StandSpreadMeters
		n := 0
		for n < len(out) && cost[out[n]] <= limit {
			n++
		}
		a.mu.Lock()
		a.rng.Shuffle(n, func(x, y int) { out[x], out[y] = out[y], out[x] })
		a.mu.Unlock()
	}
	return out
}

// ReserveRoute records the taxi route owner is about to follow and returns
// the other owners whose reserved routes share a node with it (a first,
// warning-only version of segment reservation). A new reservation replaces
// the owner's previous one.
func (a *StandAllocator) ReserveRoute(owner string, nodes []airport.NodeID) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	mine := map[airport.NodeID]bool{}
	for _, n := range nodes {
		mine[n] = true
	}
	var clash []string
	for other, route := range a.routes {
		if other == owner {
			continue
		}
		if slices.ContainsFunc(route, func(n airport.NodeID) bool { return mine[n] }) {
			clash = append(clash, other)
		}
	}
	sort.Strings(clash)
	a.routes[owner] = slices.Clone(nodes)
	return clash
}

// ReleaseRoute drops owner's taxi route reservation.
func (a *StandAllocator) ReleaseRoute(owner string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.routes, owner)
}

// Scan asks the simulator for every aircraft (AI and the user) around the
// airport; Handle turns the answers into detected stand occupants.
func (a *StandAllocator) Scan() error {
	if a.client == nil {
		return ErrNotConnected
	}
	if !a.registered {
		for i, v := range []struct{ name, unit string }{
			{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"SIM ON GROUND", "bool"},
			{"GROUND VELOCITY", "knots"}, {"WING SPAN", "feet"},
		} {
			if err := a.client.AddToDataDefinition(a.defBase, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
				return err
			}
		}
		a.registered = true
	}
	if err := a.client.RequestDataOnSimObjectType(a.reqBase+standReqAircraft, a.defBase, a.radius, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT); err != nil {
		return err
	}
	return a.client.RequestDataOnSimObjectType(a.reqBase+standReqUser, a.defBase, 0, types.SIMCONNECT_SIMOBJECT_TYPE_USER)
}

// Handle consumes the allocator's scan answers; it reports whether msg was
// one of them.
func (a *StandAllocator) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE {
		return false
	}
	d := msg.AsSimObjectDataBType()
	req := uint32(d.DwRequestID)
	if req != a.reqBase+standReqAircraft && req != a.reqBase+standReqUser {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if d.DwOutOf > 0 {
		a.partial[req] = append(a.partial[req], scanned{object: uint32(d.DwObjectID), data: *engine.CastDataAs[standScanData](&d.DwData)})
	}
	if d.DwOutOf == 0 || d.DwEntryNumber >= d.DwOutOf { // entries count from 1
		a.lastScan[req] = a.partial[req]
		delete(a.partial, req)
		a.detect()
	}
	return true
}

// detect rebuilds the detected occupants from the latest scans: an aircraft
// on the ground, below StandDetectKts, within a stand's RADIUS of it holds
// the nearest such stand.
func (a *StandAllocator) detect() {
	l := a.g.Layout
	a.detected = map[int]Occupant{}
	seen := map[uint32]bool{}
	for _, list := range a.lastScan {
		for _, s := range list {
			if seen[s.object] || s.data.OnGround < 0.5 || s.data.GroundSpeedKt > StandDetectKts {
				continue
			}
			seen[s.object] = true
			pos := airport.LatLon{Lat: s.data.Lat, Lon: s.data.Lon}
			best, bestD := -1, math.Inf(1)
			for _, p := range l.Parking {
				if p.Size() == airport.StandNone {
					continue
				}
				if d := localDist(pos, p.Position); d < p.Radius && d < bestD {
					best, bestD = p.Index, d
				}
			}
			if best < 0 {
				continue
			}
			half := s.data.WingSpanFt * 0.3048 / 2
			if half <= 0 {
				half = DefaultHalfSpanMeters
			}
			a.detected[best] = Occupant{Detected: true, ObjectID: s.object, HalfSpan: half}
		}
	}
}

// observe takes the aircraft on the ground at the airport from a
// TrafficPicture, in place of a Scan of its own.
func (a *StandAllocator) observe(list []scanned) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastScan = map[uint32][]scanned{a.reqBase + standReqAircraft: list}
	a.detect()
}

// TakenFrom is what now keeps owner, whose aircraft is object (0: not yet
// known), off the stand it holds ("" if nothing): another aircraft detected
// on it, or on an overlapping stand in the way. A reservation does not stop
// MSFS AI or the user parking there: an arrival reserves its stand long
// before it lands (#479).
func (a *StandAllocator) TakenFrom(stand int, owner string, object uint32) string {
	if !a.valid(stand) {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	l := a.g.Layout
	half := DefaultHalfSpanMeters
	if o, ok := a.reserved[stand]; ok && o.Owner == owner && o.HalfSpan > 0 {
		half = o.HalfSpan
	}
	if d, ok := a.detected[stand]; ok && d.ObjectID != object {
		return fmt.Sprintf("%s holds %s", d.describe(), l.Parking[stand].Label())
	}
	for _, c := range l.ParkingConflicts(stand) {
		d, ok := a.detected[c]
		if !ok || d.ObjectID == object {
			continue
		}
		if half+d.HalfSpan+StandWingtipClearanceMeters > localDist(l.Parking[stand].Position, l.Parking[c].Position) {
			return fmt.Sprintf("%s on %s is in the way", d.describe(), l.Parking[c].Label())
		}
	}
	return ""
}

// pushConflict is what taking stand costs an aircraft due off it at
// offBlock, in meters of taxi-in: StandPushConflictMeters for each
// reserved stand within StandPushNeighbourMeters whose aircraft is due off
// at the same time, less as the times are further apart, nothing from
// StandPushConflictWindow. a.mu is held.
func (a *StandAllocator) pushConflict(stand int, offBlock time.Time) float64 {
	if offBlock.IsZero() {
		return 0
	}
	l := a.g.Layout
	pen := 0.0
	for j, o := range a.reserved {
		if j == stand || o.OffBlock.IsZero() || !a.valid(j) {
			continue
		}
		if localDist(l.Parking[stand].Position, l.Parking[j].Position) > StandPushNeighbourMeters {
			continue
		}
		dt := o.OffBlock.Sub(offBlock).Abs()
		if dt < StandPushConflictWindow {
			pen += StandPushConflictMeters * (1 - float64(dt)/float64(StandPushConflictWindow))
		}
	}
	return pen
}

// SetOffBlock records when owner's aircraft is due off the stands it holds
// (StandRequirements.OffBlock): a turnaround's departure time once it is
// known, or a delay.
func (a *StandAllocator) SetOffBlock(owner string, t time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, o := range a.reserved {
		if o.Owner == owner {
			o.OffBlock = t
			a.reserved[i] = o
		}
	}
}

// Transfer passes every stand held by fromOwner to toOwner: a turnaround,
// the arrival's aircraft staying on its stand as the departure (#470). The
// aircraft detected there is then the new owner's own, not in the way:
// releasing the arrival's stand and occupying it for the departure failed
// with ErrStandTaken ("aircraft N holds A4").
func (a *StandAllocator) Transfer(fromOwner, toOwner string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, o := range a.reserved {
		if o.Owner == fromOwner {
			o.Owner = toOwner
			a.reserved[i] = o
		}
	}
	delete(a.routes, fromOwner)
}
