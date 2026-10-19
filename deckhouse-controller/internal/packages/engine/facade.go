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

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"k8s.io/client-go/util/workqueue"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/core"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcerequests"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// The facade: the API the controllers, the boot code and the status side call, in controller
// terms, translated to the core. It holds no lifecycle policy. Calling order:
//
//	Build → Start → Cleanup → Boot → (Resume, without EnableModuleV2) → Apply*/Remove*/... → Stop

const (
	defaultNamespace = "default"

	// applicationMaintenanceMetric is 1 while an application is in maintenance; the
	// ApplicationIsInMaintenanceMode alert fires on it.
	applicationMaintenanceMetric = "deckhouse_application_maintenance"
)

// Application is an Application resource as the controller hands it to the engine.
type Application struct {
	Namespace string
	Name      string
	// PackageName is spec.packageName: the package in the repository.
	PackageName string
	// Version is spec.packageVersion: the version that should become Current.
	Version    string
	Repository registry.Remote
	// Settings and SettingsVersion are spec.settings and the settings-schema version they are
	// written for (spec.version).
	Settings        addonutils.Values
	SettingsVersion int
	Maintenance     string
	// ResourceRequests are spec.resourceRequests.
	ResourceRequests []resourcerequests.Request
	// UpdateSettings are the update settings the application follows, already merged by the
	// controller (spec.update, global.applications.update, the deckhouse ModuleConfig).
	UpdateSettings releaseupdater.Settings
	// ApplyNow skips the update windows (model.Policy.Force): the Application controller sets it
	// while the apply-now annotation names spec.packageVersion. It never skips the requirements,
	// the dependents or the settings check.
	ApplyNow bool
	// Requirements of the ApplicationPackageVersion of Version (application.requirementsOf).
	Requirements schedule.Constraints
}

// Module is a Module resource as the controller hands it to the engine.
type Module struct {
	Name string
	// Version is spec.packageVersion; ignored for an embedded module, whose version is the one the
	// deckhouse image ships.
	Version    string
	Repository registry.Remote
	// Embedded is set for a module the deckhouse image ships.
	Embedded        bool
	Settings        addonutils.Values
	SettingsVersion int
	Maintenance     string
	// Enabled is Module.spec.enabled, the module's enable intent; nil means none.
	Enabled *bool
	// UpdateSettings and ApplyNow are Application's for a module. No controller sets them yet, so
	// a module follows no update window: modules get them with the module update work.
	UpdateSettings releaseupdater.Settings
	ApplyNow       bool
	// Requirements of the ModulePackageVersion of Version.
	Requirements schedule.Constraints
}

// BootApplication is what Boot needs of an installed Application (see model.BootPackage).
type BootApplication struct {
	Namespace      string
	Name           string
	PackageName    string
	Repository     registry.Remote // Repository is the one in spec.
	CurrentVersion string          // CurrentVersion is status.currentVersion; empty restores nothing.
	// LastApplied is the raw status.lastAppliedConfiguration (checkpointSettings).
	LastApplied      []byte
	Maintenance      string
	ResourceRequests []resourcerequests.Request
	// Removing: the Application is being deleted and has a CurrentVersion; Boot restores it to
	// uninstall it (model.BootPackage.Removing).
	Removing bool
}

// BootModule is what Boot needs of an installed Module; global is not passed.
type BootModule struct {
	Name       string
	Repository registry.Remote
	Embedded   bool
	// CurrentVersion is status.currentVersion; for an embedded module only whether it is set
	// matters (it is restored with the version the image ships).
	CurrentVersion string
	LastApplied    []byte // LastApplied is the raw status.lastAppliedConfiguration.
	Maintenance    string
	// Enabled must be passed: an unknown intent falls to the module floor (Disable).
	Enabled *bool
	// Removing: the Module is being deleted and has a CurrentVersion (BootApplication.Removing).
	Removing bool
}

// Cleanup brings the disks to a consistent state before Boot: each removes the images and mounts
// its list does not name. It is the only place files are purged. The lists name every version the
// cluster claims, packages being deleted included unless their release is deleted by name: Boot
// restores them to uninstall them. Releases are left alone: a package's release goes with its
// uninstall. An entry without a version is dropped. Without EnableModuleV2 the module disk is
// addon-operator's and modules is ignored; with it the module list is acted on whatever it holds,
// so the caller must not pass one it failed to read. Every failure is returned; none stops the
// other parts.
func (e *Engine) Cleanup(ctx context.Context, apps, modules []disk.PreservePackage) error {
	var errs error

	if err := e.appsDisk.Cleanup(ctx, withVersion(apps)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("cleanup application packages: %w", err))
	}

	if !e.cfg.EnableModuleV2 {
		return errs
	}

	if err := e.modulesDisk.Cleanup(ctx, withVersion(modules)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("cleanup module packages: %w", err))
	}

	return errs
}

// Boot is core.Engine.Boot over the controllers' view of the installed packages. global is the raw
// lastAppliedConfiguration of the global Module; without EnableModuleV2 it is ignored and modules
// must be empty.
func (e *Engine) Boot(ctx context.Context, global []byte, bootApps []BootApplication, bootModules []BootModule) error {
	var bootGlobal *core.BootGlobal

	if e.cfg.EnableModuleV2 {
		bootGlobal = &core.BootGlobal{Module: e.global, Settings: e.checkpointSettings(schedule.GlobalNode, global)}
	} else if len(bootModules) > 0 {
		return errors.New("modules run in addon-operator without EnableModuleV2: boot them there")
	}

	packages := make([]model.BootPackage, 0, len(bootApps)+len(bootModules))

	for _, a := range bootApps {
		namespace := namespaceOrDefault(a.Namespace)

		packages = append(packages, model.BootPackage{
			Name:             apps.BuildName(namespace, a.Name),
			Kind:             model.KindApplication,
			Version:          a.CurrentVersion,
			Settings:         e.checkpointSettings(apps.BuildName(namespace, a.Name), a.LastApplied),
			Maintenance:      nelm.MaintenanceState(a.Maintenance),
			ResourceRequests: a.ResourceRequests,
			Source:           model.Source{Repository: a.Repository, Package: a.PackageName},
			Removing:         a.Removing,
		})
	}

	for _, m := range bootModules {
		version := m.CurrentVersion
		if m.Embedded && version != "" {
			// After a deckhouse upgrade the image ships only its own version; the settings schemas
			// of embedded modules stay compatible, so the checkpoint stays valid.
			version = app.EmbeddedPackageVersion()
		}

		packages = append(packages, model.BootPackage{
			Name:        m.Name,
			Kind:        model.KindModule,
			Version:     version,
			Settings:    e.checkpointSettings(m.Name, m.LastApplied),
			Maintenance: nelm.MaintenanceState(m.Maintenance),
			Intent:      m.Enabled,
			Source:      model.Source{Repository: m.Repository, Package: m.Name},
			Embedded:    m.Embedded,
			Removing:    m.Removing,
		})
	}

	err := e.core.Boot(ctx, bootGlobal, packages)
	if errors.Is(err, core.ErrGlobalNotSynced) {
		if cause := e.globalSyncCause(); cause != "" {
			return fmt.Errorf("%w: %s", err, cause)
		}
	}

	return err
}

// globalSyncCause names what holds Global's synchronization, for Boot's error: the controllers that
// would project it are not running yet. A synchronization item that keeps failing is in the
// observations; bindings that fail to start are not (each failed attempt resets them) and are in
// Global's Report instead. Empty when neither says.
func (e *Engine) globalSyncCause() string {
	if f := e.observed.Get(schedule.GlobalNode).HookFailure; f != nil {
		return fmt.Sprintf("hook %s: %s", f.Hook, f.Message)
	}

	if r, ok := e.core.Report(schedule.GlobalNode); ok && r.Current.HooksProcessed.Status == model.ConditionFalse {
		return fmt.Sprintf("%s: %s", r.Current.HooksProcessed.Reason, r.Current.HooksProcessed.Message)
	}

	return ""
}

// checkpointSettings is the settings checkpoint of raw status.lastAppliedConfiguration: nil when
// absent (empty raw), so Current waits for the first Apply; otherwise the effective values, already
// in the latest schema. An unreadable one is treated as absent: refusing to boot over it would
// crash-loop deckhouse until someone edits the status.
func (e *Engine) checkpointSettings(name string, raw []byte) *model.Settings {
	if len(raw) == 0 {
		return nil
	}

	values := make(addonutils.Values)
	if err := json.Unmarshal(raw, &values); err != nil {
		e.logger.Warn("ignore unreadable lastAppliedConfiguration, wait for the first apply",
			slog.String("name", name), log.Err(err))

		return nil
	}

	// JSON null decodes to a nil map: the package ran without user settings.
	if values == nil {
		values = make(addonutils.Values)
	}

	// SchemaVersion is the restored object's latest; the core sets it once the object is loaded.
	return &model.Settings{Values: values}
}

// Resume is core.Engine.Resume.
func (e *Engine) Resume() {
	e.core.Resume()
}

// Recheck is core.Engine.Recheck.
func (e *Engine) Recheck() {
	e.core.Recheck()
}

// ApplyApplication passes the desired state of an application. Call it on every reconcile of an
// Application that is not being deleted. Non-blocking.
func (e *Engine) ApplyApplication(a Application) {
	namespace := namespaceOrDefault(a.Namespace)
	name := apps.BuildName(namespace, a.Name)
	maintenance := nelm.MaintenanceState(a.Maintenance)

	e.core.Apply(model.Desired{
		Name:    name,
		Kind:    model.KindApplication,
		Version: a.Version,
		Configuration: model.Configuration{
			Settings:         model.Settings{Values: valuesOrEmpty(a.Settings), SchemaVersion: a.SettingsVersion},
			Maintenance:      maintenance,
			ResourceRequests: a.ResourceRequests,
		},
		Policy:       policyOf(a.UpdateSettings, a.ApplyNow),
		Requirements: a.Requirements,
		Source:       model.Source{Repository: a.Repository, Package: a.PackageName},
	})
}

// ApplyModule passes the desired state of a module, its enable intent included. Ignored without
// EnableModuleV2. Non-blocking.
func (e *Engine) ApplyModule(m Module) {
	if !e.cfg.EnableModuleV2 {
		e.logger.Warn("ignore module, modules run in addon-operator", slog.String("name", m.Name))

		return
	}

	version := m.Version
	if m.Embedded {
		version = app.EmbeddedPackageVersion()
	}

	e.core.Apply(model.Desired{
		Name:    m.Name,
		Kind:    model.KindModule,
		Version: version,
		Configuration: model.Configuration{
			Settings:    model.Settings{Values: valuesOrEmpty(m.Settings), SchemaVersion: m.SettingsVersion},
			Maintenance: nelm.MaintenanceState(m.Maintenance),
		},
		Policy:       policyOf(m.UpdateSettings, m.ApplyNow),
		Requirements: m.Requirements,
		Source:       model.Source{Repository: m.Repository, Package: m.Name},
		Embedded:     m.Embedded,
		Intent:       m.Enabled,
	})
}

// ApplyGlobal passes the global settings to Global; dropped without EnableModuleV2. Non-blocking.
func (e *Engine) ApplyGlobal(settings addonutils.Values, settingsVersion int) {
	e.core.Apply(model.Desired{
		Name: schedule.GlobalNode,
		Kind: model.KindGlobal,
		Configuration: model.Configuration{
			Settings: model.Settings{Values: valuesOrEmpty(settings), SchemaVersion: settingsVersion},
		},
	})
}

// RemoveApplication uninstalls an application and reports whether the removal has finished, so
// the controller can release the finalizer; the status queue is notified when it does.
// Non-blocking and idempotent.
func (e *Engine) RemoveApplication(namespace, name string) bool {
	fullName := apps.BuildName(namespaceOrDefault(namespace), name)

	return e.remove(fullName, model.KindApplication)
}

// RemoveModule is RemoveApplication for a module. It reports true at once for global and without
// EnableModuleV2.
func (e *Engine) RemoveModule(name string) bool {
	if !e.cfg.EnableModuleV2 || name == schedule.GlobalNode {
		return true
	}

	return e.remove(name, model.KindModule)
}

// remove forgets name's observations once its removal is reported done: here, because the core
// never touches them. A late health transition may record name again, a small entry a package of
// the same name overwrites.
func (e *Engine) remove(name string, kind model.Kind) bool {
	e.core.Remove(name, kind)

	report, ok := e.core.Report(name)
	if !ok || !report.Removed {
		return false
	}

	e.observed.Forget(name)
	e.sink.forget(name)

	return true
}

// Report is core.Engine.Report: the lifecycle half of name's status.
func (e *Engine) Report(name string) (model.Report, bool) {
	return e.core.Report(name)
}

// Observed returns the observed half of name's status; the projection builds the status from
// Report and Observed.
func (e *Engine) Observed(name string) observed.Observation {
	return e.observed.Get(name)
}

// GetAppStatusQueue returns the queue of application names whose status may have changed.
func (e *Engine) GetAppStatusQueue() workqueue.TypedRateLimitingInterface[string] {
	return e.sink.apps
}

// GetModuleStatusQueue is GetAppStatusQueue for modules and global.
func (e *Engine) GetModuleStatusQueue() workqueue.TypedRateLimitingInterface[string] {
	return e.sink.modules
}

// GetDeckhouseSettingsCh carries the latest effective settings of the deckhouse module
// (statusSink.publishSettings). Only with EnableModuleV2.
func (e *Engine) GetDeckhouseSettingsCh() <-chan addonutils.Values {
	return e.sink.deckhouseSettings
}

// GetGlobalSettingsCh carries the latest effective global settings.
func (e *Engine) GetGlobalSettingsCh() <-chan addonutils.Values {
	return e.sink.globalSettings
}

// Ready is core.Engine.Ready.
func (e *Engine) Ready() bool {
	return e.core.Ready()
}

// policyOf translates the update settings: Manual updates at once; any other mode, AutoPatch
// included, is Auto and waits for the windows for every version the controller asks for, unless
// applyNow (the apply-now annotation naming that version) forces it.
func policyOf(settings releaseupdater.Settings, applyNow bool) model.Policy {
	policy := model.Policy{Mode: model.UpdateModeAuto, Force: applyNow}
	if settings.InManualMode() {
		policy.Mode = model.UpdateModeManual
	}

	// A nil Windows is "any time"; an empty slice inside the interface would not be.
	if len(settings.Windows) > 0 {
		policy.Windows = settings.Windows
	}

	return policy
}

// publishMaintenance keeps the maintenance gauge of every application the sink was notified of in
// line with its Report: set while the accepted configuration is in maintenance and the application
// is enabled, cleared otherwise. From the Report, because only it carries the decision; off the
// Notify path (runSink), because Notify must not call into the engine.
func (e *Engine) publishMaintenance() {
	for _, name := range e.sink.takeMaintenance() {
		state := nelm.Managed
		if report, ok := e.core.Report(name); ok && !report.Removed && report.Current.Decision != nil &&
			report.Current.Decision.Enabled {
			state = report.Current.Maintenance
		}

		e.setMaintenanceMetric(name, state)
	}
}

func (e *Engine) setMaintenanceMetric(name string, state nelm.MaintenanceState) {
	if state != nelm.NoResourceReconciliation {
		// Expire only this feature's gauge in the group, not every collector's.
		e.metrics.Grouped().ExpireGroupMetricByName(name, applicationMaintenanceMetric)
		return
	}

	// name is "<namespace>.<instance>"; a namespace never contains a dot.
	namespace, instance, _ := strings.Cut(name, ".")
	e.metrics.Grouped().GaugeSet(name, applicationMaintenanceMetric, 1, map[string]string{
		"namespace": namespace,
		"name":      instance,
		"state":     string(state),
	})
}

func namespaceOrDefault(namespace string) string {
	if namespace == "" {
		return defaultNamespace
	}

	return namespace
}

// valuesOrEmpty returns empty values for nil, so absent and empty settings are one configuration.
func valuesOrEmpty(values addonutils.Values) addonutils.Values {
	if values == nil {
		return make(addonutils.Values)
	}

	return values
}

// withVersion drops the entries without a version (nothing activated or asked for); the disks keep
// sets, so duplicates stay.
func withVersion(packages []disk.PreservePackage) []disk.PreservePackage {
	return slices.DeleteFunc(slices.Clone(packages), func(p disk.PreservePackage) bool { return p.Version == "" })
}
