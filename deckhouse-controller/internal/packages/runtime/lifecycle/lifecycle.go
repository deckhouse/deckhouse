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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
)

// Event types determine context cancellation behavior in newContext.
//
// Root events (EventVersionChanged, EventRemove) cancel all existing contexts
// and create a new root. Child events cancel only the previous context for the
// same event key, enabling selective cancellation.
//
// EventSchedule is shared by both enable and disable flows, providing mutual
// cancellation: enabling cancels a pending disable and vice versa.
const (
	EventSettingsChanged = iota
	EventVersionChanged
	EventRemove
	EventSchedule
	EventRun
)

// runtimePackage constrains the generic Package to known runtime types.
type runtimePackage interface {
	*apps.Application | *modules.Module
}

// Package holds the lifecycle state for a single runtime package: its loaded instance,
// version tracking, and a tree of cancellable contexts for coordinating concurrent operations.
//
// Context hierarchy:
//
//	root (ctx/cancel) — created by EventVersionChanged or EventRemove
//	├── EventSettingsChanged child — cancelled on next settings change
//	├── EventSchedule child — cancelled on enable↔disable transition
//	└── EventRun child — cancelled on next NELM drift re-run
type Package[P runtimePackage] struct {
	version  string // package version, cleared on remove
	checksum string // settings checksum for change detection

	pkg P // loaded runtime instance, nil until Load task completes

	ctx    context.Context    // root context, cancelled on version change or remove
	cancel context.CancelFunc // cancels root and all children

	cancels map[int]context.CancelFunc // per-event child context cancels
}

// newContext creates or renews a context for the given event type.
//
// For root events (EventVersionChanged, EventRemove): cancels the entire context tree
// and creates a fresh root. All in-flight tasks for this package see ctx.Done().
//
// For child events: cancels only the previous context for the same event key,
// then creates a new child of the current root. Other event types are unaffected.
func (p *Package[P]) newContext(event int) context.Context {
	if event == EventVersionChanged || event == EventRemove {
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
