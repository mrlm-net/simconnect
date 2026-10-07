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
	if err := connectWithRetry(ctx, client, tc.Stderr); err != nil {
		return err
	}
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

	// Transmit the event (at the highest priority: no group to set up)
	if err := transmitEvent(client, mapping.eventID, dataValues); err != nil {
		return err
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
