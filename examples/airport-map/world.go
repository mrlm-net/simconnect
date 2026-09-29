//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The traffic picture on the map (#366): GET /api/world gives the centre,
// radius, airports in range and every aircraft; POST /api/world sets the
// centre ({"follow":true}, {"icao":"LKPR"} or {"lat":..,"lon":..}) and the
// radius ({"radiusNM":250}).

type worldView struct {
	Centre   airport.LatLon            `json:"centre"`
	HasCtr   bool                      `json:"hasCentre"`
	Follow   bool                      `json:"follow"`
	ICAO     string                    `json:"icao,omitempty"`
	RadiusNM float64                   `json:"radiusNM"`
	Airports []traffic.AirportRef      `json:"airports"`
	Aircraft []traffic.TrackedAircraft `json:"aircraft"`
}

func registerWorld(mux *http.ServeMux, st *state) {
	center := func(w http.ResponseWriter) *controlCenter {
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		if cc == nil {
			http.Error(w, "simulator not connected", http.StatusServiceUnavailable)
		}
		return cc
	}
	mux.HandleFunc("GET /api/world", func(w http.ResponseWriter, r *http.Request) {
		cc := center(w)
		if cc == nil {
			return
		}
		c, ok := cc.world.Centre()
		o := cc.world.Options()
		v := worldView{Centre: c, HasCtr: ok, Follow: o.Centre.FollowUser, ICAO: o.Centre.ICAO, RadiusNM: o.RadiusNM,
			Airports: cc.world.Airports(), Aircraft: cc.world.Aircraft()}
		if v.Airports == nil {
			v.Airports = []traffic.AirportRef{}
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("POST /api/world", func(w http.ResponseWriter, r *http.Request) {
		cc := center(w)
		if cc == nil {
			return
		}
		var req struct {
			Follow   bool    `json:"follow"`
			ICAO     string  `json:"icao"`
			Lat      float64 `json:"lat"`
			Lon      float64 `json:"lon"`
			RadiusNM float64 `json:"radiusNM"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch {
		case req.Follow:
			cc.world.SetCentre(traffic.Centre{FollowUser: true})
		case req.ICAO != "":
			icao := strings.ToUpper(strings.TrimSpace(req.ICAO))
			c := traffic.Centre{ICAO: icao}
			if l, ok := st.cache.Layout(icao); ok {
				c.Position = airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}
			}
			cc.world.SetCentre(c)
		case req.Lat != 0 || req.Lon != 0:
			cc.world.SetCentre(traffic.Centre{Position: airport.LatLon{Lat: req.Lat, Lon: req.Lon}})
		}
		if req.RadiusNM > 0 {
			cc.world.SetRadius(req.RadiusNM)
		}
		tlog.printf("traffic picture: centre %+v, radius %.0f NM", cc.world.Options().Centre, cc.world.Options().RadiusNM)
		w.WriteHeader(http.StatusNoContent)
	})
}
