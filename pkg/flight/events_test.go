package flight

import "testing"

// A whole flight, sample by sample: start, push (backwards), taxi, roll,
// rotate, lift-off, gear up, touchdown, vacate, park.
func TestEvents(t *testing.T) {
	var ss []Sample
	lat, lon := 50.1, 14.26
	at := func(dt float64, f func(s *Sample)) {
		s := Sample{T: 100, Lat: lat, Lon: lon, Heading: 90, OnGround: true, GearHandle: true, EngineCount: 2, ParkingBrake: true}
		if n := len(ss); n > 0 {
			s = ss[n-1]
			s.T += dt
		}
		f(&s)
		lat, lon = s.Lat, s.Lon
		ss = append(ss, s)
	}
	at(0, func(s *Sample) {})                                                     // 0 parked, dark
	at(10, func(s *Sample) { s.N1[0], s.N1[1] = 20, 20; s.ParkingBrake = false }) // 10 engines
	at(10, func(s *Sample) { s.Lon -= 0.00003; s.GS = 2 })                        // 20 backwards (heading 90, moving west)
	at(10, func(s *Sample) { s.Lon += 0.0002; s.GS = 12 })                        // 30 taxi
	at(60, func(s *Sample) { s.IAS, s.GS = 45, 45; s.Lon += 0.003 })              // 90 roll
	at(20, func(s *Sample) { s.IAS, s.Pitch = 140, 6; s.Lon += 0.01 })            // 110 rotate
	at(2, func(s *Sample) { s.OnGround = false; s.IAS = 150 })                    // 112 lift-off
	at(5, func(s *Sample) { s.GearHandle = false })                               // 117 gear up
	at(600, func(s *Sample) { s.GearHandle = true; s.VS = -180; s.IAS = 135 })    // 717 final
	at(5, func(s *Sample) { s.OnGround = true; s.Pitch = 3 })                     // 722 touchdown
	at(30, func(s *Sample) { s.IAS, s.GS = 25, 25 })                              // 752 vacate
	at(200, func(s *Sample) { s.GS, s.IAS, s.N1[0], s.N1[1], s.ParkingBrake = 0, 0, 0, 0, true })
	ev := Events(&Track{Samples: ss})
	want := []struct {
		kind string
		t    float64
	}{
		{EventEnginesStart, 10}, {EventPushback, 20}, {EventTaxi, 30}, {EventTakeoffRoll, 90}, {EventRotate, 110},
		{EventLiftoff, 112}, {EventGearUp, 117}, {EventTouchdown, 722}, {EventVacate, 752}, {EventPark, 952},
	}
	if len(ev) != len(want) {
		t.Fatalf("events %+v", ev)
	}
	for i, w := range want {
		if ev[i].Kind != w.kind || ev[i].T != w.t {
			t.Errorf("event %d: %s at %.1f, want %s at %.1f", i, ev[i].Kind, ev[i].T, w.kind, w.t)
		}
	}
	if Events(&Track{}) != nil {
		t.Error("an empty track has events")
	}
}
