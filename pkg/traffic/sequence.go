package traffic

import (
	"slices"
	"time"
)

// SequenceStep is one step of an injected take-off or landing, for review
// against the type's procedures: when it happened, at what height above
// the runway and at what ground speed.
type SequenceStep struct {
	// At is the time since the sequence began: the start of the take-off
	// roll, or the takeover on final.
	At       time.Duration `json:"at"`
	Step     string        `json:"step"`
	HeightFt float64       `json:"heightFt"`
	Kts      float64       `json:"kts"`
}

// sequence records the steps of one take-off or landing.
type sequence struct {
	start time.Time
	steps []SequenceStep
}

// begin starts the sequence at now, dropping any earlier steps.
func (s *sequence) begin(now time.Time) {
	s.start, s.steps = now, nil
}

// add records step at now; nothing before begin.
func (s *sequence) add(now time.Time, step string, heightFt, kts float64) {
	if s.start.IsZero() {
		return
	}
	s.steps = append(s.steps, SequenceStep{At: now.Sub(s.start), Step: step, HeightFt: heightFt, Kts: kts})
}

// list is a copy of the steps.
func (s *sequence) list() []SequenceStep {
	return slices.Clone(s.steps)
}
