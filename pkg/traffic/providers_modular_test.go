package traffic

import "testing"

// An MSFS 2024 livery spawns as "<preset title> :: <livery name>" and is
// its preset's model: a flyable Fenix, not a packed stock model, so not our
// traffic while an AI model is there (live, LKPR: MyCrew OK-MYV on the
// Fenix, jumping, its GPU on, above the ground).
func TestModularLiveryIsItsPresets(t *testing.T) {
	SetModelSources(map[string]ModelSource{
		"fenixa320 cfm sl":      {Package: "fnx-aircraft-320", Creator: "FenixSim", Flyable: true},
		"fsltl a320 mcr mycrew": {Package: "mycrew-fsltl-traffic", Creator: "MyCrew"},
	})
	defer SetModelSources(nil)
	fenix := "FenixA320 CFM SL :: MyCrew Heritage OK-MYV"
	if p := ModelProvider(fenix); p != "FenixSim" {
		t.Errorf("provider %q, want FenixSim (its preset's package)", p)
	}
	if TitleOn(fenix) {
		t.Error("the Fenix livery used as traffic")
	}
	if got, _ := PreferProviders([]string{fenix, "FSLTL A320 MCR MyCrew"}); len(got) != 1 || got[0] != "FSLTL A320 MCR MyCrew" {
		t.Errorf("chosen %v, want the FSLTL model only", got)
	}
}
