package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Stairs at LKPR's remote stands drive in to the door, wait and drive
// home without a jump: no step farther than a frame's drive, no turn of
// more than 3° in a frame (they swung 180° in a frame to leave, and up to
// 12° squared to the door).
func TestSimObjectStairsNoJump(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	prof := ProfileFor("A320").Motion
	n := 0
	for _, stand := range l.Parking {
		if stand.IsGate() || stand.Size() == airport.StandNone || stand.Radius*2 < prof.SpanMeters {
			continue
		}
		n++
		label := stand.Label()
		c := &tugClient{}
		inj := NewInjector(c)
		s := NewSimObjectStairs(c, inj, "FSDT_Staircase", 9003, prof)
		s.Layout = l
		pose := GroundPose{Position: StandPoint(stand, 0), Heading: stand.Heading}
		if err := s.Attach(pose); err != nil {
			t.Fatal(err)
		}
		s.Handle(assignedMsg(9003, 57))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 57, 1200, 3))
		var last *GroundPose
		jumped := false
		step := func(leave bool) {
			if err := s.Update(pose, leave, 1.0/60); err != nil {
				t.Fatal(err)
			}
			p := s.pose
			if last != nil && !jumped {
				d, turn := localDist(last.Position, p.Position), math.Abs(math.Remainder(p.Heading-last.Heading, 360))
				if d > 0.5 || turn > 3 {
					jumped = true
					t.Errorf("%s stairs: jumped %.1f m, %.1f° in a frame", label, d, turn)
				}
			}
			last = &p
		}
		for i := 0; i < 60*900 && !s.Fuelling(); i++ {
			step(false)
		}
		if !s.Fuelling() {
			t.Fatalf("%s stairs: never at the door", label)
		}
		for i := 0; i < 60*900 && !s.Done(); i++ {
			step(true)
		}
		if !s.Done() {
			t.Fatalf("%s stairs: never home", label)
		}
	}
	if n == 0 {
		t.Fatal("no remote stand at LKPR")
	}
}
