package traffic

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// rollClient records, in order, every placement ("pos") and every yaw rate
// written for the nose wheel.
type rollClient struct {
	eventClient
	log []rollWrite
}

type rollWrite struct {
	yaw   bool // a yaw rate written (else a placement)
	value float64
}

func (c *rollClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	switch def {
	case DefaultInjectDefinitionBase + injDefSteer:
		c.log = append(c.log, rollWrite{yaw: true, value: *(*float64)(p)})
		return nil
	case DefaultInjectDefinitionBase + injDefPosition:
		c.log = append(c.log, rollWrite{})
	}
	return c.eventClient.SetDataOnSimObject(def, obj, f, n, size, p)
}

// yawsSince is the yaw rates written from log entry k on.
func (c *rollClient) yawsSince(k int) []float64 {
	var y []float64
	for _, w := range c.log[k:] {
		if w.yaw {
			y = append(y, w.value)
		}
	}
	return y
}

// The nose wheel is never left steering on the take-off roll (#1446). A
// rolling take-off begins in the line-up turn: the ground placements (Place)
// end with a yaw rate written, and the roll (PlaceAir) writes none. So the
// first roll placement writes 0 once, before it; none after, nor in the
// air; a standing start (0 already written) writes nothing; placed on the
// ground again (a rejected take-off vacating) the steering starts anew, not
// from the line-up turn.
func TestNoseWheelStraightOnTheRoll(t *testing.T) {
	for _, c := range []struct {
		name     string
		standing bool
	}{{"rolling start in the turn", false}, {"standing start", true}} {
		t.Run(c.name, func(t *testing.T) {
			ec := &rollClient{}
			inj := NewInjector(ec)
			w := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
			inj.now = func() time.Time { return w }
			const obj = 42
			inj.Takeover(obj)
			inj.SetModel(obj, "FSLTL_FAIB_A20N_easyJet")
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, obj, 1200, 12.07))
			// The line-up: a right turn of 40 m radius at 12 kt, from north
			// towards east, handed over halfway round.
			const radius, kts = 40.0, 12.0
			v := kts * ktsToMS
			arc := radius * math.Pi / 2
			s := 0.0
			for ; s < arc/2; s += v / 60 {
				a := s / radius
				pose := GroundPose{Position: offset(lkpr, radius*(1-math.Cos(a)), radius*math.Sin(a)), Heading: a * 180 / math.Pi, GroundSpeedKts: kts}
				if err := inj.Place(obj, pose); err != nil {
					t.Fatal(err)
				}
				w = w.Add(time.Second / 60)
			}
			a := s / radius
			at, hdg := offset(lkpr, radius*(1-math.Cos(a)), radius*math.Sin(a)), a*180/math.Pi
			if c.standing {
				for range 60 { // stopped, lined up
					if err := inj.Place(obj, GroundPose{Position: at, Heading: hdg}); err != nil {
						t.Fatal(err)
					}
					w = w.Add(time.Second / 60)
				}
			}
			ys := ec.yawsSince(0)
			last := ys[len(ys)-1]
			switch {
			case c.standing && last != 0:
				t.Fatalf("standing lined up: last yaw %.4f, want 0", last)
			case !c.standing && last == 0:
				t.Fatalf("in the turn: last yaw 0, want the turn's (precondition)")
			}
			// The roll: 20 s accelerating at 3.5 kt/s along the heading it
			// was handed over at, then off the ground.
			roll := len(ec.log)
			gs, d := 0.0, 0.0
			if !c.standing {
				gs = kts
			}
			for k := 0; k < 20*60; k++ {
				gs += 3.5 / 60
				d += gs * ktsToMS / 60
				p := offsetHeading(at, hdg, d)
				air := k >= 19*60
				pose := ApproachPose{Position: p, Heading: hdg, GroundSpeedKts: gs, OnGround: !air, Unpinned: !air}
				if air {
					pose.HeightFt = float64(k-19*60) / 6
				}
				if err := inj.PlaceAir(obj, pose); err != nil {
					t.Fatal(err)
				}
				w = w.Add(time.Second / 60)
			}
			got := ec.yawsSince(roll)
			switch {
			case c.standing && len(got) != 0:
				t.Errorf("standing start: yaw written on the roll %v, want none", got)
			case !c.standing && (len(got) != 1 || got[0] != 0):
				t.Errorf("rolling start: yaw written on the roll %v, want one 0", got)
			case !c.standing && !ec.log[roll].yaw:
				t.Errorf("rolling start: the 0 written after the first roll placement, want before it")
			}
			// Rejected and placed on the ground again, straight: nothing
			// left from the line-up turn.
			back := len(ec.log)
			p := offsetHeading(at, hdg, d)
			for range 30 {
				if err := inj.Place(obj, GroundPose{Position: p, Heading: hdg, GroundSpeedKts: 20}); err != nil {
					t.Fatal(err)
				}
				p = offsetHeading(p, hdg, 20*ktsToMS/60)
				w = w.Add(time.Second / 60)
			}
			for _, y := range ec.yawsSince(back) {
				if y != 0 {
					t.Errorf("on the ground again, straight: yaw %.4f written, want none", y)
					break
				}
			}
		})
	}
}
