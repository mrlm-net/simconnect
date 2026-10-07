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
	"unsafe"

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

// scriptedAPI answers GetNextDispatch with its messages, then nothing.
type scriptedAPI struct {
	simconnect.API
	msgs [][]byte
}

func (s *scriptedAPI) SubscribeToSystemEvent(uint32, string) error { return errors.New("refused") }
func (s *scriptedAPI) GetNextDispatch() (*types.SIMCONNECT_RECV, uint32, error) {
	if len(s.msgs) == 0 {
		return nil, 0, nil
	}
	m := s.msgs[0]
	s.msgs = s.msgs[1:]
	if len(m) == 0 {
		return (*types.SIMCONNECT_RECV)(unsafe.Pointer(&s.msgs)), 0, nil // non-nil, size 0
	}
	return (*types.SIMCONNECT_RECV)(unsafe.Pointer(&m[0])), uint32(len(m)), nil
}

func recvBytes(id types.SIMCONNECT_RECV_ID) []byte {
	r := types.SIMCONNECT_RECV{DwSize: 12, DwID: types.DWORD(id)}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(&r)), 12)...)
}

// A QUIT with a full queue nobody reads: Disconnect's cancel still ends the
// dispatcher (it hung on a bare send).
func TestDispatchQuitOnFullQueueEndsOnCancel(t *testing.T) {
	api := &scriptedAPI{msgs: [][]byte{recvBytes(types.SIMCONNECT_RECV_ID_OPEN), recvBytes(types.SIMCONNECT_RECV_ID_QUIT)}}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{api: api, ctx: ctx, cancel: cancel, config: &Config{Heartbeat: HEARTBEAT_6HZ},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), queue: make(chan Message, 1)}
	e.dispatch()
	time.Sleep(50 * time.Millisecond) // OPEN fills the queue, QUIT waits
	done := make(chan struct{})
	go func() { e.cancel(); e.sync.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the dispatcher did not stop")
	}
}

// An empty message is dropped, not indexed (it panicked), and a refused
// heartbeat subscription does not stop the stream.
func TestDispatchDropsEmptyMessage(t *testing.T) {
	api := &scriptedAPI{msgs: [][]byte{{}, recvBytes(types.SIMCONNECT_RECV_ID_QUIT)}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := &Engine{api: api, ctx: ctx, cancel: cancel, config: &Config{Heartbeat: HEARTBEAT_6HZ},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), queue: make(chan Message, 8)}
	e.dispatch()
	got, closed := drain(e, 2*time.Second)
	if !closed || len(got) != 1 || types.SIMCONNECT_RECV_ID(got[0].DwID) != types.SIMCONNECT_RECV_ID_QUIT {
		t.Fatalf("closed %v, %d messages", closed, len(got))
	}
}
