//go:build windows
// +build windows

package manager

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// FilenameEvent represents events that carry a filename (FlightLoaded, AircraftLoaded, FlightPlanActivated)
type FilenameEvent struct {
	Filename string
}

// FilenameSubscription is a typed subscription for filename-based system events
type FilenameSubscription interface {
	ID() string
	Events() <-chan FilenameEvent
	Done() <-chan struct{}
	Unsubscribe()
}

type filenameSubscription struct {
	id      string
	sub     Subscription
	ch      chan FilenameEvent
	done    chan struct{}
	mgr     *Instance
	closeMu sync.Mutex
}

func (s *filenameSubscription) ID() string                   { return s.id }
func (s *filenameSubscription) Events() <-chan FilenameEvent { return s.ch }
func (s *filenameSubscription) Done() <-chan struct{}        { return s.done }
func (s *filenameSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.sub != nil {
		s.sub.Unsubscribe()
		s.sub = nil
	}
	select {
	case <-s.done:
		// already closed
	default:
		close(s.done)
	}
	// ensure channel closed
	select {
	case <-s.ch:
	default:
		close(s.ch)
	}
}

// SubscribeOnFlightLoaded returns a subscription delivering FlightLoaded filenames
func (m *Instance) SubscribeOnFlightLoaded(id string, bufferSize int) FilenameSubscription {
	if id == "" {
		id = generateUUID()
	}
	// subscribe to filename messages
	msgSub := m.SubscribeWithType(id+"-fname", bufferSize, types.SIMCONNECT_RECV_ID_EVENT_FILENAME)
	fs := &filenameSubscription{id: id, sub: msgSub, ch: make(chan FilenameEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer fs.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-fs.sub.Done():
				return
			case msg, ok := <-fs.sub.Messages():
				if !ok {
					return
				}
				fname := msg.AsEventFilename()
				if fname == nil {
					continue
				}
				if fname.UEventID != types.DWORD(m.flightLoadedEventID) {
					continue
				}
				name := engine.BytesToString(fname.SzFileName[:])
				select {
				case fs.ch <- FilenameEvent{Filename: name}:
				default:
					m.logger.Debug(fmt.Sprintf("[manager] FlightLoaded subscription channel full, dropping event"))
				}
			}
		}
	}()
	return fs
}

// SubscribeOnAircraftLoaded returns a subscription delivering AircraftLoaded filenames
func (m *Instance) SubscribeOnAircraftLoaded(id string, bufferSize int) FilenameSubscription {
	if id == "" {
		id = generateUUID()
	}
	msgSub := m.SubscribeWithType(id+"-fname", bufferSize, types.SIMCONNECT_RECV_ID_EVENT_FILENAME)
	fs := &filenameSubscription{id: id, sub: msgSub, ch: make(chan FilenameEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer fs.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-fs.sub.Done():
				return
			case msg, ok := <-fs.sub.Messages():
				if !ok {
					return
				}
				fname := msg.AsEventFilename()
				if fname == nil {
					continue
				}
				if fname.UEventID != types.DWORD(m.aircraftLoadedEventID) {
					continue
				}
				name := engine.BytesToString(fname.SzFileName[:])
				select {
				case fs.ch <- FilenameEvent{Filename: name}:
				default:
					m.logger.Debug(fmt.Sprintf("[manager] AircraftLoaded subscription channel full, dropping event"))
				}
			}
		}
	}()
	return fs
}

// SubscribeOnFlightPlanActivated returns a subscription delivering FlightPlanActivated filenames
func (m *Instance) SubscribeOnFlightPlanActivated(id string, bufferSize int) FilenameSubscription {
	if id == "" {
		id = generateUUID()
	}
	msgSub := m.SubscribeWithType(id+"-fname", bufferSize, types.SIMCONNECT_RECV_ID_EVENT_FILENAME)
	fs := &filenameSubscription{id: id, sub: msgSub, ch: make(chan FilenameEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer fs.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-fs.sub.Done():
				return
			case msg, ok := <-fs.sub.Messages():
				if !ok {
					return
				}
				fname := msg.AsEventFilename()
				if fname == nil {
					continue
				}
				if fname.UEventID != types.DWORD(m.flightPlanActivatedEventID) {
					continue
				}
				name := engine.BytesToString(fname.SzFileName[:])
				select {
				case fs.ch <- FilenameEvent{Filename: name}:
				default:
					m.logger.Debug(fmt.Sprintf("[manager] FlightPlanActivated subscription channel full, dropping event"))
				}
			}
		}
	}()
	return fs
}

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
					m.logger.Debug(fmt.Sprintf("[manager] ObjectAdded subscription channel full, dropping event"))
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
					m.logger.Debug(fmt.Sprintf("[manager] ObjectRemoved subscription channel full, dropping event"))
				}
			}
		}
	}()
	return os
}

// subscription implements the Subscription interface
type subscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan engine.Message
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
	// Optional filter predicate. If non-nil, only messages for which
	// filter(msg) == true are forwarded to this subscription.
	filter func(engine.Message) bool
	// Optional set of allowed SIMCONNECT_RECV_ID values. If non-nil
	// and non-empty, only messages whose DwID matches one of the keys
	// will be forwarded.
	allowedTypes map[types.SIMCONNECT_RECV_ID]struct{}
}

// stateSubscription implements the StateSubscription interface
type connectionStateSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan ConnectionStateChange
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// connectionOpenSubscription implements the ConnectionOpenSubscription interface
type connectionOpenSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan types.ConnectionOpenData
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// connectionQuitSubscription implements the ConnectionQuitSubscription interface
type connectionQuitSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan types.ConnectionQuitData
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// Subscribe creates a new message subscription that delivers messages to a channel.
// The returned Subscription can be used to receive messages in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size.
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
func (m *Instance) Subscribe(id string, bufferSize int) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &subscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan engine.Message, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created subscription: %s", id))
	return sub
}

// SubscribeWithFilter creates a new message subscription that delivers messages
// to a channel only when the provided filter returns true for a message.
func (m *Instance) SubscribeWithFilter(id string, bufferSize int, filter func(engine.Message) bool) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &subscription{
		id:           id,
		ctx:          subCtx,
		cancel:       subCancel,
		ch:           make(chan engine.Message, bufferSize),
		done:         make(chan struct{}),
		manager:      m,
		filter:       filter,
		allowedTypes: nil,
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created filtered subscription: %s", id))
	return sub
}

// SubscribeWithType creates a new message subscription that delivers messages
// only when their DwID (SIMCONNECT_RECV_ID) is one of the provided types.
func (m *Instance) SubscribeWithType(id string, bufferSize int, recvIDs ...types.SIMCONNECT_RECV_ID) Subscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	allowed := make(map[types.SIMCONNECT_RECV_ID]struct{}, len(recvIDs))
	for _, r := range recvIDs {
		allowed[r] = struct{}{}
	}

	sub := &subscription{
		id:           id,
		ctx:          subCtx,
		cancel:       subCancel,
		ch:           make(chan engine.Message, bufferSize),
		done:         make(chan struct{}),
		manager:      m,
		filter:       nil,
		allowedTypes: allowed,
	}

	m.mu.Lock()
	m.subscriptions[id] = sub
	m.subsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created type subscription: %s", id))
	return sub
}

// watchContext monitors the subscription's context and auto-unsubscribes when cancelled
func (s *subscription) watchContext() {
	<-s.ctx.Done()
	s.Unsubscribe()
}

// GetSubscription returns an existing subscription by ID, or nil if not found.
func (m *Instance) GetSubscription(id string) Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.subscriptions[id]; ok {
		return sub
	}
	return nil
}

// generateUUID generates a simple UUID v4
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ID returns the unique identifier of the subscription
func (s *subscription) ID() string {
	return s.id
}

// Messages returns the channel for receiving messages
func (s *subscription) Messages() <-chan engine.Message {
	return s.ch
}

// Done returns a channel that is closed when the subscription ends.
// Use this to detect when to exit your consumer goroutine.
func (s *subscription) Done() <-chan struct{} {
	return s.done
}

// Unsubscribe cancels the subscription and closes the channel.
// Blocks until any pending message delivery completes.
func (s *subscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.cancel()    // Cancel the subscription's context
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close message channel

	// Remove from manager's subscription map
	s.manager.mu.Lock()
	delete(s.manager.subscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.subsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] Unsubscribed: %s", s.id))
}

// SubscribeStateChange creates a new state change subscription that delivers state changes to a channel.
// The returned StateSubscription can be used to receive state changes in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size.
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
func (m *Instance) SubscribeConnectionStateChange(id string, bufferSize int) ConnectionStateSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &connectionStateSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan ConnectionStateChange, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.connectionStateSubscriptions[id] = sub
	m.connectionStateSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created state subscription: %s", id))
	return sub
}

// watchContext monitors the state subscription's context and auto-unsubscribes when cancelled
func (s *connectionStateSubscription) watchContext() {
	<-s.ctx.Done()
	s.Unsubscribe()
}

// GetConnectionStateSubscription returns an existing connection state subscription by ID, or nil if not found.
func (m *Instance) GetConnectionStateSubscription(id string) ConnectionStateSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.connectionStateSubscriptions[id]; ok {
		return sub
	}
	return nil
}

// SubscribeSimStateChange creates a new simulator state change subscription that delivers state changes to a channel.
// The returned SimStateSubscription can be used to receive state changes in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size.
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
func (m *Instance) SubscribeSimStateChange(id string, bufferSize int) SimStateSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &simStateSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan SimStateChange, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.simStateSubscriptions[id] = sub
	m.simStateSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created SimState subscription: %s", id))
	return sub
}

// GetSimStateSubscription returns an existing simulator state subscription by ID, or nil if not found.
func (m *Instance) GetSimStateSubscription(id string) SimStateSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.simStateSubscriptions[id]; ok {
		return sub
	}
	return nil
}

// ID returns the unique identifier of the state subscription
func (s *connectionStateSubscription) ID() string {
	return s.id
}

// StateChanges returns the channel for receiving state change events
func (s *connectionStateSubscription) ConnectionStateChanges() <-chan ConnectionStateChange {
	return s.ch
}

// Done returns a channel that is closed when the state subscription ends.
// Use this to detect when to exit your consumer goroutine.
func (s *connectionStateSubscription) Done() <-chan struct{} {
	return s.done
}

// Unsubscribe cancels the state subscription and closes the channel.
// Blocks until any pending state change delivery completes.
func (s *connectionStateSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.cancel()    // Cancel the subscription's context
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close state change channel

	// Remove from manager's state subscription map
	s.manager.mu.Lock()
	delete(s.manager.connectionStateSubscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.connectionStateSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] State subscription unsubscribed: %s", s.id))
}

// simStateSubscription implements the SimStateSubscription interface
type simStateSubscription struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ch      chan SimStateChange
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
	manager *Instance
}

// ID returns the unique identifier of the subscription
func (s *simStateSubscription) ID() string {
	return s.id
}

// SimStateChanges returns the channel for receiving state changes
func (s *simStateSubscription) SimStateChanges() <-chan SimStateChange {
	return s.ch
}

// Done returns a channel that is closed when the subscription ends
func (s *simStateSubscription) Done() <-chan struct{} {
	return s.done
}

// watchContext watches for context cancellation and closes the subscription
func (s *simStateSubscription) watchContext() {
	<-s.ctx.Done()
	s.Unsubscribe()
}

// Unsubscribe cancels the sim state subscription and closes the channel.
// Blocks until any pending state change delivery completes.
func (s *simStateSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.cancel()    // Cancel the subscription's context
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close state change channel

	// Remove from manager's sim state subscription map
	s.manager.mu.Lock()
	delete(s.manager.simStateSubscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.simStateSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] SimState subscription unsubscribed: %s", s.id))
}

// SubscribeOnOpen creates a new connection open subscription that delivers open events to a channel.
// The returned ConnectionOpenSubscription can be used to receive open events in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size.
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
func (m *Instance) SubscribeOnOpen(id string, bufferSize int) ConnectionOpenSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &connectionOpenSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan types.ConnectionOpenData, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.openSubscriptions[id] = sub
	m.openSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created open subscription: %s", id))
	return sub
}

// watchContext monitors the open subscription's context and auto-unsubscribes when cancelled
func (s *connectionOpenSubscription) watchContext() {
	<-s.ctx.Done()
	s.Unsubscribe()
}

// ID returns the unique identifier of the open subscription
func (s *connectionOpenSubscription) ID() string {
	return s.id
}

// Opens returns the channel for receiving connection open events
func (s *connectionOpenSubscription) Opens() <-chan types.ConnectionOpenData {
	return s.ch
}

// Done returns a channel that is closed when the subscription ends.
// Use this to detect when to exit your consumer goroutine.
func (s *connectionOpenSubscription) Done() <-chan struct{} {
	return s.done
}

// Unsubscribe cancels the open subscription and closes the channel.
// Blocks until any pending open event delivery completes.
func (s *connectionOpenSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.cancel()    // Cancel the subscription's context
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close event channel

	// Remove from manager's open subscription map
	s.manager.mu.Lock()
	delete(s.manager.openSubscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.openSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] Open subscription unsubscribed: %s", s.id))
}

// GetOpenSubscription returns an existing open subscription by ID, or nil if not found.
func (m *Instance) GetOpenSubscription(id string) ConnectionOpenSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.openSubscriptions[id]; ok {
		return sub
	}
	return nil
}

// SubscribeOnQuit creates a new connection quit subscription that delivers quit events to a channel.
// The returned ConnectionQuitSubscription can be used to receive quit events in an isolated goroutine.
// The id parameter is a unique identifier for the subscription (use "" for auto-generated UUID).
// The channel is buffered with the specified size.
// The subscription is automatically cancelled when the manager's context is cancelled.
// Call Unsubscribe() when done to release resources.
func (m *Instance) SubscribeOnQuit(id string, bufferSize int) ConnectionQuitSubscription {
	if id == "" {
		id = generateUUID()
	}

	// Derive context from manager's context for automatic cancellation
	subCtx, subCancel := context.WithCancel(m.ctx)

	sub := &connectionQuitSubscription{
		id:      id,
		ctx:     subCtx,
		cancel:  subCancel,
		ch:      make(chan types.ConnectionQuitData, bufferSize),
		done:    make(chan struct{}),
		manager: m,
	}

	m.mu.Lock()
	m.quitSubscriptions[id] = sub
	m.quitSubsWg.Add(1)
	m.mu.Unlock()

	// Start goroutine to watch for context cancellation
	go sub.watchContext()

	m.logger.Debug(fmt.Sprintf("[manager] Created quit subscription: %s", id))
	return sub
}

// watchContext monitors the quit subscription's context and auto-unsubscribes when cancelled
func (s *connectionQuitSubscription) watchContext() {
	<-s.ctx.Done()
	s.Unsubscribe()
}

// ID returns the unique identifier of the quit subscription
func (s *connectionQuitSubscription) ID() string {
	return s.id
}

// Quits returns the channel for receiving connection quit events
func (s *connectionQuitSubscription) Quits() <-chan types.ConnectionQuitData {
	return s.ch
}

// Done returns a channel that is closed when the subscription ends.
// Use this to detect when to exit your consumer goroutine.
func (s *connectionQuitSubscription) Done() <-chan struct{} {
	return s.done
}

// Unsubscribe cancels the quit subscription and closes the channel.
// Blocks until any pending quit event delivery completes.
func (s *connectionQuitSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.cancel()    // Cancel the subscription's context
	close(s.done) // Signal consumers to stop
	close(s.ch)   // Close event channel

	// Remove from manager's quit subscription map
	s.manager.mu.Lock()
	delete(s.manager.quitSubscriptions, s.id)
	s.manager.mu.Unlock()

	// Signal WaitGroup that this subscription is done
	s.manager.quitSubsWg.Done()
	s.manager.logger.Debug(fmt.Sprintf("[manager] Quit subscription unsubscribed: %s", s.id))
}

// GetQuitSubscription returns an existing quit subscription by ID, or nil if not found.
func (m *Instance) GetQuitSubscription(id string) ConnectionQuitSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.quitSubscriptions[id]; ok {
		return sub
	}
	return nil
}
