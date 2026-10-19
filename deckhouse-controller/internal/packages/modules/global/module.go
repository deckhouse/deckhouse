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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	sdkutils "github.com/deckhouse/module-sdk/pkg/utils"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

type Module struct {
	name    string          // Package name
	path    string          // path to the package dir on fs
	version *semver.Version // version is the deckhouse image's: the global module ships in it

	hooks     *hooks.GlobalStorage  // Hook storage with indices
	values    *values.Storage       // Values storage with layering
	converter *conversion.Converter // Schema version converter for settings

	// initialized: the hook controllers are built; only the snapshots dump reads it. Guarded by
	// hooksMu.
	initialized bool
	// hooksMu guards the hook controllers and initialized against the debug snapshots dump:
	// InitializeHooks and DisableHooks write under it. Hook runs read the controllers without it:
	// the global hooks are disabled only by the lanes' stop (hooklane: a failed
	// synchronization, Global's exit), which waits for every run first, so a controller is never
	// cleared under a running hook.
	hooksMu sync.RWMutex
	// unlocked are the monitors whose events are unlocked, the only ones the snapshots dump reads;
	// guarded by hooksMu.
	unlocked hooks.Unlocked

	patcher           *objectpatch.ObjectPatcher
	scheduleManager   schedulemanager.ScheduleManager
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	metricStorage metricsstorage.Storage

	// dynamicMu guards dynamicEnabled: global hooks write it, the scheduler reads it through
	// IsEnabled. A leaf lock (onDynamicChange is called after it is released), so the scheduler
	// may take it under its own lock.
	dynamicMu sync.RWMutex
	// dynamicEnabled is the global hooks' signal by kebab-case module name; an absent key is no
	// signal, as addon-operator's DynamicallyEnabled extender keeps it.
	dynamicEnabled map[string]bool

	// onDynamicChange is Config.OnDynamicChange; nil when nobody listens.
	onDynamicChange func()

	// imagesDigests are the image digests of the enabled modules by module name, replaced whole by
	// SetModulesImagesDigests; module renders read them concurrently.
	imagesDigests atomic.Pointer[map[string]map[string]string]

	logger *log.Logger
}

// Config holds configuration for creating a new Module instance.
type Config struct {
	Path         string            // Path to package dir
	StaticValues addonutils.Values // Static values from values.yaml files
	// Version is the deckhouse image's version, the one the global Module's spec names; "v0.0.0" when
	// it does not parse.
	Version string

	ConfigSchema []byte // OpenAPI config schema (YAML)
	ValuesSchema []byte // OpenAPI values schema (YAML)

	Hooks []hooks.GlobalHook // Discovered hooks

	Conversions *conversion.Converter // Schema version converter

	MetricStorage metricsstorage.Storage

	Patcher           *objectpatch.ObjectPatcher
	ScheduleManager   schedulemanager.ScheduleManager
	KubeEventsManager kubeeventsmanager.KubeEventsManager

	// OnDynamicChange is called after a global hook changed a module's signal (set, flipped or
	// removed; see filterEnabledFromValuesPatch), from the goroutine that ran the hook, with no lock
	// of the module held. Why: the signal is stripped from the values patch, so the values checksum
	// does not change and nothing else tells the scheduler to read IsEnabled again; the package
	// engine wires it to its recompute. Must not block for long. Nil: nobody listens.
	OnDynamicChange func()
}

// NewModuleByConfig creates a new Module instance with the specified configuration.
// It initializes hook storage, adds all discovered hooks, and creates values storage.
//
// Returns error if hook initialization or values storage creation fails.
func NewModuleByConfig(cfg *Config, logger *log.Logger) (*Module, error) {
	m := new(Module)

	m.name = "global"
	m.dynamicEnabled = make(map[string]bool)

	m.path = cfg.Path
	m.converter = cfg.Conversions

	version, err := semver.NewVersion(cfg.Version)
	if err != nil {
		version = semver.MustParse("v0.0.0")
	}

	m.version = version
	m.patcher = cfg.Patcher
	m.metricStorage = cfg.MetricStorage
	m.scheduleManager = cfg.ScheduleManager
	m.kubeEventsManager = cfg.KubeEventsManager
	m.onDynamicChange = cfg.OnDynamicChange
	m.logger = logger

	m.hooks = hooks.NewGlobalStorage()
	if err := m.addHooks(cfg.Hooks...); err != nil {
		return nil, fmt.Errorf("add hooks: %v", err)
	}

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

// GetVersion returns the deckhouse image's version (Config.Version).
func (m *Module) GetVersion() *semver.Version {
	return m.version
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

// GetHookSnapshotsDump returns the snapshots of the hooks' unlocked monitors (hooks.SnapshotsDump).
func (m *Module) GetHookSnapshotsDump() map[string]any {
	m.hooksMu.RLock()
	defer m.hooksMu.RUnlock()

	snapshots := make(map[string]any)
	if !m.initialized {
		return snapshots
	}

	for _, hook := range m.hooks.GetHooks() {
		snapshots[hook.GetName()] = hooks.SnapshotsDump(hook.GetHookConfig().OnKubernetesEvents, m.kubeEventsManager, m.unlocked)
	}

	return snapshots
}

// GetValuesChecksum returns a checksum of the current values.
// Used to detect if values changed after hook execution.
func (m *Module) GetValuesChecksum() string {
	return m.values.GetValuesChecksum()
}

// GetValues returns values for rendering.
//
// Global values are exposed both flat (.Values.replicas) and under the "global"
// key (.Values.global.replicas) so templates written for the old addon-operator
// layout keep working.
//
// The enabled modules' image digests are merged into modulesImages.digests here, over the ones the
// global hook read from the image, and not kept in the values: GetValuesChecksum, Global's Output,
// does not hash them, so a module update reruns only that module's subscribers.
func (m *Module) GetValues() addonutils.Values {
	v := addonutils.MergeValues(m.values.GetValues(), m.imagesDigestsValues())
	return addonutils.MergeValues(
		v,
		addonutils.Values{addonutils.ModuleNameToValuesKey(m.name): v},
	)
}

// SetModulesImagesDigests replaces the image digests of the enabled modules, by module name
// (kebab-case): charts on the legacy helm_lib read another module's images from
// modulesImages.digests by its camelCase name.
func (m *Module) SetModulesImagesDigests(digests map[string]map[string]string) {
	m.imagesDigests.Store(&digests)
}

// imagesDigestsValues is modulesImages.digests built from the published digests; nil without any.
func (m *Module) imagesDigestsValues() addonutils.Values {
	published := m.imagesDigests.Load()
	if published == nil || len(*published) == 0 {
		return nil
	}

	digests := make(map[string]any, len(*published))
	for name, images := range *published {
		byImage := make(map[string]any, len(images))
		for image, digest := range images {
			byImage[image] = digest
		}

		digests[addonutils.ModuleNameToValuesKey(name)] = byImage
	}

	return addonutils.Values{"modulesImages": map[string]any{"digests": digests}}
}

// ValidateSettings converts settings to the latest schema version (if a converter
// is available), then validates against OpenAPI schema.
func (m *Module) ValidateSettings(_ context.Context, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error) {
	settings, err := m.convertSettings(settingsVersion, settings)
	if err != nil {
		return settingscheck.Result{Message: err.Error()}, nil
	}

	if err = m.values.ValidateSettings(settings); err != nil {
		return settingscheck.Result{Message: err.Error()}, nil
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

// ApplySettings converts settings to the latest schema version (if a converter
// is available), then applies them to the values storage.
func (m *Module) ApplySettings(settingsVersion int, settings addonutils.Values) error {
	settings, err := m.convertSettings(settingsVersion, settings)
	if err != nil {
		return err
	}

	return m.values.ApplySettings(settings)
}

// convertSettings converts settings to the latest schema version, as it does for a
// module: global carries conversions of its own (x-config-version 2), so a
// ModuleConfig that omits spec.version must go through the chain all the same.
// A zero settingsVersion is therefore read as version 1 - the chain is what
// materializes fields the latest schema requires, so skipping it leaves an
// unversioned config permanently invalid.
func (m *Module) convertSettings(settingsVersion int, settings addonutils.Values) (addonutils.Values, error) {
	if m.converter == nil {
		return settings, nil
	}

	if settingsVersion == 0 {
		settingsVersion = 1
	}

	// the converter skips a nil document, and an absent spec.settings is exactly
	// the case the chain has to fill in
	if settings == nil {
		settings = addonutils.Values{}
	}

	_, converted, err := m.converter.ConvertToLatest(settingsVersion, settings)
	if err != nil {
		return nil, fmt.Errorf("convert settings: %w", err)
	}

	return converted, nil
}

// LatestSettingsVersion returns the latest settings-schema version, the one effective settings
// are in; 0 without a converter.
func (m *Module) LatestSettingsVersion() int {
	if m.converter == nil {
		return 0
	}

	return m.converter.LatestVersion()
}

// GetSettings returns the effective settings: user config merged with
// config-schema defaults. Same payload exposed to templates as .Module.Settings.
func (m *Module) GetSettings() addonutils.Values {
	return m.values.GetSettings()
}

// InitializeHooks initializes hook controllers and bind them to Kubernetes events and schedules
func (m *Module) InitializeHooks() {
	m.hooksMu.Lock()
	defer m.hooksMu.Unlock()

	for _, hook := range m.hooks.GetHooks() {
		hookCtrl := hookcontroller.NewHookController()
		hookCtrl.InitKubernetesBindings(hook.GetHookConfig().OnKubernetesEvents, m.kubeEventsManager, m.logger)
		hookCtrl.InitScheduleBindings(hook.GetHookConfig().Schedules, m.scheduleManager)

		hook.WithHookController(hookCtrl)
		hook.WithTmpDir(os.TempDir())
	}

	m.initialized = true
}

// DisableHooks stops the hook bindings and drops the controllers, so the next InitializeHooks starts
// fresh. Called by the lanes' stop (hooklane): after a synchronization that failed
// midway and when Global exits.
func (m *Module) DisableHooks() {
	m.hooksMu.Lock()
	defer m.hooksMu.Unlock()

	m.initialized = false
	m.unlocked = nil

	for _, hook := range m.hooks.GetHooksByBinding(shtypes.Schedule) {
		if ctrl := hook.GetHookController(); ctrl != nil {
			ctrl.DisableScheduleBindings()
		}
	}

	for _, hook := range m.hooks.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		if ctrl := hook.GetHookController(); ctrl != nil {
			ctrl.DisableKubernetesBindings()
		}
	}

	for _, hook := range m.hooks.GetHooks() {
		hook.WithHookController(nil)
	}
}

// GetHooksQueues returns the queues of the global hooks' schedule and Kubernetes bindings, sorted
// and without duplicates.
func (m *Module) GetHooksQueues() []string {
	var res []string //nolint:prealloc

	for _, hook := range m.hooks.GetHooksByBinding(shtypes.Schedule) {
		for _, binding := range hook.GetHookConfig().Schedules {
			res = append(res, binding.Queue)
		}
	}

	for _, hook := range m.hooks.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		for _, binding := range hook.GetHookConfig().OnKubernetesEvents {
			res = append(res, binding.Queue)
		}
	}

	slices.Sort(res)

	return slices.Compact(res)
}

// GetHooksByBinding returns the global hooks for the binding as the ControllableHook
// view, so the hook lanes drive global like any package.
func (m *Module) GetHooksByBinding(binding shtypes.BindingType) []hooks.ControllableHook {
	return hooks.ToControllable(m.hooks.GetHooksByBinding(binding))
}

// UnlockKubernetesMonitors is called after a binding's synchronization ran, so its events flow.
func (m *Module) UnlockKubernetesMonitors(hook string, monitors ...string) {
	h := m.hooks.GetHookByName(hook)
	if h == nil {
		return
	}

	for _, monitorID := range monitors {
		h.GetHookController().UnlockKubernetesEventsFor(monitorID)
	}

	// Recorded after the unlock: a dump in between skips the monitor rather than read it locked.
	m.hooksMu.Lock()
	defer m.hooksMu.Unlock()

	if m.unlocked == nil {
		m.unlocked = make(hooks.Unlocked, len(monitors))
	}

	for _, monitorID := range monitors {
		m.unlocked[monitorID] = struct{}{}
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

	hookConfigValues := m.values.GetSettings()
	hookValues := m.GetValues()
	hookVersion := h.GetConfigVersion()

	metricLabels := map[string]string{
		pkg.MetricKeyHook: h.GetName(),
		pkg.LogKeyModule:  m.GetName(),
	}

	hookResult, err := h.Execute(ctx, hookVersion, bctx, m.GetName(), hookConfigValues, hookValues, make(map[string]string))

	if hookResult != nil && hookResult.Usage != nil {
		m.metricStorage.HistogramObserve(metrics.ModuleHookRunSysCPUSeconds, hookResult.Usage.Sys.Seconds(), metricLabels, nil)
		m.metricStorage.HistogramObserve(metrics.ModuleHookRunUserCPUSeconds, hookResult.Usage.User.Seconds(), metricLabels, nil)
		m.metricStorage.GaugeSet(metrics.ModuleHookRunMaxRSSBytes, float64(hookResult.Usage.MaxRss)*1024, metricLabels)
	}

	if hookResult != nil && len(hookResult.Metrics) > 0 {
		metricsErr := m.metricStorage.ApplyBatchOperations(hookResult.Metrics, metricLabels)
		if metricsErr != nil {
			return metricsErr
		}
	}

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
		signals, filterErr := filterEnabledFromValuesPatch(valuesPatch)
		if filterErr != nil {
			return fmt.Errorf("filter enabled signals: %w", filterErr)
		}

		if err = m.values.ApplyValuesPatchWithLegacyRoot(*valuesPatch); err != nil {
			return fmt.Errorf("apply hook values patch: %w", err)
		}

		// Only a real change is reported: hooks repeat their signals on every run, and each
		// report recomputes the whole graph.
		if m.recordDynamicEnabled(signals) && m.onDynamicChange != nil {
			m.onDynamicChange()
		}
	}

	return nil
}

// filterEnabledFromValuesPatch pulls the dynamic enable signals out of a global hook's values patch,
// in place, and returns them by kebab-case module name: *true or *false for a set, nil for a remove.
//
// Global hooks signal a module through a virtual "/<moduleKey>Enabled" key (e.g. "/cniCiliumEnabled"),
// as addon-operator's DynamicallyEnabled extender reads it. The key is no global value: it must not
// reach the values storage nor its checksum. Every other operation stays in the patch for the caller
// to apply. A later operation on the same key wins. A set with a value other than a JSON boolean is
// an error, as in addon-operator.
func filterEnabledFromValuesPatch(valuesPatch *addonutils.ValuesPatch) (map[string]*bool, error) {
	signals := make(map[string]*bool)
	kept := valuesPatch.Operations[:0]

	for _, op := range valuesPatch.Operations {
		pathParts := strings.Split(op.Path, "/")
		if len(pathParts) < 2 || !strings.HasSuffix(pathParts[1], "Enabled") {
			kept = append(kept, op)
			continue
		}

		module := strcase.ToKebab(strings.TrimSuffix(pathParts[1], "Enabled"))
		if op.Op == "remove" {
			signals[module] = nil
			continue
		}

		enabled, err := enabledValue(op.Value)
		if err != nil {
			return nil, fmt.Errorf("signal of module '%s': %w", module, err)
		}

		signals[module] = &enabled
	}

	valuesPatch.Operations = kept

	return signals, nil
}

// enabledValue reads a signal's value as addon-operator's ModuleEnabledValue reads a patch value.
func enabledValue(raw json.RawMessage) (bool, error) {
	switch strings.ToLower(string(bytes.TrimSpace(raw))) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	return false, fmt.Errorf("unsupported enabled value '%s'", raw)
}

// recordDynamicEnabled applies the signals of one hook run to dynamicEnabled, a nil one dropping its
// key, and reports whether any module's signal changed.
func (m *Module) recordDynamicEnabled(signals map[string]*bool) bool {
	m.dynamicMu.Lock()
	defer m.dynamicMu.Unlock()

	changed := false

	for name, enabled := range signals {
		old, had := m.dynamicEnabled[name]

		switch {
		case enabled == nil && had:
			delete(m.dynamicEnabled, name)
			changed = true
		case enabled != nil && (!had || old != *enabled):
			m.dynamicEnabled[name] = *enabled
			changed = true
		}
	}

	return changed
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

// SetCapabilities injects the GVKs Global's ensureCRDs step installed
// into the .capabilities global value, exposed to charts as .Platform.capabilities
func (m *Module) SetCapabilities(apiVersions []string) {
	if len(apiVersions) == 0 {
		return
	}

	// keep apiVersions sorted to prevent helm rollout on each restart
	sort.Strings(apiVersions)
	data, _ := json.Marshal(apiVersions)

	patch := addonutils.ValuesPatch{Operations: []*sdkutils.ValuesPatchOperation{
		{
			Op:    "add",
			Path:  "/capabilities",
			Value: data,
		},
		// backward compatibility: set apiVersions to .global.discovery.apiVersions
		// TODO(ipaqsa): get rid of it further
		{
			Op:    "add",
			Path:  "/discovery/apiVersions",
			Value: data,
		},
	}}

	if err := m.values.ApplyValuesPatch(patch); err != nil {
		m.logger.Error(fmt.Sprintf("failed to set capabilities to global: %v", err.Error()))
	}
}

// IsEnabled is the global hooks' signal of a module (kebab-case name) for the scheduler: the value a
// hook set last, or nil when none is set. Safe for concurrent use.
func (m *Module) IsEnabled(moduleName string) *bool {
	m.dynamicMu.RLock()
	defer m.dynamicMu.RUnlock()

	enabled, ok := m.dynamicEnabled[moduleName]
	if !ok {
		return nil
	}

	return &enabled
}
