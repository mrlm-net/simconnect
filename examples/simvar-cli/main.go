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
	)

	fs := flag.NewFlagSet("simvar-cli", flag.ContinueOnError)
	fs.StringVar(&dllPath, "dll-path", "", "Path to SimConnect.dll")
	fs.BoolVar(&autoDetect, "auto-detect", false, "Auto-detect SimConnect.dll location")
	fs.StringVar(&logLevel, "log-level", "warn", "Log level (debug, info, warn, error)")
	fs.IntVar(&timeout, "timeout", 10, "Timeout in seconds for operations")

	// Find the subcommand position (first non-flag argument)
	args := os.Args[1:]
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	remaining := fs.Args()

	// Build engine options from global flags
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
