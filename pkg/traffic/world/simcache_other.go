//go:build !windows

package world

// packageStamp: no simulator packages here; titles, layouts and
// procedures are not kept (simCache).
func packageStamp() string { return "" }
