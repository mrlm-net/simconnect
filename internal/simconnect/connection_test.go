//go:build windows

package simconnect

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConnectBadDLL: a SimConnect.dll that does not load is an error from
// Connect, not a panic (#1402): empty, not a DLL, or missing altogether; and
// a real DLL without SimConnect_Open.
func TestConnectBadDLL(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for name, path := range map[string]string{
		"empty":     write("empty.dll", ""),
		"not a DLL": write("text.dll", "this is not a DLL"),
		"missing":   filepath.Join(dir, "missing.dll"),
		"no Open":   "kernel32.dll",
	} {
		t.Run(name, func(t *testing.T) {
			if err := New("test", &Config{DLLPath: path}).Connect(); err == nil {
				t.Fatal("no error")
			}
		})
	}
}
