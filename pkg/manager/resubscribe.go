//go:build windows
// +build windows

package manager

import (
	"sync"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// userSubs records what the application subscribed through the manager's
// pass-through calls, to subscribe it again on a new connection when
// Config.ResubscribeOnReconnect is on (review E10). Each entry keeps the
// connection generation (connGen) it was last subscribed on: one made on the
// current connection before its OPEN is not subscribed twice.
type userSubs struct {
	mu           sync.Mutex
	flow         uint64 // generation of SubscribeToFlowEvent, 0 = not subscribed
	inputEvents  map[uint64]uint64
	systemEvents map[uint32]systemEventSub
	facilities   map[types.SIMCONNECT_FACILITY_LIST_TYPE]facilitySub
}

type systemEventSub struct {
	name string
	gen  uint64
}

// facilitySub is a facility list subscription: SubscribeToFacilities
// (requestID) or SubscribeToFacilitiesEX1 (newReq, oldReq, with the halves
// still on).
type facilitySub struct {
	ex1       bool
	requestID uint32
	newReq    uint32
	oldReq    uint32
	newOn     bool
	oldOn     bool
	gen       uint64
}

// userSubscriber is the part of engine.Client replayUserSubscriptions uses.
type userSubscriber interface {
	SubscribeToFlowEvent() error
	SubscribeInputEvent(hash uint64) error
	SubscribeToSystemEvent(eventID uint32, eventName string) error
	SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error
	SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newElemInRangeRequestID uint32, oldElemOutRangeRequestID uint32) error
	UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, unsubscribeNewInRange bool, unsubscribeOldOutRange bool) error
}

// recordingLocked reports whether subscriptions are recorded, and the current
// connection generation. m.mu must be held (read is enough).
func (m *Instance) recordingLocked() (bool, uint64) {
	return m.config.ResubscribeOnReconnect, m.connGen
}

func (u *userSubs) reset() {
	u.mu.Lock()
	u.flow = 0
	u.inputEvents = nil
	u.systemEvents = nil
	u.facilities = nil
	u.mu.Unlock()
}

func (u *userSubs) setFlow(gen uint64) {
	u.mu.Lock()
	u.flow = gen
	u.mu.Unlock()
}

func (u *userSubs) addInputEvent(hash uint64, gen uint64) {
	u.mu.Lock()
	if u.inputEvents == nil {
		u.inputEvents = make(map[uint64]uint64)
	}
	u.inputEvents[hash] = gen
	u.mu.Unlock()
}

func (u *userSubs) removeInputEvent(hash uint64) {
	u.mu.Lock()
	delete(u.inputEvents, hash)
	u.mu.Unlock()
}

func (u *userSubs) addSystemEvent(eventID uint32, name string, gen uint64) {
	u.mu.Lock()
	if u.systemEvents == nil {
		u.systemEvents = make(map[uint32]systemEventSub)
	}
	u.systemEvents[eventID] = systemEventSub{name: name, gen: gen}
	u.mu.Unlock()
}

func (u *userSubs) removeSystemEvent(eventID uint32) {
	u.mu.Lock()
	delete(u.systemEvents, eventID)
	u.mu.Unlock()
}

func (u *userSubs) setFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, f facilitySub) {
	u.mu.Lock()
	if u.facilities == nil {
		u.facilities = make(map[types.SIMCONNECT_FACILITY_LIST_TYPE]facilitySub)
	}
	u.facilities[listType] = f
	u.mu.Unlock()
}

// unsubscribeFacilities records UnsubscribeToFacilitiesEX1: the halves
// turned off; the list type is forgotten once nothing of it is on (a
// SubscribeToFacilities one when both halves are turned off).
func (u *userSubs) unsubscribeFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, newInRange, oldOutRange bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	f, ok := u.facilities[listType]
	if !ok {
		return
	}
	if !f.ex1 {
		if newInRange && oldOutRange {
			delete(u.facilities, listType)
		}
		return
	}
	if newInRange {
		f.newOn = false
	}
	if oldOutRange {
		f.oldOn = false
	}
	if !f.newOn && !f.oldOn {
		delete(u.facilities, listType)
		return
	}
	u.facilities[listType] = f
}

// replayUserSubscriptions subscribes again on client what was recorded and
// not yet subscribed on the current connection (Config.ResubscribeOnReconnect,
// review E10). Off by default: nothing is recorded then.
func (m *Instance) replayUserSubscriptions(client userSubscriber) {
	m.mu.RLock()
	on, gen := m.recordingLocked()
	m.mu.RUnlock()
	if !on {
		return
	}

	u := &m.userSubs
	u.mu.Lock()
	flow := u.flow != 0 && u.flow != gen
	inputs := make([]uint64, 0, len(u.inputEvents))
	for h, g := range u.inputEvents {
		if g != gen {
			inputs = append(inputs, h)
		}
	}
	type sysEv struct {
		id   uint32
		name string
	}
	sys := make([]sysEv, 0, len(u.systemEvents))
	for id, s := range u.systemEvents {
		if s.gen != gen {
			sys = append(sys, sysEv{id, s.name})
		}
	}
	type facEv struct {
		listType types.SIMCONNECT_FACILITY_LIST_TYPE
		f        facilitySub
	}
	facs := make([]facEv, 0, len(u.facilities))
	for lt, f := range u.facilities {
		if f.gen != gen {
			facs = append(facs, facEv{lt, f})
		}
	}
	u.mu.Unlock()

	if flow {
		if err := client.SubscribeToFlowEvent(); err != nil {
			m.logger.Error("[manager] Failed to subscribe flow events again", "error", err)
		} else {
			u.mu.Lock()
			if u.flow != 0 {
				u.flow = gen
			}
			u.mu.Unlock()
		}
	}
	for _, h := range inputs {
		if err := client.SubscribeInputEvent(h); err != nil {
			m.logger.Error("[manager] Failed to subscribe an input event again", "hash", h, "error", err)
			continue
		}
		u.mu.Lock()
		if _, ok := u.inputEvents[h]; ok {
			u.inputEvents[h] = gen
		}
		u.mu.Unlock()
	}
	for _, s := range sys {
		if err := client.SubscribeToSystemEvent(s.id, s.name); err != nil {
			m.logger.Error("[manager] Failed to subscribe a system event again", "event", s.name, "id", s.id, "error", err)
			continue
		}
		u.mu.Lock()
		if cur, ok := u.systemEvents[s.id]; ok && cur.name == s.name {
			cur.gen = gen
			u.systemEvents[s.id] = cur
		}
		u.mu.Unlock()
	}
	for _, fe := range facs {
		var err error
		if fe.f.ex1 {
			err = client.SubscribeToFacilitiesEX1(fe.listType, fe.f.newReq, fe.f.oldReq)
			if err == nil && (!fe.f.newOn || !fe.f.oldOn) {
				// one half was turned off: off again
				err = client.UnsubscribeToFacilitiesEX1(fe.listType, !fe.f.newOn, !fe.f.oldOn)
			}
		} else {
			err = client.SubscribeToFacilities(fe.listType, fe.f.requestID)
		}
		if err != nil {
			m.logger.Error("[manager] Failed to subscribe facilities again", "listType", fe.listType, "error", err)
			continue
		}
		u.mu.Lock()
		if cur, ok := u.facilities[fe.listType]; ok && cur == fe.f {
			cur.gen = gen
			u.facilities[fe.listType] = cur
		}
		u.mu.Unlock()
	}
}
