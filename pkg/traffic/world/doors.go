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
}

var modelDoors = &doorTable{byTitle: map[string]float64{}, byType: map[string]float64{}}

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
	modelDoors.mu.Lock()
	modelDoors.byTitle, modelDoors.byType = t, y
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
