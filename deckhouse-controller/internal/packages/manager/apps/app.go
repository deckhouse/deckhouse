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
	"fmt"

	"github.com/flant/addon-operator/pkg"
	"github.com/flant/addon-operator/pkg/hook/types"
	addonhooks "github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/values"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
)

// DependencyContainer provides access to shared services needed by applications.
type DependencyContainer interface {
	KubeObjectPatcher() *objectpatch.ObjectPatcher
}

// Application represents a running instance of a package.
// It contains hooks, values storage, and configuration for execution.
//
// Thread Safety: The Application itself is not thread-safe, but its hooks and values
// storage components use internal synchronization.
type Application struct {
	name string // Application instance name
	path string // path to the package dir on fs

	definition Definition // Application definition

	hooks  *hooks.Storage  // Hook storage with indices
	values *values.Storage // Values storage with layering
}

// ApplicationConfig holds configuration for creating a new Application instance.
type ApplicationConfig struct {
	StaticValues addonutils.Values // Static values from values.yaml files

	Definition Definition // Application definition

	ConfigSchema []byte // OpenAPI config schema (YAML)
	ValuesSchema []byte // OpenAPI values schema (YAML)

	Hooks []*addonhooks.ModuleHook // Discovered hooks
}

// NewApplication creates a new Application instance with the specified configuration.
// It initializes hook storage, adds all discovered hooks, and creates values storage.
//
// Returns error if hook initialization or values storage creation fails.
func NewApplication(name string, cfg ApplicationConfig) (*Application, error) {
	a := new(Application)

	a.name = name
	a.definition = cfg.Definition

	a.hooks = hooks.NewStorage()
	if err := a.addHooks(cfg.Hooks...); err != nil {
		return nil, fmt.Errorf("add hooks: %v", err)
	}

	var err error
	a.values, err = values.NewStorage(a.definition.Name, cfg.StaticValues, cfg.ConfigSchema, cfg.ValuesSchema)
	if err != nil {
		return nil, fmt.Errorf("new values storage: %v", err)
	}

	return a, nil
}

// addHooks initializes and adds hooks to the application's hook storage.
// For each hook, it initializes the configuration and sets up logging/metrics labels.
func (a *Application) addHooks(found ...*addonhooks.ModuleHook) error {
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

// GetName returns the full application identifier in format "namespace:name".
func (a *Application) GetName() string {
	return a.name
}

// BuildName returns the full application identifier in format "namespace:name".
func BuildName(namespace, name string) string {
	return fmt.Sprintf("%s:%s", namespace, name)
}

// GetPath returns path to the package dir
func (a *Application) GetPath() string {
	return a.path
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

// GetValues returns values for rendering
func (a *Application) GetValues() addonutils.Values {
	return a.values.GetValues()
}

// ApplySettings apply setting values to application
func (a *Application) ApplySettings(settings addonutils.Values) error {
	return a.values.ApplyConfigValues(settings)
}

// GetChecks return scheduler checks, their determine if an app should be enabled/disabled
func (a *Application) GetChecks() schedule.Checks {
	deps := make(map[string]dependency.Dependency)
	for module, dep := range a.definition.Requirements.Modules {
		deps[module] = dependency.Dependency{
			Constraint: dep.Constraints,
			Optional:   dep.Optional,
		}
	}

	return schedule.Checks{
		Kubernetes: a.definition.Requirements.Kubernetes,
		Deckhouse:  a.definition.Requirements.Deckhouse,
		Modules:    deps,
	}
}

// GetHooks returns all hooks for this application in arbitrary order.
func (a *Application) GetHooks() []*addonhooks.ModuleHook {
	return a.hooks.GetHooks()
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
func (a *Application) GetHooksByBinding(binding shtypes.BindingType) []*addonhooks.ModuleHook {
	return a.hooks.GetHooksByBinding(binding)
}

// RunHooksByBinding executes all hooks for a specific binding type in order.
// It creates a binding context with snapshots for BeforeHelm/AfterHelm/AfterDeleteHelm hooks.
func (a *Application) RunHooksByBinding(ctx context.Context, binding shtypes.BindingType, dc DependencyContainer) error {
	ctx, span := otel.Tracer(a.GetName()).Start(ctx, "RunHooksByBinding")
	defer span.End()

	span.SetAttributes(attribute.String("binding", string(binding)))

	for _, hook := range a.hooks.GetHooksByBinding(binding) {
		bc := bindingcontext.BindingContext{
			Binding: string(binding),
		}
		// Update kubernetes snapshots just before execute a hook
		if binding == types.BeforeHelm || binding == types.AfterHelm || binding == types.AfterDeleteHelm {
			bc.Snapshots = hook.GetHookController().KubernetesSnapshots()
			bc.Metadata.IncludeAllSnapshots = true
		}
		bc.Metadata.BindingType = binding

		if err := a.runHook(ctx, hook, []bindingcontext.BindingContext{bc}, dc); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("run hook '%s': %w", hook.GetName(), err)
		}
	}

	return nil
}

// RunHookByName executes a specific hook by name with the provided binding context.
// Returns nil if hook is not found (silent no-op).
func (a *Application) RunHookByName(ctx context.Context, name string, bctx []bindingcontext.BindingContext, dc DependencyContainer) error {
	ctx, span := otel.Tracer(a.GetName()).Start(ctx, "RunHookByName")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	hook := a.hooks.GetHookByName(name)
	if hook == nil {
		return nil
	}

	// Update kubernetes snapshots just before execute a hook
	bctx = hook.GetHookController().UpdateSnapshots(bctx)

	return a.runHook(ctx, hook, bctx, dc)
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
func (a *Application) runHook(ctx context.Context, h *addonhooks.ModuleHook, bctx []bindingcontext.BindingContext, dc DependencyContainer) error {
	hookConfigValues := a.values.GetConfigValues()
	hookValues := a.values.GetValues()
	hookVersion := h.GetConfigVersion()

	hookResult, err := h.Execute(ctx, hookVersion, bctx, a.GetName(), hookConfigValues, hookValues, make(map[string]string))
	if err != nil {
		// we have to check if there are some status patches to apply
		if hookResult != nil && len(hookResult.ObjectPatcherOperations) > 0 {
			patchErr := dc.KubeObjectPatcher().ExecuteOperations(hookResult.ObjectPatcherOperations)
			if patchErr != nil {
				return fmt.Errorf("exec hook: %w, and exec operations: %w", err, patchErr)
			}
		}

		return fmt.Errorf("exec hook: %w", err)
	}

	if len(hookResult.ObjectPatcherOperations) > 0 {
		if err = dc.KubeObjectPatcher().ExecuteOperations(hookResult.ObjectPatcherOperations); err != nil {
			return fmt.Errorf("exec operations: %w", err)
		}
	}

	if valuesPatch, has := hookResult.Patches[addonutils.MemoryValuesPatch]; has && valuesPatch != nil {
		if err = a.values.ApplyPatch(*valuesPatch); err != nil {
			return fmt.Errorf("apply hook values patch: %w", err)
		}
	}

	return nil
}
