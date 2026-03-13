//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds values loaded from the TOML config file.
// Zero values mean "not set by config" — caller applies its own defaults.
type Config struct {
	DLLPath    string `toml:"dll_path"`
	AutoDetect bool   `toml:"auto_detect"`
	Timeout    int    `toml:"timeout"`
	LogLevel   string `toml:"log_level"`
	Format     string `toml:"format"`
}

// loadConfig resolves the config file using the lookup order below,
// decodes the TOML, and returns the populated Config.
//
// Lookup order (first found and readable wins):
//  1. explicit — the --config flag value (non-empty); missing = error
//  2. SIMVAR_CLI_CONFIG env var; missing file = error
//  3. %APPDATA%\simvar-cli\config.toml; missing = silently ignored
//  4. .\simvar-cli.toml in working dir; missing = silently ignored
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
			candidates = append(candidates, candidate{filepath.Join(appdata, "simvar-cli", "config.toml"), false})
		}
		candidates = append(candidates, candidate{"simvar-cli.toml", false})
	}

	for _, c := range candidates {
		_, err := os.Stat(c.path)
		if err != nil {
			if c.mustExist {
				return Config{}, fmt.Errorf("config: file not found: %q", c.path)
			}
			continue
		}
		var cfg Config
		if _, err := toml.DecodeFile(c.path, &cfg); err != nil {
			return Config{}, fmt.Errorf("config: decode %q: %w", c.path, err)
		}
		return cfg, nil
	}
	return Config{}, nil
}
