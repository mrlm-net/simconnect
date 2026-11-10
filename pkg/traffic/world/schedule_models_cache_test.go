package world

import (
	"fmt"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Whether there is a model is looked up once per airline and type for a
// titles list, not for every flight of every extension (live LKPR
// 2026-11-10: 7,072 titles ranked for each of 30 flights under the
// manager's lock, our traffic held still 400 ms); a new list looks again.
func TestScheduleModelsLookedUpOncePerType(t *testing.T) {
	s := &scheduler{cc: &controlCenter{log: &trafficLog{}}, airlines: map[string]traffic.Airline{}}
	list := []string{"FSLTL_FAIB_A320_CSA-Czech Airlines"}
	for k := 0; k < 7000; k++ {
		list = append(list, fmt.Sprintf("ZZ Livery %d B738", k))
	}
	s.models = list
	var fs []traffic.Flight
	for k := 0; k < 30; k++ {
		fs = append(fs, traffic.Flight{Callsign: fmt.Sprintf("CSA%d", 100+k), Airline: "CSA", Type: "A320"},
			traffic.Flight{Callsign: fmt.Sprintf("OK%03d", k), Type: "PC12"})
	}
	got := s.withModels(append([]traffic.Flight(nil), fs...))
	if len(got) != 30 {
		t.Fatalf("kept %d, want the 30 A320s", len(got))
	}
	if len(s.haveModel) != 2 {
		t.Fatalf("%d looked up, want 2 (CSA A320, PC12): %v", len(s.haveModel), s.haveModel)
	}
	if again := s.withModels(append([]traffic.Flight(nil), fs...)); len(again) != 30 {
		t.Fatalf("again kept %d", len(again))
	}
	// A PC12 installed: a new list, looked up again.
	s.models = append([]string{"Asobo PC12"}, list...)
	if got := s.withModels(append([]traffic.Flight(nil), fs...)); len(got) != 60 {
		t.Fatalf("with a PC12 kept %d, want all 60", len(got))
	}
}
