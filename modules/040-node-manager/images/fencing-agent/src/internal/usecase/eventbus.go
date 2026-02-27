/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
