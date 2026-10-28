package traffic

import (
	"maps"
	"os"
	"testing"
)

// testProviders: the package's tests pick from the live tests' models
// (FSLTL), switched on over the game's defaults as the airport map does.
var testProviders = map[string]bool{"FSLTL": true}

func TestMain(m *testing.M) {
	SetProviders(testProviders)
	os.Exit(m.Run())
}

// gameDefaults forgets the switches for a test of the defaults; the
// tests' switches come back after it.
func gameDefaults(t *testing.T) {
	t.Helper()
	setSwitches(map[string]bool{})
	t.Cleanup(func() { setSwitches(maps.Clone(testProviders)) })
}

func setSwitches(on map[string]bool) {
	providers.mu.Lock()
	providers.on = on
	providers.mu.Unlock()
}
