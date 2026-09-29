//go:build windows
// +build windows

package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

func TestDistanceToGo(t *testing.T) {
	thr := airport.LatLon{Lat: 50, Lon: 14}
	east := func(nm float64) airport.LatLon { return offsetHeading(thr, 270, nm*1852) } // west of the threshold
	route := []airport.LatLon{east(30), east(20), east(10)}
	for _, c := range []struct {
		at, want float64
	}{{25, 25}, {15, 15}, {5, 5}, {40, 40}} {
		if d := DistanceToGo(east(c.at), route, thr); math.Abs(d-c.want) > 0.2 {
			t.Errorf("at %.0f NM: %.1f NM to go", c.at, d)
		}
	}
	// A dog-leg: off the straight line, the track is longer.
	route = []airport.LatLon{offsetHeading(thr, 300, 20*1852), offsetHeading(thr, 240, 10*1852)}
	if d := DistanceToGo(offsetHeading(thr, 300, 25*1852), route, thr); d < 5+20 {
		t.Errorf("dog-leg: %.1f NM", d)
	}
}

func arr(cs string, typ string, nm float64) ApproachAircraft {
	return ApproachAircraft{Callsign: cs, Wake: WakeFor(typ), DistanceToGoNM: nm, GroundKts: 250, FinalKts: 140}
}

func TestSequencerSpacing(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{})
	now := time.Now()
	// A heavy and two mediums arriving almost together.
	seq := s.Update(now, []ApproachAircraft{arr("QTR1", "B77W", 40), arr("CSA2", "A320", 40.5), arr("DLH3", "A320", 41)})
	if len(seq) != 3 || seq[0].Callsign != "QTR1" || seq[1].Callsign != "CSA2" || seq[2].Callsign != "DLH3" {
		t.Fatalf("order %+v", seq)
	}
	if seq[1].SpacingNM != 5 || seq[2].SpacingNM != 3 {
		t.Errorf("spacing %.0f, %.0f NM; want 5 behind the heavy, 3", seq[1].SpacingNM, seq[2].SpacingNM)
	}
	for i := 1; i < len(seq); i++ {
		gap := seq[i].Landing.Sub(seq[i-1].Landing)
		if min := SeparationTime(seq[i].SpacingNM, 140); gap < min-time.Second {
			t.Errorf("%s lands %v after %s, minimum %v", seq[i].Callsign, gap, seq[i-1].Callsign, min)
		}
	}
	// The ones behind absorb the delay; the first none.
	if seq[0].Delay != 0 || seq[1].Delay <= 0 || seq[2].Delay <= seq[1].Delay {
		t.Errorf("delays %v %v %v", seq[0].Delay, seq[1].Delay, seq[2].Delay)
	}
}

func TestSequencerFixedAndFreeze(t *testing.T) {
	s := NewApproachSequencer("24", SequencerOptions{})
	now := time.Now()
	other := arr("AI1", "A320", 20)
	other.Fixed = true             // other traffic: never delayed
	near := arr("CSA1", "A320", 6) // established: frozen
	seq := s.Update(now, []ApproachAircraft{arr("OUR1", "A320", 19.5), other, near})
	var ai, our SequenceEntry
	for _, e := range seq {
		switch e.Callsign {
		case "AI1":
			ai = e
		case "OUR1":
			our = e
		case "CSA1":
			if !e.Fixed || e.Delay != 0 || e.Number != 1 {
				t.Errorf("established aircraft moved: %+v", e)
			}
		}
	}
	if ai.Delay != 0 || our.Number <= ai.Number || our.Delay <= 0 {
		t.Fatalf("our arrival should fall in behind the fixed one: ai %+v our %+v", ai, our)
	}
}

func TestSequencerChanges(t *testing.T) {
	var got []SequenceChange
	s := NewApproachSequencer("24", SequencerOptions{OnChange: func(c SequenceChange) { got = append(got, c) }})
	now := time.Now()
	s.Update(now, []ApproachAircraft{arr("A1", "A320", 30), arr("B2", "A320", 40)})
	if len(got) != 2 {
		t.Fatalf("%d changes for two new arrivals", len(got))
	}
	got = nil
	s.Update(now.Add(time.Second), []ApproachAircraft{arr("A1", "A320", 29.9), arr("B2", "A320", 39.9)})
	if len(got) != 0 {
		t.Fatalf("%d changes when nothing changed: %+v", len(got), got)
	}
	// B2 now first; A1 gone... then lands.
	s.Update(now.Add(2*time.Second), []ApproachAircraft{arr("A1", "A320", 50), arr("B2", "A320", 20)})
	if len(got) != 2 || got[0].Entry.Number == got[0].Previous {
		t.Fatalf("swap: %+v", got)
	}
	got = nil
	s.Update(now.Add(3*time.Second), []ApproachAircraft{arr("A1", "A320", 49)})
	if len(got) != 2 {
		t.Fatalf("B2 gone and A1 now #1: %+v", got)
	}
	gone := false
	for _, c := range got {
		gone = gone || c.Gone && c.Entry.Callsign == "B2"
	}
	if !gone {
		t.Error("B2 not reported gone")
	}
}

// TestSequencerKeepsOrder: arrivals whose predictions are close keep the
// order they were given; a clear overtake still changes it.
func TestSequencerKeepsOrder(t *testing.T) {
	s := NewApproachSequencer("06", SequencerOptions{})
	now := time.Now()
	first := s.Update(now, []ApproachAircraft{arr("A1", "A320", 40), arr("B2", "A320", 40.2)})
	if first[0].Callsign != "A1" {
		t.Fatalf("order %s %s", first[0].Callsign, first[1].Callsign)
	}
	// B2 is now predicted a few seconds earlier: the order stays.
	seq := s.Update(now.Add(time.Second), []ApproachAircraft{arr("A1", "A320", 40.1), arr("B2", "A320", 39.9)})
	if seq[0].Callsign != "A1" || seq[1].Landing.Sub(seq[0].Landing) < SeparationTime(3, 140)-time.Second {
		t.Fatalf("close call swapped or spacing lost: %s first, gap %v", seq[0].Callsign, seq[1].Landing.Sub(seq[0].Landing))
	}
	// B2 clearly ahead now (10 NM closer): it goes first.
	seq = s.Update(now.Add(2*time.Second), []ApproachAircraft{arr("A1", "A320", 40), arr("B2", "A320", 30)})
	if seq[0].Callsign != "B2" {
		t.Fatalf("a clear overtake did not change the order")
	}
}

// TestSequencerNewcomersQueueBehind: arrivals that slowed down and flew a
// longer path to lose their delay keep their places; a newcomer behind
// them is sequenced behind them, even when its prediction is now earlier.
func TestSequencerNewcomersQueueBehind(t *testing.T) {
	s := NewApproachSequencer("06", SequencerOptions{})
	now := time.Now()
	s.Update(now, []ApproachAircraft{arr("A1", "A320", 60), arr("B2", "A320", 60.3), arr("C3", "A320", 60.6)})
	// A minute later B2 and C3 fly slower and longer (they lost delay);
	// D4 appears at the entry, fast and on the direct route.
	later := now.Add(time.Minute)
	b, c := arr("B2", "A320", 62), arr("C3", "A320", 66)
	b.GroundKts, c.GroundKts = 210, 210
	seq := s.Update(later, []ApproachAircraft{arr("A1", "A320", 56), b, c, arr("D4", "A320", 60)})
	order := ""
	for _, e := range seq {
		order += e.Callsign + " "
	}
	if order != "A1 B2 C3 D4 " {
		t.Fatalf("order %s, want the newcomer last", order)
	}
}

// TestSequencerMinSpacing: a unit's minimum spacing wins over a smaller
// wake minimum, never over a larger one.
func TestSequencerMinSpacing(t *testing.T) {
	s := NewApproachSequencer("06", SequencerOptions{MinSpacingNM: 5})
	seq := s.Update(time.Now(), []ApproachAircraft{arr("QTR1", "A388", 40), arr("A2", "A320", 40.5), arr("B3", "A320", 41)})
	if seq[1].SpacingNM != 7 || seq[2].SpacingNM != 5 { // J→M 7 NM stays; M→M 3 → 5
		t.Fatalf("spacing %.0f and %.0f NM", seq[1].SpacingNM, seq[2].SpacingNM)
	}
	if gap := seq[2].Landing.Sub(seq[1].Landing); gap < SeparationTime(5, 140)-time.Second {
		t.Errorf("5 NM kept as %v", gap)
	}
}
