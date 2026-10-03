//go:build windows
// +build windows

// Package avionics sets the user aircraft's radios: COM active and standby
// frequencies, the swap, and the transponder code, through the simulator's
// key events (MSFS 2024 "Aircraft Radio Navigation Events":
// COM_STBY_RADIO_SET_HZ, COM2_/COM3_STBY_RADIO_SET_HZ, COM_RADIO_SET_HZ,
// COM2_/COM3_RADIO_SET_HZ, COM1_/COM2_/COM3_RADIO_SWAP, XPNDR_SET).
package avionics

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/systems"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Client is what Radios needs of a connection (engine.Engine, a manager
// instance).
type Client interface {
	MapClientEventToSimEvent(eventID uint32, eventName string) error
	TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error
}

// DefaultEventBase is the first client event ID Radios maps (it uses
// eventCount IDs from there).
const DefaultEventBase uint32 = 0x7A00

const (
	evCOM1Stby = iota
	evCOM2Stby
	evCOM3Stby
	evCOM1Active
	evCOM2Active
	evCOM3Active
	evCOM1Swap
	evCOM2Swap
	evCOM3Swap
	evXPNDR
	eventCount
)

var eventNames = [eventCount]string{
	"COM_STBY_RADIO_SET_HZ", "COM2_STBY_RADIO_SET_HZ", "COM3_STBY_RADIO_SET_HZ",
	"COM_RADIO_SET_HZ", "COM2_RADIO_SET_HZ", "COM3_RADIO_SET_HZ",
	"COM1_RADIO_SWAP", "COM2_RADIO_SWAP", "COM3_RADIO_SWAP",
	"XPNDR_SET",
}

// ErrBadRadio: no such COM (1–3); ErrBadFrequency: outside the airband;
// ErrBadSquawk: not four octal digits.
var (
	ErrBadRadio     = errors.New("avionics: COM radio must be 1, 2 or 3")
	ErrBadFrequency = errors.New("avionics: frequency outside 118.000–136.990 MHz")
	ErrBadSquawk    = errors.New("avionics: squawk must be four digits 0–7")
)

// Radios sets the user aircraft's radios. Its events are mapped on first
// use; after a reconnect, call Reset so they are mapped again.
type Radios struct {
	client Client
	base   uint32

	mu     sync.Mutex
	mapped bool
	// actions: the model's (Use); pressDefs: the data definition of each
	// button variable pressed.
	actions   map[string]systems.Action
	pressDefs map[string]uint32
}

// New returns Radios on client, its events from base (0: DefaultEventBase).
func New(client Client, base uint32) *Radios {
	if base == 0 {
		base = DefaultEventBase
	}
	return &Radios{client: client, base: base}
}

// Reset forgets the event mapping (a new connection).
func (r *Radios) Reset() {
	r.mu.Lock()
	r.mapped = false
	r.mu.Unlock()
}

func (r *Radios) send(ev int, data uint32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.mapped {
		for i, name := range eventNames {
			if err := r.client.MapClientEventToSimEvent(r.base+uint32(i), name); err != nil {
				return fmt.Errorf("avionics: mapping %s: %w", name, err)
			}
		}
		r.mapped = true
	}
	return r.client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, r.base+uint32(ev), data,
		types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
}

// SetCOMStandby sets COM n's (1–3) standby frequency, in MHz (e.g. 134.560;
// 8.33 kHz channels included).
func (r *Radios) SetCOMStandby(n int, mhz float64) error {
	hz, err := comHz(n, mhz)
	if err != nil {
		return err
	}
	return r.send(evCOM1Stby+n-1, hz)
}

// SetCOMActive sets COM n's (1–3) active frequency, in MHz.
func (r *Radios) SetCOMActive(n int, mhz float64) error {
	hz, err := comHz(n, mhz)
	if err != nil {
		return err
	}
	return r.send(evCOM1Active+n-1, hz)
}

// SwapCOM swaps COM n's (1–3) active and standby frequencies: with the
// key event, or the model's transfer key when its actions give one (Use).
func (r *Radios) SwapCOM(n int) error {
	if n < 1 || n > 3 {
		return ErrBadRadio
	}
	r.mu.Lock()
	a, ok := r.actions[fmt.Sprintf("com%dSwap", n)]
	if ok && a.Press != "" {
		defer r.mu.Unlock()
		return r.press(a.Press) // the model's transfer key
	}
	r.mu.Unlock()
	return r.send(evCOM1Swap+n-1, 0)
}

// SetSquawk sets the transponder code, e.g. "4521" (MSFS has one
// transponder).
func (r *Radios) SetSquawk(code string) error {
	bcd, err := SquawkBCD(code)
	if err != nil {
		return err
	}
	return r.send(evXPNDR, bcd)
}

// comHz checks n and mhz and gives the frequency in Hz.
func comHz(n int, mhz float64) (uint32, error) {
	if n < 1 || n > 3 {
		return 0, ErrBadRadio
	}
	if mhz < 118 || mhz >= 137 {
		return 0, ErrBadFrequency
	}
	return uint32(math.Round(mhz*1000)) * 1000, nil
}

// SquawkBCD encodes a squawk as XPNDR_SET takes it, BCD16: "7000" →
// 0x7000.
func SquawkBCD(code string) (uint32, error) {
	if len(code) != 4 {
		return 0, ErrBadSquawk
	}
	v := uint32(0)
	for _, c := range code {
		if c < '0' || c > '7' {
			return 0, ErrBadSquawk
		}
		v = v<<4 | uint32(c-'0')
	}
	return v, nil
}

// Presser is what Radios needs of a client to press a model's button
// variable (an L:var) instead of a key event: engine.Engine and the manager
// have both.
type Presser interface {
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error
}

// Use takes a model's actions (systems.Profile.Actions): a COM swap that is
// a button press there ("com1Swap": the Fenix's RMP transfer key) replaces
// the swap event. nil goes back to the key events.
func (r *Radios) Use(actions map[string]systems.Action) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions, r.pressDefs = actions, map[string]uint32{}
}

// press presses var name: 1, then 0 after pressHold, as a click. r.mu held
// by the caller is released while it waits.
func (r *Radios) press(name string) error {
	p, ok := r.client.(Presser)
	if !ok {
		return errors.New("avionics: the client cannot set variables (Presser)")
	}
	def, ok := r.pressDefs[name]
	if !ok {
		def = r.base + uint32(len(r.pressDefs)) // data definition IDs: their own space
		if err := p.AddToDataDefinition(def, name, "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
			return fmt.Errorf("avionics: define %s: %w", name, err)
		}
		r.pressDefs[name] = def
	}
	for i, v := range []float64{1, 0} {
		val := v
		if err := p.SetDataOnSimObject(def, types.SIMCONNECT_OBJECT_ID_USER, 0, 0, 8, unsafe.Pointer(&val)); err != nil {
			return err
		}
		if i == 0 {
			r.mu.Unlock()
			time.Sleep(pressHold)
			r.mu.Lock()
		}
	}
	return nil
}

// pressHold is how long a pressed button is held.
const pressHold = 300 * time.Millisecond
