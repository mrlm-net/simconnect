package worldtest

import (
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Client events: the lights set on an aircraft (NAV_LIGHTS_SET, …) are
// kept on it and read back by their SimVars (LIGHT NAV, …), as the
// simulator does; other events do nothing.

// lightEvents are the light events by the SimVar each sets.
var lightEvents = map[string]string{
	"NAV_LIGHTS_SET": "LIGHT NAV", "BEACON_LIGHTS_SET": "LIGHT BEACON", "STROBES_SET": "LIGHT STROBE",
	"TAXI_LIGHTS_SET": "LIGHT TAXI", "LANDING_LIGHTS_SET": "LIGHT LANDING", "LOGO_LIGHTS_SET": "LIGHT LOGO",
	"WING_LIGHTS_SET": "LIGHT WING",
}

// MapClientEventToSimEvent notes the event's name.
func (l *Loopback) MapClientEventToSimEvent(eventID uint32, eventName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.events == nil {
		l.events = map[uint32]string{}
	}
	l.events[eventID] = strings.ToUpper(eventName)
	return nil
}

// SetLocalOffset is how far the local time is ahead of zulu, seconds.
func (l *Loopback) SetLocalOffset(sec float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.localOffset = sec
}

// setZulu is ZULU_HOURS_SET or ZULU_MINUTES_SET: the clock moves, the rest of it
// stays. Under l.mu.
func (l *Loopback) setZulu(name string, v uint32) {
	now := l.clock().UTC()
	h, m := now.Hour(), now.Minute()
	at := float64(h*3600+m*60) + l.zuluShift
	at = math.Mod(math.Mod(at, 86400)+86400, 86400)
	ch, cm := int(at)/3600, int(at)%3600/60
	if name == "ZULU_HOURS_SET" {
		ch = int(v)
	} else {
		cm = int(v)
	}
	l.zuluShift = float64(ch*3600+cm*60) - float64(h*3600+m*60)
}

// TransmitClientEvent sets a light of objectID; other events do nothing.
func (l *Loopback) TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if objectID == types.SIMCONNECT_OBJECT_ID_USER {
		objectID = UserObjectID
	}
	if name := l.events[eventID]; name == "ZULU_HOURS_SET" || name == "ZULU_MINUTES_SET" {
		l.setZulu(name, data)
		return nil
	}
	v, ok := lightEvents[l.events[eventID]]
	o := l.objects[objectID]
	if !ok || o == nil {
		return nil
	}
	if o.Lights == nil {
		o.Lights = map[string]bool{}
	}
	o.Lights[v] = data != 0
	return nil
}
