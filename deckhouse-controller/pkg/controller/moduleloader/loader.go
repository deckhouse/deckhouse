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
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/app"
	"github.com/flant/addon-operator/pkg/module_manager/loader"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/utils"
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
	log            *log.Logger
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	modules        map[string]*moduletypes.Module
	version        string
	modulesDirs    []string
	// global module dir
	globalDir string

	dependencyContainer dependency.Container

	downloadedModulesDir string
	symlinksDir          string
	clusterUUID          string
}

func New(client client.Client, version, modulesDir, globalDir string, dc dependency.Container, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, logger *log.Logger) *Loader {
	return &Loader{
		client:               client,
		log:                  logger,
		modulesDirs:          utils.SplitToPaths(modulesDir),
		globalDir:            globalDir,
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		modules:              make(map[string]*moduletypes.Module),
		embeddedPolicy:       embeddedPolicy,
		version:              version,
		dependencyContainer:  dc,
	}
}

// Sync syncs fs and cluster, restores or deletes modules
func (l *Loader) Sync(ctx context.Context) error {
	l.clusterUUID = d8utils.GetClusterUUID(ctx, l.client)

	l.log.Debug("init module loader")

	l.log.Debug("restore absent modules from overrides")
	if err := l.restoreAbsentModulesFromOverrides(ctx); err != nil {
		return fmt.Errorf("restore absent modules from overrides: %w", err)
	}

	l.log.Debug("restore absent modules from releases")
	if err := l.restoreAbsentModulesFromReleases(ctx); err != nil {
		return fmt.Errorf("restore absent modules from releases: %w", err)
	}

	l.log.Debug("delete modules with absent release")
	if err := l.deleteModulesWithAbsentRelease(ctx); err != nil {
		return fmt.Errorf("delete modules with absent releases: %w", err)
	}

	l.log.Debug("module loader initialized")

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

	module, err := l.processModuleDefinition(def)
	if err != nil {
		return nil, err
	}
	l.modules[def.Name] = module

	return module.GetBasicModule(), nil
}

func (l *Loader) processModuleDefinition(def *moduletypes.Definition) (*moduletypes.Module, error) {
	if err := validateModuleName(def.Name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// load values for the module
	valuesModuleName := utils.ModuleNameToValuesKey(def.Name)

	// 1. from static values.yaml inside the module
	moduleStaticValues, err := utils.LoadValuesFileFromDir(def.Path, app.StrictModeEnabled)
	if err != nil {
		return nil, fmt.Errorf("load values file from the %q dir: %w", def.Path, err)
	}

	if moduleStaticValues.HasKey(valuesModuleName) {
		moduleStaticValues = moduleStaticValues.GetKeySection(valuesModuleName)
	}

	// 2. from openapi defaults
	configBytes, vb, err := utils.ReadOpenAPIFiles(filepath.Join(def.Path, "openapi"))
	if err != nil {
		return nil, fmt.Errorf("read openapi files: %w", err)
	}

	module, err := moduletypes.NewModule(def, moduleStaticValues, configBytes, vb, l.log.Named("module"))
	if err != nil {
		return nil, err
	}

	// load conversions
	if _, err = os.Stat(filepath.Join(def.Path, "openapi", "conversions")); err == nil {
		l.log.Debugf("conversions for the '%s' module found", def.Name)
		if err = conversion.Store().Add(def.Name, filepath.Join(def.Path, "openapi", "conversions")); err != nil {
			return nil, fmt.Errorf("load conversions for the %q module: %w", def.Name, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("load conversions for the %q module: %w", def.Name, err)
	}

	// load constrains
	if err = extenders.AddConstraints(def.Name, def.Requirements); err != nil {
		return nil, fmt.Errorf("load constraints for the %q module: %w", def.Name, err)
	}

	return module, nil
}

func validateModuleName(name string) error {
	// check if name is consistent for conversions between kebab-case and camelCase.
	restoredName := utils.ModuleNameFromValuesKey(utils.ModuleNameToValuesKey(name))

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

// LoadModulesFromFS parses and ensures modules from FS
func (l *Loader) LoadModulesFromFS(ctx context.Context) error {
	// load the 'global' module conversions
	if _, err := os.Stat(filepath.Join(l.globalDir, "openapi", "conversions")); err == nil {
		l.log.Debug("conversions for the 'global' module found")
		if err = conversion.Store().Add("global", filepath.Join(l.globalDir, "openapi", "conversions")); err != nil {
			return fmt.Errorf("load conversions for the 'global' module: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("load conversions for the 'global' module: %w", err)
	}

	for _, dir := range l.modulesDirs {
		l.log.Debugf("parse modules from the '%s' dir", dir)
		definitions, err := l.parseModulesDir(dir)
		if err != nil {
			return fmt.Errorf("parse modules from the %q dir: %w", dir, err)
		}
		l.log.Debugf("%d parsed modules from the '%s' dir", len(definitions), dir)
		for _, def := range definitions {
			l.log.Debugf("process the '%s' module definition from the '%s' dir", def.Name, dir)
			module, err := l.processModuleDefinition(def)
			if err != nil {
				return fmt.Errorf("process the '%s' module definition: %w", def.Name, err)
			}

			if _, ok := l.modules[def.Name]; ok {
				l.log.Warnf("the %q module already exists, skip it from the %q", def.Name, def.Path)
				continue
			}

			l.log.Debugf("ensure the '%s' module", def.Name)
			if err = l.ensureModule(ctx, def, strings.HasPrefix(def.Path, embeddedModulesDir)); err != nil {
				return fmt.Errorf("ensure the '%s' embedded module: %w", def.Name, err)
			}

			l.modules[def.Name] = module
		}
	}

	// clear deleted embedded modules
	modulesList := new(v1alpha1.ModuleList)
	if err := l.client.List(ctx, modulesList); err != nil {
		return fmt.Errorf("list all modules: %w", err)
	}
	for _, module := range modulesList.Items {
		if module.IsEmbedded() && l.modules[module.Name] == nil {
			l.log.Debugf("delete the '%s' embedded module", module.Name)
			if err := l.client.Delete(ctx, &module); err != nil {
				return fmt.Errorf("delete the '%s' emebedded module: %w", module.Name, err)
			}
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
					l.log.Warnf("the '%s' downloaded module does not exist, skip it", def.Name)
					return nil
				}
				module = &v1alpha1.Module{
					TypeMeta: metav1.TypeMeta{
						Kind:       v1alpha1.ModuleGVK.Kind,
						APIVersion: v1alpha1.ModuleGVK.GroupVersion().String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: def.Name,
					},
					Properties: v1alpha1.ModuleProperties{
						Weight:       def.Weight,
						Description:  def.Description,
						Stage:        def.Stage,
						Source:       v1alpha1.ModuleSourceEmbedded,
						Requirements: def.Requirements,
					},
				}
				l.log.Debugf("the '%s' embedded module not found, create it", def.Name)
				if err = l.client.Create(ctx, module); err != nil {
					return fmt.Errorf("create the '%s' embedded module: %w", def.Name, err)
				}
			}

			var needsUpdate bool
			if module.Properties.Weight != def.Weight {
				module.Properties.Weight = def.Weight
				needsUpdate = true
			}

			if module.Properties.Description != def.Description {
				module.Properties.Description = def.Description
				needsUpdate = true
			}

			if module.Properties.Stage != def.Stage {
				module.Properties.Stage = def.Stage
				needsUpdate = true
			}

			if !maps.Equal(module.Properties.Requirements, def.Requirements) {
				module.Properties.Requirements = def.Requirements
				needsUpdate = true
			}

			if embedded {
				// set deckhouse release channel to embedded modules
				if module.Properties.ReleaseChannel != l.embeddedPolicy.Get().ReleaseChannel {
					module.Properties.ReleaseChannel = l.embeddedPolicy.Get().ReleaseChannel
					needsUpdate = true
				}

				// set deckhouse version to embedded modules
				if module.Properties.Version != l.version {
					module.Properties.Version = l.version
					needsUpdate = true
				}

				// set embedded source to embedded modules
				// TODO(ipaqsa): it is needed for migration, can be removed after 1.68
				if module.Properties.Source != v1alpha1.ModuleSourceEmbedded {
					module.Properties.Source = v1alpha1.ModuleSourceEmbedded
					needsUpdate = true
				}
			}

			// TODO(ipaqsa): it is needed for migration, can be removed after 1.68
			if !embedded {
				if module.IsEmbedded() {
					module.Properties.Source = ""
				}
			}

			if needsUpdate {
				if err := l.client.Update(ctx, module); err != nil {
					return err
				}
			}

			return nil
		})
	})
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
	// Check if entry is a symlink to a directory.
	targetPath, err := resolveSymlinkToDir(dirPath, entry)
	if err != nil {
		// TODO: probably we can use os.IsNotExist here
		if e, ok := err.(*fs.PathError); ok {
			if e.Err.Error() == "no such file or directory" {
				l.log.Warnf("symlink target '%s' does not exist, ignoring module", dirPath)
				return "", "", nil
			}
		}

		return "", "", fmt.Errorf("resolve '%s' as a possible symlink: %w", absPath, err)
	}

	if targetPath != "" {
		return name, targetPath, nil
	}

	if name != utils.ValuesFileName {
		log.Warnf("ignore '%s' while searching for modules", absPath)
	}

	return "", "", nil
}

func resolveSymlinkToDir(dirPath string, entry os.DirEntry) (string, error) {
	info, err := entry.Info()
	if err != nil {
		return "", err
	}

	targetDirPath, isTargetDir, err := utils.SymlinkInfo(filepath.Join(dirPath, info.Name()), info)
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
		l.log.Debugf("module.yaml for the '%s' module does not exist", moduleName)
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
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	def := new(moduletypes.Definition)
	if err = yaml.NewDecoder(f).Decode(def); err != nil {
		return nil, err
	}

	if def.Name == "" || def.Weight == 0 {
		return nil, nil
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
