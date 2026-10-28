package traffic

import "testing"

// Providers: a title's package names its provider, the patterns a title
// without one, a title the sim lists on no package is MSFS's; by default
// the game's own (MSFS) and the vehicles' (GSX) on, other aircraft (FSLTL)
// and flyable add-ons off; an add-on is passed over unless nothing else is
// there.
func TestModelProviders(t *testing.T) {
	gameDefaults(t)
	SetModelSources(map[string]ModelSource{
		"fsltl a20n dlh lufthansa": {Package: "fsltl-traffic-base", Creator: "FSLTL"},
		"airbus a320neo flybywire": {Package: "flybywire-aircraft-a320-neo", Creator: "FlyByWire Simulations", Flyable: true},
		"fsdt_pushback_03":         {Package: "fsdreamteam-gsx-pro", Creator: "Fsdreamteam"},
	})
	defer SetModelSources(nil)
	for title, want := range map[string]string{
		"FSLTL A20N DLH Lufthansa": "FSLTL", "Airbus A320neo FlyByWire": "FlyByWire Simulations",
		"FSDT_Pushback_03": "GSX", "Airbus A320 Neo Asobo": ProviderStock,
	} {
		if got := ModelProvider(title); got != want {
			t.Errorf("%s: %s, want %s", title, got, want)
		}
	}
	if ProviderOn("FlyByWire Simulations") || ProviderOn("FSLTL") || !ProviderOn(ProviderStock) || !ProviderOn("GSX") || !ProviderFlyable("FlyByWire Simulations") {
		t.Error("defaults: MSFS and GSX on, FSLTL and flyable add-ons off")
	}
	models := []string{"Airbus A320neo FlyByWire", "FSLTL A20N DLH Lufthansa"}
	if got := ModelsFor(models, "DLH", "", "A20N", 0); len(got) != 1 || got[0] != models[1] {
		t.Errorf("with the add-on off: %v", got)
	}
	var fell []string
	OnProviderFallback = func(titles []string) { fell = titles }
	defer func() { OnProviderFallback = nil }()
	if got := ModelsFor(models[:1], "DLH", "", "A20N", 0); len(got) != 1 || len(fell) != 1 {
		t.Errorf("only the add-on installed: %v, reported %v", got, fell)
	}
	SetProviders(map[string]bool{"FlyByWire Simulations": true, "FSLTL": false})
	if got := ModelsFor(models, "DLH", "", "A20N", 0); len(got) != 1 || got[0] != models[0] {
		t.Errorf("switched: %v", got)
	}
}
