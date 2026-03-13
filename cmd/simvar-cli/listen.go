//go:build windows
// +build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/mrlm-net/cure/pkg/terminal"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type listenCommand struct {
	engineOpts []engine.Option
	timeout    int
}

func (c *listenCommand) Name() string        { return "listen" }
func (c *listenCommand) Description() string { return "Monitor client events from the simulator" }
func (c *listenCommand) Usage() string {
	return "listen <event-name> [event-name...]\n\nExamples:\n  listen GEAR_TOGGLE\n  listen AP_MASTER GEAR_TOGGLE"
}
func (c *listenCommand) Flags() *flag.FlagSet { return nil }

func (c *listenCommand) Run(ctx context.Context, tc *terminal.Context) error {
	if len(tc.Args) < 1 {
		return fmt.Errorf("usage: listen <event-name> [event-name...]")
	}

	eventNames := tc.Args

	// Create client with engine options and context
	opts := append([]engine.Option{engine.WithContext(ctx)}, c.engineOpts...)
	client := engine.New("SimVar CLI - Listen", opts...)

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
	// Build reverse map for event ID -> name lookup
	reverseMap := make(map[uint32]string)

	// Map each event and add to notification group
	for _, name := range eventNames {
		mapping, err := getOrMapEvent(client, name)
		if err != nil {
			return err
		}
		if err := client.AddClientEventToNotificationGroup(listenGroupID, mapping.eventID, false); err != nil {
			return fmt.Errorf("AddClientEventToNotificationGroup(%s): %w", mapping.name, err)
		}
		reverseMap[mapping.eventID] = mapping.name
	}

	// Set notification group priority once
	if err := client.SetNotificationGroupPriority(listenGroupID, listenGroupPriority); err != nil {
		return fmt.Errorf("SetNotificationGroupPriority: %w", err)
	}

	// Collect names for display
	names := make([]string, 0, len(eventNames))
	for _, name := range eventNames {
		names = append(names, strings.ToUpper(name))
	}
	fmt.Fprintf(tc.Stderr, "Listening for: %s (Ctrl+C to stop)\n", strings.Join(names, ", "))

	// Stream loop
	for {
		select {
		case <-ctx.Done():
			// Cleanup notification group before exiting
			client.ClearNotificationGroup(listenGroupID)
			return nil
		case msg, ok := <-stream:
			if !ok {
				client.ClearNotificationGroup(listenGroupID)
				return nil
			}
			if msg.Err != nil {
				msg.Release()
				continue
			}

			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EVENT:
				evt := msg.AsEvent()
				if evt != nil {
					eid := uint32(evt.UEventID)
					if name, ok := reverseMap[eid]; ok {
						ts := time.Now().Format(time.RFC3339)
						fmt.Fprintf(tc.Stdout, "[%s] %s data=%d\n", ts, name, uint32(evt.DwData))
					}
				}
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				exc := msg.AsException()
				if exc != nil {
					fmt.Fprintf(tc.Stderr, "Exception: ID=%d, SendID=%d, Index=%d\n",
						exc.DwException, exc.DwSendID, exc.DwIndex)
				}
			}

			msg.Release()
		}
	}
}
