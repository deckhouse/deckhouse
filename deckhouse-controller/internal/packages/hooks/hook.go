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

package hooks

import (
	"context"
	"sort"
	"sync"

	addonhooks "github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	"github.com/flant/addon-operator/pkg/utils"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
)

// Hook represents a module-scoped hook that executes during a module's lifecycle.
// Module hooks use ModuleHookConfig and are organized by binding type for ordered execution.
type Hook interface {
	GetName() string
	GetConfigVersion() string
	GetHookConfig() *addonhooks.ModuleHookConfig
	Order(binding shtypes.BindingType) float64

	InitializeHookConfig() error

	GetHookController() *controller.HookController
	WithHookController(ctrl *controller.HookController)
	WithTmpDir(tmpDir string)

	SynchronizationNeeded() bool

	Execute(ctx context.Context, version string, bctx []bctx.BindingContext, packageName string, configValues, values utils.Values, logLabels map[string]string) (*kind.HookResult, error)
}

// Storage provides thread-safe storage for hooks with multiple access patterns.
// It maintains two indices:
//   - byName: Fast lookup by hook name (O(1))
//   - byBinding: Fast lookup by binding type (O(1))
//
// Thread Safety: All methods use RWMutex for concurrent access.
type Storage struct {
	mu        sync.RWMutex                   // Protects all fields
	byBinding map[shtypes.BindingType][]Hook // Hooks grouped by binding type
	byName    map[string]Hook                // Hooks indexed by name
}

// NewStorage creates a new empty hook storage.
func NewStorage() *Storage {
	return &Storage{
		byBinding: make(map[shtypes.BindingType][]Hook),
		byName:    make(map[string]Hook),
	}
}

// Initialized reports whether the hook storage has been fully initialized.
// It checks if any stored hook has a controller attached — the controller is set
// during initialization, so its presence indicates that loading is complete.
// Returns true if the storage is empty (no hooks to initialize) or if hooks
// have their controllers set.
func (s *Storage) Initialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, hook := range s.byName {
		// if controller set - hooks storage already initialized
		return hook.GetHookController() != nil
	}

	return true
}

// Add adds a hook to storage, indexing it by name and all its bindings.
// If a hook with the same name exists, it will be replaced.
// Each binding type the hook declares will have the hook added to its list.
func (s *Storage) Add(hook Hook) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byName[hook.GetName()] = hook
	for _, binding := range hook.GetHookConfig().Bindings() {
		s.byBinding[binding] = append(s.byBinding[binding], hook)
	}
}

// GetHooks returns all hooks in storage in arbitrary order.
// The returned slice is safe to use - it's a copy of internal data.
func (s *Storage) GetHooks() []Hook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]Hook, 0, len(s.byName))
	for _, hook := range s.byName {
		res = append(res, hook)
	}

	return res
}

// GetHooksByBinding returns copied slices of all hooks for a specific binding type, sorted by order.
func (s *Storage) GetHooksByBinding(binding shtypes.BindingType) []Hook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, ok := s.byBinding[binding]
	if !ok {
		return nil
	}

	res := make([]Hook, len(stored))
	copy(res, stored)

	sort.Slice(res, func(i, j int) bool {
		return res[i].Order(binding) < res[j].Order(binding)
	})

	return res
}

// GetHookByName returns the hook with the specified name, or nil if not found.
func (s *Storage) GetHookByName(name string) Hook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byName[name]
}

// Clear removes all hooks from storage, resetting it to empty state.
func (s *Storage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byBinding = make(map[shtypes.BindingType][]Hook)
	s.byName = make(map[string]Hook)
}
