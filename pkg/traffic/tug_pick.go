package traffic

import (
	"sort"
	"strings"
)

// Tugs by clearance under the aircraft (user, live LKPR 2026-11-07: "some
// tugs are too much under plane so they conflict"). DefaultTugTitle sits
// with its slot round the nose wheel: its 2.17 m cab 3.3 to 5 m ahead of
// the wheel, under the radome, and its 1.25 to 1.42 m arms 4.2 m behind it,
// under the fuselage. Only a widebody clears it. Narrowbodies and regional
// jets get TugNarrowTitle, turboprops and business jets TugLowTitle; both
// stick tugs (tug_bar.go), the hook on the nose wheel and the tug a bar's
// length ahead. Measured from the GSX and FSLTL models (tug_pick_test.go).
const (
	TugNarrowTitle = "FSDT_Pushback_Trepel_700"
	TugLowTitle    = "FSDT_Pushback_M1A"
	// TugWideMinSpanM: an unknown jet wider than this gets DefaultTugTitle.
	TugWideMinSpanM = 45.0
)

// tugForType is the tug for each type designator: DefaultTugTitle where it
// clears the aircraft by 0.3 m or more, else TugNarrowTitle (jets),
// TugLowTitle (turboprops, business jets, low fuselages). Types not
// measured (no model on disk) take TugLowTitle, which cleared all.
var tugForType = map[string]string{
	// Widebodies: DefaultTugTitle.
	"A306": DefaultTugTitle, "A30B": DefaultTugTitle,
	"A332": DefaultTugTitle, "A333": DefaultTugTitle, "A338": DefaultTugTitle, "A339": DefaultTugTitle,
	"A343": DefaultTugTitle, "A346": DefaultTugTitle,
	"A359": DefaultTugTitle, "A35K": DefaultTugTitle, "A388": DefaultTugTitle,
	"B744": DefaultTugTitle, "B74F": DefaultTugTitle, "B748": DefaultTugTitle,
	"B762": DefaultTugTitle, "B763": DefaultTugTitle, "B764": DefaultTugTitle,
	"B772": DefaultTugTitle, "B77L": DefaultTugTitle, "B77F": DefaultTugTitle, "B77W": DefaultTugTitle, "B773": DefaultTugTitle,
	"B78X": DefaultTugTitle,
	// The 787-8 and -9: the default tug's cab touches their nose.
	"B788": TugNarrowTitle, "B789": TugNarrowTitle,
	// Narrowbodies and regional jets.
	"A319": TugNarrowTitle, "A19N": TugNarrowTitle, "A320": TugNarrowTitle, "A20N": TugNarrowTitle,
	"A321": TugNarrowTitle, "A21N": TugNarrowTitle,
	"B733": TugNarrowTitle, "B737": TugNarrowTitle, "B37M": TugNarrowTitle, "B738": TugNarrowTitle, "B38M": TugNarrowTitle,
	"B739": TugNarrowTitle, "B39M": TugNarrowTitle, "B73X": TugNarrowTitle,
	"BCS1": TugNarrowTitle, "BCS3": TugNarrowTitle,
	"E170": TugNarrowTitle, "E175": TugNarrowTitle, "E190": TugNarrowTitle, "E195": TugNarrowTitle, "E290": TugNarrowTitle, "E295": TugNarrowTitle,
	"CRJ7": TugNarrowTitle, "CRJ9": TugNarrowTitle, "CRJX": TugNarrowTitle,
	"SU95": TugNarrowTitle,
	// Low fuselages: turboprops, the 737-400, business jets, small
	// regional jets.
	"B734": TugLowTitle,
	"AT43": TugLowTitle, "AT45": TugLowTitle, "AT46": TugLowTitle, "AT72": TugLowTitle, "AT75": TugLowTitle, "AT76": TugLowTitle,
	"DH8A": TugLowTitle, "DH8B": TugLowTitle, "DH8C": TugLowTitle,
	// The Q400: the low tug's bar 0.12 m under its nose, the narrow tug's
	// 0.19 m.
	"DH8D": TugNarrowTitle,
	"SF34": TugLowTitle, "SB20": TugLowTitle, "D328": TugLowTitle, "JS41": TugLowTitle, "B190": TugLowTitle, "E120": TugLowTitle, "F50": TugLowTitle,
	"CRJ1": TugLowTitle, "CRJ2": TugLowTitle, "E135": TugLowTitle, "E145": TugLowTitle, "E35L": TugLowTitle,
	"LJ25": TugLowTitle, "LJ35": TugLowTitle, "LJ45": TugLowTitle, "LJ60": TugLowTitle, "LJ75": TugLowTitle,
	"C25A": TugLowTitle, "C25B": TugLowTitle, "C25C": TugLowTitle, "C510": TugLowTitle, "C525": TugLowTitle,
	"C550": TugLowTitle, "C560": TugLowTitle, "C56X": TugLowTitle, "C650": TugLowTitle, "C680": TugLowTitle,
	"C68A": TugLowTitle, "C700": TugLowTitle, "C750": TugLowTitle,
	"CL30": TugLowTitle, "CL35": TugLowTitle, "CL60": TugLowTitle, "GLEX": TugLowTitle, "GL5T": TugLowTitle, "GL7T": TugLowTitle,
	"GLF4": TugLowTitle, "GLF5": TugLowTitle, "GLF6": TugLowTitle, "G280": TugLowTitle,
	"E50P": TugLowTitle, "E55P": TugLowTitle, "E545": TugLowTitle, "E550": TugLowTitle, "H25B": TugLowTitle, "HDJT": TugLowTitle,
	"F2TH": TugLowTitle, "FA7X": TugLowTitle, "FA8X": TugLowTitle, "F900": TugLowTitle,
	"PC12": TugLowTitle, "PC24": TugLowTitle, "BE20": TugLowTitle, "BE9L": TugLowTitle, "B350": TugLowTitle,
}

// tugTypeKeys: tugForType's keys, longest first.
var tugTypeKeys = func() []string {
	k := make([]string, 0, len(tugForType))
	for t := range tugForType {
		k = append(k, t)
	}
	sort.Slice(k, func(i, j int) bool { return len(k[i]) > len(k[j]) || len(k[i]) == len(k[j]) && k[i] < k[j] })
	return k
}()

// tugTitleForType is the tug of model (a title or type) by tugForType: the
// type a token of the title starts with (FSLTL_FAIB_B738_TVS, FSLTL_A332F),
// else the type ProfileFor resolves.
func tugTitleForType(model string) (string, bool) {
	tokens := strings.FieldsFunc(strings.ToUpper(model), func(r rune) bool {
		return (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
	for _, t := range tokens {
		for _, k := range tugTypeKeys {
			if strings.HasPrefix(t, k) {
				return tugForType[k], true
			}
		}
	}
	t, ok := tugForType[ProfileFor(model).Type]
	return t, ok
}

// TugTitleForModel is the tug for model (a title) of span spanM (0 not
// known), when tug asked is "" or does not suit it (TugTitleOr): a robot
// for a light aircraft, else by its type (tugForType), else by kind and
// size: turboprops and pistons TugLowTitle, jets wider than
// TugWideMinSpanM DefaultTugTitle, other jets TugNarrowTitle. A tug never
// used (ModelNever) gives way to TugLowTitle, TugNarrowTitle, then
// DefaultTugTitle.
func TugTitleForModel(asked, model string, spanM float64) string {
	if asked != "" && !(TugIsRobot(asked) && (spanM <= 0 || spanM > TugSmallMaxSpanM)) {
		return asked
	}
	if spanM > 0 && spanM <= TugSmallMaxSpanM {
		if t := TugTitleFor(spanM); t != DefaultTugTitle {
			return t // a robot
		}
	}
	title, ok := tugTitleForType(model)
	if !ok {
		p := ProfileFor(model)
		span := spanM
		if span <= 0 && p.Type != "" {
			span = p.WingspanM
		}
		switch {
		case p.Type != "" && (p.Category == CategoryTurboprop || p.Category == CategoryPiston):
			title = TugLowTitle
		case span > TugWideMinSpanM:
			title = DefaultTugTitle
		default:
			title = TugNarrowTitle
		}
	}
	for _, t := range []string{title, TugLowTitle, TugNarrowTitle} {
		if !ModelNever(t) {
			return t
		}
	}
	return DefaultTugTitle
}
