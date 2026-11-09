//go:build windows

package world

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/addons"
)

// packageStamp is the installed packages' stamp (simCache): every package
// folder in Community, Community2024 and the Official folders, with its
// manifest's size and time, which an install, removal or update changes.
// Streamed packages do not count (they come and go as they are cached).
// Only the folders and one file each are looked at, no manifest read; ""
// when the simulator's packages are not found.
func packageStamp() string {
	in, err := addons.Find()
	if err != nil || in.Packages == "" {
		return ""
	}
	var lines []string
	add := func(dir, src string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			line := src + "/" + strings.ToLower(e.Name())
			if fi, err := os.Stat(filepath.Join(dir, e.Name(), "manifest.json")); err == nil {
				line += fmt.Sprintf("@%d@%d", fi.Size(), fi.ModTime().UnixNano())
			}
			lines = append(lines, line)
		}
	}
	add(filepath.Join(in.Packages, "Community"), "Community")
	add(filepath.Join(in.Packages, "Community2024"), "Community2024")
	for _, off := range []string{"Official2024", "Official2020"} {
		stores, _ := os.ReadDir(filepath.Join(in.Packages, off))
		for _, st := range stores {
			if st.IsDir() {
				add(filepath.Join(in.Packages, off, st.Name()), off+"/"+st.Name())
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	sort.Strings(lines)
	h := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(h[:])
}
