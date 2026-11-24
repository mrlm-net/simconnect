package traffic

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// bytesClient keeps every data write as sent.
type bytesClient struct {
	eventClient
	sum   [32]byte
	n     int
	write func(def uint32, b []byte)
}

func (c *bytesClient) SetDataOnSimObject(def, obj uint32, flag types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if n == 0 {
		n = 1
	}
	c.n++
	if c.write != nil {
		c.write(def, unsafe.Slice((*byte)(p), n*size))
	}
	return c.eventClient.SetDataOnSimObject(def, obj, flag, n, size, p)
}

// normalJoinWrites flies a LOMKI arrival to LKPR 24, MSFS AI at the join
// 300 ft above the final (as MSFS AI brings them in), taken over and flown
// to the runway: every write's bytes hashed, and how many.
func normalJoinWrites(t *testing.T) (string, int) {
	t.Helper()
	j := joinFlight(t, 300)
	return j.sum, j.n
}

// joinWrite is an altitude written, when.
type joinWrite struct {
	at time.Time
	ft float64
}

// joinRun is a flight joinFlight flew: its controller, its writes' hash
// and number, its altitudes written and MSFS AI's at the join.
type joinRun struct {
	ctl     *ArrivalController
	sum     string
	n       int
	alts    []joinWrite
	joinAlt float64
}

// joinFlight is normalJoinWrites with MSFS AI aboveFt above the final at
// the join.
func joinFlight(t *testing.T, aboveFt float64) joinRun {
	t.Helper()
	return joinFlightAt(t, aboveFt, 0, false)
}

// joinFlightAt is joinFlight with its join joinNM out (0: the procedure's);
// lands: it lands from an approach not stable (SetLandsUnstable: the
// user's own with held gates).
func joinFlightAt(t *testing.T, aboveFt, joinNM float64, lands bool) joinRun {
	t.Helper()
	SetRandomSeed(1681)
	t.Cleanup(func() { SetRandomSeed(0) })
	g := lkprGraph(t)
	route, err := lkprProcedures(t).Arrival("24", "LOMKI")
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	ec := &bytesClient{}
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	var alts []joinWrite
	ec.write = func(def uint32, b []byte) {
		var d [4]byte
		binary.LittleEndian.PutUint32(d[:], def)
		h.Write(d[:])
		h.Write(b)
		if def == DefaultInjectDefinitionBase+injDefPosition && len(b) == int(unsafe.Sizeof(types.SIMCONNECT_DATA_INITPOSITION{})) {
			alts = append(alts, joinWrite{now, (*types.SIMCONNECT_DATA_INITPOSITION)(unsafe.Pointer(&b[0])).Altitude})
		}
	}
	inj := NewInjector(ec)
	inj.now = func() time.Time { return now }
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj), ArrivalWithSeed(1681))
	ctl.now = func() time.Time { return now }
	c17, _ := g.Layout.ParkingIndex("C17")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c17, Model: "B738", Tail: "TVS354",
		InjectApproach: true, RollThroughChance: -1, Procedure: route}); err != nil {
		t.Fatal(err)
	}
	ctl.SetLandsUnstable(lands)
	// Its track miles given already (extendForHeight): one too high joins
	// as it is, judged at the gate.
	ctl.heightExtends = maxHeightExtends
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := ctl.Plan()
	ctl.mu.Lock()
	if joinNM > 0 {
		ctl.proc.JoinMeters = joinNM * 1852
		ctl.proc.Join = offsetHeading(p.End.LandingThreshold(), p.End.Heading+180, ctl.proc.JoinMeters)
	}
	joinAt, joinM := ctl.proc.Join, ctl.proc.JoinMeters
	pathFt := NewApproachMover(p.End.LandingThreshold(), p.End.Heading, joinM, ctl.approachProfile()).Pose().HeightFt
	ctl.mu.Unlock()
	runwayFt := p.Runway.Altitude / 0.3048
	ctl.Handle(flownMsg(mon, 77, joinAt, runwayFt+pathFt+aboveFt, pathFt+aboveFt, p.End.Heading, 160))
	if ctl.approach == nil {
		t.Fatal("not taken over at the join")
	}
	if got := ctl.blend.dFt; math.Abs(got-aboveFt) > 50 {
		t.Fatalf("taken over %.0f ft above the final, want %.0f", got, aboveFt)
	}
	from := len(alts)
	for i := 0; i < 60*600 && ctl.State() == ArrivalApproaching; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	}
	return joinRun{ctl: ctl, sum: fmt.Sprintf("%x", h.Sum(nil)), n: ec.n, alts: alts[from:], joinAlt: runwayFt + pathFt + aboveFt}
}

// TestNormalJoinWrites: a normal arrival's takeover, its writes' hash
// logged to compare between trees (#1681: the same before and after the
// rate-limited blend).
func TestNormalJoinWrites(t *testing.T) {
	sum, n := normalJoinWrites(t)
	if n < 600 {
		t.Fatalf("%d writes", n)
	}
	t.Logf("normal join: %d writes, sha256 %s", n, sum)
}
