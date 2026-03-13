//go:build windows
// +build windows

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/cure/pkg/terminal"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// pendingRequest tracks an in-flight get request waiting for a response.
type pendingRequest struct {
	defID    uint32
	dataType types.SIMCONNECT_DATATYPE
	result   chan string
}

type replCommand struct {
	engineOpts []engine.Option
	timeout    int
}

func (c *replCommand) Name() string { return "repl" }
func (c *replCommand) Description() string {
	return "Interactive REPL mode for reading/writing SimVars"
}
func (c *replCommand) Usage() string {
	return "repl\n\nStarts an interactive session. Commands:\n  get <variable-name> <unit> <datatype>\n  set <variable-name> <unit> <datatype> <value>\n  emit <event-name> [data...]\n  listen <event-name> [event-name...]\n  unlisten <event-name>\n  listeners\n  help\n  exit | quit"
}
func (c *replCommand) Flags() *flag.FlagSet { return nil }

func (c *replCommand) Run(ctx context.Context, tc *terminal.Context) error {
	// Create client with engine options and context
	opts := append([]engine.Option{engine.WithContext(ctx)}, c.engineOpts...)
	client := engine.New("SimVar CLI - REPL", opts...)

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
	connTimer := time.NewTimer(time.Duration(c.timeout) * time.Second)
	defer connTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-connTimer.C:
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
	fmt.Fprintf(tc.Stderr, "Connected. Type 'help' for commands, 'exit' or 'quit' to leave.\n")

	// Pending requests map: reqID -> pendingRequest
	var pending sync.Map

	// Listened events map: eventID(uint32) -> eventName(string)
	var listenedEvents sync.Map
	var listenGroupSetup bool

	// Start stream consumer goroutine
	consumerCtx, consumerCancel := context.WithCancel(ctx)
	defer consumerCancel()

	go func() {
		for {
			select {
			case <-consumerCtx.Done():
				return
			case msg, ok := <-stream:
				if !ok {
					return
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
						if nameVal, ok := listenedEvents.Load(eid); ok {
							name := nameVal.(string)
							ts := time.Now().Format(time.RFC3339)
							fmt.Fprintf(tc.Stderr, "[%s] %s data=%d\n", ts, name, uint32(evt.DwData))
						}
					}
				case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
					data := msg.AsSimObjectData()
					if data != nil {
						rID := uint32(data.DwRequestID)
						if val, ok := pending.Load(rID); ok {
							pr := val.(*pendingRequest)
							result := formatValue(&data.DwData, pr.dataType)
							pr.result <- result
							pending.Delete(rID)
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
	}()

	// REPL input loop — use tc.Stdin if available, fall back to os.Stdin
	var stdinReader io.Reader = os.Stdin
	if tc.Stdin != nil {
		stdinReader = tc.Stdin
	}
	scanner := bufio.NewScanner(stdinReader)
	for {
		fmt.Fprintf(tc.Stdout, "simvar> ")

		// Check context before blocking on scan
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !scanner.Scan() {
			// EOF or error
			return scanner.Err()
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		tokens := tokenize(line)
		if len(tokens) == 0 {
			continue
		}

		cmd := strings.ToLower(tokens[0])
		switch cmd {
		case "exit", "quit":
			// Cleanup listen group before exiting
			client.ClearNotificationGroup(listenGroupID)
			return nil

		case "help":
			fmt.Fprintf(tc.Stderr, "Commands:\n")
			fmt.Fprintf(tc.Stderr, "  get <variable-name> <unit> <datatype>        Read a SimVar value\n")
			fmt.Fprintf(tc.Stderr, "  set <variable-name> <unit> <datatype> <value> Write a SimVar value\n")
			fmt.Fprintf(tc.Stderr, "  emit <event-name> [data...]                  Fire a client event\n")
			fmt.Fprintf(tc.Stderr, "  listen <event-name> [event-name...]           Subscribe to events\n")
			fmt.Fprintf(tc.Stderr, "  unlisten <event-name>                         Unsubscribe from event\n")
			fmt.Fprintf(tc.Stderr, "  listeners                                     Show active listeners\n")
			fmt.Fprintf(tc.Stderr, "  help                                          Show this help\n")
			fmt.Fprintf(tc.Stderr, "  exit | quit                                   End the session\n")

		case "get":
			if len(tokens) < 4 {
				fmt.Fprintf(tc.Stderr, "Usage: get <variable-name> <unit> <datatype>\n")
				continue
			}
			if err := c.replGet(client, tc, &pending, tokens[1], tokens[2], tokens[3]); err != nil {
				fmt.Fprintf(tc.Stderr, "Error: %v\n", err)
			}

		case "set":
			if len(tokens) < 5 {
				fmt.Fprintf(tc.Stderr, "Usage: set <variable-name> <unit> <datatype> <value>\n")
				continue
			}
			if err := c.replSet(client, tc, tokens[1], tokens[2], tokens[3], tokens[4]); err != nil {
				fmt.Fprintf(tc.Stderr, "Error: %v\n", err)
			}

		case "emit":
			if len(tokens) < 2 {
				fmt.Fprintf(tc.Stderr, "Usage: emit <event-name> [data...]\n")
				continue
			}
			if err := c.replEmit(client, tc, tokens[1], tokens[2:]); err != nil {
				fmt.Fprintf(tc.Stderr, "Error: %v\n", err)
			}

		case "listen":
			if len(tokens) < 2 {
				fmt.Fprintf(tc.Stderr, "Usage: listen <event-name> [event-name...]\n")
				continue
			}
			if err := c.replListen(client, tc, &listenedEvents, &listenGroupSetup, tokens[1:]); err != nil {
				fmt.Fprintf(tc.Stderr, "Error: %v\n", err)
			}

		case "unlisten":
			if len(tokens) < 2 {
				fmt.Fprintf(tc.Stderr, "Usage: unlisten <event-name>\n")
				continue
			}
			if err := c.replUnlisten(client, tc, &listenedEvents, tokens[1]); err != nil {
				fmt.Fprintf(tc.Stderr, "Error: %v\n", err)
			}

		case "listeners":
			c.replListeners(tc, &listenedEvents)

		default:
			fmt.Fprintf(tc.Stderr, "Unknown command: %s (use help for available commands)\n", cmd)
		}
	}
}

// replGet executes a get operation within the REPL, using the pending map for async response handling.
func (c *replCommand) replGet(client engine.Client, tc *terminal.Context, pending *sync.Map, varName, unit, dtStr string) error {
	dt, err := parseDataType(dtStr)
	if err != nil {
		return err
	}

	defID := nextDefID()
	reqID := nextReqID()

	if err := client.AddToDataDefinition(defID, varName, unit, dt, 0, 0); err != nil {
		return fmt.Errorf("AddToDataDefinition failed: %w", err)
	}
	defer client.ClearDataDefinition(defID)

	resultCh := make(chan string, 1)
	pending.Store(reqID, &pendingRequest{
		defID:    defID,
		dataType: dt,
		result:   resultCh,
	})

	if err := client.RequestDataOnSimObject(
		reqID, defID,
		types.SIMCONNECT_OBJECT_ID_USER,
		types.SIMCONNECT_PERIOD_ONCE,
		types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT,
		0, 0, 0,
	); err != nil {
		pending.Delete(reqID)
		return fmt.Errorf("RequestDataOnSimObject failed: %w", err)
	}

	// Wait for response with timeout
	select {
	case result := <-resultCh:
		fmt.Fprintf(tc.Stdout, "%s\n", result)
		return nil
	case <-time.After(time.Duration(c.timeout) * time.Second):
		pending.Delete(reqID)
		return fmt.Errorf("timeout waiting for response after %ds", c.timeout)
	}
}

// replSet executes a set operation within the REPL.
func (c *replCommand) replSet(client engine.Client, tc *terminal.Context, varName, unit, dtStr, valStr string) error {
	dt, err := parseDataType(dtStr)
	if err != nil {
		return err
	}

	parsed, err := parseValue(valStr, dt)
	if err != nil {
		return err
	}

	defID := nextDefID()
	if err := client.AddToDataDefinition(defID, varName, unit, dt, 0, 0); err != nil {
		return fmt.Errorf("AddToDataDefinition failed: %w", err)
	}
	defer client.ClearDataDefinition(defID)

	// Reuse the shared setDataValue helper from set.go
	if err := setDataValue(client, defID, dt, parsed); err != nil {
		return err
	}

	// Brief wait handled by the consumer goroutine for exceptions
	time.Sleep(200 * time.Millisecond)
	fmt.Fprintf(tc.Stdout, "OK\n")
	return nil
}

// replEmit fires a client event within the REPL.
func (c *replCommand) replEmit(client engine.Client, tc *terminal.Context, eventName string, dataArgs []string) error {
	if len(dataArgs) > 5 {
		return fmt.Errorf("too many data values (max 5, got %d)", len(dataArgs))
	}

	dataValues, err := parseEventData(dataArgs)
	if err != nil {
		return err
	}

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

	// Brief wait for exception (consistent with replSet pattern)
	time.Sleep(200 * time.Millisecond)
	fmt.Fprintf(tc.Stdout, "OK\n")
	return nil
}

// replListen subscribes to one or more client events within the REPL.
func (c *replCommand) replListen(client engine.Client, tc *terminal.Context, listenedEvents *sync.Map, listenGroupSetup *bool, eventNames []string) error {
	for _, name := range eventNames {
		mapping, err := getOrMapEvent(client, name)
		if err != nil {
			return err
		}

		if mapping.listening {
			fmt.Fprintf(tc.Stderr, "Already listening for %s\n", mapping.name)
			continue
		}

		if err := client.AddClientEventToNotificationGroup(listenGroupID, mapping.eventID, false); err != nil {
			return fmt.Errorf("AddClientEventToNotificationGroup(%s): %w", mapping.name, err)
		}

		if !*listenGroupSetup {
			if err := client.SetNotificationGroupPriority(listenGroupID, listenGroupPriority); err != nil {
				return fmt.Errorf("SetNotificationGroupPriority: %w", err)
			}
			*listenGroupSetup = true
		}

		mapping.listening = true
		listenedEvents.Store(mapping.eventID, mapping.name)
		fmt.Fprintf(tc.Stderr, "Listening for %s\n", mapping.name)
	}
	return nil
}

// replUnlisten unsubscribes from a client event within the REPL.
func (c *replCommand) replUnlisten(client engine.Client, tc *terminal.Context, listenedEvents *sync.Map, name string) error {
	key := strings.ToUpper(name)
	val, ok := eventCache.Load(key)
	if !ok {
		return fmt.Errorf("event %q not found (not mapped)", key)
	}

	mapping := val.(*eventMapping)
	if !mapping.listening {
		return fmt.Errorf("not listening for %s", key)
	}

	if err := client.RemoveClientEvent(listenGroupID, mapping.eventID); err != nil {
		return fmt.Errorf("RemoveClientEvent(%s): %w", key, err)
	}

	mapping.listening = false
	listenedEvents.Delete(mapping.eventID)
	fmt.Fprintf(tc.Stderr, "Stopped listening for %s\n", key)
	return nil
}

// replListeners shows all active event listeners.
func (c *replCommand) replListeners(tc *terminal.Context, listenedEvents *sync.Map) {
	var names []string
	listenedEvents.Range(func(key, value any) bool {
		names = append(names, value.(string))
		return true
	})

	if len(names) == 0 {
		fmt.Fprintf(tc.Stderr, "No active listeners\n")
		return
	}

	sort.Strings(names)
	fmt.Fprintf(tc.Stderr, "Active listeners: %s\n", strings.Join(names, ", "))
}

// tokenize splits a line into tokens, respecting double-quoted strings.
// Quoted strings have their quotes stripped. Empty quoted strings ("") produce an empty token.
func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '"' && !inQuote:
			inQuote = true
		case ch == '"' && inQuote:
			// End of quoted string, emit token even if empty
			tokens = append(tokens, current.String())
			current.Reset()
			inQuote = false
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	// Flush remaining token
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
