package traffic

import "testing"

// tugClearance: by type, the worst clearance (m) between each tug and the
// aircraft over it, at the tug's offset from the nose wheel (TugAheadFor;
// DefaultTugTitle at 0): {DefaultTugTitle, TugNarrowTitle, TugLowTitle}.
// From the GSX tug and FSLTL aircraft glTF models rasterised to 10 cm cells:
// the tug's top against the aircraft's underside, the aircraft at its
// static_cg_height and static_pitch (as the injector places it), its nose
// wheel at its [CONTACT_POINTS], gear and doors retracted; the nose gear's
// own 1.6 x 1.6 m left out (scratchpad tugclip/clip.js, 2026-11-07). The
// stick tugs (TugNarrowTitle, TugLowTitle) measured again whole: the bar
// with the tug hung on it straight (TugBarStraight), the hook on the nose
// wheel (scratchpad tugfit2/clip2.js, 2026-11-08; the first measure had the
// bar alone); 9.99: no part of the aircraft over the tug.
var tugClearance = map[string][3]float64{
	"A20N": {-0.05, 0.86, 1.21},
	"A21N": {0.02, 1.01, 1.25},
	"A306": {0.67, 1.41, 1.77},
	"A30B": {0.60, 1.34, 1.69},
	"A319": {-0.08, 0.82, 1.18},
	"A320": {-0.03, 0.87, 1.23},
	"A321": {-0.04, 0.92, 1.21},
	"A332": {0.82, 1.56, 1.93},
	"A333": {0.67, 1.41, 1.78},
	"A343": {0.70, 1.45, 1.80},
	"A346": {0.69, 1.47, 1.72},
	"A359": {0.81, 1.85, 1.83},
	"A388": {0.72, 1.80, 1.79},
	"AT45": {-0.21, 9.99, 9.99},
	"AT46": {-0.21, 9.99, 9.99},
	"AT75": {-0.41, 0.26, 0.35},
	"AT76": {-0.41, 0.26, 0.35},
	"B733": {-0.13, 0.69, 0.65},
	"B734": {-0.33, 0.53, 0.50},
	"B737": {-0.12, 0.79, 0.74},
	"B738": {-0.10, 0.79, 0.75},
	"B739": {-0.20, 0.70, 0.65},
	"B73X": {-0.10, 0.79, 0.75},
	"B744": {1.26, 1.98, 2.29},
	"B748": {1.28, 2.02, 2.30},
	"B763": {0.97, 1.85, 1.86},
	"B772": {1.67, 3.95, 4.12},
	"B77L": {1.67, 3.95, 4.12},
	"B77W": {1.11, 1.92, 2.06},
	"B788": {-0.04, 0.71, 1.15},
	"B789": {-0.09, 0.73, 1.08},
	"B78X": {0.48, 1.20, 1.63},
	"BCS1": {0.63, 1.40, 1.35},
	"BCS3": {0.58, 1.40, 1.34},
	"C25C": {0.22, 9.99, 9.99},
	"CRJ7": {0.23, 0.76, 0.74},
	"DH8C": {-0.31, 0.24, 0.42},
	"DH8D": {-0.39, 0.19, 0.12},
	"E170": {-0.28, 0.89, 1.00},
	"E190": {-0.25, 0.92, 1.03},
	"LJ25": {-0.54, 0.03, -0.04},
	"SF34": {-0.03, 0.57, 0.51},
	"SU95": {-0.18, 0.74, 0.73},
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
// low tug, which clears every measured type by 0.1 m (a Q400 by 0.12 m: it
// gets the narrow tug). The default tug is kept only where it clears by
// 0.3 m. A type light enough for a robot by its span (TugTitleForModel) never
// gets its table tug: the Learjet 25, which no stick tug clears.
func TestTugClearsEveryType(t *testing.T) {
	robot := func(typ string) bool {
		p := ProfileFor(typ)
		return p.Type == typ && p.WingspanM > 0 && p.WingspanM <= TugSmallMaxSpanM
	}
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
		if !robot(m) && (c[i] < 0.15 || title == DefaultTugTitle && c[i] < 0.3) {
			t.Errorf("%s: %s clears it by %.2f m (default %.2f, narrow %.2f, low %.2f)", typ, title, c[i], c[0], c[1], c[2])
		}
	}
	for typ := range tugClearance {
		if _, ok := tugForType[typ]; !ok {
			t.Errorf("%s measured but not in the table", typ)
		}
	}
	for typ, c := range tugClearance {
		if !robot(typ) && c[2] < 0.1 {
			t.Errorf("the low tug clears %s by %.2f m: not the fallback for unmeasured types", typ, c[2])
		}
	}
	if !robot("LJ25") {
		t.Error("LJ25 not light enough for a robot: give it a tug that clears it")
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
		if a, ok := TugAheadFor(title); ok {
			t.Errorf("TugAheadFor(%s) = %.1f: a stick tug's hook goes on the nose wheel, TugAheadMeters", title, a)
		}
		if _, ok := stickTugFor(title); !ok {
			t.Errorf("%s measured as a stick tug, not in stickTugs", title)
		}
	}
}
