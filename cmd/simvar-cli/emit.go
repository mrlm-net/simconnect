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

type emitCommand struct {
	engineOpts []engine.Option
	timeout    int
}

func (c *emitCommand) Name() string        { return "emit" }
func (c *emitCommand) Description() string { return "Fire a client event in the simulator" }
func (c *emitCommand) Usage() string {
	return "emit <event-name> [data...]\n\nExamples:\n  emit AP_MASTER\n  emit TOGGLE_AIRCRAFT_EXIT 3\n  emit AXIS_ELEVATOR_SET -8000\n  emit SOME_EVENT 1 2 3 4 5"
}
func (c *emitCommand) Flags() *flag.FlagSet { return nil }

func (c *emitCommand) Run(ctx context.Context, tc *terminal.Context) error {
	if len(tc.Args) < 1 {
		return fmt.Errorf("usage: emit <event-name> [data...]")
	}

	eventName := tc.Args[0]
	dataArgs := tc.Args[1:]

	if len(dataArgs) > 5 {
		return fmt.Errorf("too many data values (max 5, got %d)", len(dataArgs))
	}

	dataValues, err := parseEventData(dataArgs)
	if err != nil {
		return err
	}

	// Create client with engine options and context
	opts := append([]engine.Option{engine.WithContext(ctx)}, c.engineOpts...)
	client := engine.New("SimVar CLI - Emit", opts...)

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

	// Wait for OPEN message to confirm connection
	stream := client.Stream()
	timer := time.NewTimer(time.Duration(c.timeout) * time.Second)
	defer timer.Stop()

	for {
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
				msg.Release()
				goto ready
			}
			msg.Release()
		}
	}

ready:
	// Map the event
	mapping, err := getOrMapEvent(client, eventName)
	if err != nil {
		return err
	}

	// Setup notification group for emit
	if err := client.AddClientEventToNotificationGroup(emitGroupID, mapping.eventID, false); err != nil {
		return fmt.Errorf("AddClientEventToNotificationGroup: %w", err)
	}
	if err := client.SetNotificationGroupPriority(emitGroupID, 1); err != nil {
		return fmt.Errorf("SetNotificationGroupPriority: %w", err)
	}

	// Transmit the event
	if len(dataValues) <= 1 {
		var data uint32
		if len(dataValues) == 1 {
			data = dataValues[0]
		}
		if err := client.TransmitClientEvent(
			types.SIMCONNECT_OBJECT_ID_USER,
			mapping.eventID,
			data,
			emitGroupID,
			types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY,
		); err != nil {
			return fmt.Errorf("TransmitClientEvent: %w", err)
		}
	} else {
		var dataArray [5]uint32
		for i, v := range dataValues {
			dataArray[i] = v
		}
		if err := client.TransmitClientEventEx1(
			types.SIMCONNECT_OBJECT_ID_USER,
			mapping.eventID,
			emitGroupID,
			types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY,
			dataArray,
		); err != nil {
			return fmt.Errorf("TransmitClientEventEx1: %w", err)
		}
	}

	// Wait briefly for exception response
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
