package traffic

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// A departure's or arrival's monitor request is its frame clock: every
// frame it is driven, the simulator answers it and the controller steps.
// Once the injector places the aircraft, what the full monitor reads back
// (its position, heading, lights) is what was placed and commanded: so
// then only the simulation time is read (the tick definition), the rest
// taken from the injector, and a dozen SimVars a frame are not read on the
// simulator's main thread for each driven aircraft. The full monitor comes
// back whenever the aircraft is not placed by us (MSFS AI flies it,
// released, or not yet placed), and is read at least once first.

// Tick definitions, in each controller's ID block beside its monitor.
const (
	defOffTick = 5 // TaxiController
	arrDefTick = 6 // ArrivalController
)

// monitorReq is a controller's monitor request as last asked for.
type monitorReq struct {
	asked    bool
	tick     bool // the tick definition asked for
	haveFull bool // a full monitor answer had
	period   types.SIMCONNECT_PERIOD
	interval uint32
}

type monitorClient interface {
	RequestDataOnSimObject(uint32, uint32, uint32, types.SIMCONNECT_PERIOD, types.SIMCONNECT_DATA_REQUEST_FLAG, uint32, uint32, uint32) error
}

// request asks for objectID's monitor at period and interval, with the
// tick definition when tick, else the full one.
func (r *monitorReq) request(client monitorClient, req, full, tickDef, objectID uint32, period types.SIMCONNECT_PERIOD, interval uint32, tick bool) error {
	r.asked, r.tick, r.period, r.interval = true, tick, period, interval
	def := full
	if tick {
		def = tickDef
	}
	return client.RequestDataOnSimObject(req, def, objectID, period, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, interval, 0)
}

// def is the definition asked for last.
func (r *monitorReq) def(full, tickDef uint32) uint32 {
	if r.tick {
		return tickDef
	}
	return full
}

// tickTime is the simulation time a tick answer carries.
func tickTime(d *types.SIMCONNECT_RECV_SIMOBJECT_DATA) float64 {
	return *engine.CastDataAs[float64](&d.DwData)
}

// Shown is how objectID was last placed and the ground under it, while it
// is taken over and placed; ok false otherwise.
func (i *Injector) Shown(objectID uint32) (p types.SIMCONNECT_DATA_INITPOSITION, groundFt float64, ok bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	o := i.objects[objectID]
	if o == nil || !o.taken || !o.rate.haveLast {
		return p, 0, false
	}
	return o.rate.last, o.groundFt, true
}

// ShownLights is the lights last set on objectID (SetLights); ok false
// before any.
func (i *Injector) ShownLights(objectID uint32) (Lights, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	o := i.objects[objectID]
	if o == nil || !o.taken || !o.lightsSent {
		return Lights{}, false
	}
	return o.lights, true
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// tickable reports whether the controller's aircraft can be clocked by the
// tick: placed by the injector, a full answer had.
func tickable(r *monitorReq, inj *Injector, objectID uint32) bool {
	if inj == nil || objectID == 0 || !r.haveFull {
		return false
	}
	_, _, ok := inj.Shown(objectID)
	return ok
}

// requestMonitor asks for the departure's monitor (every period frames at
// interval), by the tick once it can.
func (c *TaxiController) requestMonitor(client monitorClient, period types.SIMCONNECT_PERIOD, interval uint32) error {
	return c.mon.request(client, c.reqBase+reqOffMonitor, c.defBase+defOffMonitor, c.defBase+defOffTick, c.objectID, period, interval, tickable(&c.mon, c.inj, c.objectID))
}

// syncMonitor switches the monitor between the tick and the full one as
// the aircraft is placed by us or not, at the same rate.
func (c *TaxiController) syncMonitor() {
	if !c.mon.asked || c.monitorStopped || c.objectID == 0 || c.state.Terminal() {
		return
	}
	if t := tickable(&c.mon, c.inj, c.objectID); t != c.mon.tick {
		if client := c.fleet.clientOrNil(); client != nil {
			c.note("monitor definition", c.requestMonitor(client, c.mon.period, c.mon.interval))
		}
	}
}

// tickMonitor is the departure's monitor at a tick: the last full answer
// with the time, where it was placed and the lights set.
func (c *TaxiController) tickMonitor(simTime float64) taxiMonitor {
	m := c.monLast
	m.SimTime = simTime
	if c.inj == nil {
		return m
	}
	if p, _, ok := c.inj.Shown(c.objectID); ok {
		m.Latitude, m.Longitude, m.Heading, m.OnGround = p.Latitude, p.Longitude, p.Heading, float64(p.OnGround)
	}
	if l, ok := c.inj.ShownLights(c.objectID); ok {
		m.Lights = [7]float64{boolF(l.Landing), boolF(l.Taxi), boolF(l.Strobe), boolF(l.Beacon), boolF(l.Nav), boolF(l.Logo), boolF(l.Wing)}
	}
	return m
}

// requestMonitor asks for the arrival's monitor, by the tick once it can.
func (c *ArrivalController) requestMonitor(client monitorClient, period types.SIMCONNECT_PERIOD, interval uint32) error {
	return c.mon.request(client, c.reqBase+arrReqMonitor, c.defBase+arrDefMonitor, c.defBase+arrDefTick, c.objectID, period, interval, c.tickable())
}

// tickable: placed by the injector, not flying a procedure (which reads
// the monitor itself).
func (c *ArrivalController) tickable() bool {
	return !c.flyingProc && tickable(&c.mon, c.inj, c.objectID)
}

// syncMonitor switches the monitor between the tick and the full one.
func (c *ArrivalController) syncMonitor() {
	if !c.mon.asked || c.monitorStopped || c.objectID == 0 || c.state.Terminal() && c.fm == nil {
		return
	}
	if t := c.tickable(); t != c.mon.tick {
		if client := c.fleet.clientOrNil(); client != nil {
			c.note("monitor definition", c.requestMonitor(client, c.mon.period, c.mon.interval))
		}
	}
}

// tickMonitor is the arrival's monitor at a tick: the last full answer
// with the time, where it was placed, its height and the lights set.
func (c *ArrivalController) tickMonitor(simTime float64) arrivalMonitor {
	m := c.monLast
	m.SimTime = simTime
	if c.inj == nil {
		return m
	}
	if p, ground, ok := c.inj.Shown(c.objectID); ok {
		m.Latitude, m.Longitude, m.Heading, m.OnGround = p.Latitude, p.Longitude, p.Heading, float64(p.OnGround)
		m.AltFt, m.AGL = p.Altitude, p.Altitude-ground
	}
	if l, ok := c.inj.ShownLights(c.objectID); ok {
		m.Lights = [5]float64{boolF(l.Landing), boolF(l.Taxi), boolF(l.Strobe), boolF(l.Beacon), boolF(l.Nav)}
		m.Logo, m.Wing = boolF(l.Logo), boolF(l.Wing)
	}
	return m
}
