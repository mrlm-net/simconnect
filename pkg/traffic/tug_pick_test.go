package traffic

import "testing"

// tugClearance: by type, the worst clearance (m) between each tug and the
// aircraft over it, at the tug's offset from the nose wheel (TugAheadFor;
// DefaultTugTitle at 0): {DefaultTugTitle, TugNarrowTitle, TugLowTitle}.
// From the GSX tug and FSLTL aircraft glTF models rasterised to 10 cm cells:
// the tug's top against the aircraft's underside, the aircraft at its
// static_cg_height and static_pitch (as the injector places it), its nose
// wheel at its [CONTACT_POINTS], gear and doors retracted; the nose gear's
// own 1.6 x 1.6 m left out (scratchpad tugclip/clip.js, 2026-11-07).
var tugClearance = map[string][3]float64{
	"A20N": {-0.05, 0.99, 1.48},
	"A21N": {0.02, 1.03, 1.53},
	"A306": {0.67, 1.48, 1.75},
	"A30B": {0.60, 1.40, 1.66},
	"A319": {-0.08, 0.96, 1.47},
	"A320": {-0.03, 1.01, 1.51},
	"A321": {-0.04, 0.99, 1.50},
	"A332": {0.82, 1.64, 1.89},
	"A333": {0.67, 1.49, 1.75},
	"A343": {0.70, 1.51, 1.76},
	"A346": {0.69, 1.44, 1.66},
	"A359": {0.81, 1.57, 2.00},
	"A388": {0.72, 1.53, 1.85},
	"AT45": {-0.21, 0.35, 0.54},
	"AT46": {-0.21, 0.35, 0.54},
	"AT75": {-0.41, -0.07, 0.22},
	"AT76": {-0.41, -0.07, 0.22},
	"B733": {-0.13, 0.35, 0.73},
	"B734": {-0.33, 0.20, 0.60},
	"B737": {-0.12, 0.46, 0.88},
	"B738": {-0.10, 0.46, 0.87},
	"B739": {-0.20, 0.37, 0.79},
	"B73X": {-0.10, 0.46, 0.87},
	"B744": {1.26, 2.02, 2.26},
	"B748": {1.28, 2.04, 2.29},
	"B763": {0.97, 1.52, 1.74},
	"B772": {1.67, 2.70, 2.90},
	"B77L": {1.67, 2.70, 2.90},
	"B77W": {1.11, 1.75, 2.00},
	"B788": {-0.04, 0.89, 1.28},
	"B789": {-0.09, 0.83, 1.22},
	"B78X": {0.48, 1.40, 1.81},
	"BCS1": {0.63, 1.07, 1.46},
	"BCS3": {0.58, 1.07, 1.46},
	"C25C": {0.22, 0.46, 0.68},
	"CRJ7": {0.23, 0.42, 0.81},
	"DH8C": {-0.31, -0.09, 0.24},
	"DH8D": {-0.39, -0.13, 0.32},
	"E170": {-0.28, 0.77, 1.25},
	"E190": {-0.25, 0.80, 1.28},
	"LJ25": {-0.54, -0.29, 0.15},
	"SF34": {-0.03, 0.23, 0.67},
	"SU95": {-0.18, 0.47, 0.92},
}

// tugClearanceAlias: table types not measured, by the measured type their
// fuselage and nose gear share.
var tugClearanceAlias = map[string]string{
	"A338": "A333", "A339": "A333", "A35K": "A359", "A19N": "A319",
	"B74F": "B744", "B762": "B763", "B764": "B763", "B77F": "B77L", "B773": "B77W",
	"B37M": "B737", "B38M": "B738", "B39M": "B739",
	"E175": "E170", "E195": "E190", "E290": "E190", "E295": "E190",
	"CRJ9": "CRJ7", "CRJX": "CRJ7",
}

// The tug picked for each type clears it by 0.15 m or more (the user, live
// LKPR 2026-11-07: tugs under the aircraft, in the fuselage; the default tug
// for every type over 16 m span had its cab in a B738's radome by 0.10 m
// and its arms in an ATR's belly by 0.41 m). A type not measured takes the
// low tug, which cleared every measured type. The default tug is kept only
// where it clears by 0.3 m.
func TestTugClearsEveryType(t *testing.T) {
	idx := map[string]int{DefaultTugTitle: 0, TugNarrowTitle: 1, TugLowTitle: 2}
	for typ, title := range tugForType {
		i, ok := idx[title]
		if !ok {
			t.Errorf("%s: tug %q not measured", typ, title)
			continue
		}
		m := typ
		if a, ok := tugClearanceAlias[typ]; ok {
			m = a
		}
		c, ok := tugClearance[m]
		if !ok {
			if title != TugLowTitle {
				t.Errorf("%s: not measured, gets %s, want %s", typ, title, TugLowTitle)
			}
			continue
		}
		if c[i] < 0.15 || title == DefaultTugTitle && c[i] < 0.3 {
			t.Errorf("%s: %s clears it by %.2f m (default %.2f, narrow %.2f, low %.2f)", typ, title, c[i], c[0], c[1], c[2])
		}
	}
	for typ := range tugClearance {
		if _, ok := tugForType[typ]; !ok {
			t.Errorf("%s measured but not in the table", typ)
		}
	}
	for _, c := range tugClearance {
		if c[2] < 0.15 {
			t.Errorf("the low tug clears %.2f m somewhere: not the fallback for unmeasured types", c[2])
		}
	}
}

// TugTitleForModel by title: the live departures' models, the fallbacks by
// kind and span, an asked tug, a light aircraft's robot.
func TestTugTitleForModel(t *testing.T) {
	for _, c := range []struct {
		asked, model string
		span         float64
		want         string
	}{
		{"", "FSLTL_FAIB_B738_TVS-Smartwings_NC", 35.8, TugNarrowTitle},
		{"", "Asobo PassiveAircraft Citation XLS :: CITATION_XLS_0", 17.17, TugLowTitle},
		{"", "FSLTL_A332F_MAS", 60.3, DefaultTugTitle},
		{"", "FSLTL_B789_BAW", 60.1, TugNarrowTitle},
		{"", "FSLTL_AT76_CSA", 27.1, TugLowTitle},
		{"", "FSLTL_B734_TVS", 28.9, TugLowTitle},
		{"", "FSLTL_FAIB_A320_SmartWings", 35.8, TugNarrowTitle},
		{"", "Some Unknown Widebody", 64, DefaultTugTitle},
		{"", "Some Unknown Jet", 30, TugNarrowTitle},
		{"", "Some Unknown Jet", 0, TugNarrowTitle},
		{"FSDT_TPX_200", "FSLTL_B738_RYR", 35.8, "FSDT_TPX_200"},
		{TugSmallTitle, "FSLTL_B738_RYR", 35.8, TugNarrowTitle},
		{"", "Asobo PassiveAircraft Cessna 172", 11, TugMediumTitle}, // TugSmallTitle is never used (model rules)
	} {
		if got := TugTitleForModel(c.asked, c.model, c.span); got != c.want {
			t.Errorf("TugTitleForModel(%q, %q, %.1f) = %s, want %s", c.asked, c.model, c.span, got, c.want)
		}
	}
	for _, title := range []string{TugNarrowTitle, TugLowTitle} {
		if a, ok := TugAheadFor(title); !ok || a != -1.8 {
			t.Errorf("TugAheadFor(%s) = %.1f %v, measured at -1.8", title, a, ok)
		}
	}
}
