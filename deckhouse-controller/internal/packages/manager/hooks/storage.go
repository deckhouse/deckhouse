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

package hooks

import (
	"sort"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
)

// Storage provides thread-safe storage for hooks with multiple access patterns.
// It maintains two indices:
//   - byName: Fast lookup by hook name (O(1))
//   - byBinding: Fast lookup by binding type (O(1))
//
// Thread Safety: All methods use RWMutex for concurrent access.
type Storage struct {
	mu        sync.RWMutex                                // Protects all fields
	byBinding map[shtypes.BindingType][]*hooks.ModuleHook // Hooks grouped by binding type
	byName    map[string]*hooks.ModuleHook                // Hooks indexed by name
}

// NewStorage creates a new empty hook storage.
func NewStorage() *Storage {
	return &Storage{
		byBinding: make(map[shtypes.BindingType][]*hooks.ModuleHook),
		byName:    make(map[string]*hooks.ModuleHook),
	}
}

// Add adds a hook to storage, indexing it by name and all its bindings.
// If a hook with the same name exists, it will be replaced.
// Each binding type the hook declares will have the hook added to its list.
func (s *Storage) Add(hook *hooks.ModuleHook) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byName[hook.GetName()] = hook
	for _, binding := range hook.GetHookConfig().Bindings() {
		s.byBinding[binding] = append(s.byBinding[binding], hook)
	}
}

// GetHooks returns all hooks in storage in arbitrary order.
// The returned slice is safe to use - it's a copy of internal data.
func (s *Storage) GetHooks() []*hooks.ModuleHook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*hooks.ModuleHook, 0, len(s.byName))
	for _, hook := range s.byName {
		res = append(res, hook)
	}

	return res
}

// GetHooksByBinding returns copied slices of all hooks for a specific binding type, sorted by order.
func (s *Storage) GetHooksByBinding(binding shtypes.BindingType) []*hooks.ModuleHook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, ok := s.byBinding[binding]
	if !ok {
		return nil
	}

	res := make([]*hooks.ModuleHook, 0, len(stored))
	copy(res, stored)

	sort.Slice(res, func(i, j int) bool {
		return res[i].Order(binding) < res[j].Order(binding)
	})

	return res
}

// GetHookByName returns the hook with the specified name, or nil if not found.
func (s *Storage) GetHookByName(name string) *hooks.ModuleHook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byName[name]
}

// Clear removes all hooks from storage, resetting it to empty state.
func (s *Storage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byBinding = make(map[shtypes.BindingType][]*hooks.ModuleHook)
	s.byName = make(map[string]*hooks.ModuleHook)
}
