package registry

import (
	"strings"
	"testing"
)

// TestLookupExact verifies that an exact-case name resolves correctly.
func TestLookupExact(t *testing.T) {
	sv, ok := Lookup("PLANE LATITUDE")
	if !ok {
		t.Fatal("Lookup(\"PLANE LATITUDE\") returned ok=false")
	}
	if sv.Name != "PLANE LATITUDE" {
		t.Errorf("expected Name=PLANE LATITUDE, got %q", sv.Name)
	}
}

// TestLookupCaseInsensitive verifies that lowercase input resolves to the same entry.
func TestLookupCaseInsensitive(t *testing.T) {
	svUpper, _ := Lookup("PLANE LATITUDE")
	svLower, ok := Lookup("plane latitude")
	if !ok {
		t.Fatal("Lookup(\"plane latitude\") returned ok=false")
	}
	if svLower.Name != svUpper.Name {
		t.Errorf("case-insensitive lookup mismatch: got %q, want %q", svLower.Name, svUpper.Name)
	}
}

// TestLookupMixedCase verifies that mixed-case input resolves to the same entry.
func TestLookupMixedCase(t *testing.T) {
	svUpper, _ := Lookup("PLANE LATITUDE")
	svMixed, ok := Lookup("Plane Latitude")
	if !ok {
		t.Fatal("Lookup(\"Plane Latitude\") returned ok=false")
	}
	if svMixed.Name != svUpper.Name {
		t.Errorf("mixed-case lookup mismatch: got %q, want %q", svMixed.Name, svUpper.Name)
	}
}

// TestLookupIndexSuffix verifies that a :1 suffix is stripped before lookup.
func TestLookupIndexSuffix(t *testing.T) {
	sv, ok := Lookup("ENG RPM:1")
	if !ok {
		t.Fatal("Lookup(\"ENG RPM:1\") returned ok=false")
	}
	if sv.Name != "ENG RPM" {
		t.Errorf("expected Name=ENG RPM, got %q", sv.Name)
	}
	if !sv.Indexed {
		t.Error("expected Indexed=true for ENG RPM")
	}
}

// TestLookupIndexSuffix4 verifies that a :4 suffix is also stripped.
func TestLookupIndexSuffix4(t *testing.T) {
	sv, ok := Lookup("ENG RPM:4")
	if !ok {
		t.Fatal("Lookup(\"ENG RPM:4\") returned ok=false")
	}
	if sv.Name != "ENG RPM" {
		t.Errorf("expected Name=ENG RPM, got %q", sv.Name)
	}
}

// TestLookupNonDigitSuffix verifies that a :foo suffix is NOT stripped.
func TestLookupNonDigitSuffix(t *testing.T) {
	// "ENG RPM:foo" should not match anything (non-digit suffix preserved)
	_, ok := Lookup("ENG RPM:foo")
	if ok {
		t.Error("Lookup(\"ENG RPM:foo\") should return ok=false (non-digit suffix not stripped)")
	}
}

// TestLookupNotFound verifies that an unknown name returns ok=false.
func TestLookupNotFound(t *testing.T) {
	_, ok := Lookup("BOGUS SIMVAR THAT DOES NOT EXIST")
	if ok {
		t.Error("expected ok=false for unknown SimVar")
	}
}

// TestLookupEmptyString verifies that an empty string returns ok=false.
func TestLookupEmptyString(t *testing.T) {
	_, ok := Lookup("")
	if ok {
		t.Error("expected ok=false for empty string lookup")
	}
}

// TestAllMinimumCount verifies that All() returns at least 80 entries.
func TestAllMinimumCount(t *testing.T) {
	all := All()
	if len(all) < 80 {
		t.Errorf("All() returned %d entries; want >= 80", len(all))
	}
}

// TestAllIndependentCopy verifies that modifying the result of All() does not
// affect the package-level list.
func TestAllIndependentCopy(t *testing.T) {
	first := All()
	n := len(first)
	// Append a dummy entry to the returned slice
	first = append(first, SimVarMeta{Name: "INJECTED"})

	second := All()
	if len(second) != n {
		t.Errorf("All() length changed after appending to prior result: got %d, want %d", len(second), n)
	}
}

// TestValidateGoodUnit verifies that a known-valid unit returns nil.
func TestValidateGoodUnit(t *testing.T) {
	if err := Validate("PLANE LATITUDE", "degrees"); err != nil {
		t.Errorf("Validate returned unexpected error: %v", err)
	}
}

// TestValidateCaseInsensitiveUnit verifies that unit comparison is case-insensitive.
func TestValidateCaseInsensitiveUnit(t *testing.T) {
	if err := Validate("PLANE LATITUDE", "Degrees"); err != nil {
		t.Errorf("Validate(\"PLANE LATITUDE\", \"Degrees\") returned unexpected error: %v", err)
	}
}

// TestValidateCaseInsensitiveName verifies that name comparison is case-insensitive.
func TestValidateCaseInsensitiveName(t *testing.T) {
	if err := Validate("plane latitude", "degrees"); err != nil {
		t.Errorf("Validate(\"plane latitude\", \"degrees\") returned unexpected error: %v", err)
	}
}

// TestValidateUnknownSimVar verifies that an unknown SimVar returns an error containing the name.
func TestValidateUnknownSimVar(t *testing.T) {
	err := Validate("BOGUS VAR", "degrees")
	if err == nil {
		t.Fatal("expected non-nil error for unknown SimVar")
	}
	if !strings.Contains(err.Error(), "BOGUS VAR") {
		t.Errorf("error message should contain the unknown var name; got: %v", err)
	}
}

// TestValidateInvalidUnit verifies that an invalid unit returns an error containing valid units.
func TestValidateInvalidUnit(t *testing.T) {
	err := Validate("PLANE LATITUDE", "parsecs")
	if err == nil {
		t.Fatal("expected non-nil error for invalid unit")
	}
	if !strings.Contains(err.Error(), "degrees") {
		t.Errorf("error message should contain valid unit 'degrees'; got: %v", err)
	}
}

// TestValidateIndexedVar verifies that Validate strips the index suffix before lookup.
func TestValidateIndexedVar(t *testing.T) {
	if err := Validate("ENG RPM:2", "rpm"); err != nil {
		t.Errorf("Validate(\"ENG RPM:2\", \"rpm\") returned unexpected error: %v", err)
	}
}

// TestByUnitDegrees verifies that ByUnit("degrees") includes PLANE LATITUDE and PLANE LONGITUDE.
func TestByUnitDegrees(t *testing.T) {
	results := ByUnit("degrees")
	if len(results) == 0 {
		t.Fatal("ByUnit(\"degrees\") returned no results")
	}

	wantNames := map[string]bool{
		"PLANE LATITUDE":  false,
		"PLANE LONGITUDE": false,
	}
	for _, sv := range results {
		if _, want := wantNames[sv.Name]; want {
			wantNames[sv.Name] = true
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Errorf("ByUnit(\"degrees\") did not include %q", name)
		}
	}
}

// TestByUnitCaseInsensitive verifies that ByUnit is case-insensitive.
func TestByUnitCaseInsensitive(t *testing.T) {
	lower := ByUnit("degrees")
	upper := ByUnit("Degrees")
	if len(lower) != len(upper) {
		t.Errorf("ByUnit case mismatch: ByUnit(\"degrees\")=%d, ByUnit(\"Degrees\")=%d", len(lower), len(upper))
	}
}

// TestByUnitUnknown verifies that ByUnit returns nil for an unknown unit.
func TestByUnitUnknown(t *testing.T) {
	results := ByUnit("unobtainium")
	if len(results) != 0 {
		t.Errorf("expected nil/empty for unknown unit, got %d entries", len(results))
	}
}

// TestByCategoryAircraft verifies that ByCategory("aircraft") returns at least 20 entries.
func TestByCategoryAircraft(t *testing.T) {
	results := ByCategory("aircraft")
	if len(results) < 20 {
		t.Errorf("ByCategory(\"aircraft\") returned %d entries; want >= 20", len(results))
	}
}

// TestByCategoryCaseInsensitive verifies that ByCategory is case-insensitive.
func TestByCategoryCaseInsensitive(t *testing.T) {
	lower := ByCategory("aircraft")
	upper := ByCategory("Aircraft")
	if len(lower) != len(upper) {
		t.Errorf("ByCategory case mismatch: ByCategory(\"aircraft\")=%d, ByCategory(\"Aircraft\")=%d", len(lower), len(upper))
	}
}

// TestByCategoryUnknown verifies that ByCategory returns nil for an unknown category.
func TestByCategoryUnknown(t *testing.T) {
	results := ByCategory("dragons")
	if len(results) != 0 {
		t.Errorf("expected nil/empty for unknown category, got %d entries", len(results))
	}
}

// TestNoDuplicates verifies that the simvars slice has no duplicate names.
// Uses the internal simvarMap (same package — white-box test).
func TestNoDuplicates(t *testing.T) {
	if len(simvars) != len(simvarMap) {
		t.Errorf("duplicate SimVar names detected: simvars has %d entries, simvarMap has %d keys",
			len(simvars), len(simvarMap))
	}
}

// TestStripIndexSuffix is a table-driven test of the unexported stripIndexSuffix helper.
func TestStripIndexSuffix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"ENG RPM:1", "ENG RPM"},
		{"ENG RPM:42", "ENG RPM"},
		{"ENG RPM:0", "ENG RPM"},
		{"ENG RPM:foo", "ENG RPM:foo"}, // non-digit suffix: unchanged
		{"SIMPLE", "SIMPLE"},           // no colon: unchanged
		{"A:0", "A"},
		{"A:", "A:"},   // empty after colon: unchanged
		{":1", ""},     // colon at start with digit: strip to empty string
		{"", ""},       // empty input: unchanged
		{"NO:COLON:1", "NO:COLON"}, // last colon with digit: strip
		{"NO:COLON:foo", "NO:COLON:foo"}, // last colon with non-digit: unchanged
	}

	for _, tc := range cases {
		got := stripIndexSuffix(tc.input)
		if got != tc.want {
			t.Errorf("stripIndexSuffix(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}
