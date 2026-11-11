package world

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/jetway"
)

// LKPR B15 and B14 are 9.8 m apart edge to edge, C23 and C24 9.6 m, C22
// and C23 10.2 m (crowded within the margin), B16 15.4 m from both sides.
func TestCrowdedGatesLKPR(t *testing.T) {
	b := newJWBed(t)
	near := crowdedGates(b.g)
	b15, b14, c23, c24, c22 := b.stand("B15"), b.stand("B14"), b.stand("C23"), b.stand("C24"), b.stand("C22")
	if !slices.Contains(near[b15], b14) || !slices.Contains(near[b14], b15) {
		t.Errorf("B15 and B14 not crowded: %v / %v", near[b15], near[b14])
	}
	if !slices.Contains(near[c23], c24) {
		t.Errorf("C23 and C24 not crowded: %v", near[c23])
	}
	if !slices.Contains(near[c23], c22) {
		t.Errorf("C22 (10.2 m, within the margin) not crowding C23: %v", near[c23])
	}
	if b16 := b.stand("B16"); len(near[b16]) != 0 {
		t.Errorf("B16 (15.4 m) crowded: %v", near[b16])
	}
}

// A dock at a crowded gate (B15) with another gate docked (C24) first
// takes away the placeholder next to it (B14): the sim ranks a free gate
// with a held one that close below every docked gate, its helper took a
// docked gate's jetway (live, LKPR B15 and C23 lost 26 of 26). B14 is
// not filled again while a gate is docked.
func TestJetflowCrowdedGateClearsNeighbour(t *testing.T) {
	b := newJWBed(t)
	x, s, u := b.stand("B15"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	b.park(2, "CSA871", jwA320, u, 7002, true)
	b.tick(time.Second)
	h := b.tick(jetway.HelperDelay)
	if len(h) != 1 {
		t.Fatalf("C24 helpers %v", h)
	}
	obj := b.assign(h[0])
	b.list(jwState{u, 1, obj})
	b.list(jwState{u, 7, obj})
	if st := b.svc.Status("LKPR", u); st != jetway.StatusDocked {
		t.Fatalf("C24 %s", st)
	}
	removed := len(b.sim.removed)
	b.park(1, "SWR662", jwA320, x, 7001, true)
	b.tick(time.Second)
	if st := b.svc.Status("LKPR", x); st != jetway.StatusDocking {
		t.Fatalf("B15 %s, want docking", st)
	}
	if st := b.svc.Status("LKPR", s); st != jetway.StatusNone {
		t.Errorf("B14 %s, want its placeholder taken away", st)
	}
	if got := len(b.sim.removed) - removed; got != 2 {
		t.Errorf("%d objects removed, want B15's and B14's placeholders", got)
	}
	h = b.tick(jetway.HelperDelay)
	if len(h) != 1 {
		t.Fatalf("B15 helpers %v", h)
	}
	hx := b.assign(h[0])
	b.list(jwState{x, 1, hx})
	b.list(jwState{x, 7, hx})
	if st := b.svc.Status("LKPR", x); st != jetway.StatusDocked {
		t.Fatalf("B15 %s", st)
	}
	created := len(b.sim.created)
	b.ticks(120)
	if n := len(b.sim.created) - created; n != 0 {
		t.Errorf("%d placeholders made while gates are docked (B14 would take a docked gate)", n)
	}
}

// A gate whose aircraft got no jetway (its helper put nowhere) is refilled
// once; that placeholder taken by no gate, it is not refilled again: the
// dock asked elsewhere goes ahead (live, LKPR 2026-11-11: C26 with our
// aircraft on stairs, a placeholder every 13 s, every dock held "still
// filling").
func TestJetflowRefillNoGateNotRepeated(t *testing.T) {
	b := newJWBed(t)
	x, s, u := b.stand("C30"), b.stand("B14"), b.stand("C24")
	b.fill([]int{x, s, u}, x, s, u)
	b.park(1, "CSA1896", jwA320, x, 7001, true)
	b.tick(time.Second)
	h := b.tick(jetway.HelperDelay)
	if len(h) != 1 {
		t.Fatalf("C30 helpers %v", h)
	}
	b.assign(h[0]) // on no jetway gate: removed after ClaimTimeout, C30 failed
	b.ticks(int(jetway.ClaimTimeout/time.Second) + 2)
	if st := b.svc.Status("LKPR", x); st != jetway.StatusFailed {
		t.Fatalf("C30 %s, want failed", st)
	}
	placeholders := func() int {
		n := 0
		for _, c := range b.sim.created {
			if !strings.HasPrefix(c.tail, "JW") {
				n++
			}
		}
		return n
	}
	before := placeholders()
	b.park(2, "KLM1106", jwA320, u, 7002, true)
	var helpers []jwCreated
	for i := 0; i < 120 && len(helpers) == 0; i++ {
		helpers = b.tick(time.Second) // refill placeholders land nowhere (no claim)
	}
	if n := placeholders() - before; n != 1 {
		t.Errorf("%d refill placeholders for C30, want 1", n)
	}
	if len(helpers) == 0 {
		t.Fatal("C24's dock held by refills")
	}
}
