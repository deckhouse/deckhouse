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

package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/flant/addon-operator/pkg"
	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/objectprefix"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Module represents a running instance of a package.
// It contains hooks, values storage, and configuration for execution.
//
// Thread Safety: The Module itself is not thread-safe, but its hooks and values
// storage components use internal synchronization.
type Module struct {
	name string // Package name
	path string // path to the package dir on fs

	definition Definition        // Module definition
	digests    map[string]string // Package digests
	repository registry.Remote   // Module repository

	hooks         *hooks.Storage      // Hook storage with indices
	values        *values.Storage     // Values storage with layering
	settingsCheck *kind.SettingsCheck // Hook to validate settings

	patcher           *objectpatch.ObjectPatcher
	scheduleManager   schedulemanager.ScheduleManager
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	globalValuesGetter GlobalValuesGetter

	logger *log.Logger
}

// Config holds configuration for creating a new Module instance.
type Config struct {
	Path         string            // Path to package dir
	StaticValues addonutils.Values // Static values from values.yaml files

	Definition Definition // Module definition

	Digests    map[string]string // Package images digests(images_digests.json)
	Repository registry.Remote   // Package repository options

	ConfigSchema []byte // OpenAPI config schema (YAML)
	ValuesSchema []byte // OpenAPI values schema (YAML)

	Hooks []hooks.Hook // Discovered hooks

	SettingsCheck *kind.SettingsCheck

	Patcher           *objectpatch.ObjectPatcher
	ScheduleManager   schedulemanager.ScheduleManager
	KubeEventsManager kubeeventsmanager.KubeEventsManager

	GlobalValuesGetter GlobalValuesGetter
}

type GlobalValuesGetter func(prefix bool) addonutils.Values

// NewModuleByConfig creates a new Module instance with the specified configuration.
// It initializes hook storage, adds all discovered hooks, and creates values storage.
//
// Returns error if hook initialization or values storage creation fails.
func NewModuleByConfig(name string, cfg *Config, logger *log.Logger) (*Module, error) {
	m := new(Module)

	m.name = name

	m.path = cfg.Path
	m.definition = cfg.Definition
	m.digests = cfg.Digests
	m.repository = cfg.Repository
	m.settingsCheck = cfg.SettingsCheck
	m.patcher = cfg.Patcher
	m.scheduleManager = cfg.ScheduleManager
	m.kubeEventsManager = cfg.KubeEventsManager
	m.globalValuesGetter = cfg.GlobalValuesGetter
	m.logger = logger

	m.hooks = hooks.NewStorage()
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
func (m *Module) addHooks(found ...hooks.Hook) error {
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

// RuntimeValues holds runtime values that are not part of schema.
// These values are passed to helm templates under .Runtime prefix.
type RuntimeValues struct {
	Package addonutils.Values
}

// GetRuntimeValues returns values that are not part of schema.
// Instance contains name and namespace of the running instance.
// Package contains package metadata (name, version, digests, registry).
func (m *Module) GetRuntimeValues() RuntimeValues {
	return RuntimeValues{
		Package: addonutils.Values{
			"Name":     m.definition.Name,
			"Digests":  m.digests,
			"Registry": m.repository,
			"Version":  m.definition.Version,
		},
	}
}

// GetExtraNelmValues returns runtime values in string format
func (m *Module) GetExtraNelmValues() string {
	runtimeValues := m.GetRuntimeValues()
	packageJSON, _ := json.Marshal(runtimeValues.Package)

	globalValues := m.globalValuesGetter(false)
	globalJSON, _ := json.Marshal(globalValues)

	return fmt.Sprintf("Package=%s,Deckhouse=%s", packageJSON, globalJSON)
}

// GetName returns the full module identifier.
func (m *Module) GetName() string {
	return m.name
}

// GetVersion return the package version
func (m *Module) GetVersion() string {
	return m.definition.Version
}

// GetPath returns path to the package dir
func (m *Module) GetPath() string {
	return m.path
}

// GetQueues returns package queues from all hooks
func (m *Module) GetQueues() []string {
	var res []string //nolint:prealloc
	scheduleHooks := m.hooks.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range scheduleHooks {
		for _, hookBinding := range hook.GetHookConfig().Schedules {
			res = append(res, hookBinding.Queue)
		}
	}

	kubeEventsHooks := m.hooks.GetHooksByBinding(shtypes.OnKubernetesEvent)
	for _, hook := range kubeEventsHooks {
		for _, hookBinding := range hook.GetHookConfig().OnKubernetesEvents {
			res = append(res, hookBinding.Queue)
		}
	}

	slices.Sort(res)
	return slices.Compact(res)
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

// ValidateSettings validates settings against openAPI and call setting check if exists
func (m *Module) ValidateSettings(ctx context.Context, settings addonutils.Values) (settingscheck.Result, error) {
	if err := m.values.ValidateConfigValues(settings); err != nil {
		return settingscheck.Result{}, err
	}

	// apply defaults from config values spec
	settings = m.values.ApplyDefaultsConfigValues(settings)

	// no need to call the settings check if nothing changed
	if m.values.GetConfigChecksum() == settings.Checksum() {
		return settingscheck.Result{Valid: true}, nil
	}

	if m.settingsCheck != nil {
		return m.settingsCheck.Check(ctx, settings)
	}

	return settingscheck.Result{
		Valid: true,
	}, nil
}

// GetValues returns values for rendering
func (m *Module) GetValues() addonutils.Values {
	return addonutils.MergeValues(
		addonutils.Values{"global": m.globalValuesGetter(false)},
		m.values.GetValues())
}

// ApplySettings apply settings values
func (m *Module) ApplySettings(settings addonutils.Values) error {
	return m.values.ApplyConfigValues(settings)
}

// GetChecks return scheduler checks, their determine if an app should be enabled/disabled
func (m *Module) GetChecks() schedule.Checks {
	return m.definition.Requirements.Checks()
}

// GetHooks returns all hooks for this module in arbitrary order.
func (m *Module) GetHooks() []hooks.Hook {
	return m.hooks.GetHooks()
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

// GetHooksByBinding returns all hooks for the specified binding type, sorted by order.
func (m *Module) GetHooksByBinding(binding shtypes.BindingType) []hooks.Hook {
	return m.hooks.GetHooksByBinding(binding)
}

// RunHooksByBinding executes all hooks for a specific binding type in order.
// It creates a binding context with snapshots for BeforeHelm/AfterHelm/AfterDeleteHelm hooks.
func (m *Module) RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error {
	ctx, span := otel.Tracer(m.GetName()).Start(ctx, "RunHooksByBinding")
	defer span.End()

	span.SetAttributes(attribute.String("binding", string(binding)))

	for _, hook := range m.hooks.GetHooksByBinding(binding) {
		bc := bctx.BindingContext{
			Binding: string(binding),
		}
		// Update kubernetes snapshots just before execute m hook
		if binding == addontypes.BeforeHelm || binding == addontypes.AfterHelm || binding == addontypes.AfterDeleteHelm {
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

// RunHookByName executes a specific hook by name with the provided binding context.
// Returns nil if hook is not found (silent no-op).
func (m *Module) RunHookByName(ctx context.Context, name string, bctx []bctx.BindingContext) error {
	ctx, span := otel.Tracer(m.GetName()).Start(ctx, "RunHookByName")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	hook := m.hooks.GetHookByName(name)
	if hook == nil {
		return nil
	}

	// Update kubernetes snapshots just before execute m hook
	bctx = hook.GetHookController().UpdateSnapshots(bctx)

	return m.runHook(ctx, hook, bctx)
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
func (m *Module) runHook(ctx context.Context, h hooks.Hook, bctx []bctx.BindingContext) error {
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
			objectprefix.NormalizeManagedServicesPrefix(hookResult.ObjectPatcherOperations)
			patchErr := m.patcher.ExecuteOperations(hookResult.ObjectPatcherOperations)
			if patchErr != nil {
				return fmt.Errorf("exec hook: %w, and exec operations: %w", err, patchErr)
			}
		}

		return fmt.Errorf("exec hook '%s': %w", h.GetName(), err)
	}

	if len(hookResult.ObjectPatcherOperations) > 0 {
		objectprefix.NormalizeManagedServicesPrefix(hookResult.ObjectPatcherOperations)
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
