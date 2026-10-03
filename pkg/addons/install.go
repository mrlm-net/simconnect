//go:build windows
// +build windows

// Package addons reads what is installed in Microsoft Flight Simulator:
// where the packages live, the Community and streamed packages, the
// package that holds the loaded aircraft, a fingerprint of the set and the
// running processes. It only reads files; no SimConnect connection is
// needed. Deciding what a package or process means (ATC, traffic, GSX…)
// is left to the caller.
package addons

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Install is one simulator installation found on this machine.
type Install struct {
	Sim      string // "2024" or "2020"
	Store    string // "steam" or "store"
	UserCfg  string // path of UserCfg.opt
	Packages string // InstalledPackagesPath from UserCfg.opt
}

// ErrNotFound: no UserCfg.opt with an InstalledPackagesPath was found.
var ErrNotFound = errors.New("addons: no simulator installation found")

// userCfgCandidates are the places UserCfg.opt lives. The 2024 Steam path
// was checked on a real install; the Store and 2020 paths are the usual
// ones and not checked here.
func userCfgCandidates() []Install {
	appData := os.Getenv("APPDATA")
	local := os.Getenv("LOCALAPPDATA")
	return []Install{
		{Sim: "2024", Store: "steam", UserCfg: filepath.Join(appData, "Microsoft Flight Simulator 2024", "UserCfg.opt")},
		{Sim: "2024", Store: "store", UserCfg: filepath.Join(local, "Packages", "Microsoft.Limitless_8wekyb3d8bbwe", "LocalCache", "UserCfg.opt")},
		{Sim: "2020", Store: "steam", UserCfg: filepath.Join(appData, "Microsoft Flight Simulator", "UserCfg.opt")},
		{Sim: "2020", Store: "store", UserCfg: filepath.Join(local, "Packages", "Microsoft.FlightSimulator_8wekyb3d8bbwe", "LocalCache", "UserCfg.opt")},
	}
}

// Installs returns every installation whose UserCfg.opt names a packages
// path, MSFS 2024 first.
func Installs() []Install {
	var out []Install
	for _, in := range userCfgCandidates() {
		p, err := ReadPackagesPath(in.UserCfg)
		if err != nil || p == "" {
			continue
		}
		in.Packages = p
		out = append(out, in)
	}
	return out
}

// Find returns the first installation (MSFS 2024 before 2020).
func Find() (Install, error) {
	if ins := Installs(); len(ins) > 0 {
		return ins[0], nil
	}
	return Install{}, ErrNotFound
}

// ReadPackagesPath reads InstalledPackagesPath from a UserCfg.opt, e.g.
// InstalledPackagesPath "C:\...\Packages". It returns "" when the line is
// missing.
func ReadPackagesPath(userCfg string) (string, error) {
	f, err := os.Open(userCfg)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "InstalledPackagesPath")
		if !ok {
			continue
		}
		return strings.Trim(strings.TrimSpace(rest), `"`), nil
	}
	return "", sc.Err()
}
