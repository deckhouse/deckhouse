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
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	utils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
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

// ControllableHook is the minimal hook view the runtime needs to enable bindings
// and route events: the hook's name and its controller. Both module hooks (Hook)
// and global hooks (GlobalHook) satisfy it, so tasks shared across them (Enable,
// hook routing) can depend on this instead of a concrete hook type.
type ControllableHook interface {
	GetName() string
	GetHookController() *controller.HookController
}

// ToControllable adapts a slice of concrete hooks to the ControllableHook view.
func ToControllable[H ControllableHook](in []H) []ControllableHook {
	out := make([]ControllableHook, len(in))
	for i, h := range in {
		out[i] = h
	}
	return out
}

// Storage provides thread-safe storage for hooks with multiple access patterns.
// `all` is the authoritative set; the two maps are indices over it:
//   - byName: Fast lookup by hook name (O(1)); a name is NOT unique — see Add
//   - byBinding: Fast lookup by binding type (O(1))
//
// Thread Safety: All methods use RWMutex for concurrent access.
type Storage struct {
	mu        sync.RWMutex                   // Protects all fields
	all       []Hook                         // Every hook added, in insertion order — the authoritative set
	byBinding map[shtypes.BindingType][]Hook // Hooks grouped by binding type
	byName    map[string]Hook                // Hooks indexed by name; one entry per name, last writer wins
}

// NewStorage creates a new empty hook storage.
func NewStorage() *Storage {
	return &Storage{
		byBinding: make(map[shtypes.BindingType][]Hook),
		byName:    make(map[string]Hook),
	}
}

// Add stores a hook and indexes it by name and by every binding it declares.
//
// Names are not unique: the SDK derives a Go hook's name from the file that
// registered it, so a file with two RegisterFunc calls yields two distinct
// hooks under one name. Only the name index collapses them — `all` and
// byBinding keep both, which is what lets every hook get a hook controller.
func (s *Storage) Add(hook Hook) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.all = append(s.all, hook)

	s.byName[hook.GetName()] = hook
	for _, binding := range hook.GetHookConfig().Bindings() {
		s.byBinding[binding] = append(s.byBinding[binding], hook)
	}
}

// GetHooks returns every hook in storage, in insertion order, as a copy.
// It reads `all`, never byName: two hooks registered from one file share a name,
// and building the result from the map would silently drop one — leaving it
// without a hook controller for the rest of the package's life.
func (s *Storage) GetHooks() []Hook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]Hook, len(s.all))
	copy(res, s.all)

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
