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

// Both level at levels 1000 ft apart (live, AFR613 at FL390 and THY1987 at
// FL380 read 974 ft apart): separated within the altimetry tolerance; one
// climbing or descending through, the full 1000 ft.
func TestAirborneSeparationLevelTolerance(t *testing.T) {
	p := pictureLKPR.Position
	level := []TrackedAircraft{
		{Observation: Observation{Tail: "AFR613", Position: p, AltFt: 38974}},
		{Observation: Observation{Tail: "THY1987", Position: offsetHeading(p, 90, 2*1852), AltFt: 38000}},
	}
	if pairs := AirborneSeparation(level, EnrouteSeparationNM, VerticalSeparationFt); pairs[0].Loss {
		t.Errorf("both level 974 ft apart: a loss, want separated")
	}
	level[0].VSFpm = -1500 // descending through
	if pairs := AirborneSeparation(level, EnrouteSeparationNM, VerticalSeparationFt); !pairs[0].Loss {
		t.Errorf("descending 974 ft above it: separated, want a loss")
	}
}

// Placed on a stand or a taxiway a few feet up, not reported on the ground
// by the simulator (live, CSA1682 at its gate at 8 ft, a TCAS TA with
// UAE530 departing): on the ground all the same, no separation counted.
func TestGroundedLowAboveTheGround(t *testing.T) {
	p := pictureLKPR.Position
	ac := []TrackedAircraft{
		{Observation: Observation{Tail: "CSA1682", Position: p, AltFt: 1250, AGLFt: 8}},
		{Observation: Observation{Tail: "UAE530", Position: offsetHeading(p, 90, 0.7*1852), AltFt: 1260, AGLFt: 20, GroundKts: 150}},
	}
	if !ac[0].Grounded() || len(AirborneSeparation(ac, EnrouteSeparationNM, VerticalSeparationFt)) != 0 {
		t.Errorf("an aircraft 8 ft above its gate counted in the air")
	}
}
