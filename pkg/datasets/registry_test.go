//go:build windows
// +build windows

package datasets

import (
	"sync"
	"testing"
)

// newTestConstructor returns a constructor that produces a non-nil *DataSet
// containing a single definition with the provided name as a marker.
func newTestConstructor(marker string) func() *DataSet {
	return func() *DataSet {
		return &DataSet{
			Definitions: []DataDefinition{
				{Name: marker},
			},
		}
	}
}

// resetRegistry clears the global registry for test isolation.
// Only safe to call from tests; guarded by the registry's own mutex.
func resetRegistry(entries map[string]registryEntry) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.entries = entries
}

func TestRegisterAndGet_RoundTrip(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(orig)

	ctor := newTestConstructor("register-and-get")
	Register("test/foo", "test", ctor)

	got, ok := Get("test/foo")
	if !ok {
		t.Fatal("Get: expected ok=true, got false")
	}
	if got == nil {
		t.Fatal("Get: expected non-nil constructor")
	}
	ds := got()
	if len(ds.Definitions) != 1 || ds.Definitions[0].Name != "register-and-get" {
		t.Errorf("Get: unexpected dataset content: %+v", ds)
	}
}

func TestGet_Miss(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(orig)

	got, ok := Get("test/does-not-exist")
	if ok {
		t.Error("Get: expected ok=false for unregistered name, got true")
	}
	if got != nil {
		t.Error("Get: expected nil constructor for unregistered name")
	}
}

func TestList_Sorted(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(make(map[string]registryEntry))

	Register("test/gamma", "test", newTestConstructor("gamma"))
	Register("test/alpha", "test", newTestConstructor("alpha"))
	Register("test/beta", "test", newTestConstructor("beta"))

	names := List()
	if len(names) != 3 {
		t.Fatalf("List: expected 3 names, got %d: %v", len(names), names)
	}
	expected := []string{"test/alpha", "test/beta", "test/gamma"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("List[%d]: expected %q, got %q", i, expected[i], name)
		}
	}
	_ = orig // suppress "declared and not used"
}

func TestCategories_DistinctSorted(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(make(map[string]registryEntry))

	Register("test/a1", "test-z", newTestConstructor("a1"))
	Register("test/a2", "test-a", newTestConstructor("a2"))
	Register("test/a3", "test-z", newTestConstructor("a3"))
	Register("test/a4", "test-m", newTestConstructor("a4"))

	cats := Categories()
	if len(cats) != 3 {
		t.Fatalf("Categories: expected 3 distinct categories, got %d: %v", len(cats), cats)
	}
	expected := []string{"test-a", "test-m", "test-z"}
	for i, cat := range cats {
		if cat != expected[i] {
			t.Errorf("Categories[%d]: expected %q, got %q", i, expected[i], cat)
		}
	}
	_ = orig
}

func TestListByCategory_Matching(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(make(map[string]registryEntry))

	Register("test/c1", "cat-one", newTestConstructor("c1"))
	Register("test/c2", "cat-two", newTestConstructor("c2"))
	Register("test/c3", "cat-one", newTestConstructor("c3"))

	names := ListByCategory("cat-one")
	if len(names) != 2 {
		t.Fatalf("ListByCategory: expected 2 names, got %d: %v", len(names), names)
	}
	expected := []string{"test/c1", "test/c3"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("ListByCategory[%d]: expected %q, got %q", i, expected[i], name)
		}
	}
	_ = orig
}

func TestListByCategory_UnknownCategory(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(orig)

	names := ListByCategory("test-unknown-category-xyz")
	if names != nil {
		t.Errorf("ListByCategory: expected nil for unknown category, got %v", names)
	}
}

func TestRegister_EmptyNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register: expected panic for empty name, got none")
		}
	}()
	Register("", "test", newTestConstructor("should-panic"))
}

func TestRegister_DuplicateSilentlyOverwrites(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(orig)

	firstCtor := newTestConstructor("first")
	secondCtor := newTestConstructor("second")

	Register("test/dup", "test", firstCtor)
	Register("test/dup", "test", secondCtor)

	got, ok := Get("test/dup")
	if !ok {
		t.Fatal("Get: expected ok=true after duplicate register")
	}
	ds := got()
	if len(ds.Definitions) == 0 || ds.Definitions[0].Name != "second" {
		t.Errorf("Register: expected second constructor to win, got %+v", ds)
	}
}

func TestConcurrentAccess(t *testing.T) {
	orig := cloneEntries()
	defer resetRegistry(orig)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Writers
	for range goroutines {
		go func() {
			defer wg.Done()
			Register("test/concurrent", "test", newTestConstructor("concurrent"))
		}()
	}

	// Readers via Get
	for range goroutines {
		go func() {
			defer wg.Done()
			_, _ = Get("test/concurrent")
		}()
	}

	// Readers via List
	for range goroutines {
		go func() {
			defer wg.Done()
			_ = List()
		}()
	}

	wg.Wait()
}

// cloneEntries takes a snapshot of the current registry entries so a test
// can restore the registry to its original state on cleanup.
func cloneEntries() map[string]registryEntry {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	snapshot := make(map[string]registryEntry, len(globalRegistry.entries))
	for k, v := range globalRegistry.entries {
		snapshot[k] = v
	}
	return snapshot
}
