//go:build !windows

package world

// loadInstalledDoors: no simulator packages here; the host gives the doors
// (SetModelDoors).
func loadInstalledDoors() {}
