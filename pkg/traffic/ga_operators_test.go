//go:build windows
// +build windows

package traffic

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestGAOperatorsAt: a school and a club with their own OK- aircraft and
// private owners at LKPR, the same each time.
func TestGAOperatorsAt(t *testing.T) {
	ops := GAOperatorsAt("LKPR")
	if !reflect.DeepEqual(ops, GAOperatorsAt("LKPR")) {
		t.Fatal("not the same operators twice")
	}
	if len(ops) != 3 || ops[0].Kind != GASchool || len(ops[0].Fleet) != 5 || ops[1].Kind != GAClub || len(ops[1].Fleet) != 3 || ops[2].Kind != GAPrivate || len(ops[2].Fleet) != 0 {
		t.Fatalf("operators %+v", ops)
	}
	seen := map[string]bool{}
	for _, o := range ops {
		for _, a := range o.Fleet {
			if !strings.HasPrefix(a.Registration, "OK") || seen[a.Registration] || ProfileFor(a.Type).Category != CategoryPiston {
				t.Errorf("%s: %+v", o.Name, a)
			}
			seen[a.Registration] = true
		}
	}
}

// TestVFROperators: asked hour by hour with a seed per hour (as the map's
// Source is), the VFR flights are the operators': school arrivals fly two to
// four circuits, private owners none, and no fleet aircraft flies two
// flights at once.
func TestVFROperators(t *testing.T) {
	l := smallField(lkprGraph(t).Layout)
	fleet := map[string]GAKind{}
	for _, o := range GAOperatorsAt("LKPR") {
		for _, a := range o.Fleet {
			fleet[a.Registration] = o.Kind
		}
	}
	day := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	var fs []Flight
	for h := day; h.Before(day.Add(24 * time.Hour)); h = h.Add(time.Hour) {
		opts := VFROptions{Focus: []string{"LKPR"}, Layouts: map[string]*airport.Layout{"LKPR": l}, PerHour: 2, Seed: uint64(h.Unix() / 3600)}
		fs = append(fs, VFRFlights(opts, h, h.Add(time.Hour))...)
	}
	kinds := map[GAKind]int{}
	byReg := map[string][]Flight{}
	for _, f := range fs {
		kind := GAPrivate
		if k, ok := fleet[f.Callsign]; ok {
			kind = k
			byReg[f.Callsign] = append(byReg[f.Callsign], f)
		}
		kinds[kind]++
		want := map[GAKind]string{GASchool: "LKPR flying school", GAClub: "LKPR aero club", GAPrivate: "private"}[kind]
		if f.Operator != want {
			t.Errorf("%s: operator %q, want %q", f.Callsign, f.Operator, want)
		}
		arr := f.Origin == ""
		switch {
		case !arr && (f.TouchAndGos != 0 || f.StopAndGo):
			t.Errorf("%s: a departure with circuits", f.Callsign)
		case kind == GAPrivate && f.TouchAndGos != 0:
			t.Errorf("%s: a private owner flying %d circuits", f.Callsign, f.TouchAndGos)
		case kind == GASchool && arr && f.TouchAndGos > 4:
			t.Errorf("%s: %d circuits", f.Callsign, f.TouchAndGos)
		case f.StopAndGo && f.TouchAndGos == 0:
			t.Errorf("%s: stop-and-go without circuits", f.Callsign)
		}
	}
	for reg, list := range byReg {
		for i := range list {
			for j := i + 1; j < len(list); j++ {
				af, at := gaBusy(list[i])
				bf, bt := gaBusy(list[j])
				if af.Before(bt) && bf.Before(at) {
					t.Errorf("%s flies twice at once: %v and %v", reg, list[i].STD, list[j].STD)
				}
			}
		}
	}
	if kinds[GASchool] == 0 || kinds[GAClub] == 0 || kinds[GAPrivate] == 0 {
		t.Errorf("flights by kind %v", kinds)
	}
	t.Logf("%d flights, by kind %v", len(fs), kinds)
}
