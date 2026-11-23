package worldtest

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// waitsForLock is the goroutine a hung World's dump must name.
func waitsForLock(mu *sync.Mutex, done chan<- struct{}) {
	mu.Lock()
	defer mu.Unlock()
	close(done)
}

// TestHungDumpsAllStacks (#1652): a hung World's panic names the blocked
// goroutine; the log had only the test's own, the one holding the lock
// unknown.
func TestHungDumpsAllStacks(t *testing.T) {
	var mu sync.Mutex
	done := make(chan struct{})
	mu.Lock()
	go waitsForLock(&mu, done)
	defer func() { <-done }()
	defer mu.Unlock()

	var out bytes.Buffer
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		out.Reset()
		var said any
		func() {
			defer func() { said = recover() }()
			hung(&out, "no frame")
		}()
		if said != "no frame" {
			t.Fatalf("panicked with %v, want its message", said)
		}
		dump := out.String()
		if !strings.HasPrefix(dump, stacksHead+"no frame\n") || !strings.Contains(dump, stacksTail) {
			t.Fatalf("the dump is not marked:\n%.300s", dump)
		}
		for _, g := range strings.Split(dump, "\n\n") {
			if strings.Contains(g, "waitsForLock(") && strings.Contains(g, "sync.Mutex.Lock") {
				return
			}
		}
	}
	t.Fatalf("the dump does not name the goroutine waiting for the lock:\n%s", out.String())
}

// TestStallDumpsStacks (#1652): a World that takes no frame panics with
// every goroutine's stack written, the World's among them. A tick nobody
// takes stalls the step.
func TestStallDumpsStacks(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	h, err := Start(Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.Advance(time.Second)
	stray := h.Clock.NewTicker(h.frame)
	defer stray.Stop()
	var out bytes.Buffer
	h.stallAfter, h.stacksTo = 300*time.Millisecond, &out
	var said any
	func() {
		defer func() { said = recover() }()
		h.Advance(h.frame)
	}()
	msg, _ := said.(string)
	if !strings.Contains(msg, "did not take a frame") {
		t.Fatalf("panicked with %v, want a stall", said)
	}
	dump := out.String()
	if !strings.HasPrefix(dump, stacksHead+msg+"\n") || !strings.Contains(dump, stacksTail) {
		t.Fatalf("no stacks written:\n%.300s", dump)
	}
	if n := strings.Count(dump, "\ngoroutine "); n < 2 || !strings.Contains(dump, "pkg/traffic/world.") {
		t.Errorf("%d goroutines written, none of the World's", n+1)
	}
}
