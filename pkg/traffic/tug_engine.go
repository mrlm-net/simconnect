package traffic

// TugEngineResendSeconds is how often a tug's engine is re-sent while it is
// ours. Ground vehicle lights (Asobo GroundVehicles.xml templates, GSX gyro
// lights) burn only while GENERAL ENG COMBUSTION SOUND PERCENT > 1, so a
// vehicle spawned with its engine off drives dark.
const TugEngineResendSeconds = 10.0

// vehicleEngine keeps an injected vehicle's engine running.
type vehicleEngine struct {
	sent bool
	in   float64 // seconds until the next re-send
}

// keep starts the engine on the first call and re-sends it every
// TugEngineResendSeconds. A refused write is retried on the next call and
// never stops the vehicle.
func (e *vehicleEngine) keep(inj *Injector, objectID uint32, dt float64) {
	if inj == nil || objectID == 0 {
		return
	}
	e.in -= dt
	if e.sent && e.in > 0 {
		return
	}
	if err := inj.SetEngines(objectID, 1, true); err != nil {
		e.in = 0
		return
	}
	e.sent = true
	e.in = TugEngineResendSeconds
}
