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
