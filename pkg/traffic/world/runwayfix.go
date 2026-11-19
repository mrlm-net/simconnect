package world

import (
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// placeRunwayFixesLocked puts the runway fixes of icao's stored procedures
// on its cached layout's runway ends (airport.Procedures.AtRunwayEnds:
// the facility data has them at the runway's centre, #1573), so every
// reader of st.procedures — the arrivals' approaches and finals, the SIDs,
// the flight plans, the AI's waypoints, the charts — has them there. It is
// called as procedures or a layout arrive, in whichever order; it returns
// the procedures and whether any fix moved. Call with st.mu held.
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
	placed, moved := p.AtRunwayEnds(l)
	if moved {
		st.procedures[icao] = placed
	}
	return placed, moved
}
