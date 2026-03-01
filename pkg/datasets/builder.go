//go:build windows
// +build windows

package datasets

import "github.com/mrlm-net/simconnect/pkg/types"

// Builder provides a fluent API for constructing DataSet values incrementally.
// All mutating methods return the receiver to allow method chaining.
// Build() is non-destructive and may be called multiple times; each call
// returns an independent snapshot of the current builder state.
//
// Builder is not safe for concurrent use by multiple goroutines.
// Callers that share a Builder across goroutines must synchronise externally.
type Builder struct {
	definitions []DataDefinition
}

// NewBuilder returns an empty Builder ready for use.
func NewBuilder() *Builder {
	return &Builder{}
}

// Add appends a DataDefinition to the builder.
// Returns the builder for chaining.
func (b *Builder) Add(def DataDefinition) *Builder {
	b.definitions = append(b.definitions, def)
	return b
}

// AddField appends a field specified by individual parameters.
// Returns the builder for chaining.
func (b *Builder) AddField(name, unit string, dataType types.SIMCONNECT_DATATYPE, epsilon float32) *Builder {
	return b.Add(DataDefinition{
		Name:    name,
		Unit:    unit,
		Type:    dataType,
		Epsilon: epsilon,
	})
}

// Remove removes the first definition with the given Name.
// If no definition with that Name exists, Remove is a no-op.
// Returns the builder for chaining.
func (b *Builder) Remove(name string) *Builder {
	for i, def := range b.definitions {
		if def.Name == name {
			b.definitions = append(b.definitions[:i], b.definitions[i+1:]...)
			return b
		}
	}
	return b
}

// Build returns a new DataSet snapshot of the current builder state.
// The operation is repeatable: calling Build() multiple times produces
// independent copies; adding to or removing from the builder after a Build()
// call does not affect previously built DataSets.
func (b *Builder) Build() DataSet {
	// Use append into an empty slice to guarantee an independent backing array,
	// preventing slice aliasing between the builder and the returned DataSet.
	defs := append([]DataDefinition{}, b.definitions...)
	return DataSet{Definitions: defs}
}

// Len returns the number of definitions currently held in the builder.
func (b *Builder) Len() int {
	return len(b.definitions)
}

// Reset removes all definitions from the builder.
// Returns the builder for chaining.
func (b *Builder) Reset() *Builder {
	b.definitions = nil
	return b
}
