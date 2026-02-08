//go:build windows
// +build windows

package dll

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrDLLNotFound is returned when no SimConnect.dll can be located.
var ErrDLLNotFound = errors.New("simconnect: SimConnect.dll not found in any known location")

// dllRelPaths lists relative paths from an SDK root to the DLL,
// checked in order. Different SDK versions may use different layouts.
var dllRelPaths = []string{
	"SimConnect SDK/lib/SimConnect.dll",
	"lib/SimConnect.dll",
}

// envVars lists environment variables that may point to an SDK root,
// checked in priority order.
var envVars = []string{
	"MSFS_SDK",
	"MSFS2024_SDK",
	"MSFS2020_SDK",
}

// commonRoots lists well-known SDK installation directories to probe.
var commonRoots = []string{
	"C:/MSFS 2024 SDK",
	"C:/MSFS SDK",
	"C:/MSFS 2020 SDK",
	"C:/Program Files/MSFS 2024 SDK",
	"C:/Program Files/MSFS SDK",
	"C:/Program Files/MSFS 2020 SDK",
	"C:/Program Files (x86)/MSFS 2024 SDK",
	"C:/Program Files (x86)/MSFS SDK",
	"C:/Program Files (x86)/MSFS 2020 SDK",
}

// Detect searches for SimConnect.dll on the local filesystem.
// It checks SIMCONNECT_DLL for a direct path first, then SDK root
// environment variables, common installation paths, and finally the
// user's home directory. The first path where the file exists is returned.
// If no file is found, ErrDLLNotFound is returned.
func Detect() (string, error) {
	// 0. Direct DLL path via SIMCONNECT_DLL env var
	if direct := os.Getenv("SIMCONNECT_DLL"); direct != "" {
		if fileExists(direct) {
			return filepath.ToSlash(direct), nil
		}
	}

	// 1. SDK root environment variables
	for _, env := range envVars {
		if root := os.Getenv(env); root != "" {
			if found, ok := checkRoot(root); ok {
				return found, nil
			}
		}
	}

	// 2. Common installation paths
	for _, root := range commonRoots {
		if found, ok := checkRoot(root); ok {
			return found, nil
		}
	}

	// 3. User home directory paths
	if home, err := os.UserHomeDir(); err == nil {
		homeRoots := []string{
			filepath.Join(home, "MSFS 2024 SDK"),
			filepath.Join(home, "MSFS SDK"),
			filepath.Join(home, "MSFS 2020 SDK"),
		}
		for _, root := range homeRoots {
			if found, ok := checkRoot(root); ok {
				return found, nil
			}
		}
	}

	return "", ErrDLLNotFound
}

// checkRoot tries each known relative DLL path under the given root directory.
// Returns the slash-normalised path and true if found.
func checkRoot(root string) (string, bool) {
	for _, rel := range dllRelPaths {
		candidate := filepath.Join(root, rel)
		if fileExists(candidate) {
			return filepath.ToSlash(candidate), true
		}
	}
	return "", false
}

// fileExists reports whether the given path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
