//go:build windows
// +build windows

package datasets

// Merge combines multiple DataSets into one, deduplicating definitions by Name.
//
// Deduplication uses last-wins semantics: when the same Name appears more than
// once across all input datasets, only the last encountered definition is kept,
// and it occupies the position of that last occurrence (earlier duplicates are
// removed, maintaining relative order of surviving entries).
//
// Example: Merge([A,B,C], [B,D]) produces [A,C,B,D]
//   - B from the first dataset is removed because B reappears later.
//   - The B from the second dataset occupies the position it was last seen.
//
// The returned DataSet is always a fresh independent copy; callers may mutate
// it without affecting the input datasets.
//
// Special cases:
//   - Merge() with zero arguments returns an empty DataSet.
//   - Merge() with a single argument is equivalent to Clone.
func Merge(datasets ...DataSet) DataSet {
	if len(datasets) == 0 {
		return DataSet{}
	}

	// order tracks the insertion order of surviving definitions. Because
	// last-wins means we remove an earlier entry and re-insert at the end,
	// we rebuild the ordered slice from the index map below.
	//
	// index maps each Name to its current position in 'order'.
	order := make([]DataDefinition, 0)
	index := make(map[string]int) // name → position in order

	for _, ds := range datasets {
		for _, def := range ds.Definitions {
			if pos, exists := index[def.Name]; exists {
				// Remove the previous occurrence by compacting the slice.
				order = append(order[:pos], order[pos+1:]...)
				// Shift all indices that were after the removed position.
				for k, v := range index {
					if v > pos {
						index[k] = v - 1
					}
				}
			}
			// Append the new (or updated) definition at the end.
			index[def.Name] = len(order)
			order = append(order, def)
		}
	}

	defs := make([]DataDefinition, len(order))
	copy(defs, order)
	return DataSet{Definitions: defs}
}
