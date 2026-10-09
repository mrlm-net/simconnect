package flight

import (
	"bytes"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func testTrack() *Track {
	t := &Track{Title: "Airbus A320neo", Model: "A320", User: true, Note: "LKPR 24"}
	for i := range 3 {
		s := Sample{T: float64(i), Lat: 50.1 + float64(i)*0.001, Lon: 14.26, AltFt: 1200 + float64(i)*10, Heading: 359 + float64(i),
			Pitch: 7.5, Bank: -2, IAS: 150, OnGround: i == 0, GearHandle: true, FlapsIndex: 2, FlapsPct: 40, Lights: LightLanding | LightStrobe,
			EngineCount: 2, Throttle: [Engines]float64{80, 80}, N1: [Engines]float64{85.5, 85.6}}
		s.AP.Master, s.AP.AltitudeSel = i == 2, 5000
		t.Samples = append(t.Samples, s)
	}
	return t
}

// TestTrackRoundTrip: written and read back, plain and gzipped, the same.
func TestTrackRoundTrip(t *testing.T) {
	tr := testTrack()
	var b bytes.Buffer
	if err := tr.Write(&b); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTrack(&b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != TrackVersion || got.Title != tr.Title || got.Model != "A320" || !got.User || len(got.Samples) != 3 {
		t.Fatalf("header %+v, %d samples", got, len(got.Samples))
	}
	for i := range tr.Samples {
		if got.Samples[i] != tr.Samples[i] {
			t.Errorf("sample %d:\n got %+v\nwant %+v", i, got.Samples[i], tr.Samples[i])
		}
	}
	path := filepath.Join(t.TempDir(), "flight.jsonl.gz")
	if err := tr.WriteFile(path); err != nil {
		t.Fatal(err)
	}
	gz, err := ReadTrackFile(path)
	if err != nil || len(gz.Samples) != 3 || gz.Samples[2] != tr.Samples[2] {
		t.Fatalf("gzipped: %v %+v", err, gz)
	}
}

// TestTrackFields: a field the reader does not know is skipped, one the
// file lacks stays zero; a newer version is refused.
func TestTrackFields(t *testing.T) {
	in := `{"version":1,"started":"2026-10-09T12:00:00Z","fields":["t","future","lat","ias"]}
[1,42,50.5,140]
`
	tr, err := ReadTrack(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if s := tr.Samples[0]; s.T != 1 || s.Lat != 50.5 || s.IAS != 140 || s.Lon != 0 {
		t.Errorf("sample %+v", s)
	}
	if _, err := ReadTrack(strings.NewReader(`{"version":99,"fields":[]}`)); err == nil {
		t.Error("a newer version read")
	}
}

// TestTrackAt: between samples numbers are linear, the heading the short
// way through north, switches as the earlier sample; outside, the ends.
func TestTrackAt(t *testing.T) {
	tr := testTrack()
	s, ok := tr.At(0.5)
	if !ok || math.Abs(s.Lat-50.1005) > 1e-9 || math.Abs(s.AltFt-1205) > 1e-9 || !s.OnGround {
		t.Errorf("at 0.5: %+v", s)
	}
	if math.Abs(s.Heading-359.5) > 1e-9 {
		t.Errorf("heading %v, want 359.5", s.Heading)
	}
	s, _ = tr.At(1.5)
	if math.Abs(s.Heading-0.5) > 1e-9 || s.AP.Master {
		t.Errorf("at 1.5 heading %v (want 0.5), AP %v (the earlier: off)", s.Heading, s.AP.Master)
	}
	if s, _ := tr.At(-5); s != tr.Samples[0] {
		t.Error("before the first: not the first")
	}
	if s, _ := tr.At(99); s != tr.Samples[2] {
		t.Error("after the last: not the last")
	}
	if d := tr.Duration(); d != 2 {
		t.Errorf("duration %v", d)
	}
	if _, ok := (&Track{}).At(0); ok {
		t.Error("an empty track answered")
	}
}
