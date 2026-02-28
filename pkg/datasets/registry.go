//go:build windows
// +build windows

package datasets

import (
	"sort"
	"sync"
)

// registryEntry holds the category and constructor for a named dataset.
type registryEntry struct {
	category    string
	constructor func() *DataSet
}

// registry is a thread-safe map of dataset name to registryEntry.
type registry struct {
	mu      sync.RWMutex
	entries map[string]registryEntry
}

// globalRegistry is the package-level singleton registry.
var globalRegistry = &registry{entries: make(map[string]registryEntry)}

// Register adds a dataset constructor to the global registry under the given
// name and category. The name must follow the "<category>/<descriptor>"
// convention (e.g. "traffic/aircraft"). Panics if name is empty.
// Silently overwrites any previously registered entry with the same name.
func Register(name, category string, constructor func() *DataSet) {
	if name == "" {
		panic("datasets.Register: name must not be empty")
	}
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.entries[name] = registryEntry{
		category:    category,
		constructor: constructor,
	}
}

// Get returns the constructor for the named dataset and true.
// Returns nil and false if no dataset with that name has been registered.
func Get(name string) (func() *DataSet, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	entry, ok := globalRegistry.entries[name]
	if !ok {
		return nil, false
	}
	return entry.constructor, true
}

// List returns the names of all registered datasets in sorted order.
func List() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	names := make([]string, 0, len(globalRegistry.entries))
	for name := range globalRegistry.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Categories returns the distinct category names of all registered datasets,
// sorted alphabetically.
func Categories() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, entry := range globalRegistry.entries {
		seen[entry.category] = struct{}{}
	}
	cats := make([]string, 0, len(seen))
	for cat := range seen {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	return cats
}

// ListByCategory returns the names of all datasets in the given category,
// sorted alphabetically. Returns nil if no datasets are registered under
// that category.
func ListByCategory(category string) []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	var names []string
	for name, entry := range globalRegistry.entries {
		if entry.category == category {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return names
}
