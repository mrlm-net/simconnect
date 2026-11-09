package traffic

import (
	"slices"
	"testing"
)

// A title resolved once is served from memory: no upper-casing, no scan
// (no allocation at all), the same profile as resolved.
func TestProfileForMemo(t *testing.T) {
	const title = "FSLTL A320 Air France SL :: some livery"
	want := resolveProfile(knownTypesNow.Load(), title)
	if got := ProfileFor(title); got != want {
		t.Fatalf("first: %+v, want %+v", got, want)
	}
	if n := testing.AllocsPerRun(100, func() { _ = ProfileFor(title) }); n != 0 {
		t.Errorf("ProfileFor of a known title: %.0f allocations a call, want 0 (memoized)", n)
	}
	if got := ProfileFor(title); got != want {
		t.Errorf("memoized: %+v, want %+v", got, want)
	}
}

// A new type table (a dictionary reload) drops what was memoized.
func TestProfileForMemoTableReload(t *testing.T) {
	old := knownTypesNow.Load()
	t.Cleanup(func() { knownTypesNow.Store(old) })
	const title = "FSLTL A320 Air France SL"
	before := ProfileFor(title)
	changed := slices.Clone(old)
	for i := range changed {
		if changed[i].Type == "A320" {
			changed[i].match = []string{"NO SUCH TITLE"}
		}
	}
	knownTypesNow.Store(changed)
	after := ProfileFor(title)
	if after == before && before.Type == "A320" && resolveProfile(changed, title).Type != "A320" {
		t.Errorf("after a table reload still %q", after.Type)
	}
	if after != resolveProfile(changed, title) {
		t.Errorf("after the reload %+v, want %+v", after, resolveProfile(changed, title))
	}
}
