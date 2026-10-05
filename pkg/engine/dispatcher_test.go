package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/internal/simconnect"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// failingAPI answers every GetNextDispatch with err.
type failingAPI struct {
	simconnect.API
	err   error
	calls atomic.Int64
}

func (f *failingAPI) SubscribeToSystemEvent(uint32, string) error { return nil }
func (f *failingAPI) GetNextDispatch() (*types.SIMCONNECT_RECV, uint32, error) {
	f.calls.Add(1)
	return nil, 0, f.err
}

func failingEngine(err error) (*Engine, *failingAPI) {
	api := &failingAPI{err: err}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{api: api, ctx: ctx, cancel: cancel, config: &Config{Heartbeat: HEARTBEAT_6HZ},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), queue: make(chan Message, 512)}
	return e, api
}

// drain reads the stream until it closes, within d: the messages, and
// whether it closed.
func drain(e *Engine, d time.Duration) ([]Message, bool) {
	var got []Message
	deadline := time.After(d)
	for {
		select {
		case m, ok := <-e.queue:
			if !ok {
				return got, true
			}
			got = append(got, m)
		case <-deadline:
			return got, false
		}
	}
}

// The simulator gone (STATUS_PIPE_DISCONNECTED): one error, then the
// stream closes, so a manager reconnects (it spun, 280,000 errors a
// second, on a sim restart).
func TestDispatchEndsWhenThePipeCloses(t *testing.T) {
	e, api := failingEngine(fmt.Errorf("%w: pipe disconnected", simconnect.ErrConnectionLost))
	defer e.cancel()
	e.dispatch()
	got, closed := drain(e, 2*time.Second)
	if !closed || len(got) != 1 || !errors.Is(got[0].Err, ErrConnectionLost) || api.calls.Load() != 1 {
		t.Fatalf("closed %v, %d messages, %d calls", closed, len(got), api.calls.Load())
	}
}

// Any other error without end: backed off, then the connection is lost.
func TestDispatchBacksOffAndGivesUpOnErrors(t *testing.T) {
	e, api := failingEngine(errors.New("SimConnect_GetNextDispatch failed with HRESULT: 0x80004005"))
	defer e.cancel()
	e.dispatch()
	got, closed := drain(e, 10*time.Second)
	if !closed || len(got) != maxErrorsInRow || !errors.Is(got[len(got)-1].Err, ErrConnectionLost) {
		t.Fatalf("closed %v, %d messages", closed, len(got))
	}
	if n := api.calls.Load(); n != maxErrorsInRow {
		t.Errorf("%d calls", n)
	}
}
