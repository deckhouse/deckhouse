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

// Package apps provides the Application type representing a running package instance.
// It manages hooks, values, and execution lifecycle for a single application.
package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg"
	"github.com/flant/addon-operator/pkg/hook/types"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	shkubetypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Application represents a running instance of a package.
// It contains hooks, values storage, and configuration for execution.
//
// Thread Safety: The Application itself is not thread-safe, but its hooks and values
// storage components use internal synchronization.
type Application struct {
	name      string // Package name(namespace.name)
	instance  string // Application instance name
	namespace string // Application instance namespace
	path      string // path to the package dir on fs

	version *semver.Version

	running atomic.Bool

	definition Definition        // Application definition
	digests    map[string]string // Package digests
	repository registry.Remote   // Application repository

	hooks         *hooks.Storage      // Hook storage with indices
	values        *values.Storage     // Values storage with layering
	settingsCheck *kind.SettingsCheck // Hook to validate settings

	patcher           *objectpatch.ObjectPatcher
	scheduleManager   schedulemanager.ScheduleManager
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	logger *log.Logger
}

// Config holds configuration for creating a new Application instance.
type Config struct {
	Path         string            // Path to package dir
	StaticValues addonutils.Values // Static values from values.yaml files

	Definition Definition // Application definition

	Digests    map[string]string // Package images digests(images_digests.json)
	Repository registry.Remote   // Package repository options

	ConfigSchema []byte // OpenAPI config schema (YAML)
	ValuesSchema []byte // OpenAPI values schema (YAML)

	Hooks []hooks.Hook // Discovered hooks

	SettingsCheck *kind.SettingsCheck

	Patcher           *objectpatch.ObjectPatcher
	ScheduleManager   schedulemanager.ScheduleManager
	KubeEventsManager kubeeventsmanager.KubeEventsManager
}

// NewAppByConfig creates a new Application instance with the specified configuration.
// It initializes hook storage, adds all discovered hooks, and creates values storage.
//
// Returns error if hook initialization or values storage creation fails.
func NewAppByConfig(name string, cfg *Config, logger *log.Logger) (*Application, error) {
	a := new(Application)

	splits := strings.Split(name, ".")
	if len(splits) != 2 {
		return nil, fmt.Errorf("invalid application name: %s", name)
	}

	a.namespace = splits[0]
	a.instance = splits[1]

	a.name = name
	a.running = atomic.Bool{}

	a.path = cfg.Path
	a.definition = cfg.Definition
	a.digests = cfg.Digests
	a.repository = cfg.Repository
	a.settingsCheck = cfg.SettingsCheck
	a.patcher = cfg.Patcher
	a.scheduleManager = cfg.ScheduleManager
	a.kubeEventsManager = cfg.KubeEventsManager
	a.logger = logger

	parsed, err := semver.NewVersion(a.definition.Version)
	if err != nil {
		parsed = semver.MustParse("0.0.0")
	}

	a.version = parsed

	a.hooks = hooks.NewStorage()
	if err = a.addHooks(cfg.Hooks...); err != nil {
		return nil, fmt.Errorf("add hooks: %v", err)
	}

	a.values, err = values.NewStorage(a.definition.Name, cfg.StaticValues, cfg.ConfigSchema, cfg.ValuesSchema)
	if err != nil {
		return nil, fmt.Errorf("build values storage: %v", err)
	}

	return a, nil
}

// addHooks initializes and adds hooks to the application's hook storage.
// For each hook, it initializes the configuration and sets up logging/metrics labels.
func (a *Application) addHooks(found ...hooks.Hook) error {
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

		a.hooks.Add(hook)
	}

	return nil
}

// RuntimeValues holds runtime values that are not part of schema.
// These values are passed to helm templates under .Runtime prefix.
type RuntimeValues struct {
	Instance addonutils.Values `json:"Instance"`
	Package  addonutils.Values `json:"Package"`
}

// GetRuntimeValues returns values that are not part of schema.
// Instance contains name and namespace of the running instance.
// Package contains package metadata (name, version, digests, registry).
func (a *Application) GetRuntimeValues() RuntimeValues {
	return RuntimeValues{
		Instance: addonutils.Values{
			"Name":      a.instance,
			"Namespace": a.namespace,
		},
		Package: addonutils.Values{
			"Name":     a.definition.Name,
			"Digests":  a.digests,
			"Registry": a.repository,
			"Version":  a.definition.Version,
		},
	}
}

// GetExtraNelmValues returns runtime values in string format
func (a *Application) GetExtraNelmValues() string {
	runtimeValues := a.GetRuntimeValues()
	marshalled, _ := json.Marshal(runtimeValues)

	return fmt.Sprintf("Application=%s", marshalled)
}

// GetName returns the full application identifier in format "namespace.name".
func (a *Application) GetName() string {
	return a.name
}

// BuildName returns the full application identifier in format "namespace.name".
func BuildName(namespace, name string) string {
	return fmt.Sprintf("%s.%s", namespace, name)
}

// GetNamespace returns the application namespace.
func (a *Application) GetNamespace() string {
	return a.namespace
}

// GetVersion return the package version
func (a *Application) GetVersion() *semver.Version {
	return a.version
}

// GetPath returns path to the package dir
func (a *Application) GetPath() string {
	return a.path
}

// GetQueues returns package queues from all hooks
func (a *Application) GetQueues() []string {
	var res []string //nolint:prealloc
	scheduleHooks := a.hooks.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range scheduleHooks {
		for _, hookBinding := range hook.GetHookConfig().Schedules {
			res = append(res, hookBinding.Queue)
		}
	}

	kubeEventsHooks := a.hooks.GetHooksByBinding(shtypes.OnKubernetesEvent)
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
func (a *Application) GetValuesChecksum() string {
	return a.values.GetValuesChecksum()
}

// GetSettingsChecksum returns a checksum of the current config values.
// Used to detect if settings changed.
func (a *Application) GetSettingsChecksum() string {
	return a.values.GetConfigChecksum()
}

// ValidateSettings validates settings against openAPI and call setting check if exists
func (a *Application) ValidateSettings(ctx context.Context, settings addonutils.Values) (settingscheck.Result, error) {
	if err := a.values.ValidateConfigValues(settings); err != nil {
		return settingscheck.Result{}, err
	}

	// apply defaults from config values spec
	settings = a.values.ApplyDefaultsConfigValues(settings)

	// no need to call the settings check if nothing changed
	if a.values.GetConfigChecksum() == settings.Checksum() {
		return settingscheck.Result{Valid: true}, nil
	}

	if a.settingsCheck != nil {
		return a.settingsCheck.Check(ctx, settings)
	}

	return settingscheck.Result{
		Valid: true,
	}, nil
}

// GetValues returns values for rendering
func (a *Application) GetValues() addonutils.Values {
	return a.values.GetValues()
}

// ApplySettings applies settings values to application
func (a *Application) ApplySettings(settings addonutils.Values) error {
	return a.values.ApplyConfigValues(settings)
}

// GetConstraints return scheduler checks, their determine if an app should be enabled/disabled
func (a *Application) GetConstraints() schedule.Constraints {
	return a.definition.Constraints()
}

// HooksInitialized reports whether the package requires a hook initialize phase.
// This is true when hooks have not yet been initialized (no controllers attached),
// meaning the pkg needs to go through the full startup sequence before it can run.
func (a *Application) HooksInitialized() bool {
	return a.hooks.Initialized()
}

// InitializeHooks initializes hook controllers and bind them to Kubernetes events and schedules
func (a *Application) InitializeHooks() {
	namespace := a.GetNamespace()
	for _, hook := range a.hooks.GetHooks() {
		kubeSubs := make([]shtypes.OnKubernetesEventConfig, 0, len(hook.GetHookConfig().OnKubernetesEvents))
		for _, sub := range hook.GetHookConfig().OnKubernetesEvents {
			sub.Monitor.NamespaceSelector = &shkubetypes.NamespaceSelector{
				NameSelector: &shkubetypes.NameSelector{MatchNames: []string{namespace}},
			}

			kubeSubs = append(kubeSubs, sub)
		}
		hookCtrl := hookcontroller.NewHookController()
		hookCtrl.InitKubernetesBindings(kubeSubs, a.kubeEventsManager, a.logger)
		hookCtrl.InitScheduleBindings(hook.GetHookConfig().Schedules, a.scheduleManager)

		hook.WithHookController(hookCtrl)
		hook.WithTmpDir(os.TempDir())
	}
}

// DisableHooks tears down all active hook bindings and clears the hook registry.
// Called by the Disable task when a package is being stopped or upgraded.
//
// Cleanup order: schedule bindings are disabled first (stops cron triggers),
// then Kubernetes monitors are stopped (stops informer watches), and finally
// the hook registry is cleared so a subsequent InitializeHooks starts fresh.
func (a *Application) DisableHooks() {
	// Disable all schedule-based hooks
	schHooks := a.hooks.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range schHooks {
		if hook.GetHookController() != nil {
			hook.GetHookController().DisableScheduleBindings()
		}
	}

	// Stop all Kubernetes event monitors
	kubeHooks := a.hooks.GetHooksByBinding(shtypes.OnKubernetesEvent)
	for _, hook := range kubeHooks {
		if hook.GetHookController() != nil {
			hook.GetHookController().StopMonitors()
		}
	}

	a.running.Store(false)
	a.hooks.Clear()
}

// UnlockKubernetesMonitors called after sync task is completed to unlock getting events
func (a *Application) UnlockKubernetesMonitors(hook string, monitors ...string) {
	h := a.hooks.GetHookByName(hook)
	if h == nil {
		return
	}

	for _, monitorID := range monitors {
		h.GetHookController().UnlockKubernetesEventsFor(monitorID)
	}
}

// GetHooksByBinding returns all hooks for the specified binding type, sorted by order.
func (a *Application) GetHooksByBinding(binding shtypes.BindingType) []hooks.Hook {
	return a.hooks.GetHooksByBinding(binding)
}

// RunHooksByBinding executes all hooks for a specific binding type in order.
// It creates a binding context with snapshots for BeforeHelm/AfterHelm/AfterDeleteHelm hooks.
func (a *Application) RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error {
	ctx, span := otel.Tracer(a.GetName()).Start(ctx, "RunHooksByBinding")
	defer span.End()

	span.SetAttributes(attribute.String("binding", string(binding)))

	if binding == shtypes.OnStartup && a.running.Load() {
		return nil
	}

	for _, hook := range a.hooks.GetHooksByBinding(binding) {
		bc := bctx.BindingContext{
			Binding: string(binding),
		}
		// Update kubernetes snapshots just before execute a hook
		if binding == types.BeforeHelm || binding == types.AfterHelm || binding == types.AfterDeleteHelm {
			bc.Snapshots = hook.GetHookController().KubernetesSnapshots()
			bc.Metadata.IncludeAllSnapshots = true
		}
		bc.Metadata.BindingType = binding

		if err := a.runHook(ctx, hook, []bctx.BindingContext{bc}); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("run hook '%s': %w", hook.GetName(), err)
		}
	}

	if binding == shtypes.OnStartup {
		a.running.Store(true)
	}

	return nil
}

// RunHookByName executes a specific hook by name with the provided binding context.
// Returns nil if hook is not found (silent no-op).
func (a *Application) RunHookByName(ctx context.Context, name string, bctx []bctx.BindingContext) error {
	ctx, span := otel.Tracer(a.GetName()).Start(ctx, "RunHookByName")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	hook := a.hooks.GetHookByName(name)
	if hook == nil {
		return nil
	}

	// Update kubernetes snapshots just before execute a hook
	bctx = hook.GetHookController().UpdateSnapshots(bctx)

	return a.runHook(ctx, hook, bctx)
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
// Returns error if hook execution or patch application fails.
func (a *Application) runHook(ctx context.Context, h hooks.Hook, bctx []bctx.BindingContext) error {
	ctx, span := otel.Tracer(a.GetName()).Start(ctx, "runHook")
	defer span.End()

	span.SetAttributes(attribute.String("hook", h.GetName()))
	span.SetAttributes(attribute.String("name", a.GetName()))

	hookConfigValues := a.values.GetConfigValues()
	hookValues := a.values.GetValues()
	hookVersion := h.GetConfigVersion()

	hookResult, err := h.Execute(ctx, hookVersion, bctx, a.GetName(), hookConfigValues, hookValues, make(map[string]string))
	if err != nil {
		// we have to check if there are some status patches to apply
		if hookResult != nil && len(hookResult.ObjectPatcherOperations) > 0 {
			patchErr := a.patcher.ExecuteOperations(hookResult.ObjectPatcherOperations)
			if patchErr != nil {
				return fmt.Errorf("exec hook: %w, and exec operations: %w", err, patchErr)
			}
		}

		return fmt.Errorf("exec hook '%s': %w", h.GetName(), err)
	}

	if len(hookResult.ObjectPatcherOperations) > 0 {
		if err = a.patcher.ExecuteOperations(hookResult.ObjectPatcherOperations); err != nil {
			return fmt.Errorf("exec operations: %w", err)
		}
	}

	if valuesPatch, has := hookResult.Patches[addonutils.MemoryValuesPatch]; has && valuesPatch != nil {
		if err = a.values.ApplyValuesPatch(*valuesPatch); err != nil {
			return fmt.Errorf("apply hook values patch: %w", err)
		}
	}

	return nil
}
