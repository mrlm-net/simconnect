//go:build windows

// Package checklist is the aircraft's normal checklists as data, and a
// runner that steps through one and checks each item against the
// aircraft's state (#1002). The simulator's own checklists are encrypted
// and SimConnect has no checklist API, so the lists are ours: a default
// per family (the A320 family's), per-model sets on top, and a local file
// over those (the GSX way: local wins per value).
//
// An item is a challenge and its response, said by a role (PF, PM, or
// either), with an optional check against systems.State by its generic
// value names ("gearDown", "seatbelts", "flapsIndex"). An item is verify
// only, or actionable: then Action names a systems action the PM may take
// (signs, lights, arming the spoilers, the autobrake), never the gear,
// flaps, autopilot or thrust, which the pilot flying owns.
package checklist

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/systems"
)

// Roles: who says an item's response, or who does a list's part.
const (
	PF     = "PF"
	PM     = "PM"
	Both   = "both"
	Either = "" // whoever is there
)

// Check is an item's test against the state: Value (a systems.State.Values
// name, or one of Derived) is Is (a switch: on true, off false), or within
// Min and Max (each optional).
type Check struct {
	Value string   `json:"value"`
	Is    *bool    `json:"is,omitempty"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}

// Item is one line of a list.
type Item struct {
	Challenge string `json:"challenge"`        // "SEAT BELTS"
	Response  string `json:"response"`         // "ON"
	Role      string `json:"role,omitempty"`   // who responds: PF, PM, both, "" either
	Check     *Check `json:"check,omitempty"`  // nil: confirmed by the crew only
	Action    string `json:"action,omitempty"` // a systems action the PM may take to make it so; "" verify only
	// On: what Action sets (systems Controls.Set on); nil true.
	On   *bool  `json:"on,omitempty"`
	Note string `json:"note,omitempty"`
	// Remove drops the item with this challenge (in an override).
	Remove bool `json:"remove,omitempty"`
}

// List is one checklist: Name its key ("before-start", "landing"), Title
// as shown, and when it is due: at a pilot phase (pkg/pilot's names:
// "takeoff", "climb", "cruise", "descent", "approach", "landing"…) or a
// stage the caller names ("before-start", "after-start", "parking").
// CalledBy calls for it and ReadBy reads it (PF and PM by default).
type List struct {
	Name     string   `json:"name"`
	Title    string   `json:"title,omitempty"`
	Phases   []string `json:"phases,omitempty"`
	Stages   []string `json:"stages,omitempty"`
	CalledBy string   `json:"calledBy,omitempty"`
	ReadBy   string   `json:"readBy,omitempty"`
	Items    []Item   `json:"items"`
}

// Set is a family's or a model's lists. A base set (Base) is the family's
// default; a model's set (Extends: the base's name) goes on top of it.
type Set struct {
	Name    string        `json:"name"`
	Base    bool          `json:"base,omitempty"`
	Extends string        `json:"extends,omitempty"`
	Match   systems.Match `json:"match"`
	Note    string        `json:"note,omitempty"`
	Lists   []List        `json:"lists"`
}

// Matches reports whether s is for a.
func (s Set) Matches(a systems.Aircraft) bool {
	return systems.Profile{Match: s.Match}.Matches(a)
}

// Lists by name.
func (s Set) List(name string) (List, bool) {
	for _, l := range s.Lists {
		if l.Name == name {
			return l, true
		}
	}
	return List{}, false
}

// actionsNever are the systems actions a checklist never takes: the pilot
// flying's (gear, flaps, autopilot, thrust, flight controls, brakes).
var actionsNever = []string{systems.GearDown, systems.FlapsLever, systems.FlapsUp, systems.FlapsDown,
	systems.Throttle, systems.ReverseThrust, systems.Reversers, systems.TOGA, systems.Elevator, systems.Aileron,
	systems.Rudder, systems.BrakeLeft, systems.BrakeRight, systems.ATHR, systems.FD}

// actionAllowed reports whether a checklist may take action a.
func actionAllowed(a string) bool {
	return !slices.Contains(actionsNever, a) && !strings.HasPrefix(a, "ap") && !strings.HasPrefix(a, "throttle")
}

// Validate checks s: names given, lists named once, actions allowed.
func (s Set) Validate() error {
	seen := map[string]bool{}
	for _, l := range s.Lists {
		if l.Name == "" {
			return fmt.Errorf("checklist: set %q: a list without a name", s.Name)
		}
		if seen[l.Name] {
			return fmt.Errorf("checklist: set %q: list %s twice", s.Name, l.Name)
		}
		seen[l.Name] = true
		for _, it := range l.Items {
			if it.Action != "" && !actionAllowed(it.Action) {
				return fmt.Errorf("checklist: %s %q: action %s is the pilot flying's, verify only", l.Name, it.Challenge, it.Action)
			}
			if c := it.Check; c != nil && c.Value == "" {
				return fmt.Errorf("checklist: %s %q: a check without a value", l.Name, it.Challenge)
			}
		}
	}
	return nil
}

// Merge is base with over on top: over's lists replace base's fields they
// give, items matched by challenge (any case) replaced field by field,
// removed (Remove) or added at the end; lists base lacks are added.
func Merge(base, over Set) Set {
	out := base
	out.Lists = slices.Clone(base.Lists)
	for _, ol := range over.Lists {
		i := slices.IndexFunc(out.Lists, func(l List) bool { return l.Name == ol.Name })
		if i < 0 {
			out.Lists = append(out.Lists, ol)
			continue
		}
		l := out.Lists[i]
		if ol.Title != "" {
			l.Title = ol.Title
		}
		if len(ol.Phases) > 0 {
			l.Phases = ol.Phases
		}
		if len(ol.Stages) > 0 {
			l.Stages = ol.Stages
		}
		if ol.CalledBy != "" {
			l.CalledBy = ol.CalledBy
		}
		if ol.ReadBy != "" {
			l.ReadBy = ol.ReadBy
		}
		l.Items = mergeItems(l.Items, ol.Items)
		out.Lists[i] = l
	}
	if over.Name != "" {
		out.Name = over.Name
	}
	return out
}

func mergeItems(base, over []Item) []Item {
	out := slices.Clone(base)
	for _, o := range over {
		i := slices.IndexFunc(out, func(it Item) bool { return strings.EqualFold(it.Challenge, o.Challenge) })
		switch {
		case o.Remove && i >= 0:
			out = slices.Delete(out, i, i+1)
		case o.Remove:
		case i < 0:
			out = append(out, o)
		default:
			it := out[i]
			if o.Response != "" {
				it.Response = o.Response
			}
			if o.Role != "" {
				it.Role = o.Role
			}
			if o.Check != nil {
				it.Check = o.Check
			}
			if o.Action != "" {
				it.Action, it.On = o.Action, o.On
			}
			if o.Note != "" {
				it.Note = o.Note
			}
			out[i] = it
		}
	}
	return out
}
