package airport

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestParkingLabelText: a free-text label replaces the composed name; none
// keeps it (#1556).
func TestParkingLabelText(t *testing.T) {
	p := Parking{Name: types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_C, Number: 22}
	if got := p.Label(); got != "C22" {
		t.Fatalf("no label: %q, want C22", got)
	}
	for _, c := range []struct{ text, want string }{
		{"H17-1", "H17-1"}, {"1B A380", "1B A380"}, {" FDX6 ", "FDX6"}, {"C", "C"}, {"   ", "C22"},
	} {
		p.LabelText = c.text
		if got := p.Label(); got != c.want {
			t.Errorf("label %q: Label %q, want %q", c.text, got, c.want)
		}
		if got := p.ComposedLabel(); got != "C22" {
			t.Errorf("label %q: ComposedLabel %q, want C22", c.text, got)
		}
	}
}

// TestParkingLabelJSON: a layout without labels writes exactly what it
// wrote before; a label round-trips through Layout and RawAirport JSON.
func TestParkingLabelJSON(t *testing.T) {
	l := loadLKPR(t)
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"label"`)) {
		t.Fatal(`a layout without labels writes "label"`)
	}
	var back Layout
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(&back)
	if !bytes.Equal(b, again) {
		t.Fatal("a layout without labels does not round-trip byte for byte")
	}

	back.Parking[0].LabelText = "H17-1"
	b, _ = json.Marshal(&back)
	if !bytes.Contains(b, []byte(`"label":"H17-1"`)) {
		t.Fatalf(`no "label":"H17-1" in the layout`)
	}
	var lab Layout
	if err := json.Unmarshal(b, &lab); err != nil {
		t.Fatal(err)
	}
	if got := lab.Parking[0].Label(); got != "H17-1" {
		t.Fatalf("read back: %q", got)
	}

	raw := RawAirport{ICAO: "TEST", Latitude: 50, Longitude: 14,
		Parking: []RawParking{{Name: int32(types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_C), Number: 22}, {Number: 7, Label: " 1B A380 "}}}
	rb, _ := json.Marshal(raw.Parking[0])
	if strings.Contains(string(rb), "label") {
		t.Fatalf("raw parking without a label: %s", rb)
	}
	rb, _ = json.Marshal(raw)
	var rawBack RawAirport
	if err := json.Unmarshal(rb, &rawBack); err != nil {
		t.Fatal(err)
	}
	built, err := BuildLayout(rawBack)
	if err != nil {
		t.Fatal(err)
	}
	if got := built.Parking[0].Label(); got != "C22" {
		t.Errorf("built, no label: %q", got)
	}
	if got := built.Parking[1]; got.Label() != "1B A380" || got.LabelText != "1B A380" {
		t.Errorf("built, label: %q (%q)", got.Label(), got.LabelText)
	}
}

// TestParkingByLabelText: a labelled stand is found by its label, and by
// its composed name only when no label matches.
func TestParkingByLabelText(t *testing.T) {
	gc := types.SIMCONNECT_FACILITY_TAXI_PARKING_NAME_GATE_C
	l := &Layout{Parking: []Parking{
		{Index: 0, Name: gc, Number: 22, LabelText: "H17-1"},
		{Index: 1, Name: gc, Number: 23},
		{Index: 2, Name: gc, Number: 24, LabelText: "C23"},
	}}
	for _, c := range []struct {
		q    string
		want []int
	}{
		{"h17-1", []int{0}}, {"C22", []int{0}}, {"C23", []int{1, 2}}, {"C24", []int{2}}, {"X1", nil},
	} {
		var got []int
		for _, p := range l.ParkingByLabel(c.q) {
			got = append(got, p.Index)
		}
		if len(got) != len(c.want) {
			t.Errorf("%q: %v, want %v", c.q, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: %v, want %v", c.q, got, c.want)
			}
		}
	}
	if i, err := l.ParkingIndex("H17-1"); err != nil || i != 0 {
		t.Errorf("ParkingIndex H17-1: %d %v", i, err)
	}
}

// TestRawParkingWireSize: the record cast from the sim's data is the 32
// bytes of TAXI_PARKING, whatever RawParking carries besides.
func TestRawParkingWireSize(t *testing.T) {
	if n := unsafe.Sizeof(rawParkingWire{}); n != 32 {
		t.Fatalf("rawParkingWire is %d bytes, want 32", n)
	}
}
