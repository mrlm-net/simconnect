package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// defUserPlace is the INITPOSITION written on the user aircraft (PlaceUser).
const defUserPlace uint32 = 2017

// placeUserMaxKts: above this ground speed the user aircraft is moving and
// is not placed.
const placeUserMaxKts = 1.0

// Why PlaceUser refuses; each wrapped with the detail.
var (
	ErrUserUnknown    = errors.New("the user aircraft is not known yet")
	ErrUserAirborne   = errors.New("the user aircraft is airborne")
	ErrUserMoving     = errors.New("the user aircraft is moving")
	ErrUserEngines    = errors.New("the user aircraft's engines are running")
	ErrStandOccupied  = errors.New("the stand is occupied")
	ErrNotConnectedUs = errors.New("simulator not connected")
)

// PlaceUser puts the user aircraft on stand (its label, "B14") at icao: at
// the point an aircraft parks there (traffic.StandPoint, nose toward the
// stop mark) facing the stand's heading, on the ground at the field
// elevation, stopped. A scenario's start when the player chose to be moved.
// It refuses while the aircraft is airborne, moving (over 1 kt) or has an
// engine running, and when one of ours holds the stand or any aircraft is
// on it or on an overlapping stand in the way; the error says which. The
// stand scan then finds the user there, so none of ours is sent to it.
func (w *World) PlaceUser(icao, stand string) error {
	st := w.st
	st.mu.Lock()
	cc, ac := st.control, st.aircraft
	st.mu.Unlock()
	if cc == nil {
		return ErrNotConnectedUs
	}
	switch {
	case ac == nil:
		return ErrUserUnknown
	case !ac.OnGround:
		return ErrUserAirborne
	case ac.GroundKts > placeUserMaxKts:
		return fmt.Errorf("%w: %.0f kt", ErrUserMoving, ac.GroundKts)
	case ac.EnginesRunning:
		return ErrUserEngines
	}
	g, err := cc.graph(icao)
	if err != nil {
		return err
	}
	l := g.Layout
	si, err := l.ParkingIndex(stand)
	if err != nil {
		return err
	}
	p := l.Parking[si]
	if why := userStandBlocked(cc.allocator(g), l, si, ac.ObjectID); why != "" {
		return fmt.Errorf("%w: %s", ErrStandOccupied, why)
	}
	at := traffic.StandPoint(p, 0)
	pos := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude: at.Lat, Longitude: at.Lon, Altitude: convert.MetersToFeet(l.Altitude),
		Heading: p.Heading, OnGround: 1,
	}
	err = cc.do(func() error {
		if err := cc.client.AddToDataDefinition(defUserPlace, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0); err != nil {
			return err
		}
		defer cc.client.ClearDataDefinition(defUserPlace)
		return cc.client.SetDataOnSimObject(defUserPlace, types.SIMCONNECT_OBJECT_ID_USER, 0, 0, uint32(unsafe.Sizeof(pos)), unsafe.Pointer(&pos))
	})
	if err != nil {
		return err
	}
	cc.log.printf("user aircraft placed on %s %s", l.ICAO, p.Label())
	return nil
}

// userStandBlocked is what keeps the user aircraft (object user) off
// stand ("" if nothing): one of ours holding it, or an aircraft detected on
// it or on an overlapping stand close enough to touch wings. The user
// aircraft itself does not count.
func userStandBlocked(a *traffic.StandAllocator, l *airport.Layout, stand int, user uint32) string {
	occ := a.Occupancy()
	mine := func(o traffic.Occupant) bool { return o.Owner == "" && user != 0 && o.ObjectID == user }
	if o, ok := occ[stand]; ok && !mine(o) {
		return occupantName(o) + " holds " + l.Parking[stand].Label()
	}
	for _, c := range l.ParkingConflicts(stand) {
		o, ok := occ[c]
		if !ok || mine(o) {
			continue
		}
		if traffic.DefaultHalfSpanMeters+o.HalfSpan+traffic.StandWingtipClearanceMeters >
			calc.HaversineMeters(l.Parking[stand].Position.Lat, l.Parking[stand].Position.Lon, l.Parking[c].Position.Lat, l.Parking[c].Position.Lon) {
			return occupantName(o) + " on " + l.Parking[c].Label() + " is in the way"
		}
	}
	return ""
}

func occupantName(o traffic.Occupant) string {
	switch {
	case o.Owner != "":
		return o.Owner
	case o.ObjectID != 0:
		return fmt.Sprintf("aircraft %d", o.ObjectID)
	}
	return "an aircraft"
}

// registerPlaceUser serves POST /api/user/place ({"icao", "stand"}):
// PlaceUser. A refusal is 409 with its reason, an unknown stand 404.
func registerPlaceUser(mux *http.ServeMux, w *World) {
	mux.HandleFunc("POST /api/user/place", func(rw http.ResponseWriter, r *http.Request) {
		var req struct {
			ICAO  string `json:"icao"`
			Stand string `json:"stand"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ICAO == "" || req.Stand == "" {
			http.Error(rw, "icao and stand needed", http.StatusBadRequest)
			return
		}
		err := w.PlaceUser(strings.ToUpper(strings.TrimSpace(req.ICAO)), strings.TrimSpace(req.Stand))
		switch {
		case err == nil:
			writeJSON(rw, map[string]any{"placed": true, "icao": req.ICAO, "stand": req.Stand})
		case errors.Is(err, ErrNotConnectedUs):
			http.Error(rw, err.Error(), http.StatusServiceUnavailable)
		case errors.Is(err, airport.ErrAmbiguousParking):
			http.Error(rw, err.Error(), http.StatusBadRequest)
		case errors.Is(err, airport.ErrUnknownParking):
			http.Error(rw, err.Error(), http.StatusNotFound)
		case errors.Is(err, ErrUserUnknown), errors.Is(err, ErrUserAirborne), errors.Is(err, ErrUserMoving),
			errors.Is(err, ErrUserEngines), errors.Is(err, ErrStandOccupied):
			http.Error(rw, err.Error(), http.StatusConflict)
		default:
			http.Error(rw, err.Error(), http.StatusBadGateway)
		}
	})
}
