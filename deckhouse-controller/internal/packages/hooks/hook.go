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
	"time"

	addonhooks "github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	shconfig "github.com/flant/shell-operator/pkg/hook/config"
	"github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
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
// and global hooks (GlobalHook) satisfy it, so the hook lanes start and route both kinds
// through this instead of a concrete hook type.
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

// moduleConfigured and globalConfigured are hooks with a module or global hook config.
type (
	moduleConfigured interface {
		GetHookConfig() *addonhooks.ModuleHookConfig
	}
	globalConfigured interface {
		GetHookConfig() *addonhooks.GlobalHookConfig
	}
)

// ExecutionSettings are the execution rate settings of h's config (executionMinInterval and
// executionBurst); nil when it sets none or its config is not a module or global hook's.
func ExecutionSettings(h ControllableHook) *shtypes.Settings {
	if cfg := shellConfig(h); cfg != nil {
		return cfg.Settings
	}

	return nil
}

// BindingType is the binding type a run's contexts carry, the binding label of the hook run
// metrics; "" without contexts.
func BindingType(contexts []bctx.BindingContext) string {
	if len(contexts) == 0 {
		return ""
	}

	return string(contexts[0].Metadata.BindingType)
}

// defaultQueue is the queue of a run outside a Kubernetes or schedule binding, as addon-operator
// labels it.
const defaultQueue = "main"

// RunMetrics are the names one kind of hook writes its runs under.
type RunMetrics struct {
	Seconds       string
	Success       string
	Errors        string
	AllowedErrors string
}

// Run metric names of module (and application) hooks and of global hooks.
var (
	ModuleRunMetrics = RunMetrics{
		Seconds:       metrics.ModuleHookRunSeconds,
		Success:       metrics.ModuleHookSuccessTotal,
		Errors:        metrics.ModuleHookErrorsTotal,
		AllowedErrors: metrics.ModuleHookAllowedErrorsTotal,
	}
	GlobalRunMetrics = RunMetrics{
		Seconds:       metrics.GlobalHookRunSeconds,
		Success:       metrics.GlobalHookSuccessTotal,
		Errors:        metrics.GlobalHookErrorsTotal,
		AllowedErrors: metrics.GlobalHookAllowedErrorsTotal,
	}
)

// runStorage is the part of the metric storage the run metrics need.
type runStorage interface {
	HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64)
	CounterAdd(metric string, value float64, labels map[string]string)
}

// RunBinding is the queue of the binding a run's contexts name and whether that binding allows a
// failure; "main" and false outside a Kubernetes or schedule binding.
func RunBinding(h any, contexts []bctx.BindingContext) (string, bool) {
	cfg := shellConfig(h)
	if cfg == nil || len(contexts) == 0 {
		return defaultQueue, false
	}

	name := contexts[0].Binding

	switch contexts[0].Metadata.BindingType {
	case shtypes.OnKubernetesEvent:
		for _, b := range cfg.OnKubernetesEvents {
			if b.BindingName == name || (b.Group != "" && b.Group == name) {
				return queueOr(b.Queue), b.AllowFailure
			}
		}
	case shtypes.Schedule:
		for _, b := range cfg.Schedules {
			if b.BindingName == name || (b.Group != "" && b.Group == name) {
				return queueOr(b.Queue), b.AllowFailure
			}
		}
	}

	return defaultQueue, false
}

// queueOr is queue, or defaultQueue for a binding that names none.
func queueOr(queue string) string {
	if queue == "" {
		return defaultQueue
	}

	return queue
}

// RecordRun writes a run's duration and, for a Kubernetes or schedule run (the ones addon-operator
// counts), its outcome: success, error, or an error its binding allows. A run ended by ctx counts
// no outcome: the lane stopped, the hook did not fail. Without a storage it records nothing.
func RecordRun(ctx context.Context, storage runStorage, names RunMetrics, labels map[string]string, contexts []bctx.BindingContext, allowFailure bool, took time.Duration, err error) {
	if storage == nil {
		return
	}

	storage.HistogramObserve(names.Seconds, took.Seconds(), labels, nil)

	switch BindingType(contexts) {
	case string(shtypes.OnKubernetesEvent), string(shtypes.Schedule):
	default:
		return
	}

	switch {
	case err == nil:
		storage.CounterAdd(names.Success, 1, labels)
	case ctx.Err() != nil:
	case allowFailure:
		storage.CounterAdd(names.AllowedErrors, 1, labels)
	default:
		storage.CounterAdd(names.Errors, 1, labels)
	}
}

// shellConfig is the shell-operator config module and global hooks embed; nil for another hook.
func shellConfig(h any) *shconfig.HookConfig {
	switch h := h.(type) {
	case moduleConfigured:
		if cfg := h.GetHookConfig(); cfg != nil {
			return &cfg.HookConfig
		}
	case globalConfigured:
		if cfg := h.GetHookConfig(); cfg != nil {
			return &cfg.HookConfig
		}
	}

	return nil
}
