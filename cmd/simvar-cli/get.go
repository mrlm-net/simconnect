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

type getCommand struct {
	engineOpts []engine.Option
	timeout    int
}

func (c *getCommand) Name() string        { return "get" }
func (c *getCommand) Description() string { return "Read a SimVar value from the simulator" }
func (c *getCommand) Usage() string {
	return "get <variable-name> <unit> <datatype>\n\nExample: get \"PLANE ALTITUDE\" feet float64"
}
func (c *getCommand) Flags() *flag.FlagSet { return nil }

func (c *getCommand) Run(ctx context.Context, tc *terminal.Context) error {
	if len(tc.Args) < 3 {
		return fmt.Errorf("usage: get <variable-name> <unit> <datatype>")
	}

	varName := tc.Args[0]
	unit := tc.Args[1]
	dtStr := tc.Args[2]

	dt, err := parseDataType(dtStr)
	if err != nil {
		return err
	}

	// Create client with engine options and context
	opts := append([]engine.Option{engine.WithContext(ctx)}, c.engineOpts...)
	client := engine.New("SimVar CLI - Get", opts...)

	// Retry connection loop
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
		types.SIMCONNECT_PERIOD_ONCE,
		types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT,
		0, 0, 0,
	); err != nil {
		return fmt.Errorf("RequestDataOnSimObject failed: %w", err)
	}

	// Wait for response with timeout
	stream := client.Stream()
	timer := time.NewTimer(time.Duration(c.timeout) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("timeout waiting for response after %ds", c.timeout)
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
					result := formatValue(&data.DwData, dt)
					fmt.Fprintf(tc.Stdout, "%s\n", result)
					msg.Release()
					return nil
				}
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				exc := msg.AsException()
				if exc != nil {
					err := fmt.Errorf("SimConnect exception: ID=%d, SendID=%d, Index=%d",
						exc.DwException, exc.DwSendID, exc.DwIndex)
					msg.Release()
					return err
				}
			case types.SIMCONNECT_RECV_ID_OPEN:
				// Connection confirmed, continue waiting for data
			}

			msg.Release()
		}
	}
}
