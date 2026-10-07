package world

import (
	"context"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A first load given up before its request went out leaves no waiter: the
// next load asks again (#71).
func TestLoadCancelledFirstAsksAgain(t *testing.T) {
	st := &state{cache: airport.NewCache(), fetched: map[string]time.Time{}, waiters: map[string][]chan error{}, live: true}
	reqs := make(chan string) // nobody reads: the first request cannot go out
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := st.load(ctx, "LKPR", false, reqs); err == nil {
		t.Fatal("a cancelled load succeeded")
	}
	if n := len(st.waiters["LKPR"]); n != 0 {
		t.Fatalf("%d waiters left behind", n)
	}
	asked := make(chan string, 1)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	go st.load(ctx2, "LKPR", false, asked)
	select {
	case icao := <-asked:
		if icao != "LKPR" {
			t.Errorf("asked %s", icao)
		}
	case <-time.After(time.Second):
		t.Fatal("the next load never asked")
	}
}
