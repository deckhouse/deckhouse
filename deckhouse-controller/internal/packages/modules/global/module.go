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
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"
	"github.com/ettle/strcase"
	"github.com/flant/addon-operator/pkg"
	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/goccy/go-yaml"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"
	sdkutils "github.com/deckhouse/module-sdk/pkg/utils"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Module struct {
	name string // Package name
	path string // path to the package dir on fs

	hooks  *hooks.GlobalStorage // Hook storage with indices
	values *values.Storage      // Values storage with layering

	// running tracks whether OnStartup hooks have completed successfully.
	// When true, subsequent OnStartup binding calls are skipped (idempotency guard).
	running atomic.Bool

	// initialized tracks whether hook controllers have been built, so the Enable
	// task skips re-initialization on every reschedule.
	initialized atomic.Bool

	patcher           *objectpatch.ObjectPatcher
	scheduleManager   schedulemanager.ScheduleManager
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	// enabledMu guards dynamicEnabled and configEnabled: global hooks and the
	// config controller write them while the scheduler reads the resolved state
	// concurrently through IsEnabled. It is a leaf lock (no global method calls
	// back into the scheduler or runtime), so it is safe to take under either
	// r.mu (writer) or the scheduler's lock (reader) without ordering cycles.
	enabledMu      sync.RWMutex
	dynamicEnabled map[string]bool // Dynamic enabled state set by global hooks, keyed by kebab-case module name.
	configEnabled  map[string]bool // Explicit ModuleConfig enabled intent, keyed by kebab-case module name; key present iff the user expressed an opinion.

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
	m.running = atomic.Bool{}
	m.dynamicEnabled = make(map[string]bool)
	m.configEnabled = make(map[string]bool)

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
func (m *Module) GetVersion() *semver.Version {
	return semver.MustParse("v0.0.0")
}

// GetConstraints returns the scheduler constraints for the global module: it
// sits at order 0 (the barrier ahead of every other package) and is always
// enabled. Not registered with the scheduler yet — see the global-node wiring
// in runtime.
func (m *Module) GetConstraints() schedule.Constraints {
	return schedule.Constraints{
		Order: 0,
		Floor: rule.Static(rule.Enable),
	}
}

// GetPath returns path to the package dir
func (m *Module) GetPath() string {
	return m.path
}

// GetHookSnapshotsDump returns a YAML snapshot of hook controller snapshots.
// If include is provided, only hooks matching those names are included.
func (m *Module) GetHookSnapshotsDump(include ...string) []byte {
	d := make(map[string]any)
	for _, h := range m.hooks.GetHooks() {
		if len(include) == 0 || slices.Contains(include, h.GetName()) {
			d[h.GetName()] = h.GetHookController().SnapshotsDump()
		}
	}

	marshalled, _ := yaml.Marshal(d)
	return marshalled
}

// GetValuesChecksum returns a checksum of the current values.
// Used to detect if values changed after hook execution.
func (m *Module) GetValuesChecksum() string {
	return m.values.GetValuesChecksum()
}

// GetSettingsChecksum returns a checksum of the current config values.
// Used to detect if settings changed.
func (m *Module) GetSettingsChecksum() string {
	return m.values.GetSettingsChecksum()
}

// GetValues returns values for rendering.
//
// Global values are exposed both flat (.Values.replicas) and under the "global"
// key (.Values.global.replicas) so templates written for the old addon-operator
// layout keep working.
func (m *Module) GetValues() addonutils.Values {
	v := m.values.GetValues()
	return addonutils.MergeValues(
		v,
		addonutils.Values{addonutils.ModuleNameToValuesKey(m.name): v},
	)
}

// ValidateSettings validates settings against openAPI
func (m *Module) ValidateSettings(_ context.Context, settings addonutils.Values) (settingscheck.Result, error) {
	if err := m.values.ValidateSettings(settings); err != nil {
		return settingscheck.Result{}, err
	}

	// apply defaults from config values spec
	settings = m.values.ApplySettingsDefaults(settings)

	// no need to call the settings check if nothing changed
	if m.values.GetSettingsChecksum() == settings.Checksum() {
		return settingscheck.Result{Valid: true}, nil
	}

	return settingscheck.Result{
		Valid: true,
	}, nil
}

// ApplySettings apply settings values
func (m *Module) ApplySettings(settings addonutils.Values) error {
	return m.values.ApplySettings(settings)
}

// GetSettings returns the effective settings: user config merged with
// config-schema defaults. Same payload exposed to templates as .Module.Settings.
func (m *Module) GetSettings() addonutils.Values {
	return m.values.GetSettings()
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

	m.initialized.Store(true)
}

// HooksInitialized reports whether InitializeHooks has built the hook controllers.
func (m *Module) HooksInitialized() bool {
	return m.initialized.Load()
}

// GetHooksByBinding returns the global hooks for the binding as the ControllableHook
// view, so the shared Enable task can drive global like any package.
func (m *Module) GetHooksByBinding(binding shtypes.BindingType) []hooks.ControllableHook {
	return hooks.ToControllable(m.hooks.GetHooksByBinding(binding))
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

	m.running.Store(true)

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

	hookConfigValues := m.values.GetSettings()
	hookValues := m.GetValues()
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
		// filterEnabledFromValuesPatch strips the dynamic-enable signals from the
		// patch in place, so the apply below sees only real global values.
		enabled := filterEnabledFromValuesPatch(valuesPatch)
		if err = m.values.ApplyValuesPatchWithLegacyRoot(*valuesPatch); err != nil {
			return fmt.Errorf("apply hook values patch: %w", err)
		}

		m.enabledMu.Lock()
		maps.Copy(m.dynamicEnabled, enabled)
		m.enabledMu.Unlock()
	}

	return nil
}

// filterEnabledFromValuesPatch extracts dynamic module-enable signals from a
// global hook's values patch and removes them from the patch in place, returning
// a map of kebab-case module name to enabled state.
//
// Global hooks enable other modules by patching a virtual "/<moduleKey>Enabled"
// key (e.g. "/cniCiliumEnabled") in global values. These keys are not part of
// the global values schema, so they must not reach the values storage: every
// operation whose first path segment ends in "Enabled" is treated as an enable
// signal (mapped to true) — regardless of its Op or Value — then translated to
// its module name (cni-cilium) and pulled out of the patch. Every remaining
// operation is a real global value and stays in the patch for the caller to
// apply.
func filterEnabledFromValuesPatch(valuesPatch *addonutils.ValuesPatch) map[string]bool {
	enabled := make(map[string]bool)
	kept := valuesPatch.Operations[:0]

	for _, op := range valuesPatch.Operations {
		pathParts := strings.Split(op.Path, "/")
		if len(pathParts) < 2 || !strings.HasSuffix(pathParts[1], "Enabled") {
			kept = append(kept, op)
			continue
		}

		key := strings.TrimSuffix(pathParts[1], "Enabled")
		enabled[strcase.ToKebab(key)] = true
	}

	valuesPatch.Operations = kept

	return enabled
}

// SetEnabledModules inject enabledModules to the global values
// enabledModules are injected as a patch, to recalculate on every global values change
func (m *Module) SetEnabledModules(enabledModules []string) {
	if len(enabledModules) == 0 {
		return
	}

	// keep them sorted to prevent helm rollout on each restart
	sort.Strings(enabledModules)
	data, _ := json.Marshal(enabledModules)

	patch := addonutils.ValuesPatch{Operations: []*sdkutils.ValuesPatchOperation{
		{
			Op:    "add",
			Path:  "/enabledModules",
			Value: data,
		},
	}}

	if err := m.values.ApplyValuesPatch(patch); err != nil {
		m.logger.Error(fmt.Sprintf("failed to set enabled modules to global: %v", err.Error()))
	}
}

// SetCapabilities injects GVK values, discovered during executing ModuleEnsureCRDs tasks, into .global.discovery.apiVersions values
func (m *Module) SetCapabilities(apiVersions []string) {
	if len(apiVersions) == 0 {
		return
	}

	// keep apiVersions sorted to prevent helm rollout on each restart
	sort.Strings(apiVersions)
	data, _ := json.Marshal(apiVersions)

	// backward compatibility: set apiVersions to .global.discovery.apiVersions
	// TODO(ipaqsa): get rid of it further and add Capabilities field
	patch := addonutils.ValuesPatch{Operations: []*sdkutils.ValuesPatchOperation{
		{
			Op:    "add",
			Path:  "/discovery/apiVersions",
			Value: data,
		},
	}}

	if err := m.values.ApplyValuesPatch(patch); err != nil {
		m.logger.Error(fmt.Sprintf("failed to set enabled modules to global: %v", err.Error()))
	}
}

// IsEnabled answers a module's resolved enablement intent for the scheduler as a
// tri-state, folding the two external signals global tracks:
//   - non-nil true/false - an explicit ModuleConfig opinion; it is authoritative
//     and can both enable and disable;
//   - non-nil true       - absent a ModuleConfig opinion, a global hook
//     dynamically enabled the module (hooks are enable-only);
//   - nil                - neither signal has an opinion; resolution defers to
//     the bundle floor.
//
// moduleName is the kebab-case module name. Safe for concurrent use: the
// scheduler reads this while global hooks and the config controller write.
func (m *Module) IsEnabled(moduleName string) *bool {
	m.enabledMu.RLock()
	defer m.enabledMu.RUnlock()

	if enabled, ok := m.configEnabled[moduleName]; ok {
		return &enabled
	}

	if m.dynamicEnabled[moduleName] {
		on := true
		return &on
	}

	return nil
}

// SetConfigEnabled records the explicit ModuleConfig enabled intent for a module:
// a non-nil value sets the tri-state, nil clears any prior opinion. It reports
// whether the stored state changed, so the caller can decide whether to trigger
// a reschedule. It is the config-side counterpart to the dynamic enabled state
// set by global hooks; both feed IsEnabled.
func (m *Module) SetConfigEnabled(moduleName string, enabled *bool) bool {
	m.enabledMu.Lock()
	defer m.enabledMu.Unlock()

	prev, had := m.configEnabled[moduleName]

	if enabled == nil {
		if !had {
			return false
		}

		delete(m.configEnabled, moduleName)

		return true
	}

	if had && prev == *enabled {
		return false
	}

	m.configEnabled[moduleName] = *enabled

	return true
}
