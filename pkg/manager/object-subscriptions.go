//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/subscriptions"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// ObjectEvent represents an object add/remove event
type ObjectEvent struct {
	ObjectID uint32
	ObjType  types.SIMCONNECT_SIMOBJECT_TYPE
}

// ObjectSubscription is a typed subscription for object add/remove events
type ObjectSubscription interface {
	ID() string
	Events() <-chan ObjectEvent
	Done() <-chan struct{}
	Unsubscribe()
}

// SubscribeOnObjectAdded returns a subscription delivering ObjectAdded events
func (m *Instance) SubscribeOnObjectAdded(id string, bufferSize int) ObjectSubscription {
	return m.subscribeObjectEvent(id, bufferSize, "-objadded", "ObjectAdded", m.objectAddedEventID)
}

// SubscribeOnObjectRemoved returns a subscription delivering ObjectRemoved events
func (m *Instance) SubscribeOnObjectRemoved(id string, bufferSize int) ObjectSubscription {
	return m.subscribeObjectEvent(id, bufferSize, "-objremoved", "ObjectRemoved", m.objectRemovedEventID)
}

// subscribeObjectEvent is an object subscription for eventID. Its message
// subscription is id+suffix, a suffix of its own per kind: ObjectAdded and
// ObjectRemoved with the same id do not replace each other (review #19).
func (m *Instance) subscribeObjectEvent(id string, bufferSize int, suffix, kind string, eventID uint32) ObjectSubscription {
	id = subscriptions.GenerateID(id)
	bufferSize = subscriptions.ValidateBufferSize(bufferSize)
	msgSub := m.SubscribeWithType(id+suffix, bufferSize, []types.SIMCONNECT_RECV_ID{types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE})
	os := newTypedSubscription[ObjectEvent](id, msgSub, bufferSize)
	forwardTyped(m, os, kind, func(msg engine.Message) (ObjectEvent, bool) {
		o := msg.AsEventObjectAddRemove()
		if o == nil || o.UEventID != types.DWORD(eventID) {
			return ObjectEvent{}, false
		}
		return ObjectEvent{ObjectID: uint32(o.DwData), ObjType: o.EObjType}, true
	})
	return os
}
