package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Model rules (#1024): a title never used is passed over by ModelsFor and
// UsableModels; a local rule adds one or takes a shipped one back.
func TestModelRules(t *testing.T) {
	if !ModelNever("Car Ground Power Unit") || !ModelNever(TugSmallTitle) {
		t.Fatal("shipped rules not in use")
	}
	models := []string{"FSLTL_FAIB_A320_CSA-Czech Airlines", "FSLTL_FAIB_A320_DLH-Lufthansa"}
	if err := dict.Use("traffic.modelRules", []byte(`[{"title":"FSLTL_FAIB_A320_CSA-Czech Airlines","kind":"aircraft","never":true},{"title":"Car Ground Power Unit","never":false}]`)); err != nil {
		t.Fatal(err)
	}
	defer dict.Reset("traffic.modelRules")
	if got := ModelsFor(models, "CSA", "", "A320", 0); len(got) != 1 || got[0] != models[1] {
		t.Errorf("models with CSA's never used: %v", got)
	}
	if ModelNever("Car Ground Power Unit") {
		t.Error("the GPU van not taken back")
	}
	if got := UsableModels([]string{"Car Ground Power Unit", TugSmallTitle}); len(got) != 1 || got[0] != "Car Ground Power Unit" {
		t.Errorf("usable %v", got)
	}
}

// A rule ending in "*" matches every title beginning with it: our own
// helpers ("MyCrew Jetway Helper A320 AFR", live: picked as AFR342) are
// never traffic; an exact rule or a longer prefix wins.
func TestModelRulesPrefix(t *testing.T) {
	if !ModelNever("MyCrew Jetway Helper A320 AFR") || ModelNever("FSLTL_FAIB_A320_AFR-Air France") {
		t.Fatal("shipped prefix rule")
	}
	if got := ModelsFor([]string{"MyCrew Jetway Helper A320 AFR", "FSLTL_FAIB_A320_AFR-Air France"}, "AFR", "", "A320", 0); len(got) != 1 || got[0] != "FSLTL_FAIB_A320_AFR-Air France" {
		t.Errorf("models %v", got)
	}
	if err := dict.Use("traffic.modelRules", []byte(`[{"title":"MyCrew Demo *","never":false},{"title":"MyCrew Tool X","never":false}]`)); err != nil {
		t.Fatal(err)
	}
	defer dict.Reset("traffic.modelRules")
	if ModelNever("MyCrew Demo A320") || ModelNever("MyCrew Tool X") || !ModelNever("MyCrew Tool Y") {
		t.Error("exact or longer prefix did not win")
	}
}
