// Copyright 2025 Flant JSC
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

package lifecycle

import (
	"context"

	addonutils "github.com/flant/addon-operator/pkg/utils"
)

// Event types determine context cancellation behavior in newContext.
//
// Root events (EventUpdate, EventRemove) cancel the entire context tree
// and create a fresh root. All in-flight tasks for the package see ctx.Done().
//
// Child events (EventSchedule) cancel only the previous context for the same
// event key, then create a new child of the current root.
//
// EventSchedule is shared by both enable and disable flows, providing mutual
// cancellation: enabling cancels a pending disable and vice versa.
const (
	EventUpdate = iota
	EventRemove
	EventSchedule
)

// Package holds the lifecycle state for a single runtime package: version tracking,
// pending settings, and a tree of cancellable contexts for coordinating concurrent operations.
//
// Package does not hold the loaded runtime instance (Application/Module) — those live
// in plain maps on Runtime. This keeps the Store type-agnostic.
//
// Context hierarchy:
//
//	root (ctx/cancel) — created by EventUpdate or EventRemove
//	└── EventSchedule child — cancelled on enable↔disable transition
type Package struct {
	version  string            // package version, cleared on remove
	settings addonutils.Values // pending settings, updated by Update, consumed by GetPendingSettings

	ctx    context.Context    // root context, cancelled on version change or remove
	cancel context.CancelFunc // cancels root and all children

	cancels map[int]context.CancelFunc // per-event child context cancels
}

// newContext creates or renews a context for the given event type.
//
// For root events (EventUpdate, EventRemove): cancels the entire context tree
// and creates a fresh root. All in-flight tasks for this package see ctx.Done().
//
// For child events (EventSchedule): cancels only the previous context for the
// same event key, then creates a new child of the current root.
func (p *Package) newContext(event int) context.Context {
	if event == EventUpdate || event == EventRemove {
		clear(p.cancels)
		if p.cancel != nil {
			p.cancel()
		}
		p.ctx, p.cancel = context.WithCancel(context.Background())
		return p.ctx
	}

	if stored, ok := p.cancels[event]; ok {
		stored()
	}

	ctx, cancel := context.WithCancel(p.ctx)
	p.cancels[event] = cancel
	return ctx
}
