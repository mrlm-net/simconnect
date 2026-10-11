package world

import (
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The installed models' main doors (#1026): stairs go to the door of the
// model they serve, read from its own files: by the title spawned
// (addons.DoorsByTitle, a livery with its base model's door), else by its
// type (addons.DoorsByType). On Windows the World reads them from the
// simulator's packages in the background (doors_windows.go); a host
// elsewhere (a director) gives them with SetModelDoors. A model without
// one keeps the estimate (traffic.StairsDoorAftShare).

// doorTable keeps the doors, meters ahead of the model's reference point,
// by lower-case title and by type.
type doorTable struct {
	mu              sync.Mutex
	byTitle, byType map[string]float64
	// jetway: the jetway's door (interactive_point.0) by lower-case title,
	// feet ahead (Long) and right (Lat) of the reference point.
	jetway map[string]addons.InteractivePoint
}

var modelDoors = &doorTable{byTitle: map[string]float64{}, byType: map[string]float64{}, jetway: map[string]addons.InteractivePoint{}}

// jetwayDoorOffset puts the helper of title helper (its jetway's door)
// under the jetway door of our model: the helper's place from our
// aircraft, meters ahead and right, from both models' interactive_point.0.
// The jetway docks to the helper's own door (its flight_model.cfg, the
// jetway data's ExitDoorRelativePos), not to our model's: live, the FSLTL
// A320neo's door 37.65 ft ahead and its helper's 27.93 ft, the hood 3 m
// aft of the door with the helper on our aircraft (#572).
func jetwayDoorOffset(model, helper string) (ahead, right float64, ok bool) {
	modelDoors.mu.Lock()
	defer modelDoors.mu.Unlock()
	o, ok1 := modelDoors.jetway[strings.ToLower(model)]
	h, ok2 := modelDoors.jetway[strings.ToLower(helper)]
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return (o.Long - h.Long) * 0.3048, (o.Lat - h.Lat) * 0.3048, true
}

// SetModelDoors sets the main doors of the installed models, by title
// (addons.DoorsByTitle) and by ICAO type (addons.DoorsByType), replacing
// those known.
func (w *World) SetModelDoors(byTitle, byType map[string]addons.ModelDoor) {
	setDoors(byTitle, byType)
}

func setDoors(byTitle, byType map[string]addons.ModelDoor) {
	ahead := func(in map[string]addons.ModelDoor, key func(string) string) map[string]float64 {
		m := make(map[string]float64, len(in))
		for k, d := range in {
			m[key(k)] = d.Door.Long * 0.3048
		}
		return m
	}
	t, y := ahead(byTitle, strings.ToLower), ahead(byType, strings.ToUpper)
	j := make(map[string]addons.InteractivePoint, len(byTitle))
	for k, d := range byTitle {
		if d.First.Raw != "" || d.First.Long != 0 {
			j[strings.ToLower(k)] = d.First
		}
	}
	modelDoors.mu.Lock()
	modelDoors.byTitle, modelDoors.byType, modelDoors.jetway = t, y, j
	modelDoors.mu.Unlock()
}

// doorAhead is the main door of model (a title) ahead of its reference
// point, meters: its own, else its type's; ok false when neither is known.
func doorAhead(model string) (float64, bool) {
	modelDoors.mu.Lock()
	defer modelDoors.mu.Unlock()
	if a, ok := modelDoors.byTitle[strings.ToLower(model)]; ok {
		return a, true
	}
	t := traffic.ProfileFor(model).Type
	if t == "" {
		return 0, false
	}
	a, ok := modelDoors.byType[strings.ToUpper(t)]
	return a, ok
}
