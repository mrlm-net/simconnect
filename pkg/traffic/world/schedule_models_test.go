package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A type the simulator has no model of is left out of the schedule, not
// planned and held in the landing flow until its spawn fails (offline LKPR:
// a PC12 and a BE58, 16 minutes without a landing); before the models are
// known, all are kept.
func TestScheduleLeavesOutTypesWithNoModel(t *testing.T) {
	s := &scheduler{cc: &controlCenter{log: &trafficLog{}}, airlines: map[string]traffic.Airline{}}
	fs := []traffic.Flight{{Callsign: "CSA123", Airline: "CSA", Type: "A320"}, {Callsign: "OKCMZ", Type: "PC12"}}
	if got := s.withModels(append([]traffic.Flight(nil), fs...)); len(got) != 2 {
		t.Fatalf("models unknown: %d flights, want both kept", len(got))
	}
	s.models = []string{"FSLTL_FAIB_A320_CSA-Czech Airlines"}
	got := s.withModels(append([]traffic.Flight(nil), fs...))
	if len(got) != 1 || got[0].Callsign != "CSA123" {
		t.Fatalf("kept %v, want CSA123 only", got)
	}
}
