// Copyright 2024 Flant JSC
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

package moduleloader

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/module_manager/loader"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8utils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	defaultModuleWeight = 900

	moduleOrderIdx = 2
	moduleNameIdx  = 3

	embeddedModulesDir = "/deckhouse/modules"
)

var (
	// some ephemeral modules, which we even don't want to load
	excludeModules = map[string]struct{}{
		"000-common":           {},
		"007-registrypackages": {},
	}

	// validModuleNameRe defines a valid module name. It may have a number prefix: it is an order of the module.
	validModuleNameRe = regexp.MustCompile(`^(([0-9]+)-)?(.+)$`)

	ErrModuleIsNotFound = errors.New("module is not found")
)

var _ loader.ModuleLoader = &Loader{}

type Loader struct {
	client         client.Client
	logger         *log.Logger
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	modules        map[string]*moduletypes.Module
	version        string
	modulesDirs    []string
	// global module dir
	globalDir string

	dependencyContainer dependency.Container
	exts                *extenders.ExtendersStack

	downloadedModulesDir string
	symlinksDir          string
	clusterUUID          string
}

func New(client client.Client, version, modulesDir, globalDir string, dc dependency.Container, exts *extenders.ExtendersStack, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, logger *log.Logger) *Loader {
	return &Loader{
		client:               client,
		logger:               logger,
		modulesDirs:          addonutils.SplitToPaths(modulesDir),
		globalDir:            globalDir,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		modules:              make(map[string]*moduletypes.Module),
		embeddedPolicy:       embeddedPolicy,
		version:              version,
		dependencyContainer:  dc,
		exts:                 exts,
	}
}

// Sync syncs fs and cluster, restores or deletes modules
func (l *Loader) Sync(ctx context.Context) error {
	l.clusterUUID = d8utils.GetClusterUUID(ctx, l.client)

	l.logger.Debug("init module loader")

	l.logger.Debug("restore absent modules from overrides")
	if err := l.restoreAbsentModulesFromOverrides(ctx); err != nil {
		return fmt.Errorf("restore absent modules from overrides: %w", err)
	}

	l.logger.Debug("restore absent modules from releases")
	if err := l.restoreAbsentModulesFromReleases(ctx); err != nil {
		return fmt.Errorf("restore absent modules from releases: %w", err)
	}

	l.logger.Debug("delete modules with absent release")
	if err := l.deleteModulesWithAbsentRelease(ctx); err != nil {
		return fmt.Errorf("delete modules with absent releases: %w", err)
	}

	go l.runDeleteStaleModuleReleasesLoop(ctx)

	l.logger.Debug("module loader initialized")

	return nil
}

// LoadModules implements the module loader interface from addon-operator, used for registering modules in addon-operator
func (l *Loader) LoadModules() ([]*modules.BasicModule, error) {
	result := make([]*modules.BasicModule, 0, len(l.modules))

	for _, module := range l.modules {
		result = append(result, module.GetBasicModule())
	}

	return result, nil
}

// LoadModule implements the module loader interface from addon-operator, it reads single directory and returns BasicModule
// modulePath is in the following format: /deckhouse-controller/downloaded/<module_name>/<module_version>
func (l *Loader) LoadModule(_, modulePath string) (*modules.BasicModule, error) {
	if _, err := readDir(modulePath); err != nil {
		return nil, err
	}

	// run moduleDefinitionByDir("<module_name>", "/deckhouse-controller/downloaded/<module_name>/<module_version>")
	def, err := l.moduleDefinitionByDir(filepath.Base(filepath.Dir(modulePath)), modulePath)
	if err != nil {
		return nil, err
	}

	module, err := l.processModuleDefinition(context.TODO(), def)
	if err != nil {
		return nil, err
	}
	l.modules[def.Name] = module

	return module.GetBasicModule(), nil
}

func (l *Loader) processModuleDefinition(ctx context.Context, def *moduletypes.Definition) (*moduletypes.Module, error) {
	if err := validateModuleName(def.Name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// load values for the module
	valuesModuleName := addonutils.ModuleNameToValuesKey(def.Name)

	// 1. from static values.yaml inside the module
	moduleStaticValues, err := addonutils.LoadValuesFileFromDir(def.Path, app.StrictModeEnabled)
	if err != nil {
		return nil, fmt.Errorf("load values file from the %q dir: %w", def.Path, err)
	}

	if moduleStaticValues.HasKey(valuesModuleName) {
		moduleStaticValues = moduleStaticValues.GetKeySection(valuesModuleName)
	}

	// 2. from openapi defaults
	rawConfig, rawValues, err := addonutils.ReadOpenAPIFiles(filepath.Join(def.Path, "openapi"))
	if err != nil {
		return nil, fmt.Errorf("read openapi files: %w", err)
	}

	module, err := moduletypes.NewModule(def, moduleStaticValues, rawConfig, rawValues, l.logger.Named("module"))
	if err != nil {
		return nil, fmt.Errorf("build %q module: %w", def.Name, err)
	}

	// load conversions
	if _, err = os.Stat(filepath.Join(def.Path, "openapi", "conversions")); err == nil {
		l.logger.Debug("conversions for the module found", slog.String("name", def.Name))
		if err = conversion.Store().Add(def.Name, filepath.Join(def.Path, "openapi", "conversions")); err != nil {
			return nil, fmt.Errorf("load conversions for the %q module: %w", def.Name, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("load conversions for the %q module: %w", def.Name, err)
	}

	// load constraints
	if err = l.exts.AddConstraints(def.Name, def.Critical, def.Accessibility, def.Requirements); err != nil {
		return nil, fmt.Errorf("load constraints for the %q module: %w", def.Name, err)
	}

	// ensure settings
	if err = l.ensureModuleSettings(ctx, def.Name, rawConfig); err != nil {
		return nil, fmt.Errorf("ensure the %q module settings: %w", def.Name, err)
	}

	// ensure module
	if err = l.ensureModule(ctx, def, strings.HasPrefix(def.Path, embeddedModulesDir)); err != nil {
		return nil, fmt.Errorf("ensure the '%s' embedded module: %w", def.Name, err)
	}

	return module, nil
}

func validateModuleName(name string) error {
	// check if name is consistent for conversions between kebab-case and camelCase.
	restoredName := addonutils.ModuleNameFromValuesKey(addonutils.ModuleNameToValuesKey(name))

	if name != restoredName {
		return fmt.Errorf("'%s' name should be in kebab-case and be restorable from camelCase: consider renaming to '%s'", name, restoredName)
	}

	return nil
}

func (l *Loader) GetModuleByName(name string) (*moduletypes.Module, error) {
	module, ok := l.modules[name]
	if !ok {
		return nil, ErrModuleIsNotFound
	}

	return module, nil
}

func (l *Loader) GetModulesByExclusiveGroup(exclusiveGroup string) []string {
	modules := make([]string, 0, len(l.modules))
	for _, module := range l.modules {
		if module.GetModuleDefinition().ExclusiveGroup == exclusiveGroup {
			modules = append(modules, module.GetBasicModule().Name)
		}
	}
	return modules
}

// LoadModulesFromFS parses and ensures modules from FS
func (l *Loader) LoadModulesFromFS(ctx context.Context) error {
	start := time.Now()
	defer func() {
		l.logger.Info("LoadModulesFromFS completed",
			slog.Int64("took_ms", time.Since(start).Milliseconds()),
			slog.Int("modules", len(l.modules)),
		)
	}()
	// load the 'global' module conversions
	if _, err := os.Stat(filepath.Join(l.globalDir, "openapi", "conversions")); err == nil {
		l.logger.Debug("conversions for the 'global' module found")
		if err = conversion.Store().Add("global", filepath.Join(l.globalDir, "openapi", "conversions")); err != nil {
			return fmt.Errorf("load conversions for the 'global' module: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load conversions for the 'global' module: %w", err)
	}

	for _, dir := range l.modulesDirs {
		l.logger.Debug("parse modules from the dir", slog.String("path", dir))
		definitions, err := l.parseModulesDir(dir)
		if err != nil {
			return fmt.Errorf("parse modules from the %q dir: %w", dir, err)
		}
		l.logger.Debug("parsed modules from the dir", slog.Int("count", len(definitions)), slog.String("path", dir))

		ioStart := time.Now()

		jobs := make(chan *moduletypes.Definition)
		results := make(chan modIOResult, len(definitions))
		var wg sync.WaitGroup

		workerCount := runtime.NumCPU()
		if workerCount > len(definitions) {
			workerCount = len(definitions)
		}
		if workerCount < 1 {
			workerCount = 1
		}

		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for d := range jobs {
					l.logger.Debug("read module files from the dir", slog.String("name", d.Name), slog.String("path", dir))
					mv, cb, vb, err := l.readModuleData(d)
					results <- modIOResult{def: d, moduleStaticValues: mv, rawConfig: cb, rawValues: vb, err: err}
				}
			}()
		}

		for _, d := range definitions {
			jobs <- d
		}
		close(jobs)
		wg.Wait()
		close(results)

		collected := make([]modIOResult, 0, len(definitions))
		for r := range results {
			if r.err != nil {
				return fmt.Errorf("process the '%s' module definition: %w", r.def.Name, r.err)
			}
			collected = append(collected, r)
		}

		ioTook := time.Since(ioStart)
		l.logger.Info("parallel I/O phase completed",
			slog.Int64("took_ms", ioTook.Milliseconds()),
			slog.Int("workers", workerCount),
			slog.Int("modules", len(collected)))

		// OPTIMIZATION: Split into fast and slow phases for maximum parallelization
		sequentialStart := time.Now()

		// Phase 1: Fast operations (module construction, stores) - sequential but fast
		moduleConstructStart := time.Now()
		modules := make(map[string]*moduletypes.Module, len(collected))
		for _, r := range collected {
			if _, ok := l.modules[r.def.Name]; ok {
				l.logger.Warn("module already exists, skip it from path", slog.String("name", r.def.Name), slog.String("path", r.def.Path))
				continue
			}

			// Fast: module construction
			module, err := moduletypes.NewModule(r.def, r.moduleStaticValues, r.rawConfig, r.rawValues, l.logger.Named("module"))
			if err != nil {
				return fmt.Errorf("build %q module: %w", r.def.Name, err)
			}
			modules[r.def.Name] = module

			// Fast: conversions (file system check + add to store)
			if _, err = os.Stat(filepath.Join(r.def.Path, "openapi", "conversions")); err == nil {
				l.logger.Debug("conversions for the module found", slog.String("name", r.def.Name))
				if err = conversion.Store().Add(r.def.Name, filepath.Join(r.def.Path, "openapi", "conversions")); err != nil {
					return fmt.Errorf("load conversions for the %q module: %w", r.def.Name, err)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("load conversions for the %q module: %w", r.def.Name, err)
			}

			// Fast: constraints
			if err = l.exts.AddConstraints(r.def.Name, r.def.Critical, r.def.Accessibility, r.def.Requirements); err != nil {
				return fmt.Errorf("load constraints for the %q module: %w", r.def.Name, err)
			}
		}
		moduleConstructTook := time.Since(moduleConstructStart)

		// Phase 2: BATCHED K8s operations - the real bottleneck optimization!
		k8sStart := time.Now()
		if err := l.batchEnsureModulesAndSettings(ctx, collected, strings.HasPrefix(dir, embeddedModulesDir)); err != nil {
			return fmt.Errorf("batch ensure modules and settings: %w", err)
		}
		k8sTook := time.Since(k8sStart)

		// Phase 3: Store constructed modules
		for name, module := range modules {
			l.modules[name] = module
		}

		l.logger.Info("sequential processing breakdown",
			slog.Int64("construct_ms", moduleConstructTook.Milliseconds()),
			slog.Int64("k8s_batch_ms", k8sTook.Milliseconds()),
			slog.Int("modules_processed", len(modules)))

		sequentialTook := time.Since(sequentialStart)
		l.logger.Info("sequential processing phase completed",
			slog.Int64("took_ms", sequentialTook.Milliseconds()),
			slog.Int("modules_processed", len(collected)))
	}

	// OPTIMIZATION: Make cleanup async to not block module loading startup!
	// Cleanup is non-critical housekeeping that can happen in background
	go func() {
		cleanupStart := time.Now()
		if err := l.cleanupDeletedModules(ctx); err != nil {
			l.logger.Warn("async cleanup failed", slog.String("error", err.Error()))
		} else {
			cleanupTook := time.Since(cleanupStart)
			l.logger.Info("async module cleanup completed", slog.Int64("cleanup_ms", cleanupTook.Milliseconds()))
		}
	}()

	return nil
}

// modIOResult holds module data from parallel I/O phase
type modIOResult struct {
	def                  *moduletypes.Definition
	moduleStaticValues   addonutils.Values
	rawConfig, rawValues []byte
	err                  error
}

// batchEnsureModulesAndSettings performs K8s operations in optimized batches instead of one-by-one
func (l *Loader) batchEnsureModulesAndSettings(ctx context.Context, results []modIOResult, embedded bool) error {
	// OPTIMIZATION 1: Batch-fetch existing K8s resources to minimize round trips
	fetchStart := time.Now()

	moduleNames := make([]string, 0, len(results))
	for _, r := range results {
		moduleNames = append(moduleNames, r.def.Name)
	}

	// Batch fetch all modules and settings at once
	existingModules, existingSettings, err := l.batchFetchK8sResources(ctx, moduleNames)
	if err != nil {
		return fmt.Errorf("batch fetch K8s resources: %w", err)
	}
	fetchTook := time.Since(fetchStart)

	// OPTIMIZATION 2: Prepare all operations, then batch execute
	prepareStart := time.Now()
	var modulesToCreate, modulesToUpdate []*v1alpha1.Module
	var settingsToCreate, settingsToUpdate []*v1alpha1.ModuleSettingsDefinition

	for _, r := range results {
		// Prepare module operations
		if existingModule, exists := existingModules[r.def.Name]; exists {
			if updatedModule := l.prepareModuleUpdate(existingModule, r.def, embedded); updatedModule != nil {
				modulesToUpdate = append(modulesToUpdate, updatedModule)
			}
		} else if embedded {
			newModule := l.prepareModuleCreate(r.def, embedded)
			modulesToCreate = append(modulesToCreate, newModule)
		}

		// Prepare settings operations
		if existingSettings, exists := existingSettings[r.def.Name]; exists {
			if updatedSettings := l.prepareSettingsUpdate(existingSettings, r.def.Name, r.rawConfig); updatedSettings != nil {
				settingsToUpdate = append(settingsToUpdate, updatedSettings)
			}
		} else {
			newSettings := l.prepareSettingsCreate(r.def.Name, r.rawConfig)
			settingsToCreate = append(settingsToCreate, newSettings)
		}
	}
	prepareTook := time.Since(prepareStart)

	// OPTIMIZATION 3: Execute batched operations with parallel goroutines
	executeStart := time.Now()
	var wg sync.WaitGroup
	var batchErr error
	var errMux sync.Mutex

	setError := func(err error) {
		errMux.Lock()
		if batchErr == nil {
			batchErr = err
		}
		errMux.Unlock()
	}

	// Parallel batch operations
	if len(modulesToCreate) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.batchCreateModules(ctx, modulesToCreate); err != nil {
				setError(fmt.Errorf("batch create modules: %w", err))
			}
		}()
	}

	if len(modulesToUpdate) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.batchUpdateModules(ctx, modulesToUpdate); err != nil {
				setError(fmt.Errorf("batch update modules: %w", err))
			}
		}()
	}

	if len(settingsToCreate) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.batchCreateSettings(ctx, settingsToCreate); err != nil {
				setError(fmt.Errorf("batch create settings: %w", err))
			}
		}()
	}

	if len(settingsToUpdate) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.batchUpdateSettings(ctx, settingsToUpdate); err != nil {
				setError(fmt.Errorf("batch update settings: %w", err))
			}
		}()
	}

	wg.Wait()
	executeTook := time.Since(executeStart)

	l.logger.Info("K8s batch operations completed",
		slog.Int64("fetch_ms", fetchTook.Milliseconds()),
		slog.Int64("prepare_ms", prepareTook.Milliseconds()),
		slog.Int64("execute_ms", executeTook.Milliseconds()),
		slog.Int("modules_create", len(modulesToCreate)),
		slog.Int("modules_update", len(modulesToUpdate)),
		slog.Int("settings_create", len(settingsToCreate)),
		slog.Int("settings_update", len(settingsToUpdate)))

	return batchErr
}

// cleanupDeletedModules removes modules that no longer exist and updates status for modules without configs
func (l *Loader) cleanupDeletedModules(ctx context.Context) error {
	// Batch fetch all K8s resources at once for cleanup
	fetchStart := time.Now()
	modulesList := new(v1alpha1.ModuleList)
	if err := l.client.List(ctx, modulesList); err != nil {
		return fmt.Errorf("list all modules: %w", err)
	}

	moduleConfigs := new(v1alpha1.ModuleConfigList)
	if err := l.client.List(ctx, moduleConfigs); err != nil {
		return fmt.Errorf("list module configs: %w", err)
	}
	fetchTook := time.Since(fetchStart)
	l.logger.Info("cleanup fetch completed", slog.Int64("fetch_ms", fetchTook.Milliseconds()))

	// OPTIMIZATION: Create map for O(1) config lookups instead of O(N²) nested loops
	configMap := make(map[string]bool, len(moduleConfigs.Items))
	for _, config := range moduleConfigs.Items {
		configMap[config.GetName()] = true
	}

	var modulesToDelete []*v1alpha1.Module
	var modulesToUpdateStatus []*v1alpha1.Module

	// OPTIMIZATION: Collect all operations first, then execute in batches
	collectStart := time.Now()
	for _, module := range modulesList.Items {
		// Collect embedded modules that no longer exist in filesystem for deletion
		if module.IsEmbedded() && l.modules[module.Name] == nil {
			moduleCopy := module.DeepCopy()
			modulesToDelete = append(modulesToDelete, moduleCopy)
			continue
		}

		// Collect modules without configs for status update
		if !configMap[module.Name] {
			moduleCopy := module.DeepCopy()
			modulesToUpdateStatus = append(modulesToUpdateStatus, moduleCopy)
		}
	}
	collectTook := time.Since(collectStart)
	l.logger.Info("cleanup collect completed",
		slog.Int64("collect_ms", collectTook.Milliseconds()),
		slog.Int("to_delete", len(modulesToDelete)),
		slog.Int("to_update", len(modulesToUpdateStatus)))

	// OPTIMIZATION: Parallel batch execution of cleanup operations
	executeStart := time.Now()
	var wg sync.WaitGroup
	var cleanupErr error
	var errMux sync.Mutex

	setError := func(err error) {
		errMux.Lock()
		if cleanupErr == nil {
			cleanupErr = err
		}
		errMux.Unlock()
	}

	// Parallel delete operations
	if len(modulesToDelete) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deleteStart := time.Now()
			if err := l.batchDeleteModules(ctx, modulesToDelete); err != nil {
				setError(fmt.Errorf("batch delete modules: %w", err))
			}
			deleteTook := time.Since(deleteStart)
			l.logger.Info("cleanup delete completed", slog.Int64("delete_ms", deleteTook.Milliseconds()))
		}()
	}

	// Parallel status update operations
	if len(modulesToUpdateStatus) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statusStart := time.Now()
			l.batchUpdateModuleStatuses(ctx, modulesToUpdateStatus)
			statusTook := time.Since(statusStart)
			l.logger.Info("cleanup status update completed", slog.Int64("status_ms", statusTook.Milliseconds()))
		}()
	}

	wg.Wait()
	executeTook := time.Since(executeStart)
	l.logger.Info("cleanup execute completed", slog.Int64("execute_ms", executeTook.Milliseconds()))

	l.logger.Debug("cleanup summary",
		slog.Int("modules_deleted", len(modulesToDelete)),
		slog.Int("statuses_updated", len(modulesToUpdateStatus)))

	return cleanupErr
}

// batchDeleteModules deletes multiple modules efficiently
func (l *Loader) batchDeleteModules(ctx context.Context, modules []*v1alpha1.Module) error {
	for _, module := range modules {
		l.logger.Debug("delete embedded module", slog.String("name", module.Name))
		if err := l.client.Delete(ctx, module); err != nil {
			return fmt.Errorf("delete the '%s' embedded module: %w", module.Name, err)
		}
	}
	return nil
}

// batchUpdateModuleStatuses updates module statuses efficiently - MUCH faster than individual UpdateStatusWithRetry
func (l *Loader) batchUpdateModuleStatuses(ctx context.Context, modules []*v1alpha1.Module) {
	// OPTIMIZATION: Instead of individual UpdateStatusWithRetry (which does Get+Update for each),
	// we modify objects in-place and do direct Update calls

	var successCount, failureCount int

	for i, module := range modules {
		updateStart := time.Now()

		// Set the condition directly without retry/conflict handling since we have fresh objects
		module.SetConditionUnknown(v1alpha1.ModuleConditionEnabledByModuleConfig, "", "")

		// Direct status update - much faster than UpdateStatusWithRetry
		if err := l.client.Status().Update(ctx, module); err != nil {
			failureCount++
			l.logger.Warn("failed to update module status",
				slog.String("module", module.Name),
				slog.String("error", err.Error()))
			// Continue with other modules instead of failing completely
			continue
		}

		successCount++
		updateTook := time.Since(updateStart)

		// Log every 10th update to track progress on large batches
		if i%10 == 0 || updateTook.Milliseconds() > 100 {
			l.logger.Debug("status update progress",
				slog.String("module", module.Name),
				slog.Int64("update_ms", updateTook.Milliseconds()),
				slog.Int("progress", i+1),
				slog.Int("total", len(modules)))
		}
	}

	l.logger.Info("batch status update summary",
		slog.Int("success", successCount),
		slog.Int("failed", failureCount),
		slog.Int("total", len(modules)))
}

// readModuleData performs IO-bound per-module file reads and light processing only; no side effects.
func (l *Loader) readModuleData(def *moduletypes.Definition) (addonutils.Values, []byte, []byte, error) {
	if err := validateModuleName(def.Name); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid name: %w", err)
	}

	valuesModuleName := addonutils.ModuleNameToValuesKey(def.Name)

	moduleStaticValues, err := addonutils.LoadValuesFileFromDir(def.Path, app.StrictModeEnabled)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load values file from the %q dir: %w", def.Path, err)
	}

	if moduleStaticValues.HasKey(valuesModuleName) {
		moduleStaticValues = moduleStaticValues.GetKeySection(valuesModuleName)
	}

	rawConfig, rawValues, err := addonutils.ReadOpenAPIFiles(filepath.Join(def.Path, "openapi"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read openapi files: %w", err)
	}

	return moduleStaticValues, rawConfig, rawValues, nil
}

// Batch K8s operations for maximum performance
func (l *Loader) batchFetchK8sResources(ctx context.Context, moduleNames []string) (map[string]*v1alpha1.Module, map[string]*v1alpha1.ModuleSettingsDefinition, error) {
	var wg sync.WaitGroup
	var fetchErr error
	var errMux sync.Mutex

	modules := make(map[string]*v1alpha1.Module)
	settings := make(map[string]*v1alpha1.ModuleSettingsDefinition)
	var modulesMux, settingsMux sync.Mutex

	setError := func(err error) {
		errMux.Lock()
		if fetchErr == nil {
			fetchErr = err
		}
		errMux.Unlock()
	}

	// Parallel fetch modules and settings
	wg.Add(2)

	go func() {
		defer wg.Done()
		modulesList := new(v1alpha1.ModuleList)
		if err := l.client.List(ctx, modulesList); err != nil {
			setError(fmt.Errorf("list modules: %w", err))
			return
		}

		modulesMux.Lock()
		for _, module := range modulesList.Items {
			for _, name := range moduleNames {
				if module.Name == name {
					moduleCopy := module.DeepCopy()
					modules[name] = moduleCopy
					break
				}
			}
		}
		modulesMux.Unlock()
	}()

	go func() {
		defer wg.Done()
		settingsList := new(v1alpha1.ModuleSettingsDefinitionList)
		if err := l.client.List(ctx, settingsList); err != nil {
			setError(fmt.Errorf("list module settings: %w", err))
			return
		}

		settingsMux.Lock()
		for _, setting := range settingsList.Items {
			for _, name := range moduleNames {
				if setting.Name == name {
					settingsCopy := setting.DeepCopy()
					settings[name] = settingsCopy
					break
				}
			}
		}
		settingsMux.Unlock()
	}()

	wg.Wait()
	return modules, settings, fetchErr
}

func (l *Loader) prepareModuleCreate(def *moduletypes.Definition, embedded bool) *v1alpha1.Module {
	module := &v1alpha1.Module{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ModuleGVK.Kind,
			APIVersion: v1alpha1.ModuleGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        def.Name,
			Annotations: def.Annotations(),
			Labels:      def.Labels(),
		},
		Properties: v1alpha1.ModuleProperties{
			Weight:         def.Weight,
			Stage:          def.Stage,
			Source:         v1alpha1.ModuleSourceEmbedded,
			Critical:       def.Critical,
			Requirements:   def.Requirements,
			Accessibility:  def.Accessibility.ToV1Alpha1(),
			Subsystems:     def.Subsystems,
			Namespace:      def.Namespace,
			DisableOptions: def.DisableOptions,
			ExclusiveGroup: def.ExclusiveGroup,
		},
	}

	if embedded {
		module.Properties.ReleaseChannel = l.embeddedPolicy.Get().ReleaseChannel
		module.Properties.Version = l.version
	}

	return module
}

func (l *Loader) prepareModuleUpdate(existing *v1alpha1.Module, def *moduletypes.Definition, embedded bool) *v1alpha1.Module {
	module := existing.DeepCopy()
	original := existing.DeepCopy()

	module.Properties.Requirements = def.Requirements
	module.Properties.Subsystems = def.Subsystems
	module.Properties.Namespace = def.Namespace
	module.Properties.Weight = def.Weight
	module.Properties.Stage = def.Stage
	module.Properties.DisableOptions = def.DisableOptions
	module.Properties.ExclusiveGroup = def.ExclusiveGroup
	module.Properties.Critical = def.Critical
	module.Properties.Accessibility = def.Accessibility.ToV1Alpha1()

	module.SetAnnotations(def.Annotations())
	module.SetLabels(def.Labels())

	if embedded {
		module.Properties.ReleaseChannel = l.embeddedPolicy.Get().ReleaseChannel
		module.Properties.Version = l.version
	}

	// Only return if something changed
	if !reflect.DeepEqual(original.Properties, module.Properties) ||
		!reflect.DeepEqual(original.Labels, module.Labels) ||
		!reflect.DeepEqual(original.Annotations, module.Annotations) {
		return module
	}

	return nil
}

func (l *Loader) prepareSettingsCreate(moduleName string, rawConfig []byte) *v1alpha1.ModuleSettingsDefinition {
	settings := &v1alpha1.ModuleSettingsDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:   moduleName,
			Labels: map[string]string{"heritage": "deckhouse"},
		},
	}
	_ = settings.SetVersion(rawConfig)
	return settings
}

func (l *Loader) prepareSettingsUpdate(existing *v1alpha1.ModuleSettingsDefinition, moduleName string, rawConfig []byte) *v1alpha1.ModuleSettingsDefinition {
	settings := existing.DeepCopy()
	originalVersion := settings.ObjectMeta.Generation

	if err := settings.SetVersion(rawConfig); err != nil {
		l.logger.Warn("failed to set module settings version", slog.String("module", moduleName), slog.String("error", err.Error()))
		return nil
	}

	// Only return if version changed
	if settings.ObjectMeta.Generation != originalVersion {
		return settings
	}

	return nil
}

func (l *Loader) batchCreateModules(ctx context.Context, modules []*v1alpha1.Module) error {
	for _, module := range modules {
		if err := l.client.Create(ctx, module); err != nil {
			return fmt.Errorf("create module %s: %w", module.Name, err)
		}
	}
	return nil
}

func (l *Loader) batchUpdateModules(ctx context.Context, modules []*v1alpha1.Module) error {
	for _, module := range modules {
		if err := l.client.Update(ctx, module); err != nil {
			return fmt.Errorf("update module %s: %w", module.Name, err)
		}
	}
	return nil
}

func (l *Loader) batchCreateSettings(ctx context.Context, settings []*v1alpha1.ModuleSettingsDefinition) error {
	for _, setting := range settings {
		if err := l.client.Create(ctx, setting); err != nil {
			return fmt.Errorf("create setting %s: %w", setting.Name, err)
		}
	}
	return nil
}

func (l *Loader) batchUpdateSettings(ctx context.Context, settings []*v1alpha1.ModuleSettingsDefinition) error {
	for _, setting := range settings {
		if err := l.client.Update(ctx, setting); err != nil {
			return fmt.Errorf("update setting %s: %w", setting.Name, err)
		}
	}
	return nil
}

func (l *Loader) ensureModule(ctx context.Context, def *moduletypes.Definition, embedded bool) error {
	module := new(v1alpha1.Module)
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := l.client.Get(ctx, client.ObjectKey{Name: def.Name}, module); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("get the %q module: %w", def.Name, err)
				}
				if !embedded {
					l.logger.Warn("downloaded module does not exist, skip it", slog.String("name", def.Name))
					return nil
				}
				module = &v1alpha1.Module{
					TypeMeta: metav1.TypeMeta{
						Kind:       v1alpha1.ModuleGVK.Kind,
						APIVersion: v1alpha1.ModuleGVK.GroupVersion().String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        def.Name,
						Annotations: def.Annotations(),
						Labels:      def.Labels(),
					},
					Properties: v1alpha1.ModuleProperties{
						Weight:        def.Weight,
						Stage:         def.Stage,
						Source:        v1alpha1.ModuleSourceEmbedded,
						Critical:      def.Critical,
						Requirements:  def.Requirements,
						Accessibility: def.Accessibility.ToV1Alpha1(),
					},
				}
				l.logger.Debug("embedded module not found, create it", slog.String("name", def.Name))
				if err = l.client.Create(ctx, module); err != nil {
					return fmt.Errorf("create the '%s' embedded module: %w", def.Name, err)
				}
			}

			moduleCopy := module.DeepCopy()

			module.Properties.Requirements = def.Requirements
			module.Properties.Subsystems = def.Subsystems
			module.Properties.Namespace = def.Namespace
			module.Properties.Weight = def.Weight
			module.Properties.Stage = def.Stage
			module.Properties.DisableOptions = def.DisableOptions
			module.Properties.ExclusiveGroup = def.ExclusiveGroup
			module.Properties.Critical = def.Critical
			module.Properties.Accessibility = def.Accessibility.ToV1Alpha1()

			module.SetAnnotations(def.Annotations())
			module.SetLabels(def.Labels())

			if embedded {
				// set deckhouse release channel to embedded modules
				module.Properties.ReleaseChannel = l.embeddedPolicy.Get().ReleaseChannel

				// set deckhouse version to embedded modules
				module.Properties.Version = l.version
			}

			if !reflect.DeepEqual(moduleCopy.Properties, module.Properties) ||
				!reflect.DeepEqual(moduleCopy.Labels, module.Labels) ||
				!reflect.DeepEqual(moduleCopy.Annotations, module.Annotations) {
				return l.client.Update(ctx, module)
			}

			return nil
		})
	})
}

func (l *Loader) ensureModuleSettings(ctx context.Context, module string, rawConfig []byte) error {
	settings := new(v1alpha1.ModuleSettingsDefinition)
	if err := l.client.Get(ctx, client.ObjectKey{Name: module}, settings); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("get the '%s' module settings: %w", module, err)
	}

	if err := settings.SetVersion(rawConfig); err != nil {
		return fmt.Errorf("set the module settings: %w", err)
	}

	// settings not found
	if settings.UID == "" {
		settings.Name = module
		settings.Labels = map[string]string{"heritage": "deckhouse"}
		return l.client.Create(ctx, settings)
	}

	return l.client.Update(ctx, settings)
}

// parseModulesDir returns modules definitions from the target dir
func (l *Loader) parseModulesDir(modulesDir string) ([]*moduletypes.Definition, error) {
	entries, err := readDir(modulesDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	definitions := make([]*moduletypes.Definition, 0)
	for _, entry := range entries {
		name, absPath, err := l.resolveDirEntry(modulesDir, entry)
		if err != nil {
			return nil, fmt.Errorf("resolve dir entry: %w", err)
		}
		// skip non-directories.
		if name == "" {
			continue
		}

		if _, ok := excludeModules[name]; ok {
			continue
		}

		definition, err := l.moduleDefinitionByDir(name, absPath)
		if err != nil {
			return nil, fmt.Errorf("parse module definition by dir: %w", err)
		}

		definitions = append(definitions, definition)
	}

	return definitions, nil
}

// readDir checks if dir exists and returns entries
func readDir(dir string) ([]os.DirEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path '%s' does not exist", dir)
		}
		return nil, fmt.Errorf("list modules directory '%s': %w", dir, err)
	}
	return dirEntries, nil
}

func (l *Loader) resolveDirEntry(dirPath string, entry os.DirEntry) (string, string, error) {
	name := entry.Name()
	absPath := filepath.Join(dirPath, name)

	if entry.IsDir() {
		return name, absPath, nil
	}
	// Fast-path: if it's not a symlink, ignore without extra lstat syscall
	if entry.Type()&fs.ModeSymlink == 0 {
		if name != addonutils.ValuesFileName {
			log.Warn("ignore while searching for modules", slog.String("path", absPath))
		}
		return "", "", nil
	}
	// Check if entry is a symlink to a directory.
	targetPath, err := resolveSymlinkToDir(dirPath, entry)
	if err != nil {
		// TODO: probably we can use os.IsNotExist here
		if e, ok := err.(*fs.PathError); ok {
			if e.Err.Error() == "no such file or directory" {
				l.logger.Warn("symlink target does not exist, ignoring module", slog.String("path", dirPath))
				return "", "", nil
			}
		}

		return "", "", fmt.Errorf("resolve '%s' as a possible symlink: %w", absPath, err)
	}

	if targetPath != "" {
		return name, targetPath, nil
	}

	if name != addonutils.ValuesFileName {
		log.Warn("ignore while searching for modules", slog.String("path", absPath))
	}

	return "", "", nil
}

func resolveSymlinkToDir(dirPath string, entry os.DirEntry) (string, error) {
	info, err := entry.Info()
	if err != nil {
		return "", err
	}

	targetDirPath, isTargetDir, err := addonutils.SymlinkInfo(filepath.Join(dirPath, info.Name()), info)
	if err != nil {
		return "", err
	}

	if isTargetDir {
		return targetDirPath, nil
	}

	return "", nil
}

// moduleDefinitionByDir parses module's definition from the target dir
func (l *Loader) moduleDefinitionByDir(moduleName, moduleDir string) (*moduletypes.Definition, error) {
	definition, err := l.moduleDefinitionByFile(moduleDir)
	if err != nil {
		return nil, err
	}

	if definition == nil {
		l.logger.Debug("module.yaml for module does not exist", slog.String("name", moduleName))
		definition, err = l.moduleDefinitionByDirName(moduleName, moduleDir)
		if err != nil {
			return nil, err
		}
	}

	return definition, nil
}

// moduleDefinitionByFile returns Definition instance parsed from the module.yaml file
func (l *Loader) moduleDefinitionByFile(absPath string) (*moduletypes.Definition, error) {
	path := filepath.Join(absPath, moduletypes.DefinitionFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	def := new(moduletypes.Definition)
	if err = yaml.NewDecoder(f).Decode(def); err != nil {
		return nil, err
	}

	if def.Name == "" {
		return nil, nil
	}

	if def.Weight == 0 {
		def.Weight = defaultModuleWeight
	}

	def.Path = absPath

	return def, nil
}

// moduleDefinitionByDirName returns Definition instance filled with name, order and its absolute path.
func (l *Loader) moduleDefinitionByDirName(dirName string, absPath string) (*moduletypes.Definition, error) {
	matchRes := validModuleNameRe.FindStringSubmatch(dirName)
	if len(matchRes) <= moduleNameIdx {
		return nil, fmt.Errorf("'%s' is invalid name for module: should match regex '%s'", dirName, validModuleNameRe.String())
	}

	return &moduletypes.Definition{
		Name:   matchRes[moduleNameIdx],
		Path:   absPath,
		Weight: parseUintOrDefault(matchRes[moduleOrderIdx], 100),
	}, nil
}

func parseUintOrDefault(num string, defaultValue uint32) uint32 {
	val, err := strconv.ParseUint(num, 10, 31)
	if err != nil {
		return defaultValue
	}
	return uint32(val)
}
