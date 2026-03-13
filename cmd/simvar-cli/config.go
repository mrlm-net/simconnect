//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds values loaded from the JSON config file.
// Zero values mean "not set by config" — caller applies its own defaults.
type Config struct {
	DLLPath    string `json:"dll_path"`
	AutoDetect bool   `json:"auto_detect"`
	Timeout    int    `json:"timeout"`
	LogLevel   string `json:"log_level"`
	Format     string `json:"format"`
}

// loadConfig resolves the config file using the lookup order below,
// decodes the JSON, and returns the populated Config.
//
// Lookup order (first found and readable wins):
//  1. explicit — the --config flag value (non-empty); missing = error
//  2. SIMVAR_CLI_CONFIG env var; missing file = error
//  3. %APPDATA%\simvar-cli\config.json; missing = silently ignored
//  4. .\simvar-cli.json in working dir; missing = silently ignored
//
// If no candidate file exists, returns zero Config and nil error.
func loadConfig(explicit string) (Config, error) {
	type candidate struct {
		path      string
		mustExist bool
	}

	var candidates []candidate

	if explicit != "" {
		candidates = append(candidates, candidate{explicit, true})
	} else {
		if env := os.Getenv("SIMVAR_CLI_CONFIG"); env != "" {
			candidates = append(candidates, candidate{env, true})
		}
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			candidates = append(candidates, candidate{filepath.Join(appdata, "simvar-cli", "config.json"), false})
		}
		candidates = append(candidates, candidate{"simvar-cli.json", false})
	}

	for _, c := range candidates {
		_, err := os.Stat(c.path)
		if err != nil {
			if c.mustExist {
				return Config{}, fmt.Errorf("config: file not found: %q", c.path)
			}
			continue
		}
		f, err := os.Open(c.path)
		if err != nil {
			return Config{}, fmt.Errorf("config: open %q: %w", c.path, err)
		}
		var cfg Config
		decErr := json.NewDecoder(f).Decode(&cfg)
		f.Close()
		if decErr != nil {
			return Config{}, fmt.Errorf("config: decode %q: %w", c.path, decErr)
		}
		return cfg, nil
	}
	return Config{}, nil
}
