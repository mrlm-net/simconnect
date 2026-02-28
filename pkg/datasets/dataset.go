//go:build windows
// +build windows

package datasets

type DataSet struct {
	Definitions []DataDefinition
}

// Clone returns an independent deep copy of the DataSet.
// Mutations to the clone's Definitions slice (append or field mutation)
// do not affect the original, and vice versa.
func (ds DataSet) Clone() DataSet {
	defs := make([]DataDefinition, len(ds.Definitions))
	copy(defs, ds.Definitions)
	return DataSet{Definitions: defs}
}
