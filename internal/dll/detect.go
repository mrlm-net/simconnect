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

// dllRelPath is the relative path from an SDK root to the DLL.
const dllRelPath = "SimConnect SDK/lib/SimConnect.dll"

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
}

// Detect searches for SimConnect.dll on the local filesystem.
// It checks environment variables first, then common installation paths,
// and finally the user's home directory. The first path where the file
// exists is returned. If no file is found, ErrDLLNotFound is returned.
func Detect() (string, error) {
	// 1. Environment variables
	for _, env := range envVars {
		if root := os.Getenv(env); root != "" {
			candidate := filepath.Join(root, dllRelPath)
			if fileExists(candidate) {
				return filepath.ToSlash(candidate), nil
			}
		}
	}

	// 2. Common installation paths
	for _, root := range commonRoots {
		candidate := filepath.Join(root, dllRelPath)
		if fileExists(candidate) {
			return filepath.ToSlash(candidate), nil
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
			candidate := filepath.Join(root, dllRelPath)
			if fileExists(candidate) {
				return filepath.ToSlash(candidate), nil
			}
		}
	}

	return "", ErrDLLNotFound
}

// fileExists reports whether the given path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
