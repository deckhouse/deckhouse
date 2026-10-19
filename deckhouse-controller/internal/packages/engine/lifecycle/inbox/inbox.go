// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package inbox holds the latest value of every input of a loop plus a wake signal: inputs are
// states passed whole, so a loop that wakes late loses nothing by reading only the last one.
package inbox

import "sync"

// Inbox holds the latest value of every input of one receiver (the package Reconciler, Current,
// Global) plus a wake signal. Any goroutine may Put; only the owner Loads.
//   - Put writes the state before it signals, so a Load after the wake sees it; a Put after the
//     receiver's Load leaves a pending wake. No wake is lost; several Puts may share one.
//   - Every field is a level: there is no take. A receiver clears a field with its own Put, only
//     after the field's writer has stopped.
//   - f runs under the lock: short, no effects, no calls into other components.
//   - Load copies T: pointer fields must point to immutable data or to objects handed over
//     explicitly (an operation's state, after its Done).
type Inbox[T any] struct {
	mu    sync.Mutex
	state T
	wake  chan struct{} // wake has capacity 1: "state changed since your last Load".
}

// New returns an Inbox with the zero state and no pending wake.
func New[T any]() *Inbox[T] {
	return &Inbox[T]{wake: make(chan struct{}, 1)}
}

// Put applies f to the state under the lock, then signals without blocking.
func (b *Inbox[T]) Put(f func(*T)) {
	b.mu.Lock()
	f(&b.state)
	b.mu.Unlock()

	select {
	case b.wake <- struct{}{}:
	default: // a wake is pending; the receiver loads this state with it
	}
}

// Ready returns the wake channel; the receiver selects on it with its context and its own timers.
func (b *Inbox[T]) Ready() <-chan struct{} {
	return b.wake
}

// Load returns a copy of the latest state.
func (b *Inbox[T]) Load() T {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.state
}
