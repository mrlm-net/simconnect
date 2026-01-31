//go:build windows
// +build windows

package manager

import (
	"sync"

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

type objectSubscription struct {
	id      string
	sub     Subscription
	ch      chan ObjectEvent
	done    chan struct{}
	mgr     *Instance
	closeMu sync.Mutex
}

func (s *objectSubscription) ID() string                 { return s.id }
func (s *objectSubscription) Events() <-chan ObjectEvent { return s.ch }
func (s *objectSubscription) Done() <-chan struct{}      { return s.done }
func (s *objectSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.sub != nil {
		s.sub.Unsubscribe()
		s.sub = nil
	}
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	select {
	case <-s.ch:
	default:
		close(s.ch)
	}
}

// SubscribeOnObjectAdded returns a subscription delivering ObjectAdded events
func (m *Instance) SubscribeOnObjectAdded(id string, bufferSize int) ObjectSubscription {
	if id == "" {
		id = generateUUID()
	}
	msgSub := m.SubscribeWithType(id+"-obj", bufferSize, types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE)
	os := &objectSubscription{id: id, sub: msgSub, ch: make(chan ObjectEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer os.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-os.sub.Done():
				return
			case msg, ok := <-os.sub.Messages():
				if !ok {
					return
				}
				o := msg.AsEventObjectAddRemove()
				if o == nil {
					continue
				}
				if o.UEventID != types.DWORD(m.objectAddedEventID) {
					continue
				}
				select {
				case os.ch <- ObjectEvent{ObjectID: uint32(o.DwData), ObjType: o.EObjType}:
				default:
					m.logger.Debug("[manager] ObjectAdded subscription channel full, dropping event")
				}
			}
		}
	}()
	return os
}

// SubscribeOnObjectRemoved returns a subscription delivering ObjectRemoved events
func (m *Instance) SubscribeOnObjectRemoved(id string, bufferSize int) ObjectSubscription {
	if id == "" {
		id = generateUUID()
	}
	msgSub := m.SubscribeWithType(id+"-obj", bufferSize, types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE)
	os := &objectSubscription{id: id, sub: msgSub, ch: make(chan ObjectEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer os.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-os.sub.Done():
				return
			case msg, ok := <-os.sub.Messages():
				if !ok {
					return
				}
				o := msg.AsEventObjectAddRemove()
				if o == nil {
					continue
				}
				if o.UEventID != types.DWORD(m.objectRemovedEventID) {
					continue
				}
				select {
				case os.ch <- ObjectEvent{ObjectID: uint32(o.DwData), ObjType: o.EObjType}:
				default:
					m.logger.Debug("[manager] ObjectRemoved subscription channel full, dropping event")
				}
			}
		}
	}()
	return os
}
