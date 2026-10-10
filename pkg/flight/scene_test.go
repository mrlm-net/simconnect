package flight

import (
	"bytes"
	"testing"
	"time"
)

func sceneTrack(t0 float64, n int, lat float64) *Track {
	tr := &Track{Version: TrackVersion}
	for i := range n {
		tr.Samples = append(tr.Samples, Sample{T: t0 + float64(i), Lat: lat + float64(i)*0.001, Lon: 14, AltFt: 1000, GS: 120})
	}
	return tr
}

// TestSceneRoundTrip: a scene written and read back keeps its aircraft,
// their identity and samples; At has only those there at the time.
func TestSceneRoundTrip(t *testing.T) {
	s := &Scene{Version: SceneVersion, Started: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), Note: "LKPR", Aircraft: []*SceneAircraft{
		{Key: "CSA1", Callsign: "CSA1", Title: "FSLTL A320 CSA", Type: "A320", Source: SourceOurs, Track: sceneTrack(100, 10, 50)},
		{Key: "OKABC", Callsign: "OKABC", Title: "Asobo C172", Type: "C172", Source: SourceSimAI, Track: sceneTrack(105, 20, 49)},
	}}
	var buf bytes.Buffer
	if err := s.Write(&buf); err != nil {
		t.Fatal(err)
	}
	got, err := ReadScene(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Aircraft) != 2 || got.Note != "LKPR" || !got.Started.Equal(s.Started) {
		t.Fatalf("read %+v", got)
	}
	a := got.Find("OKABC")
	if a == nil || a.Source != SourceSimAI || a.Type != "C172" || len(a.Track.Samples) != 20 || a.First() != 105 || a.Last() != 124 {
		t.Fatalf("OKABC %+v", a)
	}
	if first, last := got.Span(); first != 100 || last != 124 {
		t.Errorf("span %v…%v", first, last)
	}
	if at := got.At(102); len(at) != 1 || at[0].Aircraft.Key != "CSA1" {
		t.Errorf("at 102: %d aircraft", len(at))
	}
	if at := got.At(107.5); len(at) != 2 {
		t.Errorf("at 107.5: %d aircraft", len(at))
	}
	if at := got.At(120); len(at) != 1 || at[0].Aircraft.Key != "OKABC" {
		t.Errorf("at 120: %v", at)
	}
}

// TestSceneRecorder: aircraft near the player are recorded as they come,
// each stretch kept when it leaves, one back again under a fresh key; at
// most Max at once.
func TestSceneRecorder(t *testing.T) {
	c := &fakeClient{}
	rec := NewRecorder(c, 0)
	s := NewSceneRecorder(rec, SceneOptions{Max: 2})
	frame := func(obj uint32, simT float64) {
		vals := make([]float64, len(recVars))
		for i, v := range recVars {
			if v.name == "SIMULATION TIME" {
				vals[i] = simT
			}
		}
		rec.mu.Lock()
		req := rec.byObj[obj]
		rec.mu.Unlock()
		msg, buf := frameMessage(rec.base, req, vals)
		rec.Handle(msg)
		_ = buf
	}
	a := SceneObject{ObjectID: 11, Identity: SceneAircraft{Callsign: "CSA1", Type: "A320", Source: SourceOurs}}
	b := SceneObject{ObjectID: 12, Identity: SceneAircraft{Callsign: "OKABC", Source: SourceSimAI}}
	cc := SceneObject{ObjectID: 13, Identity: SceneAircraft{Callsign: "DLH2"}}
	if err := s.Update([]SceneObject{a, b, cc}); err != nil {
		t.Fatal(err)
	}
	if s.Recording() != 2 {
		t.Fatalf("recording %d, want Max 2", s.Recording())
	}
	frame(11, 10)
	frame(12, 10)
	s.Update([]SceneObject{b, cc}) // CSA1 left; DLH2 now has room
	frame(12, 11)
	frame(13, 11)
	s.Update([]SceneObject{a, b, cc}) // CSA1 back: full, waits
	s.Update([]SceneObject{a, b})     // DLH2 left, CSA1 again
	frame(11, 20)
	sc := s.Stop()
	keys := map[string]int{}
	for _, x := range sc.Aircraft {
		keys[x.Key] = len(x.Track.Samples)
	}
	if keys["CSA1"] != 1 || keys["OKABC"] != 2 || keys["DLH2"] != 1 || keys["CSA1#2"] != 1 {
		t.Errorf("scene %v", keys)
	}
	if got := sc.Find("OKABC"); got == nil || got.Source != SourceSimAI {
		t.Errorf("identity lost: %+v", got)
	}
}
