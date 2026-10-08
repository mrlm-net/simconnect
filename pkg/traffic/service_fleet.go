package traffic

import (
	"math"
	"sync"
)

// Service vehicle fleets (#830): an airport has so many pushback tugs and
// fuel trucks, not one per departure. A departure takes one before it is
// sent and gives it back once it has driven off (or the flight is
// cancelled); with none free it waits: the push waits for its tug, and
// fuelling, optional, is left out when no truck frees up in time.

// VehicleKind is a kind of service vehicle.
type VehicleKind string

const (
	VehicleTug      VehicleKind = "tug"
	VehicleFuel     VehicleKind = "fuel"
	VehicleStairs   VehicleKind = "stairs"
	VehicleGPU      VehicleKind = "gpu"
	VehicleBus      VehicleKind = "bus"      // #887
	VehicleFollowMe VehicleKind = "followme" // #890
)

// ServiceFleet hands out an airport's service vehicles.
type ServiceFleet interface {
	// Take reserves one of kind for owner (a call sign): true when owner
	// holds one now (already, or one was free).
	Take(kind VehicleKind, owner string) bool
	// Give returns owner's; nothing when it holds none.
	Give(kind VehicleKind, owner string)
}

// VehicleFleet is a ServiceFleet of so many vehicles of each kind (0: as
// many as wanted).
type VehicleFleet struct {
	mu      sync.Mutex
	size    map[VehicleKind]int
	out     map[VehicleKind]map[string]bool
	waiting map[VehicleKind]map[string]bool
	// OnWait is told when owner first waits for one of kind, all busy of
	// them busy (nil: not told).
	OnWait func(kind VehicleKind, owner string, busy int)
}

// NewVehicleFleet is a fleet of size vehicles by kind.
func NewVehicleFleet(size map[VehicleKind]int) *VehicleFleet {
	return &VehicleFleet{size: size, out: map[VehicleKind]map[string]bool{}, waiting: map[VehicleKind]map[string]bool{}}
}

// Take implements ServiceFleet.
func (f *VehicleFleet) Take(kind VehicleKind, owner string) bool {
	f.mu.Lock()
	if f.out[kind] == nil {
		f.out[kind], f.waiting[kind] = map[string]bool{}, map[string]bool{}
	}
	if f.out[kind][owner] {
		f.mu.Unlock()
		return true
	}
	if n := f.size[kind]; n > 0 && len(f.out[kind]) >= n {
		first := !f.waiting[kind][owner]
		f.waiting[kind][owner] = true
		busy, on := len(f.out[kind]), f.OnWait
		f.mu.Unlock()
		if first && on != nil {
			on(kind, owner, busy)
		}
		return false
	}
	f.out[kind][owner] = true
	delete(f.waiting[kind], owner)
	f.mu.Unlock()
	return true
}

// Give implements ServiceFleet.
func (f *VehicleFleet) Give(kind VehicleKind, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.out[kind], owner)
	delete(f.waiting[kind], owner)
}

// Out is how many of kind are out, and the fleet's size (0: unlimited).
func (f *VehicleFleet) Out(kind VehicleKind) (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.out[kind]), f.size[kind]
}

// Fleet sizes by stands (DefaultFleetSize): one tug per TugsPerStands
// stands, one fuel truck per FuelTrucksPerStands, at least MinTugs and
// MinFuelTrucks.
const (
	TugsPerStands       = 10.0
	FuelTrucksPerStands = 15.0
	MinTugs             = 2
	MinFuelTrucks       = 1
	StairsPerStands     = 10.0
	MinStairs           = 2
	// BusesPerStairs: buses per set of stairs (#887), enough for two large
	// aircraft boarding at once on every set.
	BusesPerStairs = BusesLarge
	// Follow-me cars (#890): one per FollowMePerStands stands, at least
	// MinFollowMe.
	FollowMePerStands = 30.0
	MinFollowMe       = 1
)

// DefaultFleetSize is an airport's fleet for its number of stands.
func DefaultFleetSize(stands int) map[VehicleKind]int {
	return map[VehicleKind]int{
		VehicleTug:  max(MinTugs, int(math.Ceil(float64(stands)/TugsPerStands))),
		VehicleFuel: max(MinFuelTrucks, int(math.Ceil(float64(stands)/FuelTrucksPerStands))),
		// Stairs serve the remote stands only: one set per StairsPerStands.
		VehicleStairs:   max(MinStairs, int(math.Ceil(float64(stands)/StairsPerStands))),
		VehicleGPU:      max(MinStairs, int(math.Ceil(float64(stands)/StairsPerStands))),
		VehicleBus:      BusesPerStairs * max(MinStairs, int(math.Ceil(float64(stands)/StairsPerStands))),
		VehicleFollowMe: max(MinFollowMe, int(math.Ceil(float64(stands)/FollowMePerStands))),
	}
}
