//go:build windows
// +build windows

package addons

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Fingerprint is a stable hash of the package set: source, folder and
// version of each package, in any order. It changes when a package is added,
// removed or updated, so a cache built from scenery can be dropped. Streamed
// packages count by name only; their cached content comes and goes.
func Fingerprint(pkgs []Package) string {
	lines := make([]string, len(pkgs))
	for i, p := range pkgs {
		lines[i] = string(p.Source) + "/" + strings.ToLower(p.Folder) + "@" + p.Version
	}
	sort.Strings(lines)
	h := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(h[:])
}
