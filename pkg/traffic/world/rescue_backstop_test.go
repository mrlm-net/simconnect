package world

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The backstop (#1632): the arrival plans its exit only after the fire
// trucks took their standby points, so these were chosen blind and stand
// on its way off the runway. With points free and ways to them both move
// off it. With no way to any they are reported standing, and the arrival,
// flown here, stops short of them without a jump and goes on once they
// have gone.

// The arrival of these tests and the thing standing in its way.
const (
	backstopObject = uint32(77)
	backstopBody   = uint32(900)
	backstopFrame  = time.Second / 30
	// As #1640's limits for traffic ahead: the speed changed in a frame
	// (m/s: more is a dead stop), over half a second braking for what is
	// found near (m/s²), and the clearance.
	backstopFrameSpeed = 0.5
	backstopAccel      = 2.0
	backstopClearance  = traffic.TrafficGapMeters - traffic.TrafficOverrunMeters
)

// backstopClient keeps where the arrival is placed, which is all the
// simulator gets of it.
type backstopClient struct {
	stationClient
	placed types.SIMCONNECT_DATA_INITPOSITION
}

func (c *backstopClient) AICreateNonATCAircraft(_, _ string, _ types.SIMCONNECT_DATA_INITPOSITION, _ uint32) error {
	return nil
}

func (c *backstopClient) AICreateNonATCAircraftEX1(_, _, _ string, _ types.SIMCONNECT_DATA_INITPOSITION, _ uint32) error {
	return nil
}

func (c *backstopClient) ClearDataDefinition(uint32) error { return nil }

func (c *backstopClient) SetDataOnSimObject(_, obj uint32, _ types.SIMCONNECT_DATA_SET_FLAG, _, size uint32, p unsafe.Pointer) error {
	if obj == backstopObject && uintptr(size) == unsafe.Sizeof(c.placed) {
		c.placed = *(*types.SIMCONNECT_DATA_INITPOSITION)(p)
	}
	return nil
}

// backstopData is a data answer of the simulator carrying v.
func backstopData(req, obj uint32, v []float64) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+8*len(v))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	copy(unsafe.Slice((*float64)(unsafe.Pointer(&buf[off])), len(v)), v)
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

// backstopBed is the one-exit field with a gate at the east end of A, the
// crew standing where it was sent blind, and then the arrival with its plan.
type backstopBed struct {
	t    *testing.T
	cc   *controlCenter
	c    *stationClient
	g    *airport.Graph
	now  time.Time
	prio map[string]bool
	ac   traffic.TrackedAircraft
	arr  *planArrival
	crew *rescueCrew
	done int

	ctl      *traffic.ArrivalController
	injector *traffic.Injector
	client   *backstopClient
	prof     traffic.MotionProfile

	// holdForTaxi: it waits for its taxi clearance, a player's to give;
	// until ends fly early; unsettled: steps don't let a search end.
	holdForTaxi bool
	until       func(backstopSeen) bool
	unsettled   bool
}

func newBackstopBed(t *testing.T) *backstopBed {
	t.Helper()
	// The sixth point: the east end of A.
	const aEast = 6
	return newBackstopBedOn(t, rescueOneExitField(t), aEast)
}

// newBackstopBedOn is the bed on field l, the gate south of its taxi point
// by.
func newBackstopBedOn(t *testing.T, l *airport.Layout, by int) *backstopBed {
	t.Helper()
	lat, lon := calc.DisplaceByHeading(l.TaxiPoints[by].Position.Lat, l.TaxiPoints[by].Position.Lon, 180, 80)
	gate := len(l.Parking)
	l.Parking = append(l.Parking, airport.Parking{Index: gate, Number: 1, LabelText: "A1", Type: types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_GATE_MEDIUM,
		Heading: 180, Radius: 25, Position: airport.LatLon{Lat: lat, Lon: lon}})
	l.TaxiPaths = append(l.TaxiPaths, airport.TaxiPath{Index: len(l.TaxiPaths), Type: types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_PARKING,
		Width: 23, Start: int32(by), End: int32(gate)})
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	cc.world = traffic.NewTrafficPicture(traffic.PictureOptions{})
	g, err := cc.graph("XONE")
	if err != nil {
		t.Fatal(err)
	}
	_, end, ok := l.RunwayEnd("09")
	if !ok {
		t.Fatal("no runway 09")
	}
	b := &backstopBed{t: t, cc: cc, c: c, g: g, now: time.Now(), prio: map[string]bool{"CSA1": true},
		arr: &planArrival{}, client: &backstopClient{}, prof: traffic.ProfileFor("A320").Motion}
	it := &controlled{ID: 1, Kind: "arrival", Tail: "CSA1", ICAO: "XONE", arr: b.arr}
	it.view.Runway = "09"
	cc.items[1] = it
	w.PriorityFor("CSA1", traffic.EmergencyFire)
	lat, lon = calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, -(rescueDispatchNM-1)*1852)
	b.ac.Airport, b.ac.Tail, b.ac.Position = "XONE", "CSA1", airport.LatLon{Lat: lat, Lon: lon}
	b.ac.Heading, b.ac.GroundKts, b.ac.AGLFt = end.Heading, 140, 2000
	b.ac.Updated = b.now
	cc.rescueStep(b.now, b.prio, []traffic.TrackedAircraft{b.ac})
	b.crew = cc.rescue.by["CSA1"]
	if b.crew == nil || len(b.crew.trucks) != 2 {
		t.Fatal("no crew of 2 sent")
	}
	b.wait(10*time.Minute, nil)
	b.injector = traffic.NewInjector(b.client)
	b.ctl = traffic.NewArrivalController(traffic.NewFleet(b.client), traffic.ArrivalWithInjector(b.injector),
		traffic.ArrivalWithGroundPicture(cc.world.Ground("XONE")), traffic.ArrivalWithClock(func() time.Time { return b.now }))
	return b
}

// plan starts the arrival: from here the crew knows its way off the runway.
func (b *backstopBed) plan() []airport.LatLon {
	b.t.Helper()
	if err := b.ctl.Start(traffic.ArrivalRequest{Graph: b.g, Runway: "09", Parking: len(b.g.Layout.Parking) - 1, Model: "A320", Tail: "CSA1",
		Profile: b.prof, InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second, HoldForClearance: b.holdForTaxi}); err != nil {
		b.t.Fatal(err)
	}
	go func() {
		for range b.ctl.Events() {
		}
	}()
	b.t.Cleanup(func() { _ = b.ctl.Cancel() })
	p := b.ctl.Plan()
	b.arr.plan = p
	var way []airport.LatLon
	for _, n := range p.Exit.Path {
		way = append(way, b.g.Nodes[n].Position)
	}
	// On from the exit to the gate, less the runway.
	return append(way, p.Route.Points[min(len(p.Exit.Path), len(p.Route.Points)):]...)
}

func (b *backstopBed) step(second bool) {
	for ; b.done < len(b.c.req); b.done++ {
		b.cc.rescueHandle(assignedObject(b.c.req[b.done], 800+b.c.req[b.done]))
	}
	b.ac.Updated = b.now
	if second {
		b.cc.rescueStep(b.now, b.prio, []traffic.TrackedAircraft{b.ac})
	}
	b.cc.rescueFrame(b.now)
	if !b.unsettled {
		b.settle()
	}
}

// settle lets the crew's search end, as it has long before the next frame.
func (b *backstopBed) settle() {
	if b.crew != nil && b.crew.move != nil {
		<-b.crew.move.done
	}
}

// wait runs the crew alone for d, a tenth of a second a step.
func (b *backstopBed) wait(d time.Duration, each func()) {
	for k := 0; k < int(d/(100*time.Millisecond)); k++ {
		b.now = b.now.Add(100 * time.Millisecond)
		b.step(k%10 == 0)
		if each != nil {
			each()
		}
	}
}

// standing are the poses of the vehicles reported standing.
func (b *backstopBed) standing() []traffic.GroundPose {
	var out []traffic.GroundPose
	for t := range b.crew.reported {
		if p, _, ok := t.Track(); ok {
			out = append(out, p)
		}
	}
	return out
}

func backstopAhead(p airport.LatLon, hdg, d float64) airport.LatLon {
	lat, lon := calc.DisplaceByHeading(p.Lat, p.Lon, hdg, d)
	return airport.LatLon{Lat: lat, Lon: lon}
}

// clearance is the least distance from the aircraft at pos to a vehicle at
// v: its fuselage and its wing tips to the vehicle's length.
func (b *backstopBed) clearance(pos airport.LatLon, hdg float64, v traffic.GroundPose) float64 {
	nose := traffic.NoseGear(pos, hdg, b.prof)
	tail := b.prof.TailMeters
	if tail <= 0 {
		tail = 20.5
	}
	half := b.prof.SpanMeters / 2
	body := [][2]airport.LatLon{
		{backstopAhead(nose, hdg, 0.35*b.prof.WheelbaseMeters), backstopAhead(pos, hdg+180, tail)},
		{backstopAhead(pos, hdg+90, half), backstopAhead(pos, hdg-90, half)},
	}
	vp := traffic.RescueBodyProfile()
	near := math.Inf(1)
	front := vp.WheelbaseMeters * 1.35
	for d := -vp.TailMeters; d < front+1; d++ {
		q := backstopAhead(v.Position, v.Heading, math.Min(d, front))
		for _, s := range body {
			near = math.Min(near, segDist(q, s[0], s[1]))
		}
	}
	return near
}

// backstopStop is what the arrival did before what stood in its way.
type backstopStop struct {
	stopped bool
	state   traffic.ArrivalState // as it came to a stop
	nearest float64              // to what stood, until it was gone
	hardest float64              // m/s² over half a second while it stood
	on      bool                 // it went on once that was gone
}

// backstopSeen is the arrival as placed this frame.
type backstopSeen struct {
	at    airport.LatLon
	hdg   float64
	speed float64
	down  bool
	still time.Duration
}

// fly drives the arrival, and crew, frame by frame from the approach
// until it has gone on 15 m after what stood was gone: in gives what
// stands now, each frame. Every placement on the ground is checked: no
// dead stop (backstopFrameSpeed a frame) while anything stands, nor a
// jump.
func (b *backstopBed) fly(in func(backstopSeen) []traffic.GroundPose) backstopStop {
	b.t.Helper()
	req := traffic.DefaultArrivalRequestBase
	b.ctl.Handle(assignedObject(req, backstopObject))
	ground := make([]float64, 9)
	ground[0], ground[1] = 1200, 12
	// The controller's own injector: its first object's ground request.
	height := backstopData(traffic.DefaultInjectRequestBase+1, backstopObject, ground)
	const monitor = 2
	_, end, _ := b.g.Layout.RunwayEnd("09")
	res := backstopStop{nearest: math.Inf(1)}
	var stood []traffic.GroundPose
	var steps []float64
	var last, goneAt airport.LatLon
	var lastStep float64
	var still time.Duration
	var stillState traffic.ArrivalState
	haveStep, down := false, false
	for k := 0; k < 40*60*30; k++ {
		b.now = b.now.Add(backstopFrame)
		if k == 0 {
			if _, err := b.injector.Handle(height); err != nil {
				b.t.Fatal(err)
			}
		}
		mon := make([]float64, 16)
		mon[0], mon[1] = end.Threshold.Lat, end.Threshold.Lon
		b.ctl.Handle(backstopData(req+monitor, backstopObject, mon))
		at, hdg := airport.LatLon{Lat: b.client.placed.Latitude, Lon: b.client.placed.Longitude}, b.client.placed.Heading
		st := b.ctl.State()
		down = down || st >= traffic.ArrivalRollout
		if st.Terminal() && st != traffic.ArrivalParked {
			b.t.Fatalf("the arrival ended: %v", st)
		}
		if at.Lat != 0 {
			b.ac.Position, b.ac.Heading = at, hdg
			b.ac.OnGround, b.ac.AGLFt = down, 500
			if down {
				b.ac.AGLFt = 0
			}
		}
		if down && at.Lat != 0 {
			if last.Lat != 0 {
				step := calc.HaversineMeters(last.Lat, last.Lon, at.Lat, at.Lon)
				most := 0.05
				if len(stood) > 0 && len(steps) > 30 {
					most = backstopFrameSpeed * backstopFrame.Seconds()
				}
				if haveStep && math.Abs(step-lastStep) > most {
					b.t.Fatalf("a jump or dead stop at %v: %.3f m in a frame after %.3f m (at most %.3f m apart)", st, step, lastStep, most)
				}
				steps = append(steps, step)
				if n := len(steps); len(stood) > 0 && n > 45 {
					res.hardest = math.Max(res.hardest, math.Abs(steps[n-1]-steps[n-16])/backstopFrame.Seconds()*2)
				}
				lastStep, haveStep = step, true
				b.ac.GroundKts = step / backstopFrame.Seconds() * 3600 / 1852
				if step < 0.005 {
					if still == 0 {
						stillState = st
					}
					still += backstopFrame
				} else {
					still = 0
				}
			}
			last = at
			for _, v := range stood {
				res.nearest = math.Min(res.nearest, b.clearance(at, hdg, v))
			}
			if len(stood) > 0 && still >= 20*time.Second && !res.stopped {
				res.stopped, res.state = true, stillState
			}
		}
		had := len(stood) > 0
		b.step(k%30 == 0)
		seen := backstopSeen{at: at, hdg: hdg, speed: lastStep / backstopFrame.Seconds(), down: down && at.Lat != 0, still: still}
		stood = in(seen)
		if b.until != nil {
			if b.until(seen) {
				return res
			}
			continue
		}
		if had && len(stood) == 0 {
			goneAt = at
		}
		if goneAt.Lat != 0 && calc.HaversineMeters(goneAt.Lat, goneAt.Lon, at.Lat, at.Lon) > 15 {
			res.on = true
			return res
		}
	}
	return res
}

func backstopNear(p airport.LatLon, way []airport.LatLon) float64 {
	near := math.Inf(1)
	for k := 1; k < len(way); k++ {
		near = math.Min(near, segDist(p, way[k-1], way[k]))
	}
	return near
}

// TestRescueBackstopOffVacateWay: points clear of the way lie within reach
// either side of the exit, so both trucks move off it, each to its own,
// never backwards nor with a jump, and none is reported.
func TestRescueBackstopOffVacateWay(t *testing.T) {
	b := newBackstopBed(t)
	way := b.plan()
	keep := b.prof.SpanMeters/2 + traffic.VehicleClearMeters
	for i, tr := range b.crew.trucks {
		if p, _, ok := tr.Track(); !ok || backstopNear(p.Position, way) > keep {
			t.Fatalf("vehicle %d is not on the way off the runway before its plan: nothing checked", i)
		}
	}
	prev := make([]traffic.GroundPose, len(b.crew.trucks))
	for i, tr := range b.crew.trucks {
		prev[i], _, _ = tr.Track()
	}
	b.wait(10*time.Minute, func() {
		for i, tr := range b.crew.trucks {
			p, _, ok := tr.Track()
			if !ok {
				t.Fatalf("vehicle %d gone", i)
			}
			if d := calc.HaversineMeters(prev[i].Position.Lat, prev[i].Position.Lon, p.Position.Lat, p.Position.Lon); d > 0.005 {
				if d > 2 {
					t.Fatalf("vehicle %d jumped %.1f m", i, d)
				}
				dir := calc.BearingDegrees(prev[i].Position.Lat, prev[i].Position.Lon, p.Position.Lat, p.Position.Lon)
				if math.Abs(math.Remainder(dir-p.Heading, 360)) > 90 {
					t.Fatalf("vehicle %d drives backwards (heading %.0f, moving %.0f)", i, p.Heading, dir)
				}
				if turn := math.Abs(math.Remainder(p.Heading-prev[i].Heading, 360)); turn > 15 {
					t.Fatalf("vehicle %d swung round %.0f° in a step", i, turn)
				}
			}
			prev[i] = p
		}
	})
	var at []airport.LatLon
	for i, tr := range b.crew.trucks {
		p, route, _ := tr.Track()
		at = append(at, p.Position)
		if len(route) >= 2 {
			t.Errorf("vehicle %d still on its way after 10 min", i)
		}
		if near := backstopNear(p.Position, way); near <= keep {
			t.Errorf("vehicle %d has not moved off the arrival's way: %.1f m from it (keep %.1f m), reported %v", i, near, keep, b.crew.reported[tr] != 0)
		}
		if b.crew.reported[tr] != 0 {
			t.Errorf("vehicle %d reported standing, though free to move", i)
		}
	}
	if d := calc.HaversineMeters(at[0].Lat, at[0].Lon, at[1].Lat, at[1].Lon); d < traffic.RescueSpacingMeters {
		t.Errorf("both moved to one point, %.1f m apart", d)
	}
}

// TestRescueBackstopArrivalStops: no way to a clear point (no layout to
// drive by), both trucks are reported standing. The arrival comes to a
// stop on its rollout short of them, they stay reported as it slows (the
// crew then following it), and it goes on once they have gone home.
func TestRescueBackstopArrivalStops(t *testing.T) {
	b := newBackstopBed(t)
	for _, tr := range b.crew.trucks {
		tr.Layout = nil
	}
	b.plan()
	// The frame it is planned, not the next second's step.
	b.now = b.now.Add(100 * time.Millisecond)
	b.step(false)
	if n := len(b.standing()); n != 2 {
		t.Fatalf("%d vehicles reported standing in the frame the way was planned, want both", n)
	}
	res := b.fly(func(backstopSeen) []traffic.GroundPose { return b.standing() })
	if !res.stopped || res.state != traffic.ArrivalRollout {
		t.Errorf("stopped %v, at %v: want stopped on its rollout short of them", res.stopped, res.state)
	}
	if res.nearest < backstopClearance-0.1 {
		t.Errorf("came within %.1f m of a reported vehicle, want %.1f m", res.nearest, backstopClearance)
	}
	if !res.on {
		t.Error("did not go on once they were gone")
	}
}

// TestArrivalStopsForStanding: something reported standing on the exit,
// past it, or on the taxiway stops the arrival short of it without a
// jump, whatever phase it meets it in, and lets it go on once forgotten.
// Reported late (the late cases: only as the arrival, heading for it, is
// inside its braking distance for the gap, the room to brake at a rate to
// its clearance still there), it is braked for at a rate, never stood
// before in one frame, and no nearer than that clearance.
func TestArrivalStopsForStanding(t *testing.T) {
	for _, c := range []struct {
		name  string
		south float64 // of the exit's last node, along C
		east  float64 // along A
		from  traffic.ArrivalState
		to    traffic.ArrivalState
		late  bool
	}{
		{"rollout", 0, 0, traffic.ArrivalRollout, traffic.ArrivalRollout, false},
		{"vacate", 45, 0, traffic.ArrivalVacating, traffic.ArrivalTaxiing, false},
		{"taxi", 110, 400, traffic.ArrivalTaxiing, traffic.ArrivalTaxiing, false},
		// Found late it may run on over the edge.
		{"rollout late", 0, 0, traffic.ArrivalRollout, traffic.ArrivalTaxiing, true},
		{"vacate late", 45, 0, traffic.ArrivalRollout, traffic.ArrivalTaxiing, true},
		{"taxi late", 110, 400, traffic.ArrivalTaxiing, traffic.ArrivalTaxiing, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newBackstopBed(t)
			// The crew out of it: the body alone stands.
			b.cc.rescue.by = map[string]*rescueCrew{}
			b.prio = map[string]bool{}
			b.plan()
			p := b.ctl.Plan()
			exit := b.g.Nodes[p.Exit.Path[len(p.Exit.Path)-1]].Position
			body := traffic.GroundPose{Position: backstopAhead(backstopAhead(exit, 180, c.south), 90, c.east)}
			if c.east > 0 {
				body.Heading = 270
			}
			gp := b.cc.world.Ground("XONE")
			gone, shown := false, !c.late
			res := b.fly(func(s backstopSeen) []traffic.GroundPose {
				if !shown {
					if !s.down || s.speed < 1 {
						return nil
					}
					// Heading for it, its nose tip this near its body.
					to := calc.BearingDegrees(s.at.Lat, s.at.Lon, body.Position.Lat, body.Position.Lon)
					if math.Abs(math.Remainder(to-s.hdg, 360)) > 10 {
						return nil
					}
					room := backstopClearance + s.speed*s.speed/(2*1.5) + 0.6*s.speed + 1
					if b.clearance(s.at, s.hdg, body) > room {
						return nil
					}
					shown = true
				}
				if s.still >= 90*time.Second && !gone {
					gone = true
					gp.Forget(backstopBody)
				}
				if gone {
					return nil
				}
				gp.ReportStanding(backstopBody, body.Position, body.Heading, traffic.RescueBodyProfile(), b.now)
				return []traffic.GroundPose{body}
			})
			if !res.stopped || res.state < c.from || res.state > c.to {
				t.Errorf("stopped %v, at %v: want stopped in %v to %v", res.stopped, res.state, c.from, c.to)
			}
			if !shown {
				t.Fatal("never near it: nothing checked")
			}
			if res.nearest < backstopClearance-0.1 {
				t.Errorf("came within %.1f m of it, want %.1f m", res.nearest, backstopClearance)
			}
			if c.late && res.hardest > backstopAccel {
				t.Errorf("found near, its speed changed at %.2f m/s², want at most %.1f", res.hardest, backstopAccel)
			}
			if !res.on {
				t.Error("did not go on once it was gone")
			}
		})
	}
}
