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
	"sync"

	addonhooks "github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	"github.com/flant/addon-operator/pkg/utils"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
)

type GlobalHook interface {
	GetName() string
	GetConfigVersion() string
	GetHookConfig() *addonhooks.GlobalHookConfig
	Order(binding shtypes.BindingType) float64

	InitializeHookConfig() error

	GetHookController() *controller.HookController
	WithHookController(ctrl *controller.HookController)
	WithTmpDir(tmpDir string)

	SynchronizationNeeded() bool

	Execute(ctx context.Context, version string, bctx []bctx.BindingContext, packageName string, configValues, values utils.Values, logLabels map[string]string) (*kind.HookResult, error)
}

// GlobalStorage provides thread-safe storage for global hooks.
// It maintains a single index:
//   - byName: Fast lookup by hook name (O(1))
//
// Thread Safety: All methods use RWMutex for concurrent access.
type GlobalStorage struct {
	mu     sync.RWMutex          // Protects all fields
	byName map[string]GlobalHook // Hooks indexed by name
}

// NewGlobalStorage creates a new empty global hook storage.
func NewGlobalStorage() *GlobalStorage {
	return &GlobalStorage{
		byName: make(map[string]GlobalHook),
	}
}

// Add adds a global hook to storage, indexing it by name.
// If a hook with the same name exists, it will be replaced.
func (s *GlobalStorage) Add(hook GlobalHook) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byName[hook.GetName()] = hook
}

// GetHooks returns all hooks in storage in arbitrary order.
// The returned slice is safe to use - it's a copy of internal data.
func (s *GlobalStorage) GetHooks() []GlobalHook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]GlobalHook, 0, len(s.byName))
	for _, hook := range s.byName {
		res = append(res, hook)
	}

	return res
}

// GetHookByName returns the hook with the specified name, or nil if not found.
func (s *GlobalStorage) GetHookByName(name string) GlobalHook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byName[name]
}

// Clear removes all hooks from storage, resetting it to empty state.
func (s *GlobalStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byName = make(map[string]GlobalHook)
}
