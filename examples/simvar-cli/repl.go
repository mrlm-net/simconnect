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
	return "repl\n\nStarts an interactive session. Commands:\n  get <variable-name> <unit> <datatype>\n  set <variable-name> <unit> <datatype> <value>\n  exit | quit"
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
	fmt.Fprintf(tc.Stderr, "Connected. Type 'exit' or 'quit' to leave.\n")

	// Pending requests map: reqID -> pendingRequest
	var pending sync.Map

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
			return nil

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

		default:
			fmt.Fprintf(tc.Stderr, "Unknown command: %s (use get, set, exit)\n", cmd)
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
