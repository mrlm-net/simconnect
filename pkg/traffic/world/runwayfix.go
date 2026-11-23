package world

import (
	"reflect"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// placeRunwayFixesLocked puts the runway fixes of icao's stored procedures
// on its cached layout's runway ends (airport.Procedures.AtRunwayEnds:
// the facility data has them at the runway's centre, #1573), so every
// reader of st.procedures — the arrivals' approaches and finals, the SIDs,
// the flight plans, the AI's waypoints, the charts — has them there. It is
// called as procedures or a layout arrive, in whichever order, and places
// them from the procedures as read (st.proceduresRead): a placed fix is at
// a threshold, which AtRunwayEnds keeps, so it would stay behind when the
// layout changes (#1646). It returns the procedures and whether they
// changed. Call with st.mu held.
func (st *state) placeRunwayFixesLocked(icao string) (airport.Procedures, bool) {
	icao = strings.ToUpper(icao)
	p, ok := st.procedures[icao]
	if !ok || st.cache == nil {
		return p, false
	}
	l, ok := st.cache.Layout(icao)
	if !ok {
		return p, false
	}
	read, ok := st.proceduresRead[icao]
	if !ok {
		read = p
	}
	placed, _ := read.AtRunwayEnds(l)
	if reflect.DeepEqual(placed, p) {
		return p, false
	}
	st.procedures[icao] = placed
	return placed, true
}

// storeProceduresLocked keeps p as read, and with its runway fixes placed
// on the cached layout's runway ends (placeRunwayFixesLocked); it returns
// those and whether any fix moved. Call with st.mu held.
func (st *state) storeProceduresLocked(p airport.Procedures) (airport.Procedures, bool) {
	if st.procedures == nil {
		st.procedures = map[string]airport.Procedures{}
	}
	if st.proceduresRead == nil {
		st.proceduresRead = map[string]airport.Procedures{}
	}
	st.procedures[p.ICAO] = p
	st.proceduresRead[strings.ToUpper(p.ICAO)] = p
	return st.placeRunwayFixesLocked(p.ICAO)
}
