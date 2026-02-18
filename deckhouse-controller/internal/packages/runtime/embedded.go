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

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/lifecycle"
	taskapplysettings "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/applysettings"
	taskdisable "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/disable"
	taskload "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/load"
	taskrun "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/run"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
)

// UpdateEmbedded reconciles an embedded (built-in) module by detecting version or settings
// changes and enqueuing the appropriate task pipeline. Embedded modules skip the download
// and install phases since they are already present on the filesystem.
func (r *Runtime) UpdateEmbedded(module *Module) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Empty settings produce no checksum so the store won't detect a spurious settings change.
	settingsChecksum := ""
	if len(module.Settings) > 0 {
		settingsChecksum = module.Settings.Checksum()
	}

	name := module.Name
	version := module.Definition.Version

	// Store.Update determines the event type (version vs settings change) and calls back
	// with the lifecycle context and the currently loaded module (pkg), if any.
	r.modules.Update(name, version, settingsChecksum, func(ctx context.Context, event int, pkg *modules.Module) {
		var tasks []queue.Task
		if event == lifecycle.EventVersionChanged {
			// Skip scheduler.Check here — embedded modules are bundled with the deckhouse
			// image and are always compatible. Global values (k8s version, bootstrap state)
			// aren't available yet during init. The scheduler re-evaluates on resume.
			r.status.ClearRuntimeConditions(name)
			r.status.SetConditionTrue(name, status.ConditionRequirementsMet)

			tasks = []queue.Task{
				taskload.NewEmbeddedTask(name, module.Settings, r.loadEmbeddedModule, r.status, r.logger),
			}

			// If there's an existing module, disable it first to ensure a clean transition.
			if pkg != nil {
				tasks = slices.Insert(tasks, 0, taskdisable.NewTask(pkg, modulesNamespace, true, r.nelmService, r.queueService, r.status, r.logger))
			}
		}

		// Settings-only change: re-apply settings and re-run without a full reload.
		if event == lifecycle.EventSettingsChanged && pkg != nil {
			tasks = []queue.Task{
				taskapplysettings.NewTask(pkg, module.Settings, r.status, r.logger),
				taskrun.NewTask(pkg, modulesNamespace, r.nelmService, r.status, r.logger),
			}
		}

		for _, task := range tasks {
			r.queueService.Enqueue(ctx, name, task)
		}
	})
}

// loadEmbeddedModule builds a Module from its package files, validates settings, and registers it
// with the lifecycle store and scheduler. Called by the Load task after filesystem mount.
func (r *Runtime) loadEmbeddedModule(ctx context.Context, repo registry.Remote, settings addonutils.Values, packagePath string) (string, error) {
	ctx, span := otel.Tracer(runtimeTracer).Start(ctx, "loadModule")
	defer span.End()

	span.SetAttributes(attribute.String("path", packagePath))
	span.SetAttributes(attribute.String("repository", repo.Name))

	// Embedded module directories have a weight prefix (e.g., "modules/002-deckhouse")
	// but the task only knows the clean name (e.g., "modules/deckhouse").
	// Resolve the real path by matching the weight-prefixed directory on disk.
	realPath, err := resolveEmbeddedPath(packagePath)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	moduleName := filepath.Base(packagePath)

	conf, err := loader.LoadModuleConf(ctx, realPath, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	conf.Repository = repo
	conf.Patcher = r.objectPatcher
	conf.ScheduleManager = r.scheduleManager
	conf.KubeEventsManager = r.kubeEventsManager

	module, err := modules.NewModuleByConfig(moduleName, conf, r.logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newLoadFailedErr(err)
	}

	res, err := module.ValidateSettings(ctx, settings)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", newApplyInitialSettingsErr(err)
	}

	if !res.Valid {
		span.SetStatus(codes.Error, res.Message)
		return "", newApplyInitialSettingsErr(errors.New(res.Message))
	}

	r.mu.Lock()
	r.modules.SetPackage(module.GetName(), module)
	r.mu.Unlock()

	// r.scheduler.Register(module)

	return module.GetVersion(), nil
}

// initEmbedded discovers embedded modules shipped with the deckhouse image and
// registers each one via UpdateEmbedded. Modules live as top-level directories
// under "modules/"; 000-common is a shared library used by other modules, not
// a standalone module, so it is skipped.
func (r *Runtime) initEmbedded() error {
	return filepath.Walk("modules", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip files — only directories represent modules.
		if !info.IsDir() {
			return nil
		}

		// The root "modules" directory itself is not a module; descend into it.
		if path == "modules" {
			return nil
		}

		// 000-common contains shared helpers/libraries, not a runnable module.
		if info.Name() == "000-common" {
			return filepath.SkipDir
		}

		def, err := loadModuleDefinition(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}

			return fmt.Errorf("read definition for %s: %w", info.Name(), err)
		}

		r.UpdateEmbedded(&Module{
			Name: def.Name,
			Definition: modules.Definition{
				Name:     def.Name,
				Critical: def.Critical,
				Weight:   def.Weight,
			},
		})

		// Don't recurse into module subdirectories — we only need top-level entries.
		return filepath.SkipDir
	})
}

// resolveEmbeddedPath finds the actual directory for an embedded module whose
// path on disk includes a weight prefix (e.g., "modules/002-deckhouse" for "modules/deckhouse").
// It globs for directories matching "<parent>/*-<name>" and returns the first match.
func resolveEmbeddedPath(packagePath string) (string, error) {
	// If the path exists as-is (no weight prefix), use it directly.
	if _, err := os.Stat(packagePath); err == nil {
		return packagePath, nil
	}

	parent := filepath.Dir(packagePath)
	name := filepath.Base(packagePath)

	matches, err := filepath.Glob(filepath.Join(parent, "*-"+name))
	if err != nil {
		return "", fmt.Errorf("glob for embedded module %q: %w", name, err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("embedded module directory not found for %q in %q", name, parent)
	}

	return matches[0], nil
}

// loadModuleDefinition reads and parses the module.yaml file from the package directory.
// It validates YAML structure but doesn't validate content.
//
// Returns the parsed Definition or an error if reading or parsing fails.
// TODO(ipaqsa): get rid of it when all modules migrated to package.yaml
func loadModuleDefinition(packageDir string) (*moduletypes.Definition, error) {
	definitionPath := filepath.Join(packageDir, moduletypes.DefinitionFile)

	content, err := os.ReadFile(definitionPath)
	if err != nil {
		return nil, fmt.Errorf("read definition file '%s': %w", definitionPath, err)
	}

	def := new(moduletypes.Definition)
	if err = yaml.Unmarshal(content, def); err != nil {
		return nil, fmt.Errorf("unmarshal file '%s': %w", definitionPath, err)
	}

	return def, nil
}
