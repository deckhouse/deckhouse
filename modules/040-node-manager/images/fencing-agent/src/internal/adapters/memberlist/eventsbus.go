package memberlist

import (
	"context"
	"fencing-agent/internal/core/domain"
	"sync"
	"time"
)

type EventsBus struct {
	subscribers         []subscriber
	mu                  *sync.RWMutex
	timeoutFOrReadEvent time.Duration
}

type subscriber struct {
	ch  chan domain.Event
	ctx context.Context
}

// NewEventsBus creates and initializes a new instance of EventsBus with timeoutForReadEvent = 10 seconds
func NewEventsBus() *EventsBus {
	return &EventsBus{
		subscribers:         make([]subscriber, 0),
		mu:                  &sync.RWMutex{},
		timeoutFOrReadEvent: 10 * time.Second,
	}
}

func (e *EventsBus) Publish(event domain.Event) {
	timer := time.NewTimer(e.timeoutFOrReadEvent)
	defer timer.Stop()
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, sb := range e.subscribers {
		select {
		case sb.ch <- event:
		case <-sb.ctx.Done():
		case <-timer.C:
		}
	}
}

func (e *EventsBus) Subscribe(ctx context.Context) <-chan domain.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	ch := make(chan domain.Event)
	e.subscribers = append(e.subscribers, subscriber{ch: ch, ctx: ctx})
	go func() {
		<-ctx.Done()
		e.mu.Lock()
		defer e.mu.Unlock()
		for i, sb := range e.subscribers {
			if sb.ch == ch {
				e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			}
		}
		close(ch)
	}()
	return ch
}
