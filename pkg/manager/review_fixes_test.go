//go:build windows

package manager

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// objectMessage is a SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE for eventID and objectID.
func objectMessage(eventID, objectID uint32) engine.Message {
	ev := &types.SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE{EObjType: types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT}
	ev.UEventID = types.DWORD(eventID)
	ev.DwData = types.DWORD(objectID)
	ev.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE)
	return engine.Message{SIMCONNECT_RECV: &ev.SIMCONNECT_RECV}
}

func recvObject(t *testing.T, s ObjectSubscription) ObjectEvent {
	t.Helper()
	select {
	case ev := <-s.Events():
		return ev
	case <-time.After(time.Second):
		t.Fatalf("%s got nothing", s.ID())
	}
	return ObjectEvent{}
}

// TestObjectAddedRemovedSameID: ObjectAdded and ObjectRemoved subscriptions
// with the same id each get their own events (review #19, E9: both were
// id+"-obj", the second replacing the first).
func TestObjectAddedRemovedSameID(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	added := m.SubscribeOnObjectAdded("x", 4)
	removed := m.SubscribeOnObjectRemoved("x", 4)

	m.processMessage(objectMessage(m.objectAddedEventID, 7))
	m.processMessage(objectMessage(m.objectRemovedEventID, 8))

	if ev := recvObject(t, added); ev.ObjectID != 7 {
		t.Errorf("added got %d, want 7", ev.ObjectID)
	}
	if ev := recvObject(t, removed); ev.ObjectID != 8 {
		t.Errorf("removed got %d, want 8", ev.ObjectID)
	}
}

// TestTypedUnsubscribeKeepsBuffered: Unsubscribe closes the channel without
// eating a buffered event (review #18: its "idempotent close" read one).
func TestTypedUnsubscribeKeepsBuffered(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	s := newTypedSubscription[int]("t", m.Subscribe("t-base", 1), 2)
	if !s.send(5) {
		t.Fatal("send failed")
	}
	s.Unsubscribe()
	s.Unsubscribe()
	if v, ok := <-s.Events(); !ok || v != 5 {
		t.Fatalf("got %d %v, want the buffered 5", v, ok)
	}
	if _, ok := <-s.Events(); ok {
		t.Fatal("the channel is still open")
	}
	if s.send(6) {
		t.Fatal("sent on a closed subscription")
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("Done not closed")
	}
}

// TestTypedUnsubscribeWhileSending: sends racing Unsubscribe never panic
// with "send on closed channel" (review #18).
func TestTypedUnsubscribeWhileSending(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	for i := 0; i < 200; i++ {
		s := newTypedSubscription[int]("t", m.Subscribe("", 1), 1)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				s.send(j)
				select {
				case <-s.Events():
				default:
				}
			}
		}()
		go func() {
			defer wg.Done()
			s.Unsubscribe()
		}()
		wg.Wait()
	}
}

// TestDuplicateSubscriptionID: a second subscription with an ID in use
// replaces the first and closes it; the first one's Unsubscribe no longer
// removes the second (review #19).
func TestDuplicateSubscriptionID(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	first := m.Subscribe("dup", 4)
	second := m.Subscribe("dup", 4)

	select {
	case <-first.Done():
	case <-time.After(time.Second):
		t.Fatal("the replaced subscription is still open")
	}
	first.Unsubscribe()
	if got := m.GetSubscription("dup"); got != second {
		t.Fatalf("GetSubscription = %v, want the second subscription", got)
	}
	m.processMessage(eventMessage(1, 0))
	select {
	case <-second.Messages():
	case <-time.After(time.Second):
		t.Fatal("the second subscription got nothing")
	}

	o1 := m.SubscribeOnOpen("o", 1)
	o2 := m.SubscribeOnOpen("o", 1)
	o1.Unsubscribe()
	if m.GetOpenSubscription("o") != o2 {
		t.Fatal("the first open subscription removed the second")
	}
}

// TestCrashEventsArePulses: every Crashed fires OnCrashed, whatever its
// dwData; CrashReset clears Crashed (review #20).
func TestCrashEventsArePulses(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	crashed, resets := 0, 0
	m.OnCrashed(func() { crashed++ })
	m.OnCrashReset(func() { resets++ })

	m.processMessage(eventMessage(m.crashedEventID, 0))
	if crashed != 1 || !m.simState.Crashed || m.simState.CrashReset {
		t.Fatalf("after Crashed: %d calls, state %v/%v", crashed, m.simState.Crashed, m.simState.CrashReset)
	}
	m.processMessage(eventMessage(m.crashResetEventID, 0))
	if resets != 1 || m.simState.Crashed || !m.simState.CrashReset {
		t.Fatalf("after CrashReset: %d calls, state %v/%v", resets, m.simState.Crashed, m.simState.CrashReset)
	}
	m.processMessage(eventMessage(m.crashedEventID, 1))
	m.processMessage(eventMessage(m.crashedEventID, 1))
	if crashed != 3 {
		t.Fatalf("OnCrashed fired %d times, want 3", crashed)
	}
}

// TestCustomEventIDsFreed: an unsubscribed custom event's ID is allocated
// again and its subscriptions are closed (review #30).
func TestCustomEventIDsFreed(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	for id := CustomEventIDMin; id <= CustomEventIDMax; id++ {
		m.customSystemEvents[string(rune('a'+id-CustomEventIDMin))] = &instance.CustomSystemEvent{Name: string(rune('a' + id - CustomEventIDMin)), ID: id}
	}
	if _, err := m.allocateCustomEventIDLocked(); !errors.Is(err, ErrCustomEventIDExhausted) {
		t.Fatalf("full range: err %v, want exhausted", err)
	}
	name := "c"
	freed := m.customSystemEvents[name].ID
	sub, err := m.customEventSubscription(name, freed, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.UnsubscribeFromCustomSystemEvent(name); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sub.Done():
	case <-time.After(time.Second):
		t.Fatal("the event's subscription is still open")
	}
	id, err := m.allocateCustomEventIDLocked()
	if err != nil || id != freed {
		t.Fatalf("allocated %d %v, want the freed %d", id, err, freed)
	}
}

// TestCustomEventRangeReserved: custom event IDs are no valid user IDs
// (review #31).
func TestCustomEventRangeReserved(t *testing.T) {
	for _, id := range []uint32{CustomEventIDMin, CustomEventIDMax} {
		if IsValidUserID(id) || !IsManagerID(id) {
			t.Errorf("%d: user %v manager %v", id, IsValidUserID(id), IsManagerID(id))
		}
	}
	fixed := []uint32{CameraDefinitionID, CameraRequestID, PauseEventID, SimEventID, CrashedEventID, CrashResetEventID,
		SoundEventID, ViewEventID, FlightPlanDeactivatedEventID, FlightLoadedEventID, AircraftLoadedEventID,
		ObjectAddedEventID, ObjectRemovedEventID, FlightPlanActivatedEventID, 999999999}
	for _, id := range fixed {
		if id >= CustomEventIDMin && id <= CustomEventIDMax {
			t.Errorf("fixed ID %d inside the custom range", id)
		}
	}
}

// TestResubscribeSkipsCurrentConnection: a custom event subscribed on the
// current connection before its OPEN is not subscribed again (review #32).
func TestResubscribeSkipsCurrentConnection(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	m.connGen = 2
	m.customSystemEvents["now"] = &instance.CustomSystemEvent{Name: "now", ID: CustomEventIDMin, Conn: 2}
	m.customSystemEvents["kept"] = &instance.CustomSystemEvent{Name: "kept", ID: CustomEventIDMin + 1, Conn: 1}
	f := &fakeSubscriber{got: map[string]uint32{}}
	m.resubscribeCustomEvents(f)
	if _, ok := f.got["now"]; ok {
		t.Error("the event of this connection was subscribed twice")
	}
	if _, ok := f.got["kept"]; !ok {
		t.Error("the kept event was not subscribed again")
	}
	if m.customSystemEvents["kept"].Conn != 2 {
		t.Error("the kept event is not marked as subscribed on this connection")
	}
}

type fakeConn struct{ disconnects int }

func (f *fakeConn) Disconnect() error { f.disconnects++; return nil }

// TestLateConnectDiscarded: a timed-out attempt succeeding late is
// disconnected, a failed one left alone (review #33).
func TestLateConnectDiscarded(t *testing.T) {
	m := New("test").(*Instance)
	defer m.Stop()
	for _, tc := range []struct {
		err  error
		want int
	}{{nil, 1}, {errors.New("no sim"), 0}} {
		f := &fakeConn{}
		done := make(chan error, 1)
		done <- tc.err
		m.discardLateConnect(f, done)
		if f.disconnects != tc.want {
			t.Errorf("err %v: %d disconnects, want %d", tc.err, f.disconnects, tc.want)
		}
	}
}

// TestReconnectMaxRetriesDefault: reconnects use MaxRetries unless
// WithReconnectMaxRetries says otherwise (review #34).
func TestReconnectMaxRetriesDefault(t *testing.T) {
	m := New("test", WithMaxRetries(3)).(*Instance)
	defer m.Stop()
	if m.config.ReconnectMaxRetries != -1 {
		t.Errorf("default %d, want -1 (MaxRetries)", m.config.ReconnectMaxRetries)
	}
	m2 := New("test", WithMaxRetries(3), WithReconnectMaxRetries(0)).(*Instance)
	defer m2.Stop()
	if m2.config.ReconnectMaxRetries != 0 {
		t.Errorf("got %d, want 0", m2.config.ReconnectMaxRetries)
	}
}

// TestStopClosesOpenQuitSubscriptions: Stop returns with the open and quit
// subscriptions closed (review #35).
func TestStopClosesOpenQuitSubscriptions(t *testing.T) {
	m := New("test").(*Instance)
	o := m.SubscribeOnOpen("o", 1)
	q := m.SubscribeOnQuit("q", 1)
	m.Stop()
	for _, d := range []<-chan struct{}{o.Done(), q.Done()} {
		select {
		case <-d:
		default:
			t.Fatal("a subscription is still open after Stop")
		}
	}
}

type fakeUserSubscriber struct{ calls []string }

func (f *fakeUserSubscriber) SubscribeToFlowEvent() error {
	f.calls = append(f.calls, "flow")
	return nil
}
func (f *fakeUserSubscriber) SubscribeInputEvent(hash uint64) error {
	f.calls = append(f.calls, "input")
	return nil
}
func (f *fakeUserSubscriber) SubscribeToSystemEvent(eventID uint32, eventName string) error {
	f.calls = append(f.calls, "system "+eventName)
	return nil
}
func (f *fakeUserSubscriber) SubscribeToFacilities(listType types.SIMCONNECT_FACILITY_LIST_TYPE, requestID uint32) error {
	f.calls = append(f.calls, "facilities")
	return nil
}
func (f *fakeUserSubscriber) SubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, n, o uint32) error {
	f.calls = append(f.calls, "facilitiesEX1")
	return nil
}
func (f *fakeUserSubscriber) UnsubscribeToFacilitiesEX1(listType types.SIMCONNECT_FACILITY_LIST_TYPE, n, o bool) error {
	if n || !o {
		f.calls = append(f.calls, "unexpected half")
	}
	f.calls = append(f.calls, "unsubscribe old half")
	return nil
}

// TestResubscribeOnReconnect: with the option the recorded subscriptions
// are made again once per new connection; without it nothing is (E10).
func TestResubscribeOnReconnect(t *testing.T) {
	off := New("test").(*Instance)
	defer off.Stop()
	off.connGen = 1
	off.userSubs.setFlow(1)
	f := &fakeUserSubscriber{}
	off.replayUserSubscriptions(f)
	if len(f.calls) != 0 {
		t.Fatalf("off: %v, want nothing", f.calls)
	}

	m := New("test", WithResubscribeOnReconnect(true)).(*Instance)
	defer m.Stop()
	m.connGen = 1
	m.userSubs.setFlow(1)
	m.userSubs.addInputEvent(42, 1)
	m.userSubs.addSystemEvent(5000, "6Hz", 1)
	m.userSubs.addSystemEvent(5001, "gone", 1)
	m.userSubs.removeSystemEvent(5001)
	m.userSubs.setFacilities(types.SIMCONNECT_FACILITY_LIST_AIRPORT, facilitySub{ex1: true, newReq: 1, oldReq: 2, newOn: true, oldOn: true, gen: 1})
	m.userSubs.unsubscribeFacilities(types.SIMCONNECT_FACILITY_LIST_AIRPORT, false, true)

	f = &fakeUserSubscriber{}
	m.replayUserSubscriptions(f)
	if len(f.calls) != 0 {
		t.Fatalf("same connection: %v, want nothing", f.calls)
	}

	m.connGen = 2
	m.replayUserSubscriptions(f)
	want := map[string]bool{"flow": true, "input": true, "system 6Hz": true, "facilitiesEX1": true, "unsubscribe old half": true}
	if len(f.calls) != len(want) {
		t.Fatalf("calls %v, want %v", f.calls, want)
	}
	for _, c := range f.calls {
		if !want[c] {
			t.Errorf("unexpected call %q", c)
		}
	}
	f.calls = nil
	m.replayUserSubscriptions(f)
	if len(f.calls) != 0 {
		t.Fatalf("second OPEN of the same connection: %v", f.calls)
	}
}
