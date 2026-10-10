package world

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The ground vehicles of our traffic by kind, each on or off (all on by
// default): off, none of that kind is created from then on and nobody
// waits for one. Tugs off: departures push back on their own along the
// planned push. Fuel trucks, stairs, GPUs, buses off: the turnaround runs
// without them. Follow-me off: arrivals go without one. Vehicles already
// at work finish their job and are not replaced.

// VehicleKinds are the kinds a World creates.
var VehicleKinds = []traffic.VehicleKind{traffic.VehicleTug, traffic.VehicleFuel, traffic.VehicleStairs,
	traffic.VehicleGPU, traffic.VehicleBus, traffic.VehicleFollowMe}

type vehicleSwitches struct {
	mu  sync.Mutex
	off map[traffic.VehicleKind]bool
}

func (k *core) vehicles() *vehicleSwitches {
	k.vehiclesOnce.Do(func() { k.vehicleSw = &vehicleSwitches{off: map[traffic.VehicleKind]bool{}} })
	return k.vehicleSw
}

// vehicleOn reports whether vehicles of kind are created.
func (k *core) vehicleOn(kind traffic.VehicleKind) bool {
	v := k.vehicles()
	v.mu.Lock()
	defer v.mu.Unlock()
	return !v.off[kind]
}

// GroundVehicles are the kinds on and off now.
func (w *World) GroundVehicles() map[traffic.VehicleKind]bool {
	v := w.st.core.vehicles()
	v.mu.Lock()
	defer v.mu.Unlock()
	out := map[traffic.VehicleKind]bool{}
	for _, k := range VehicleKinds {
		out[k] = !v.off[k]
	}
	return out
}

// SetGroundVehicles turns the kinds given on or off (the others as they
// are).
func (w *World) SetGroundVehicles(kinds map[traffic.VehicleKind]bool) {
	v := w.st.core.vehicles()
	v.mu.Lock()
	for k, on := range kinds {
		v.off[k] = !on
	}
	v.mu.Unlock()
	for k, on := range kinds {
		w.st.core.log.printf("ground vehicles: %s %s", k, map[bool]string{true: "on", false: "off"}[on])
	}
}

// registerVehicles serves GET /api/vehicles (each kind on or off) and
// POST /api/vehicles {"tug": false, …}.
func registerVehicles(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/vehicles", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, w.GroundVehicles())
	})
	mux.HandleFunc("POST /api/vehicles", func(rw http.ResponseWriter, r *http.Request) {
		var req map[traffic.VehicleKind]bool
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		w.SetGroundVehicles(req)
		writeJSON(rw, w.GroundVehicles())
	})
}
