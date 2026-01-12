package memberlist

import (
	"context"
	"fencing-controller/internal/core/domain"
	"sync"
)

type EventsBus struct {
	subscribers []chan domain.Event
	mu          *sync.RWMutex
}

func NewEventsBus() *EventsBus {
	return &EventsBus{subscribers: make([]chan domain.Event, 0), mu: &sync.RWMutex{}}
}

func (e *EventsBus) Publish(event domain.Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, sb := range e.subscribers {
		select {
		case sb <- event:
		default:
		}
	}
}

func (e *EventsBus) Subscribe(ctx context.Context) <-chan domain.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	ch := make(chan domain.Event)
	e.subscribers = append(e.subscribers, ch)
	go func() {
		<-ctx.Done()
		e.mu.Lock()
		defer e.mu.Unlock()
		for i, sb := range e.subscribers {
			if sb == ch {
				e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			}
		}
		close(ch)
	}()
	return ch
}
