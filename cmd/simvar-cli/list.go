//go:build windows
// +build windows

package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/mrlm-net/cure/pkg/terminal"
	"github.com/mrlm-net/simconnect/pkg/registry"
)

type listCommand struct {
	format OutputFormat
}

func (c *listCommand) Name() string        { return "list" }
func (c *listCommand) Description() string { return "List SimVar entries from the registry (no simulator connection required)" }
func (c *listCommand) Usage() string {
	return "list [--category <name>] [--search <text>]\n\n" +
		"Examples:\n" +
		"  list\n" +
		"  list --category navigation\n" +
		"  list --category autopilot --search lock\n" +
		"  list --search altitude"
}

func (c *listCommand) Flags() *flag.FlagSet {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.String("category", "", "Filter by category (e.g. aircraft, navigation, autopilot, environment, simulator)")
	fs.String("search", "", "Case-insensitive substring match on Name and Description")
	return fs
}

func (c *listCommand) Run(ctx context.Context, tc *terminal.Context) error {
	// Re-parse flags from tc.Args (CURE pre-strips global flags; tc.Args = subcommand args).
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	category := fs.String("category", "", "")
	search := fs.String("search", "", "")
	if err := fs.Parse(tc.Args); err != nil {
		return err
	}

	// Fetch entries — ByCategory when a category is given, otherwise all.
	var entries []registry.SimVarMeta
	if *category != "" {
		entries = registry.ByCategory(*category)
	} else {
		entries = registry.All()
	}

	// Apply search filter (case-insensitive substring on Name and Description).
	if *search != "" {
		q := strings.ToLower(*search)
		filtered := entries[:0]
		for _, sv := range entries {
			if strings.Contains(strings.ToLower(sv.Name), q) ||
				strings.Contains(strings.ToLower(sv.Description), q) {
				filtered = append(filtered, sv)
			}
		}
		entries = filtered
	}

	// Render output based on the global --format flag.
	switch c.format {
	case FormatJSON:
		return renderListJSON(tc, entries)
	case FormatCSV:
		return renderListCSV(tc, entries)
	default:
		return renderListTable(tc, entries)
	}
}

// renderListTable writes a 5-column aligned table to tc.Stdout.
func renderListTable(tc *terminal.Context, entries []registry.SimVarMeta) error {
	const (
		colName    = "NAME"
		colCat     = "CATEGORY"
		colType    = "TYPE"
		colUnit    = "DEFAULT UNIT"
		colWrite   = "WRITABLE"
		headerLine = "%-48s  %-12s  %-9s  %-18s  %s\n"
		dataLine   = "%-48s  %-12s  %-9s  %-18s  %v\n"
	)

	fmt.Fprintf(tc.Stdout, headerLine, colName, colCat, colType, colUnit, colWrite)
	fmt.Fprintf(tc.Stdout, "%s\n", strings.Repeat("-", 100))
	for _, sv := range entries {
		fmt.Fprintf(tc.Stdout, dataLine, sv.Name, sv.Category, sv.Type, sv.DefaultUnit, sv.Writable)
	}
	fmt.Fprintf(tc.Stdout, "\n%d entries\n", len(entries))
	return nil
}

// renderListJSON writes one JSON object per line (NDJSON) to tc.Stdout.
func renderListJSON(tc *terminal.Context, entries []registry.SimVarMeta) error {
	type jsonEntry struct {
		Name        string `json:"name"`
		Category    string `json:"category"`
		Type        string `json:"type"`
		DefaultUnit string `json:"default_unit"`
		Units       []string `json:"units"`
		Writable    bool   `json:"writable"`
		Indexed     bool   `json:"indexed"`
		Description string `json:"description"`
	}

	for _, sv := range entries {
		b, err := json.Marshal(jsonEntry{
			Name:        sv.Name,
			Category:    sv.Category,
			Type:        sv.Type,
			DefaultUnit: sv.DefaultUnit,
			Units:       sv.Units,
			Writable:    sv.Writable,
			Indexed:     sv.Indexed,
			Description: sv.Description,
		})
		if err != nil {
			return fmt.Errorf("list json: %w", err)
		}
		fmt.Fprintf(tc.Stdout, "%s\n", b)
	}
	return nil
}

// renderListCSV writes a header row followed by one data row per entry.
func renderListCSV(tc *terminal.Context, entries []registry.SimVarMeta) error {
	cw := csv.NewWriter(tc.Stdout)
	if err := cw.Write([]string{"name", "category", "type", "default_unit", "writable", "indexed", "description"}); err != nil {
		return fmt.Errorf("list csv header: %w", err)
	}
	for _, sv := range entries {
		writable := "false"
		if sv.Writable {
			writable = "true"
		}
		indexed := "false"
		if sv.Indexed {
			indexed = "true"
		}
		if err := cw.Write([]string{sv.Name, sv.Category, sv.Type, sv.DefaultUnit, writable, indexed, sv.Description}); err != nil {
			return fmt.Errorf("list csv row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}
