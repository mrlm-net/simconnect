package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// stopCountClient counts the data requests stopped.
type stopCountClient struct {
	fakeClient
	stops int
}

func (c *stopCountClient) RequestDataOnSimObject(_, _, _ uint32, period types.SIMCONNECT_PERIOD, _ types.SIMCONNECT_DATA_REQUEST_FLAG, _, _, _ uint32) error {
	if period == types.SIMCONNECT_PERIOD_NEVER {
		c.stops++
	}
	return nil
}

// TestMonitorStoppedOnce: an aircraft parked or handed over has its monitor
// stopped, and removing it later stops it again; the simulator refuses the second
// (live, exception 3 on every such aircraft: 12 of 50 at a hold). Asked for
// frames again, it is stopped again.
func TestMonitorStoppedOnce(t *testing.T) {
	c := &stopCountClient{}
	taxi := &TaxiController{fleet: NewFleet(c), objectID: 42}
	taxi.stopMonitor()
	taxi.stopMonitor()
	if c.stops != 1 {
		t.Errorf("taxi: stopped %d times, want 1", c.stops)
	}
	taxi.monitorStopped = false // as every request for frames does
	taxi.stopMonitor()
	if c.stops != 2 {
		t.Errorf("taxi, started again: stopped %d times, want 2", c.stops)
	}

	c = &stopCountClient{}
	arr := &ArrivalController{fleet: NewFleet(c), objectID: 42}
	arr.stopMonitor()
	arr.stopMonitor()
	if c.stops != 1 {
		t.Errorf("arrival: stopped %d times, want 1", c.stops)
	}
	arr.monitorEvery(types.SIMCONNECT_PERIOD_SECOND)
	arr.stopMonitor()
	if c.stops != 2 {
		t.Errorf("arrival, started again: stopped %d times, want 2", c.stops)
	}
}
