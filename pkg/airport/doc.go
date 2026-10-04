// Package airport models an airport's ground layout as SimConnect reports it
// through facility data: runways, parking spots, taxi points, taxi paths and
// taxiway names.
//
// A [Layout] is built from raw facility records with [BuildLayout]. Every
// item keeps its SimConnect list index, so taxi paths can refer to taxi points
// and parking spots exactly as the simulator does.
//
// Facility data semantics, verified against MSFS 2024 data for LKPR:
//
//   - TAXI_PATH START and END index the taxi point list, except for PARKING
//     paths, whose END indexes the parking list.
//   - Hold-short points are identified by TAXI_POINT TYPE (HOLD_SHORT,
//     ILS_HOLD_SHORT and their NO_DRAW variants), not by path topology.
//   - Taxiways are TAXI and PATH paths; an airport may use only one of them.
package airport
