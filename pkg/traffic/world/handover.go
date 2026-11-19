package world

import "github.com/mrlm-net/simconnect/pkg/traffic"

// lastLook: ev is a departure's last event, handed to the simulator's AI
// (TaxiComplete); no later one comes. Out of a staffed station the player
// hands an aircraft over, but waiting for that here the departure climbed
// away silent, neither handed over by the tower nor checking in with
// departure (#1572): the station hands it over now, the transmission, its
// readback and the check-in said before the flight is done. it.mu held.
func (it *controlled) lastLook(ev TaxiOrArrival, pos traffic.Position) bool {
	if ev.dep == nil || ev.dep.State != traffic.TaxiComplete {
		return false
	}
	it.cc.log.printf("%-6s handed to the simulator's AI: the staffed %s hands it to %s", it.Tail, it.atc, pos)
	return true
}
