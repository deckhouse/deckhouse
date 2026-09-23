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

package lifecycle

import (
	"context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// Package holds the lifecycle state of a single runtime package: the state its controller last
// asked for, and one cancellable context per running operation.
//
// Package does not hold the loaded runtime instance (Application/Module) — those live
// in plain maps on Runtime. This keeps the Store type-agnostic.
type Package struct {
	version         string            // package version the store last adopted
	settingsVersion int               // schema version of pending settings (from ModuleConfig.Spec.Version)
	settings        addonutils.Values // pending settings, consumed by GetPendingSettings
	maintenance     string            // pending maintenance mode, consumed by GetPendingMaintenance
	removing        bool              // a teardown has begun and has not finished
	operations      map[OperationKind]operation
}

// beginOperation supersedes any operation of the same kind and returns the new one's context.
func (p *Package) beginOperation(kind OperationKind, cause error) context.Context {
	p.cancelOperation(kind, cause)

	ctx, cancel := context.WithCancelCause(context.Background())

	p.operations[kind] = operation{
		ctx:    ctx,
		cancel: cancel,
	}

	return ctx
}

// cancelOperation cancels the operation of that kind, if one runs, naming cause.
func (p *Package) cancelOperation(kind OperationKind, cause error) {
	op, ok := p.operations[kind]
	if !ok {
		return
	}

	op.cancel(cause)
	delete(p.operations, kind)
}
