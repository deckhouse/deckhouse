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
	"os"
	"slices"
	"sync"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
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
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	managerTracer = "package-manager"
)

var ErrPackageNotFound = errors.New("package not found")

// DependencyContainer provides access to shared infrastructure services.
type DependencyContainer interface {
	KubeObjectPatcher() *objectpatch.ObjectPatcher
	ScheduleManager() schedulemanager.ScheduleManager
	KubeEventsManager() kubeeventsmanager.KubeEventsManager
}

// Manager manages the lifecycle of application packages.
type Manager struct {
	mu     sync.Mutex                   // Protects apps map
	apps   map[string]*apps.Application // Loaded applications by name
	tmpDir string                       // Temporary directory for hook execution

	loader *loader.ApplicationLoader // Loads packages from filesystem
	nelm   *nelm.Service             // nelm service to install/uninstall releases
	dc     DependencyContainer       // Access to shared services

	logger *log.Logger
}

// New creates a new package manager with the specified apps directory.
func New(appsDir string, dc DependencyContainer, logger *log.Logger) *Manager {
	return &Manager{
		apps:   make(map[string]*apps.Application),
		tmpDir: os.TempDir(),

		loader: loader.NewApplicationLoader(appsDir, logger),
		dc:     dc,

		logger: logger.Named(managerTracer),
	}
}

// LoadApplication loads a package from filesystem and stores it in the manager.
// It discovers hooks, parses OpenAPI schemas, and initializes values storage.
func (m *Manager) LoadApplication(ctx context.Context, inst loader.ApplicationInstance) error {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "LoadApplication")
	defer span.End()

	span.SetAttributes(attribute.String("name", inst.Name))
	span.SetAttributes(attribute.String("namespace", inst.Namespace))
	span.SetAttributes(attribute.String("version", inst.Version))
	span.SetAttributes(attribute.String("package", inst.Package))

	app, err := m.loader.Load(ctx, inst)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("load application: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.apps[app.GetName()] = app

	return nil
}

// StartupPackage runs OnStartup hooks for a package.
// This must be called after InitializeHooks and before RunPackage.
func (m *Manager) StartupPackage(ctx context.Context, name string) error {
	ctx, span := otel.Tracer(managerTracer).Start(ctx, "StartupPackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("tmpDir", m.tmpDir))

	m.logger.Debug("startup package", slog.String("name", name))

	app, err := m.getApp(name)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if err = app.RunHooksByBinding(ctx, shtypes.OnStartup, m.dc); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("run startup hooks: %w", err)
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

	app, err := m.getApp(name)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// monitor may not be created by this time
	if m.nelm.HasMonitor(name) {
		// Hooks can delete release resources, so pause resources monitor before run hooks.
		m.nelm.PauseMonitor(name)
		defer m.nelm.ResumeMonitor(name)
	}

	if err = app.RunHooksByBinding(ctx, addontypes.BeforeHelm, m.dc); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("run before helm hooks: %w", err)
	}

	if err = m.nelm.Upgrade(ctx, app); err != nil && !errors.Is(err, nelm.ErrPackageNotHelm) {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upgrade nelm package: %w", err)
	}

	// Check if AfterHelm hooks modified values (would require nelm upgrade)
	oldChecksum := app.GetValuesChecksum()
	if err = app.RunHooksByBinding(ctx, addontypes.AfterHelm, m.dc); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("run after helm hooks: %w", err)
	}

	if oldChecksum != app.GetValuesChecksum() {
		if err = m.nelm.Upgrade(ctx, app); err != nil && !errors.Is(err, nelm.ErrPackageNotHelm) {
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("install nelm package: %w", err)
		}
	}

	return nil
}

// DisablePackage stops monitoring and disables all hooks for a package.
//
// Process:
//  1. Stop Helm resource monitoring
//  2. Uninstall Helm release
//  3. Disable all schedule hooks
//  4. Stop all Kubernetes event monitors
//  5. Clean up state (TODO: not yet implemented)
func (m *Manager) DisablePackage(ctx context.Context, name string) error {
	_, span := otel.Tracer(managerTracer).Start(ctx, "DisablePackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	m.logger.Debug("disable package", slog.String("name", name))

	app, err := m.getApp(name)
	if err != nil {
		return nil
	}

	if err = m.nelm.Delete(ctx, name); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// after delete helm hooks

	// Disable all schedule-based hooks
	schHooks := app.GetHooksByBinding(shtypes.Schedule)
	for _, hook := range schHooks {
		hook.GetHookController().DisableScheduleBindings()
	}

	// Stop all Kubernetes event monitors
	kubeHooks := app.GetHooksByBinding(shtypes.OnKubernetesEvent)
	for _, hook := range kubeHooks {
		hook.GetHookController().StopMonitors()
	}

	// TODO(ipaqsa): clean un state

	return nil
}

// UnlockKubernetesMonitors called after sync task is completed to unlock getting events
func (m *Manager) UnlockKubernetesMonitors(name, hook string, monitors ...string) {
	app, err := m.getApp(name)
	if err != nil {
		return
	}

	app.UnlockKubernetesMonitors(hook, monitors...)
}

// GetPackageQueues collects all queues from package hooks
func (m *Manager) GetPackageQueues(name string) []string {
	app, err := m.getApp(name)
	if err != nil {
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

// getApp retrieves an application from the manager's cache by name.
// Returns ErrPackageNotFound if the application is not loaded.
//
// Thread-safe: Acquires mutex lock before accessing apps map.
func (m *Manager) getApp(name string) (*apps.Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[name]
	if !ok {
		return nil, ErrPackageNotFound
	}

	return app, nil
}
