package world

import (
	"testing"
	"time"
)

// TestAgenda: on a quiet frequency the most urgent ready call goes first
// (a landing before an earlier line-up), one call per frequency at a time,
// the longest waiting first within a class; a busy frequency holds its
// calls; a call no longer wanted is dropped.
func TestAgenda(t *testing.T) {
	now := time.Date(2026, 10, 2, 20, 0, 0, 0, time.UTC)
	clear := map[string]time.Time{}
	a := &agenda{radio: func(icao, freq string) time.Time { return clear[icao+" "+freq] }}
	var said, drops []string
	add := func(freq string, prio callPrio, tail string, since time.Duration, still func() bool) {
		a.add(call{icao: "LKPR", freq: freq, prio: prio, tail: tail, since: now.Add(since), ready: now.Add(since),
			still: still, dropped: func() { drops = append(drops, tail) }, f: func() { said = append(said, tail) }})
	}
	add("tower", prioRunway, "LINEUP1", -10*time.Second, nil)
	add("tower", prioLanding, "LAND1", -2*time.Second, nil)
	add("ground", prioStand, "PUSH1", -30*time.Second, nil)
	add("ground", prioClearing, "VACATED1", -5*time.Second, nil)
	add("ground", prioStand, "PUSH2", -40*time.Second, nil)
	add("tower", prioRunway, "LINEUP2", -20*time.Second, func() bool { return false })
	add("approach", prioApproach, "LATER1", 5*time.Second, nil) // not decided yet
	a.run(now)
	if want := []string{"LAND1", "VACATED1"}; len(said) != 2 || said[0] != want[0] || said[1] != want[1] {
		t.Fatalf("first round said %v, want %v", said, want)
	}
	// The tower is busy (the landing clearance and its readback): nothing
	// more on it; ground is quiet again: the push waiting longest.
	clear["LKPR tower"] = now.Add(5 * time.Second)
	said = nil
	a.run(now.Add(time.Second))
	if len(said) != 1 || said[0] != "PUSH2" {
		t.Fatalf("second round said %v, want [PUSH2]", said)
	}
	said = nil
	a.run(now.Add(7 * time.Second))
	// LINEUP1, PUSH1, LATER1 said; LINEUP2 (no longer wanted) dropped,
	// never said.
	got := map[string]bool{}
	for _, s := range said {
		got[s] = true
	}
	if len(drops) != 1 || drops[0] != "LINEUP2" {
		t.Errorf("dropped %v, want [LINEUP2]", drops)
	}
	for _, want := range []string{"LINEUP1", "PUSH1", "LATER1"} {
		if !got[want] {
			t.Errorf("third round said %v, missing %q", said, want)
		}
	}
	if len(a.waiting()) != 0 {
		t.Errorf("left on the agenda: %d", len(a.waiting()))
	}
}

// TestAgendaSafetyFirst: a separation instruction goes before a landing
// clearance waiting longer, and right as the frequency clears (no answer
// pause), where a routine call waits atcAnswerDelay.
func TestAgendaSafetyFirst(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	clear := map[string]time.Time{"LKPR approach": now}
	a := &agenda{radio: func(icao, freq string) time.Time { return clear[icao+" "+freq] }}
	var said []string
	add := func(prio callPrio, tail string, since time.Duration) {
		a.add(call{icao: "LKPR", freq: "approach", prio: prio, tail: tail, since: now.Add(since), ready: now.Add(since),
			f: func() { said = append(said, tail) }})
	}
	add(prioApproach, "SPEED1", -30*time.Second)
	add(prioTraffic, "INFO1", -20*time.Second)
	add(prioSeparation, "STOP1", 0)
	a.run(now) // the frequency just cleared: only the safety call may go
	if len(said) != 1 || said[0] != "STOP1" {
		t.Fatalf("said %v, want [STOP1] at once", said)
	}
	clear["LKPR approach"] = now.Add(2 * time.Second)
	said = nil
	a.run(now.Add(2*time.Second + atcAnswerDelay))
	if len(said) != 1 || said[0] != "INFO1" {
		t.Fatalf("said %v, want [INFO1]: traffic information before the speed", said)
	}
}

// TestAgendaTempo: a frequency speeds up with each call waiting, up to
// tempoMax, and after a safety call is said.
func TestAgendaTempo(t *testing.T) {
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	a := &agenda{radio: func(icao, freq string) time.Time { return now.Add(-time.Minute) }}
	if got := a.tempo("LKPR", "approach", now); got != 1 {
		t.Fatalf("quiet: %.2f", got)
	}
	for range 3 {
		a.add(call{icao: "LKPR", freq: "approach", prio: prioApproach, ready: now.Add(time.Hour), f: func() {}})
	}
	if got := a.tempo("LKPR", "approach", now); got < 1.14 || got > 1.16 {
		t.Errorf("three waiting: %.2f, want 1.15", got)
	}
	for range 20 {
		a.add(call{icao: "LKPR", freq: "approach", prio: prioApproach, ready: now.Add(time.Hour), f: func() {}})
	}
	if got := a.tempo("LKPR", "approach", now); got != tempoMax {
		t.Errorf("crowded: %.2f, want %.2f", got, tempoMax)
	}
	b := &agenda{radio: func(icao, freq string) time.Time { return now.Add(-time.Minute) }}
	b.add(call{icao: "LKPR", freq: "tower", prio: prioUrgent, ready: now, f: func() {}})
	b.run(now)
	if got := b.tempo("LKPR", "tower", now.Add(5*time.Second)); got != tempoUrgent {
		t.Errorf("after a go-around: %.2f, want %.2f", got, tempoUrgent)
	}
	if got := b.tempo("LKPR", "tower", now.Add(tempoUrgentFor+time.Second)); got != 1 {
		t.Errorf("later: %.2f, want 1", got)
	}
}
