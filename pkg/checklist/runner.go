//go:build windows

package checklist

import (
	"math"
	"slices"

	"github.com/mrlm-net/simconnect/pkg/systems"
)

// Status is an item's state as checked.
type Status string

const (
	Done      Status = "done"
	NotDone   Status = "not-done"
	CantCheck Status = "cant-check" // no check, or its value is not read on this aircraft
)

// Derived values a check may name beside systems.State.Values:
// "enginesRunning" (how many), "doorsClosed" (1 when all are).
const (
	EnginesRunning = "enginesRunning"
	DoorsClosed    = "doorsClosed"
)

// value is name's value in s; false when not read.
func value(name string, s systems.State) (float64, bool) {
	switch name {
	case EnginesRunning:
		n := 0
		for i := range min(s.Engines, len(s.Running)) {
			if s.Running[i] {
				n++
			}
		}
		return float64(n), s.Engines > 0
	case DoorsClosed:
		if len(s.DoorsOpen) == 0 {
			return 0, false
		}
		if slices.Contains(s.DoorsOpen, true) {
			return 0, true
		}
		return 1, true
	}
	v, ok := s.Values[name]
	return v, ok
}

// Verify is the item's status in s.
func (it Item) Verify(s systems.State) Status {
	c := it.Check
	if c == nil {
		return CantCheck
	}
	v, ok := value(c.Value, s)
	if !ok || math.IsNaN(v) {
		return CantCheck
	}
	if c.Is != nil && (v != 0) != *c.Is {
		return NotDone
	}
	if c.Min != nil && v < *c.Min {
		return NotDone
	}
	if c.Max != nil && v > *c.Max {
		return NotDone
	}
	return Done
}

// Result is an item with its status.
type Result struct {
	Item   Item   `json:"item"`
	Status Status `json:"status"`
}

// Verify is every item of l checked against s.
func (l List) Verify(s systems.State) []Result {
	out := make([]Result, len(l.Items))
	for i, it := range l.Items {
		out[i] = Result{Item: it, Status: it.Verify(s)}
	}
	return out
}

// Complete reports whether every item that can be checked is done in s
// (and at least one can).
func (l List) Complete(s systems.State) bool {
	checked := false
	for _, it := range l.Items {
		switch it.Verify(s) {
		case NotDone:
			return false
		case Done:
			checked = true
		}
	}
	return checked
}

// Due are the lists of s due at a pilot phase ("approach") or a stage the
// caller names ("before-start"), in order.
func (s Set) Due(phaseOrStage string) []List {
	var out []List
	for _, l := range s.Lists {
		if slices.Contains(l.Phases, phaseOrStage) || slices.Contains(l.Stages, phaseOrStage) {
			out = append(out, l)
		}
	}
	return out
}

// Actor is who does a part: the player or the copilot, or both.
type Actor string

const (
	Player  Actor = "player"
	Copilot Actor = "copilot"
	Crew    Actor = "both"
)

// Resolve is who has role now: the PF is the copilot or the player
// (copilotPF), the PM the other; both both, either the PM (the one
// reading).
func Resolve(role string, copilotPF bool) Actor {
	pf, pm := Player, Copilot
	if copilotPF {
		pf, pm = Copilot, Player
	}
	switch role {
	case PF:
		return pf
	case Both:
		return Crew
	}
	return pm
}

// Runner steps through one list: the PF calls for it, the PM reads each
// challenge, the item's role responds, and the state says whether the
// item is done. Roles are resolved by who is PF at each step (they swap
// mid-flight).
type Runner struct {
	List    List
	i       int
	results []Result
}

// Run starts l.
func Run(l List) *Runner { return &Runner{List: l} }

// CalledBy and ReadBy are who calls for the list and who reads it now.
func (r *Runner) CalledBy(copilotPF bool) Actor { return Resolve(or(r.List.CalledBy, PF), copilotPF) }
func (r *Runner) ReadBy(copilotPF bool) Actor   { return Resolve(or(r.List.ReadBy, PM), copilotPF) }

func or(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// Current is the item to read next; false when the list is through.
func (r *Runner) Current() (Item, bool) {
	if r.i >= len(r.List.Items) {
		return Item{}, false
	}
	return r.List.Items[r.i], true
}

// Responder is who responds to the current item now.
func (r *Runner) Responder(copilotPF bool) Actor {
	it, _ := r.Current()
	return Resolve(it.Role, copilotPF)
}

// Next checks the current item against s, records it and moves on; false
// when the list was through already.
func (r *Runner) Next(s systems.State) (Result, bool) {
	it, ok := r.Current()
	if !ok {
		return Result{}, false
	}
	res := Result{Item: it, Status: it.Verify(s)}
	r.results = append(r.results, res)
	r.i++
	return res, true
}

// Done reports whether every item has been read.
func (r *Runner) Done() bool { return r.i >= len(r.List.Items) }

// Results are the items read so far with their status.
func (r *Runner) Results() []Result { return slices.Clone(r.results) }

// Open are the items read and found not done.
func (r *Runner) Open() []Result {
	var out []Result
	for _, res := range r.results {
		if res.Status == NotDone {
			out = append(out, res)
		}
	}
	return out
}
