//go:build windows
// +build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mrlm-net/cure/pkg/terminal"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

func main() {
	// Parse global flags before subcommand dispatch
	var (
		dllPath    string
		autoDetect bool
		logLevel   string
		timeout    int
		format     string
		configPath string
	)

	fs := flag.NewFlagSet("simvar-cli", flag.ContinueOnError)
	fs.StringVar(&dllPath, "dll-path", "", "Path to SimConnect.dll")
	fs.BoolVar(&autoDetect, "auto-detect", false, "Auto-detect SimConnect.dll location")
	// Defaults intentionally empty — handled by config + hard-default block below
	fs.StringVar(&logLevel, "log-level", "", "Log level (debug, info, warn, error)")
	fs.IntVar(&timeout, "timeout", 0, "Timeout in seconds for operations")
	fs.StringVar(&format, "format", "", "Output format: table, json, csv (default: table)")
	fs.StringVar(&configPath, "config", "", "Path to config file (TOML)")

	// Find the subcommand position (first non-flag argument)
	args := os.Args[1:]
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	remaining := fs.Args()

	// Load config file
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Apply config values for flags not explicitly set by the user.
	// flag.Visit iterates only flags the user explicitly provided.
	userSet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { userSet[f.Name] = true })

	if !userSet["dll-path"] && cfg.DLLPath != "" {
		dllPath = cfg.DLLPath
	}
	if !userSet["auto-detect"] && cfg.AutoDetect {
		autoDetect = cfg.AutoDetect
	}
	if !userSet["log-level"] && cfg.LogLevel != "" {
		logLevel = cfg.LogLevel
	}
	if !userSet["timeout"] && cfg.Timeout != 0 {
		timeout = cfg.Timeout
	}
	if !userSet["format"] && cfg.Format != "" {
		format = cfg.Format
	}

	// Apply hard defaults for anything still unset
	if logLevel == "" {
		logLevel = "warn"
	}
	if timeout == 0 {
		timeout = 10
	}
	if format == "" {
		format = string(FormatTable)
	}

	// Validate format before connecting to simulator
	outputFormat, err := parseOutputFormat(format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Build engine options from resolved global flags
	var engineOpts []engine.Option
	if dllPath != "" {
		engineOpts = append(engineOpts, engine.WithDLLPath(dllPath))
	}
	if autoDetect {
		engineOpts = append(engineOpts, engine.WithAutoDetect())
	}
	if logLevel != "" {
		engineOpts = append(engineOpts, engine.WithLogLevelFromString(logLevel))
	}

	// Create CURE router with signal handling
	router := terminal.New(
		terminal.WithStdout(os.Stdout),
		terminal.WithStderr(os.Stderr),
		terminal.WithSignalHandler(),
	)

	// Register commands
	router.Register(&getCommand{engineOpts: engineOpts, timeout: timeout})
	router.Register(&setCommand{engineOpts: engineOpts, timeout: timeout})
	router.Register(&emitCommand{engineOpts: engineOpts, timeout: timeout})
	router.Register(&listenCommand{engineOpts: engineOpts, timeout: timeout})
	router.Register(&replCommand{engineOpts: engineOpts, timeout: timeout})
	router.Register(&watchCommand{engineOpts: engineOpts, timeout: timeout, format: outputFormat})

	// Setup signal handler context
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		cancel()
	}()

	// Default to REPL mode when no subcommand is given
	if len(remaining) == 0 {
		remaining = []string{"repl"}
	}

	if err := router.RunContext(ctx, remaining); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
