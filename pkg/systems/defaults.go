//go:build windows
// +build windows

package systems

// Default is the profile of standard SimVars, for every aircraft that
// follows them (the stock ones; per-model profiles go on top).
func Default() Profile {
	ten := 10.0
	one := func(name, unit string) Value { return Value{Vars: []string{name}, Unit: unit} }
	v := map[string]Value{
		Battery:      one("ELECTRICAL MASTER BATTERY", "bool"),
		Volts:        one("ELECTRICAL MAIN BUS VOLTAGE", "volts"),
		Powered:      {Vars: []string{"ELECTRICAL MAIN BUS VOLTAGE"}, Unit: "volts", AtLeast: &ten, Note: "main bus at 10 V or more"},
		Avionics:     one("AVIONICS MASTER SWITCH", "bool"),
		ExtAvailable: one("EXTERNAL POWER AVAILABLE:1", "bool"),
		ExtOn:        one("EXTERNAL POWER ON:1", "bool"),
		COM1Power:    {Vars: []string{"COM STATUS:1"}, Unit: "enum", TrueAt: []float64{0}, Note: "COM STATUS 0: OK"},
		COM2Power:    {Vars: []string{"COM STATUS:2"}, Unit: "enum", TrueAt: []float64{0}, Note: "COM STATUS 0: OK"},
		EngineCount:  one("NUMBER OF ENGINES", "number"),
		ParkingBrake: one("BRAKE PARKING INDICATOR", "bool"),
		LightBeacon:  one("LIGHT BEACON", "bool"),
		LightNav:     one("LIGHT NAV", "bool"),
		LightStrobe:  one("LIGHT STROBE", "bool"),
		LightLanding: one("LIGHT LANDING", "bool"),
		LightTaxi:    one("LIGHT TAXI", "bool"),
		XPDRState:    one("TRANSPONDER STATE:1", "enum"),
		XPDRCode:     one("TRANSPONDER CODE:1", "Bco16"),
		FlapsPct:     one("FLAPS HANDLE PERCENT", "percent"),
		GearDown:     one("GEAR HANDLE POSITION", "bool"),
		COM1Active:   one("COM ACTIVE FREQUENCY:1", "MHz"),
		COM1Standby:  one("COM STANDBY FREQUENCY:1", "MHz"),
		COM2Active:   one("COM ACTIVE FREQUENCY:2", "MHz"),
		COM2Standby:  one("COM STANDBY FREQUENCY:2", "MHz"),
		// The sim's pushback (#666).
		PushbackAttached:  one("PUSHBACK ATTACHED", "bool"),
		PushbackAvailable: one("PUSHBACK AVAILABLE", "bool"),
		PushbackWait:      one("PUSHBACK WAIT", "bool"),
	}
	for n := 1; n <= 4; n++ {
		v[EngineRunning(n)] = one(fmtIndexed("GENERAL ENG COMBUSTION", n), "bool")
		v[Starter(n)] = one(fmtIndexed("GENERAL ENG STARTER", n), "bool")
	}
	a := map[string]Action{
		// The standard key events (#667): the exit toggled only when it is
		// not as wanted, by its index from 1 (EXIT OPEN:n is exit n+1); the
		// parking brake toggled likewise. No chocks or GPU: a model that has
		// them gives them.
		ParkingBrake: {Event: "PARKING_BRAKES", Toggle: true},
		// The sim's ground services (#666): requests, sent as asked.
		Jetway:      {Event: "TOGGLE_JETWAY"},
		Stairs:      {Event: "TOGGLE_RAMPTRUCK"},
		Baggage:     {Event: "REQUEST_LUGGAGE"},
		Catering:    {Event: "REQUEST_CATERING"},
		PowerSupply: {Event: "REQUEST_POWER_SUPPLY"},
		FuelTruck:   {Event: "REQUEST_FUEL_KEY"},
		Pushback:    {Event: "TOGGLE_PUSHBACK"},
	}
	for n := 0; n <= 3; n++ {
		v[Door(n)] = one(fmtIndexed("EXIT OPEN", n), "percent")
		exit := uint32(n + 1)
		a[Door(n)] = Action{Event: "TOGGLE_AIRCRAFT_EXIT", Toggle: true, Data: &exit}
	}
	return Profile{Name: "default", Values: v, Actions: a, Doors: []string{"Door 1", "Door 2", "Door 3", "Door 4"}}
}

func fmtIndexed(name string, n int) string { return name + ":" + string(rune('0'+n)) }
