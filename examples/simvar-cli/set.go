//go:build windows
// +build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"time"
	"unsafe"

	"github.com/mrlm-net/cure/pkg/terminal"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type setCommand struct {
	engineOpts []engine.Option
	timeout    int
}

func (c *setCommand) Name() string        { return "set" }
func (c *setCommand) Description() string { return "Write a value to a SimVar in the simulator" }
func (c *setCommand) Usage() string {
	return "set <variable-name> <unit> <datatype> <value>\n\nExample: set \"CAMERA STATE\" \"\" int32 3"
}
func (c *setCommand) Flags() *flag.FlagSet { return nil }

func (c *setCommand) Run(ctx context.Context, tc *terminal.Context) error {
	if len(tc.Args) < 4 {
		return fmt.Errorf("usage: set <variable-name> <unit> <datatype> <value>")
	}

	varName := tc.Args[0]
	unit := tc.Args[1]
	dtStr := tc.Args[2]
	valStr := tc.Args[3]

	dt, err := parseDataType(dtStr)
	if err != nil {
		return err
	}

	parsed, err := parseValue(valStr, dt)
	if err != nil {
		return err
	}

	// Create client with engine options and context
	opts := append([]engine.Option{engine.WithContext(ctx)}, c.engineOpts...)
	client := engine.New("SimVar CLI - Set", opts...)

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

	// Wait for OPEN message to confirm connection is established
	stream := client.Stream()
	timer := time.NewTimer(time.Duration(c.timeout) * time.Second)
	defer timer.Stop()

	openReceived := false
	for !openReceived {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("timeout waiting for connection confirmation after %ds", c.timeout)
		case msg, ok := <-stream:
			if !ok {
				return fmt.Errorf("stream closed unexpectedly")
			}
			if msg.Err != nil {
				msg.Release()
				continue
			}
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_OPEN {
				openReceived = true
			}
			msg.Release()
		}
	}

	// Register definition and set data
	defID := nextDefID()
	if err := client.AddToDataDefinition(defID, varName, unit, dt, 0, 0); err != nil {
		return fmt.Errorf("AddToDataDefinition failed: %w", err)
	}
	defer client.ClearDataDefinition(defID)

	// Set data using the same pattern as set-variables example
	if err := setDataValue(client, defID, dt, parsed); err != nil {
		return err
	}

	// Wait briefly for any exception response
	exTimer := time.NewTimer(1 * time.Second)
	defer exTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exTimer.C:
			fmt.Fprintf(tc.Stdout, "OK\n")
			return nil
		case msg, ok := <-stream:
			if !ok {
				return fmt.Errorf("stream closed unexpectedly")
			}
			if msg.Err != nil {
				msg.Release()
				continue
			}
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EXCEPTION {
				exc := msg.AsException()
				if exc != nil {
					err := fmt.Errorf("SimConnect exception: ID=%d, SendID=%d, Index=%d",
						exc.DwException, exc.DwSendID, exc.DwIndex)
					msg.Release()
					return err
				}
			}
			msg.Release()
		}
	}
}

// setDataValue calls SetDataOnSimObject with the correctly typed value pointer.
func setDataValue(client engine.Client, defID uint32, dt types.SIMCONNECT_DATATYPE, parsed interface{}) error {
	switch dt {
	case types.SIMCONNECT_DATATYPE_INT32:
		value := parsed.(int32)
		return client.SetDataOnSimObject(defID, types.SIMCONNECT_OBJECT_ID_USER,
			types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 1,
			uint32(unsafe.Sizeof(value)), unsafe.Pointer(&value))
	case types.SIMCONNECT_DATATYPE_INT64:
		value := parsed.(int64)
		return client.SetDataOnSimObject(defID, types.SIMCONNECT_OBJECT_ID_USER,
			types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 1,
			uint32(unsafe.Sizeof(value)), unsafe.Pointer(&value))
	case types.SIMCONNECT_DATATYPE_FLOAT32:
		value := parsed.(float32)
		return client.SetDataOnSimObject(defID, types.SIMCONNECT_OBJECT_ID_USER,
			types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 1,
			uint32(unsafe.Sizeof(value)), unsafe.Pointer(&value))
	case types.SIMCONNECT_DATATYPE_FLOAT64:
		value := parsed.(float64)
		return client.SetDataOnSimObject(defID, types.SIMCONNECT_OBJECT_ID_USER,
			types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 1,
			uint32(unsafe.Sizeof(value)), unsafe.Pointer(&value))
	default:
		return fmt.Errorf("unsupported datatype for set: %d", dt)
	}
}
