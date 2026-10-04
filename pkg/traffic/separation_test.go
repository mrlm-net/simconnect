package traffic

import "testing"

func TestAirborneSeparation(t *testing.T) {
	p := pictureLKPR.Position
	ac := []TrackedAircraft{
		{Observation: Observation{Tail: "A", Position: p, AltFt: 7000}},
		{Observation: Observation{Tail: "B", Position: offsetHeading(p, 90, 4*1852), AltFt: 7300}},  // 4 NM, 300 ft: loss
		{Observation: Observation{Tail: "C", Position: offsetHeading(p, 270, 2*1852), AltFt: 9000}}, // 2 NM, 2000 ft: separated
		{Observation: Observation{Tail: "G", Position: p, OnGround: true}},
	}
	pairs := AirborneSeparation(ac, EnrouteSeparationNM, VerticalSeparationFt)
	if len(pairs) != 3 {
		t.Fatalf("%d pairs of three airborne", len(pairs))
	}
	loss := map[string]bool{}
	for _, x := range pairs {
		loss[x.A+x.B] = x.Loss
	}
	if !loss["AB"] || loss["AC"] || loss["BC"] {
		t.Errorf("losses %v", loss)
	}
	if pairs[0].LateralNM > pairs[1].LateralNM {
		t.Error("not closest first")
	}
}
