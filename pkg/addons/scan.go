//go:build windows
// +build windows

package addons

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source says which folder a package was found in.
type Source string

const (
	Community     Source = "Community"
	Community2024 Source = "Community2024"
	Official      Source = "Official"         // Official2020/Official2024, <store>/<package>
	Streamed      Source = "StreamedPackages" // no manifest, only content/*.fsarchive
)

// Package is one package folder.
//
// For Community and Official packages the fields come from manifest.json.
// ContentType is kept as written: it is not reliable (GSX Pro and
// navigraph-nav-base both say SCENERY).
//
// Streamed packages have no manifest. Publisher and ICAO come from the
// folder name when it reads as an airport (see ParseStreamedName). What a
// streamed folder means was looked at on one MSFS 2024 Steam install (1,248
// folders): each holds content/minimal.fsarchive, a small stub, and 788
// hold further .fsarchive files (Cached > 0), downloaded content kept on
// disk. Stock and marketplace items appear alike, so a folder shows that
// the sim knows the package, not that it is owned or active.
type Package struct {
	Source      Source
	Folder      string // folder name, e.g. "fsdreamteam-gsx-pro"
	Path        string // full path of the folder
	Title       string
	Creator     string
	ContentType string
	Version     string // package_version

	Publisher string // streamed airports: e.g. "orbx"
	ICAO      string // streamed airports: e.g. "LKPR"
	Cached    int    // streamed: .fsarchive files besides minimal.fsarchive
}

type manifest struct {
	Title          string `json:"title"`
	Creator        string `json:"creator"`
	ContentType    string `json:"content_type"`
	PackageVersion string `json:"package_version"`
}

// Scan reads every package under a packages path (Install.Packages):
// Community, Community2024, Official2024/Official2020 and StreamedPackages.
// Missing folders are skipped; a package with an unreadable manifest is kept
// with only its folder name. The result is sorted by source and folder.
func Scan(packages string) ([]Package, error) {
	if _, err := os.Stat(packages); err != nil {
		return nil, err
	}
	var out []Package
	for _, src := range []Source{Community, Community2024} {
		out = append(out, scanManifests(filepath.Join(packages, string(src)), src)...)
	}
	for _, off := range []string{"Official2024", "Official2020"} {
		stores, _ := os.ReadDir(filepath.Join(packages, off))
		for _, st := range stores {
			if st.IsDir() {
				out = append(out, scanManifests(filepath.Join(packages, off, st.Name()), Official)...)
			}
		}
	}
	out = append(out, scanStreamed(filepath.Join(packages, string(Streamed)))...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Folder < out[j].Folder
	})
	return out, nil
}

func scanManifests(dir string, src Source) []Package {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Package
	for _, e := range ents {
		if !isDir(dir, e) {
			continue
		}
		p := Package{Source: src, Folder: e.Name(), Path: filepath.Join(dir, e.Name())}
		if b, err := os.ReadFile(filepath.Join(p.Path, "manifest.json")); err == nil {
			var m manifest
			if json.Unmarshal(b, &m) == nil {
				p.Title, p.Creator, p.ContentType, p.Version = m.Title, m.Creator, m.ContentType, m.PackageVersion
			}
		}
		out = append(out, p)
	}
	return out
}

func scanStreamed(dir string) []Package {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Package
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := Package{Source: Streamed, Folder: e.Name(), Path: filepath.Join(dir, e.Name())}
		p.Publisher, p.ICAO, _ = ParseStreamedName(e.Name())
		filepath.WalkDir(filepath.Join(p.Path, "content"), func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".fsarchive") &&
				!strings.EqualFold(d.Name(), "minimal.fsarchive") {
				p.Cached++
			}
			return nil
		})
		out = append(out, p)
	}
	return out
}

// isDir: a directory, or a link or junction to one (Community packages are
// often linked in from elsewhere).
func isDir(dir string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return false
	}
	fi, err := os.Stat(filepath.Join(dir, e.Name()))
	return err == nil && fi.IsDir()
}

// ParseStreamedName reads a streamed folder name as an airport:
//
//	fs20-orbx-airport-lkpr-prague             → orbx, LKPR
//	fs20-gaya-simulations-airport-loww-vienna → gaya-simulations, LOWW
//	fs24-asobo-airport-c53-lowerloon          → asobo, C53
//	fs20-fps-lkmt-ostrava-airport             → fps, LKMT
//
// The code must be 3–4 letters and digits with a letter first. ok is false
// for names that do not read as an airport (aircraft, liveries, "voloport").
func ParseStreamedName(name string) (publisher, icao string, ok bool) {
	t := strings.Split(strings.ToLower(name), "-")
	if len(t) > 0 && (t[0] == "fs20" || t[0] == "fs24") {
		t = t[1:]
	}
	for i, w := range t {
		if w == "airport" && i > 0 && i+1 < len(t) && isCode(t[i+1]) {
			return strings.Join(t[:i], "-"), strings.ToUpper(t[i+1]), true
		}
	}
	if n := len(t); n >= 3 && t[n-1] == "airport" && len(t[1]) == 4 && isCode(t[1]) {
		return t[0], strings.ToUpper(t[1]), true
	}
	return "", "", false
}

func isCode(s string) bool {
	if len(s) < 3 || len(s) > 4 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}
