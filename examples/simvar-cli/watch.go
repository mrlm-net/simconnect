//go:build windows
// +build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/mrlm-net/cure/pkg/terminal"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type watchCommand struct {
	engineOpts []engine.Option
	timeout    int
	format     OutputFormat
}

func (c *watchCommand) Name() string        { return "watch" }
func (c *watchCommand) Description() string { return "Stream a SimVar value continuously from the simulator" }
func (c *watchCommand) Usage() string {
	return "watch [--interval second|visual-frame|sim-frame] [--changed] <variable-name> <unit> <datatype>\n\n" +
		"Examples:\n" +
		"  watch \"PLANE ALTITUDE\" feet float64\n" +
		"  watch --interval visual-frame \"PLANE LATITUDE\" degrees float64\n" +
		"  watch --interval sim-frame --changed \"AUTOPILOT MASTER\" \"\" int32"
}

func (c *watchCommand) Flags() *flag.FlagSet {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.String("interval", "second", "Polling period: second, visual-frame, sim-frame")
	fs.Bool("changed", false, "Only output when value changes from previous reading")
	return fs
}

func (c *watchCommand) Run(ctx context.Context, tc *terminal.Context) error {
	// Parse local flags from tc.Args (CURE pre-parses and strips them; tc.Args = positional args only).
	// Re-parse here to access flag values.
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.String("interval", "second", "")
	changed := fs.Bool("changed", false, "")
	if err := fs.Parse(tc.Args); err != nil {
		return err
	}
	positional := fs.Args()
	if len(positional) < 3 {
		return fmt.Errorf("usage: %s", c.Usage())
	}

	varName := positional[0]
	unit := positional[1]
	dtStr := positional[2]

	dt, err := parseDataType(dtStr)
	if err != nil {
		return err
	}

	period, err := parsePeriod(*interval)
	if err != nil {
		return err
	}

	// Connect with retry loop (same pattern as get.go)
	opts := append([]engine.Option{engine.WithContext(ctx)}, c.engineOpts...)
	client := engine.New("SimVar CLI - Watch", opts...)

	fmt.Fprintf(tc.Stderr, "Connecting to simulator...\n")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := client.Connect(); err != nil {
				fmt.Fprintf(tc.Stderr, "Connection failed: %v, retrying in 2s...\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			goto connected
		}
	}

connected:
	defer client.Disconnect()

	defID := nextDefID()
	reqID := nextReqID()

	if err := client.AddToDataDefinition(defID, varName, unit, dt, 0, 0); err != nil {
		return fmt.Errorf("AddToDataDefinition failed: %w", err)
	}
	defer client.ClearDataDefinition(defID)

	if err := client.RequestDataOnSimObject(
		reqID, defID,
		types.SIMCONNECT_OBJECT_ID_USER,
		period,
		types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT,
		0, 0, 0,
	); err != nil {
		return fmt.Errorf("RequestDataOnSimObject failed: %w", err)
	}

	// Write CSV header once before streaming
	if c.format == FormatCSV {
		if err := FormatCSVHeader(tc.Stdout); err != nil {
			return err
		}
	}

	stream := client.Stream()
	lastValue := ""

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-stream:
			if !ok {
				return fmt.Errorf("stream closed unexpectedly")
			}
			if msg.Err != nil {
				msg.Release()
				fmt.Fprintf(tc.Stderr, "Stream error: %v\n", msg.Err)
				continue
			}

			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				data := msg.AsSimObjectData()
				if data != nil && uint32(data.DwRequestID) == reqID {
					raw := formatValue(&data.DwData, dt)
					if *changed && raw == lastValue {
						msg.Release()
						continue
					}
					lastValue = raw
					fv := FormattedValue{
						Name:      varName,
						Value:     raw,
						Unit:      unit,
						DataType:  dtStr,
						Timestamp: time.Now(),
					}
					if err := FormatOutput(tc.Stdout, fv, c.format); err != nil {
						msg.Release()
						return fmt.Errorf("FormatOutput: %w", err)
					}
				}
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				exc := msg.AsException()
				if exc != nil {
					fmt.Fprintf(tc.Stderr, "SimConnect exception: ID=%d, SendID=%d, Index=%d\n",
						exc.DwException, exc.DwSendID, exc.DwIndex)
				}
			case types.SIMCONNECT_RECV_ID_OPEN:
				// Connection confirmed
			}
			msg.Release()
		}
	}
}

// parsePeriod maps a watch --interval string to the SimConnect period constant.
func parsePeriod(s string) (types.SIMCONNECT_PERIOD, error) {
	switch s {
	case "second":
		return types.SIMCONNECT_PERIOD_SECOND, nil
	case "visual-frame":
		return types.SIMCONNECT_PERIOD_VISUAL_FRAME, nil
	case "sim-frame":
		return types.SIMCONNECT_PERIOD_SIM_FRAME, nil
	default:
		return types.SIMCONNECT_PERIOD_NEVER,
			fmt.Errorf("unknown interval %q: must be second, visual-frame, or sim-frame", s)
	}
}
