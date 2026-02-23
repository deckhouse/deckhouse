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

package global

import (
	"context"
	"fmt"
	"os"

	"github.com/flant/addon-operator/pkg"
	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Module struct {
	name string // Package name
	path string // path to the package dir on fs

	hooks  *hooks.GlobalStorage // Hook storage with indices
	values *values.Storage      // Values storage with layering

	patcher           *objectpatch.ObjectPatcher
	scheduleManager   schedulemanager.ScheduleManager
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	logger *log.Logger
}

// Config holds configuration for creating a new Module instance.
type Config struct {
	Path         string            // Path to package dir
	StaticValues addonutils.Values // Static values from values.yaml files

	ConfigSchema []byte // OpenAPI config schema (YAML)
	ValuesSchema []byte // OpenAPI values schema (YAML)

	Hooks []hooks.GlobalHook // Discovered hooks

	Patcher           *objectpatch.ObjectPatcher
	ScheduleManager   schedulemanager.ScheduleManager
	KubeEventsManager kubeeventsmanager.KubeEventsManager
}

// NewModuleByConfig creates a new Module instance with the specified configuration.
// It initializes hook storage, adds all discovered hooks, and creates values storage.
//
// Returns error if hook initialization or values storage creation fails.
func NewModuleByConfig(cfg *Config, logger *log.Logger) (*Module, error) {
	m := new(Module)

	m.name = "global"

	m.path = cfg.Path
	m.patcher = cfg.Patcher
	m.scheduleManager = cfg.ScheduleManager
	m.kubeEventsManager = cfg.KubeEventsManager
	m.logger = logger

	m.hooks = hooks.NewGlobalStorage()
	if err := m.addHooks(cfg.Hooks...); err != nil {
		return nil, fmt.Errorf("add hooks: %v", err)
	}

	var err error
	m.values, err = values.NewStorage(m.name, cfg.StaticValues, cfg.ConfigSchema, cfg.ValuesSchema)
	if err != nil {
		return nil, fmt.Errorf("new values storage: %v", err)
	}

	return m, nil
}

// addHooks initializes and adds hooks to the module's hook storage.
// For each hook, it initializes the configuration and sets up logging/metrics labels.
func (m *Module) addHooks(found ...hooks.GlobalHook) error {
	for _, hook := range found {
		if err := hook.InitializeHookConfig(); err != nil {
			return fmt.Errorf("initialize hook configuration: %w", err)
		}

		// Configure logging and metrics labels for Kubernetes event hooks
		for _, kubeCfg := range hook.GetHookConfig().OnKubernetesEvents {
			kubeCfg.Monitor.Metadata.LogLabels[pkg.LogKeyHook] = hook.GetName()
			kubeCfg.Monitor.Metadata.LogLabels["hook.type"] = "package"

			kubeCfg.Monitor.Metadata.MetricLabels = map[string]string{
				pkg.MetricKeyHook:    hook.GetName(),
				pkg.MetricKeyBinding: kubeCfg.BindingName,
				pkg.MetricKeyQueue:   kubeCfg.Queue,
				pkg.MetricKeyKind:    kubeCfg.Monitor.Kind,
			}
		}

		m.hooks.Add(hook)
	}

	return nil
}

// GetName returns the full module identifier.
func (m *Module) GetName() string {
	return m.name
}

// GetVersion return the package version
func (m *Module) GetVersion() string {
	return "v0.0.0"
}

// GetPath returns path to the package dir
func (m *Module) GetPath() string {
	return m.path
}

// GetValuesChecksum returns a checksum of the current values.
// Used to detect if values changed after hook execution.
func (m *Module) GetValuesChecksum() string {
	return m.values.GetValuesChecksum()
}

// GetSettingsChecksum returns a checksum of the current config values.
// Used to detect if settings changed.
func (m *Module) GetSettingsChecksum() string {
	return m.values.GetConfigChecksum()
}

// GetValues returns values for rendering
func (m *Module) GetValues() addonutils.Values {
	return m.values.GetValues()
}

// ValidateSettings validates settings against openAPI
func (m *Module) ValidateSettings(_ context.Context, settings addonutils.Values) (settingscheck.Result, error) {
	if err := m.values.ValidateConfigValues(settings); err != nil {
		return settingscheck.Result{}, err
	}

	// apply defaults from config values spec
	settings = m.values.ApplyDefaultsConfigValues(settings)

	// no need to call the settings check if nothing changed
	if m.values.GetConfigChecksum() == settings.Checksum() {
		return settingscheck.Result{Valid: true}, nil
	}

	return settingscheck.Result{
		Valid: true,
	}, nil
}

// ApplySettings apply settings values
func (m *Module) ApplySettings(settings addonutils.Values) error {
	return m.values.ApplyConfigValues(settings)
}

// InitializeHooks initializes hook controllers and bind them to Kubernetes events and schedules
func (m *Module) InitializeHooks() {
	for _, hook := range m.hooks.GetHooks() {
		hookCtrl := hookcontroller.NewHookController()
		hookCtrl.InitKubernetesBindings(hook.GetHookConfig().OnKubernetesEvents, m.kubeEventsManager, m.logger)
		hookCtrl.InitScheduleBindings(hook.GetHookConfig().Schedules, m.scheduleManager)

		hook.WithHookController(hookCtrl)
		hook.WithTmpDir(os.TempDir())
	}
}

// UnlockKubernetesMonitors called after sync task is completed to unlock getting events
func (m *Module) UnlockKubernetesMonitors(hook string, monitors ...string) {
	h := m.hooks.GetHookByName(hook)
	if h == nil {
		return
	}

	for _, monitorID := range monitors {
		h.GetHookController().UnlockKubernetesEventsFor(monitorID)
	}
}

// RunHookByName runs some specified hook by its name
func (m *Module) RunHookByName(ctx context.Context, name string, bctx []bctx.BindingContext) error {
	hook := m.hooks.GetHookByName(name)
	if hook == nil {
		return nil
	}

	// Update kubernetes snapshots just before execute m hook
	bctx = hook.GetHookController().UpdateSnapshots(bctx)

	return m.runHook(ctx, hook, bctx)
}

// RunHooksByBinding executes all hooks for a specific binding type in order.
// It creates a binding context with snapshots for BeforeAll hooks.
func (m *Module) RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error {
	ctx, span := otel.Tracer(m.GetName()).Start(ctx, "RunHooksByBinding")
	defer span.End()

	span.SetAttributes(attribute.String("binding", string(binding)))

	for _, hook := range m.hooks.GetHooksByBinding(binding) {
		bc := bctx.BindingContext{
			Binding: string(binding),
		}
		// Update kubernetes snapshots just before execute m hook
		if binding == addontypes.BeforeAll {
			bc.Snapshots = hook.GetHookController().KubernetesSnapshots()
			bc.Metadata.IncludeAllSnapshots = true
		}
		bc.Metadata.BindingType = binding

		if err := m.runHook(ctx, hook, []bctx.BindingContext{bc}); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("run hook '%s': %w", hook.GetName(), err)
		}
	}

	return nil
}

// runHook executes a single hook with the specified binding context.
// It prepares hook values, executes the hook, applies patches, and handles errors.
//
// Process:
//  1. Prepare config values and full values for the hook
//  2. Execute the hook script/binary
//  3. Apply Kubernetes object patches (even if hook fails)
//  4. Apply values patches to storage
//
// Returns error if hook execution or patch fails.
func (m *Module) runHook(ctx context.Context, h hooks.GlobalHook, bctx []bctx.BindingContext) error {
	ctx, span := otel.Tracer(m.GetName()).Start(ctx, "runHook")
	defer span.End()

	span.SetAttributes(attribute.String("hook", h.GetName()))
	span.SetAttributes(attribute.String("name", m.GetName()))

	hookConfigValues := m.values.GetConfigValues()
	hookValues := m.values.GetValues()
	hookVersion := h.GetConfigVersion()

	hookResult, err := h.Execute(ctx, hookVersion, bctx, m.GetName(), hookConfigValues, hookValues, make(map[string]string))
	if err != nil {
		// we have to check if there are some status patches to apply
		if hookResult != nil && len(hookResult.ObjectPatcherOperations) > 0 {
			patchErr := m.patcher.ExecuteOperations(hookResult.ObjectPatcherOperations)
			if patchErr != nil {
				return fmt.Errorf("exec hook: %w, and exec operations: %w", err, patchErr)
			}
		}

		return fmt.Errorf("exec hook '%s': %w", h.GetName(), err)
	}

	if len(hookResult.ObjectPatcherOperations) > 0 {
		if err = m.patcher.ExecuteOperations(hookResult.ObjectPatcherOperations); err != nil {
			return fmt.Errorf("exec operations: %w", err)
		}
	}

	if valuesPatch, has := hookResult.Patches[addonutils.MemoryValuesPatch]; has && valuesPatch != nil {
		if err = m.values.ApplyValuesPatch(*valuesPatch); err != nil {
			return fmt.Errorf("apply hook values patch: %w", err)
		}
	}

	return nil
}
