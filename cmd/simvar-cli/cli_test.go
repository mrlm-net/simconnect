//go:build windows
// +build windows

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/cure/pkg/terminal"
)

// The router parses a command's flags; the command reads them from
// tc.Flags, not from the positional args left (#29).
func TestListFlagsThroughRouter(t *testing.T) {
	var out bytes.Buffer
	r := terminal.New(terminal.WithStdout(&out), terminal.WithStderr(io.Discard))
	r.Register(&listCommand{format: FormatJSON})
	if err := r.RunContext(context.Background(), []string{"list", "--category", "simulator", "--search", "camera view"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "CAMERA VIEW TYPE AND INDEX") {
		t.Errorf("listed %q", out.String())
	}
}

func TestWatchFlags(t *testing.T) {
	c := &watchCommand{}
	fs := c.Flags()
	if err := fs.Parse([]string{"--interval", "visual-frame", "--changed", "PLANE ALTITUDE", "feet", "float64"}); err != nil {
		t.Fatal(err)
	}
	got, err := commandFlags(c, &terminal.Context{Args: fs.Args(), Flags: fs})
	if err != nil || flagString(got, "interval") != "visual-frame" || !flagBool(got, "changed") || len(got.Args()) != 3 {
		t.Errorf("interval %q changed %v args %v: %v", flagString(got, "interval"), flagBool(got, "changed"), got.Args(), err)
	}
}

type failingConnect struct {
	tries int
	tried chan struct{}
}

func (f *failingConnect) Connect() error {
	f.tries++
	select {
	case f.tried <- struct{}{}:
	default:
	}
	return errors.New("no sim")
}

// Ctrl+C ends the connection retries at once, not after the wait (#40).
func TestConnectWithRetryCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &failingConnect{tried: make(chan struct{}, 1)}
	done := make(chan error, 1)
	go func() { done <- connectWithRetry(ctx, f, io.Discard) }()
	select {
	case <-f.tried:
	case <-time.After(30 * time.Second):
		t.Fatal("never tried to connect")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || f.tries != 1 {
			t.Errorf("err %v after %d tries", err, f.tries)
		}
	case <-time.After(time.Second):
		t.Fatal("still waiting after Ctrl+C")
	}
}
