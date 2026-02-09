package usecase

import (
	"context"
	"sync"
	"sync/atomic"

	"fencing-agent/internal/domain"
)

type EventsBus struct {
	subscribers map[chan domain.Event]*subscriber
	mu          sync.RWMutex
}

type subscriber struct {
	ch     chan domain.Event
	ctx    context.Context
	closed atomic.Bool
}

func NewEventsBus() *EventsBus {
	return &EventsBus{
		subscribers: make(map[chan domain.Event]*subscriber),
	}
}

func (e *EventsBus) Publish(event domain.Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, sb := range e.subscribers {
		if sb.closed.Load() {
			continue
		}
		select {
		case <-sb.ctx.Done():
		case sb.ch <- event:
		}
	}
}

func (e *EventsBus) Subscribe(ctx context.Context) <-chan domain.Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	ch := make(chan domain.Event)
	sub := &subscriber{
		ch:  ch,
		ctx: ctx,
	}
	e.subscribers[ch] = sub

	go func() {
		<-ctx.Done()
		e.unsubscribe(ch)
	}()
	return ch
}

func (e *EventsBus) unsubscribe(ch chan domain.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sub, exists := e.subscribers[ch]
	if !exists {
		return
	}

	if sub.closed.CompareAndSwap(false, true) {
		delete(e.subscribers, ch)
		close(sub.ch)
	}
}
