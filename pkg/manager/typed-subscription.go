//go:build windows
// +build windows

package manager

import (
	"sync"
	"sync/atomic"

	"github.com/mrlm-net/simconnect/pkg/engine"
)

// typedSubscription is a message subscription (sub) whose messages are
// turned into values of T by a forwarder goroutine: the object and filename
// subscriptions. Sends and the close of ch happen under closeMu with the
// closed flag checked, so Unsubscribe never races a send (a "send on closed
// channel" panic) and never eats a buffered value (review #18).
type typedSubscription[T any] struct {
	id      string
	sub     Subscription
	ch      chan T
	done    chan struct{}
	closed  atomic.Bool
	closeMu sync.Mutex
}

func newTypedSubscription[T any](id string, sub Subscription, bufferSize int) *typedSubscription[T] {
	return &typedSubscription[T]{id: id, sub: sub, ch: make(chan T, bufferSize), done: make(chan struct{})}
}

func (s *typedSubscription[T]) ID() string            { return s.id }
func (s *typedSubscription[T]) Events() <-chan T      { return s.ch }
func (s *typedSubscription[T]) Done() <-chan struct{} { return s.done }

// Unsubscribe ends the subscription and its message subscription. It is
// idempotent and safe to call from any goroutine.
func (s *typedSubscription[T]) Unsubscribe() {
	if s.closed.Swap(true) {
		return
	}
	s.closeMu.Lock()
	close(s.done)
	close(s.ch)
	s.closeMu.Unlock()
	if s.sub != nil {
		s.sub.Unsubscribe()
	}
}

// send delivers v without blocking; false when the buffer is full or the
// subscription is closed.
func (s *typedSubscription[T]) send(v T) bool {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed.Load() {
		return false
	}
	select {
	case s.ch <- v:
		return true
	default:
		return false
	}
}

// forwardTyped runs the forwarder of s: every message of its message
// subscription convert accepts is sent on s.ch. It ends, unsubscribing s,
// when the manager stops or the message subscription ends.
func forwardTyped[T any](m *Instance, s *typedSubscription[T], kind string, convert func(engine.Message) (T, bool)) {
	sub := s.sub
	go func() {
		defer s.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-sub.Done():
				return
			case msg, ok := <-sub.Messages():
				if !ok {
					return
				}
				v, ok := convert(msg)
				if !ok {
					continue
				}
				if !s.send(v) && !s.closed.Load() {
					m.logger.Debug("[manager] " + kind + " subscription channel full, dropping event")
				}
			}
		}
	}()
}
