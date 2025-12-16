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

package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"
	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	managerTracer = "package-manager"
)

// Manager manages the lifecycle of application packages.
type Manager struct {
	mu   sync.Mutex                   // Protects apps map
	apps map[string]*apps.Application // Loaded applications by name

	onValuesChanged func(name string)

	loader            *loader.ApplicationLoader // Loads packages from filesystem
	nelm              *nelm.Service             // nelm service to install/uninstall releases
	scheduler         *schedule.Scheduler
	kubeObjectPatcher *objectpatch.ObjectPatcher
	scheduleManager   schedulemanager.ScheduleManager
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	logger *log.Logger
}

type Config struct {
	OnValuesChanged func(name string)

	NelmService       *nelm.Service
	Scheduler         *schedule.Scheduler
	KubeObjectPatcher *objectpatch.ObjectPatcher
	ScheduleManager   schedulemanager.ScheduleManager
	KubeEventsManager kubeeventsmanager.KubeEventsManager
}

// New creates a new package manager with the specified apps directory.
func New(conf Config, logger *log.Logger) *Manager {
	appsPath := filepath.Join(d8env.GetDownloadedModulesDir(), "apps")
	return &Manager{
		apps: make(map[string]*apps.Application),

		onValuesChanged:   conf.OnValuesChanged,
		loader:            loader.NewApplicationLoader(appsPath, logger),
		nelm:              conf.NelmService,
		scheduler:         conf.Scheduler,
		kubeEventsManager: conf.KubeEventsManager,
		kubeObjectPatcher: conf.KubeObjectPatcher,
		scheduleManager:   conf.ScheduleManager,

		logger: logger.Named(managerTracer),
	}
}

// LoadPackage loads a package from filesystem and stores it in the manager.
// It discovers hooks, parses OpenAPI schemas, and initializes values storage.
// It returns the loaded version
func (m *Manager) LoadPackage(ctx context.Context, registry registry.Registry, namespace, name string) (string, error) {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "LoadPackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("registry", registry.Name))

	app, err := m.loader.Load(ctx, registry, name)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	m.mu.Lock()
	m.apps[name] = app
	m.scheduler.Add(app)
	m.mu.Unlock()

	return app.GetVersion(), nil
}

// ValidateSettings validates settings against openAPI and setting check
func (m *Manager) ValidateSettings(ctx context.Context, name string, settings addonutils.Values) (*settingscheck.Result, error) {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "ValidateSettings")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[name]
	if !ok {
		return &settingscheck.Result{Allow: true}, nil
	}

	return app.ValidateSettings(ctx, settings)
}

// ApplySettings validates and applies settings to application
func (m *Manager) ApplySettings(ctx context.Context, name string, settings addonutils.Values) error {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "ApplySettings")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	app := m.apps[name]
	if app == nil {
		return nil
	}

	m.logger.Debug("apply settings", slog.String("name", name))

	res, err := app.ValidateSettings(ctx, settings)
	if err != nil {
		return newApplySettingsErr(err)
	}

	if !res.Allow {
		return newApplySettingsErr(errors.New(res.Message))
	}

	if err = app.ApplySettings(settings); err != nil {
		return newApplySettingsErr(err)
	}

	return nil
}

// StartupPackage runs OnStartup hooks for a package.
// This must be called after InitializeHooks and before RunPackage.
func (m *Manager) StartupPackage(ctx context.Context, name string) error {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "StartupPackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	m.mu.Lock()
	app := m.apps[name]
	m.mu.Unlock()
	if app == nil {
		// package can be disabled and removed before
		return nil
	}

	m.logger.Debug("run on startup hooks", slog.String("name", name))

	if err := app.RunHooksByBinding(ctx, shtypes.OnStartup, m); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newStartupHookErr(err)
	}

	return nil
}

// RunPackage executes the full package run cycle: BeforeHelm → Install/Upgrade → AfterHelm.
//
// Process:
//  1. Pause Helm resource monitoring
//  2. Run BeforeHelm hooks (can modify values or prepare resources)
//  3. Install or upgrade Helm release
//  4. Run AfterHelm hooks
//  5. If values changed during AfterHelm, trigger Helm upgrade
//  6. Resume Helm resource monitoring
func (m *Manager) RunPackage(ctx context.Context, name string) error {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "RunPackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	m.mu.Lock()
	app := m.apps[name]
	m.mu.Unlock()
	if app == nil {
		// package can be disabled and removed before
		return nil
	}

	// monitor may not be created by this time
	if m.nelm.HasMonitor(name) {
		m.logger.Debug("pause helm monitor", slog.String("name", name))
		// Hooks can delete release resources, so pause resources monitor before run hooks.
		m.nelm.PauseMonitor(name)
		defer m.nelm.ResumeMonitor(name)
	}

	m.logger.Debug("run before helm hooks", slog.String("name", name))

	if err := app.RunHooksByBinding(ctx, addontypes.BeforeHelm, m); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newBeforeHelmHookErr(err)
	}

	m.logger.Debug("run nelm upgrade", slog.String("name", name))

	if err := m.nelm.Upgrade(ctx, app); err != nil && !errors.Is(err, nelm.ErrPackageNotHelm) {
		span.SetStatus(codes.Error, err.Error())
		return newHelmUpgradeErr(err)
	}

	m.logger.Debug("run after helm hooks", slog.String("name", name))

	// Check if AfterHelm hooks modified values (would require nelm upgrade)
	oldChecksum := app.GetValuesChecksum()
	if err := app.RunHooksByBinding(ctx, addontypes.AfterHelm, m); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newAfterHelmHookErr(err)
	}

	if oldChecksum != app.GetValuesChecksum() {
		if err := m.nelm.Upgrade(ctx, app); err != nil && !errors.Is(err, nelm.ErrPackageNotHelm) {
			span.SetStatus(codes.Error, err.Error())
			return newHelmUpgradeErr(err)
		}
	}

	return nil
}

// DisablePackage stops monitoring, uninstalls helm release and disables all hooks for a package.
//
// Process:
//  1. Stop Helm resource monitoring
//  2. Uninstall Helm release
//  3. Run AfterDeleteHelm hooks
//  4. Disable all schedule hooks
//  5. Stop all Kubernetes event monitors
//  6. Remove package from manager store
func (m *Manager) DisablePackage(ctx context.Context, name string, keep bool) error {
	_, span := otel.Tracer(managerTracer).Start(ctx, "DisablePackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	m.logger.Debug("disable package", slog.String("name", name))

	m.mu.Lock()
	defer m.mu.Unlock()

	app := m.apps[name]
	if app == nil {
		return nil
	}

	// app should not get absent events
	m.nelm.RemoveMonitor(name)

	if !keep {
		m.logger.Debug("delete nelm release", slog.String("name", name))
		// Delete package release
		if err := m.nelm.Delete(ctx, app); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return err
		}

		m.logger.Debug("run after delete helm hooks", slog.String("name", name))

		// Run after delete helm hooks
		if err := app.RunHooksByBinding(ctx, addontypes.AfterDeleteHelm, m); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("run after delete helm hooks: %w", err)
		}

		delete(m.apps, name)
	}

	// Disable all schedule-based hooks
	schHooks := app.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range schHooks {
		m.logger.Debug("disable hook", slog.String("name", name), slog.String("hook", hook.GetName()))
		if hook.GetHookController() != nil {
			hook.GetHookController().DisableScheduleBindings()
		}
	}

	// Stop all Kubernetes event monitors
	kubeHooks := app.GetHooksByBinding(shtypes.OnKubernetesEvent)
	for _, hook := range kubeHooks {
		m.logger.Debug("disable hook", slog.String("name", name), slog.String("hook", hook.GetName()))
		if hook.GetHookController() != nil {
			hook.GetHookController().StopMonitors()
		}
	}

	m.scheduler.Remove(name)

	return nil
}

// UnlockKubernetesMonitors called after sync task is completed to unlock getting events
func (m *Manager) UnlockKubernetesMonitors(name, hook string, monitors ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app := m.apps[name]
	if app == nil {
		return
	}

	m.logger.Debug("unlock kubernetes monitors",
		slog.String("name", name),
		slog.String("hook", hook),
		slog.String("monitors", strings.Join(monitors, ",")))
	app.UnlockKubernetesMonitors(hook, monitors...)
}

// GetPackageQueues collects all queues from package hooks
func (m *Manager) GetPackageQueues(name string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	app := m.apps[name]
	if app == nil {
		return nil
	}

	var res []string
	scheduleHooks := app.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range scheduleHooks {
		for _, hookBinding := range hook.GetHookConfig().Schedules {
			res = append(res, hookBinding.Queue)
		}
	}

	kubeEventsHooks := app.GetHooksByBinding(shtypes.OnKubernetesEvent)
	for _, hook := range kubeEventsHooks {
		for _, hookBinding := range hook.GetHookConfig().OnKubernetesEvents {
			res = append(res, hookBinding.Queue)
		}
	}

	return slices.Compact(res)
}

func (m *Manager) GetAppInfo(name string) apps.Info {
	m.mu.Lock()
	defer m.mu.Unlock()

	app := m.apps[name]
	if app == nil {
		return apps.Info{}
	}

	return app.GetInfo()
}
