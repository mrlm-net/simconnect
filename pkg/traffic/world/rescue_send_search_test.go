package world

import (
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestRescueSendOffStep (#1670): the search for a crew's standby points
// and ways runs off the step that sends it: held, the step returns with
// no crew out and nothing created, and the frames go on; once it ends, the
// next frame sends the crew, its vehicles' own searches off it too.
func TestRescueSendOffStep(t *testing.T) {
	l := lkprLayout(t)
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	var mu sync.Mutex
	asked := 0
	open := make(chan struct{})
	var wg sync.WaitGroup
	cc.rescue.search = func(f func()) {
		mu.Lock()
		asked++
		mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-open
			f()
		}()
	}
	now := time.Now()
	stepped := make(chan *rescueCrew)
	go func() { stepped <- rescueComing(t, cc, w, l, "CSA1", traffic.EmergencyFire, now) }()
	select {
	case crew := <-stepped:
		if crew != nil || len(c.created) != 0 {
			t.Fatalf("crew %v, created %v on the step: want them sent after the search", crew != nil, c.created)
		}
	case <-time.After(5 * time.Second):
		close(open)
		t.Fatal("the step waited for the search")
	}
	mu.Lock()
	n := asked
	mu.Unlock()
	if n != 1 {
		t.Fatalf("%d searches as the crew was sent, want 1", n)
	}
	for i := 0; i < 10; i++ {
		now = now.Add(100 * time.Millisecond)
		cc.rescueFrame(now)
	}
	cc.rescue.mu.Lock()
	out, sending := cc.rescue.by["CSA1"], len(cc.rescue.sending)
	cc.rescue.mu.Unlock()
	if out != nil || sending != 1 || len(c.created) != 0 {
		t.Fatalf("frames of the search: crew out %v, %d sending, created %v; want none out, one sending", out != nil, sending, c.created)
	}
	close(open)
	wg.Wait()
	now = now.Add(100 * time.Millisecond)
	cc.rescueFrame(now)
	cc.rescue.mu.Lock()
	crew, sending := cc.rescue.by["CSA1"], len(cc.rescue.sending)
	cc.rescue.mu.Unlock()
	if crew == nil || len(crew.trucks) != 2 || sending != 0 || len(c.created) != 1 {
		t.Fatalf("the frame after the search: crew %v, %d sending, created %v; want 2 out, the first created", crew != nil, sending, c.created)
	}
	for _, tr := range crew.trucks {
		if tr.Search == nil {
			t.Error("a vehicle searches its ways on the frame's goroutine")
		}
	}
	if sent, _ := cc.rescueStatus("CSA1"); !sent {
		t.Error("not reported sent")
	}
}

// TestRescueSendDroppedOnStop (#1670): taken away while its search runs, a
// crew is never sent, and its IDs come free.
func TestRescueSendDroppedOnStop(t *testing.T) {
	l := lkprLayout(t)
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	open := make(chan struct{})
	ran := make(chan struct{})
	cc.rescue.search = func(f func()) {
		go func() {
			<-open
			f()
			close(ran)
		}()
	}
	now := time.Now()
	rescueComing(t, cc, w, l, "CSA1", traffic.EmergencyFire, now)
	cc.removeRescue()
	close(open)
	<-ran
	for i := 0; i < 10; i++ {
		now = now.Add(100 * time.Millisecond)
		cc.rescueFrame(now)
	}
	cc.rescue.mu.Lock()
	out := len(cc.rescue.by)
	cc.rescue.mu.Unlock()
	if out != 0 || len(c.created) != 0 {
		t.Fatalf("%d crews out, created %v after the stop; want none", out, c.created)
	}
	for i := 0; i < 500 && cc.ids.InUse() != 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if n := cc.ids.InUse(); n != 0 {
		t.Errorf("%d ID blocks in use after the stop, want the crew's free", n)
	}
}
