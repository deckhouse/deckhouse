package usecase

import (
	"context"
	"fencing-agent/internal/domain"
	"fmt"
	"sync"
)

type EventsBus struct {
	events    chan domain.Event
	ciliumSub *cilium
	mu        sync.Mutex
}

type cilium struct {
	ctx context.Context
}

func NewEventsBus() *EventsBus {
	return &EventsBus{
		events:    make(chan domain.Event),
		ciliumSub: nil,
	}
}

func (e *EventsBus) Publish(event domain.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ciliumSub == nil {
		return
	}

	select {
	case e.events <- event:
	case <-e.ciliumSub.ctx.Done():
		return
	}
}

func (e *EventsBus) Subscribe(ctx context.Context) (<-chan domain.Event, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ciliumSub != nil {
		return nil, fmt.Errorf("already subscribed")
	}

	e.ciliumSub = &cilium{ctx: ctx}

	go func() {
		<-ctx.Done()
		e.unsubscribe()
	}()

	return e.events, nil
}

func (e *EventsBus) unsubscribe() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ciliumSub == nil {
		return
	}

	e.ciliumSub = nil
}
