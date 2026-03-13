//go:build windows
// +build windows

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Value wrapper structs for CastDataAs generic extraction.
type valueInt32 struct{ Value int32 }
type valueInt64 struct{ Value int64 }
type valueFloat32 struct{ Value float32 }
type valueFloat64 struct{ Value float64 }

// Atomic ID counters for definition and request IDs.
var (
	defIDCounter atomic.Uint32
	reqIDCounter atomic.Uint32
)

// nextDefID returns the next unique data definition ID.
func nextDefID() uint32 {
	return defIDCounter.Add(1)
}

// nextReqID returns the next unique data request ID.
func nextReqID() uint32 {
	return reqIDCounter.Add(1)
}

// parseDataType maps a string name to the corresponding SimConnect datatype constant.
func parseDataType(s string) (types.SIMCONNECT_DATATYPE, error) {
	switch s {
	case "int32":
		return types.SIMCONNECT_DATATYPE_INT32, nil
	case "int64":
		return types.SIMCONNECT_DATATYPE_INT64, nil
	case "float32":
		return types.SIMCONNECT_DATATYPE_FLOAT32, nil
	case "float64":
		return types.SIMCONNECT_DATATYPE_FLOAT64, nil
	default:
		return types.SIMCONNECT_DATATYPE_INVALID, fmt.Errorf("unsupported datatype %q (use int32, int64, float32, float64)", s)
	}
}

// dataTypeSize returns the byte size for a given SimConnect datatype.
func dataTypeSize(dt types.SIMCONNECT_DATATYPE) uint32 {
	switch dt {
	case types.SIMCONNECT_DATATYPE_INT32, types.SIMCONNECT_DATATYPE_FLOAT32:
		return 4
	case types.SIMCONNECT_DATATYPE_INT64, types.SIMCONNECT_DATATYPE_FLOAT64:
		return 8
	default:
		return 0
	}
}

// parseValue parses a string into the Go type matching the SimConnect datatype.
func parseValue(s string, dt types.SIMCONNECT_DATATYPE) (interface{}, error) {
	switch dt {
	case types.SIMCONNECT_DATATYPE_INT32:
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid int32 value %q: %w", s, err)
		}
		return int32(v), nil
	case types.SIMCONNECT_DATATYPE_INT64:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int64 value %q: %w", s, err)
		}
		return v, nil
	case types.SIMCONNECT_DATATYPE_FLOAT32:
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid float32 value %q: %w", s, err)
		}
		return float32(v), nil
	case types.SIMCONNECT_DATATYPE_FLOAT64:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float64 value %q: %w", s, err)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported datatype for parsing: %d", dt)
	}
}

// eventMapping tracks a mapped client event.
type eventMapping struct {
	eventID   uint32
	name      string
	listening bool
}

var (
	eventCache     sync.Map      // map[string]*eventMapping (uppercase name -> mapping)
	eventIDCounter atomic.Uint32 // allocates unique event IDs
)

const (
	listenGroupID       uint32 = 900_000_000
	listenGroupPriority uint32 = 1
	emitGroupID         uint32 = 900_000_001
)

// nextEventID returns the next unique event ID.
func nextEventID() uint32 {
	return eventIDCounter.Add(1)
}

// getOrMapEvent returns the eventMapping for the given event name,
// calling MapClientEventToSimEvent if first time seen.
func getOrMapEvent(client engine.Client, name string) (*eventMapping, error) {
	key := strings.ToUpper(name)
	if val, ok := eventCache.Load(key); ok {
		return val.(*eventMapping), nil
	}
	id := nextEventID()
	if err := client.MapClientEventToSimEvent(id, key); err != nil {
		return nil, fmt.Errorf("MapClientEventToSimEvent(%q): %w", key, err)
	}
	m := &eventMapping{eventID: id, name: key}
	actual, _ := eventCache.LoadOrStore(key, m)
	return actual.(*eventMapping), nil
}

// resolveEventName finds event name by ID (reverse lookup).
func resolveEventName(eventID uint32) string {
	var found string
	eventCache.Range(func(key, value any) bool {
		m := value.(*eventMapping)
		if m.eventID == eventID {
			found = m.name
			return false
		}
		return true
	})
	return found
}

// parseEventData parses data values for emit. Accepts signed integers (int32 range), casts to uint32.
func parseEventData(args []string) ([]uint32, error) {
	result := make([]uint32, len(args))
	for i, s := range args {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid data value %q at position %d: %w", s, i, err)
		}
		result[i] = uint32(int32(v))
	}
	return result, nil
}

// formatValue uses CastDataAs to extract and format the typed value from a SimObject data response.
func formatValue(dwData *types.DWORD, dt types.SIMCONNECT_DATATYPE) string {
	switch dt {
	case types.SIMCONNECT_DATATYPE_INT32:
		v := engine.CastDataAs[valueInt32](dwData)
		return fmt.Sprintf("%d", v.Value)
	case types.SIMCONNECT_DATATYPE_INT64:
		v := engine.CastDataAs[valueInt64](dwData)
		return fmt.Sprintf("%d", v.Value)
	case types.SIMCONNECT_DATATYPE_FLOAT32:
		v := engine.CastDataAs[valueFloat32](dwData)
		return fmt.Sprintf("%g", v.Value)
	case types.SIMCONNECT_DATATYPE_FLOAT64:
		v := engine.CastDataAs[valueFloat64](dwData)
		return fmt.Sprintf("%g", v.Value)
	default:
		return "<unsupported type>"
	}
}

// ── Output formatting ────────────────────────────────────────────────────────

// OutputFormat selects the rendering style for FormatOutput.
// String alias chosen over iota: format values originate as strings from
// CLI flags and TOML config files — no parse indirection required.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
)

// parseOutputFormat validates and normalises a raw string into an OutputFormat.
// Accepts "table", "json", "csv" (case-insensitive).
// Returns FormatTable and an error for any unrecognised value.
func parseOutputFormat(s string) (OutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "table":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "csv":
		return FormatCSV, nil
	default:
		return FormatTable, fmt.Errorf("unknown output format %q: must be table, json, or csv", s)
	}
}

// FormattedValue is the data transfer object for FormatOutput.
// It carries one SimVar reading with all context needed to render in any format.
type FormattedValue struct {
	Name      string    // SimVar name as supplied by the caller
	Value     string    // Pre-formatted value string (from formatValue)
	Unit      string    // Unit string as supplied by the caller
	DataType  string    // Datatype label ("int32", "float64", etc.)
	Timestamp time.Time // Time of value receipt
}

// FormatOutput renders fv to w in the requested format.
//
// table: four labelled lines — Name, Value, DataType, Time — with a
//
//	fixed 9-character label column for alignment.
//
// json:  single JSON object with RFC3339Nano timestamp, followed by a newline.
//
//	Each call writes exactly one complete, parseable JSON object (NDJSON).
//
// csv:   single data row (no header). Call FormatCSVHeader once before the
//
//	first FormatOutput(csv) call to write the header row.
//
// Returns a non-nil error for an unknown format or any write failure.
func FormatOutput(w io.Writer, fv FormattedValue, format OutputFormat) error {
	switch format {
	case FormatTable:
		_, err := fmt.Fprintf(w, "%-9s %s\n%-9s %s %s\n%-9s %s\n%-9s %s\n",
			"Name:", fv.Name,
			"Value:", fv.Value, fv.Unit,
			"DataType:", fv.DataType,
			"Time:", fv.Timestamp.Format(time.RFC3339Nano),
		)
		return err
	case FormatJSON:
		obj := struct {
			Name      string `json:"name"`
			Value     string `json:"value"`
			Unit      string `json:"unit"`
			DataType  string `json:"datatype"`
			Timestamp string `json:"timestamp"`
		}{
			Name:      fv.Name,
			Value:     fv.Value,
			Unit:      fv.Unit,
			DataType:  fv.DataType,
			Timestamp: fv.Timestamp.Format(time.RFC3339Nano),
		}
		b, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("FormatOutput json: %w", err)
		}
		_, err = fmt.Fprintf(w, "%s\n", b)
		return err
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{
			fv.Name,
			fv.Value,
			fv.Unit,
			fv.DataType,
			fv.Timestamp.Format(time.RFC3339Nano),
		}); err != nil {
			return fmt.Errorf("FormatOutput csv: %w", err)
		}
		cw.Flush()
		return cw.Error()
	default:
		return fmt.Errorf("FormatOutput: unknown format %q", format)
	}
}

// FormatCSVHeader writes the CSV header row to w.
// Call this exactly once before the first FormatOutput(FormatCSV, ...) call.
func FormatCSVHeader(w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"name", "value", "unit", "datatype", "timestamp"}); err != nil {
		return fmt.Errorf("FormatCSVHeader: %w", err)
	}
	cw.Flush()
	return cw.Error()
}
