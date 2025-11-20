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
	"strconv"
	"strings"

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
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
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
	conversionsStore     *conversion.ConversionsStore
}

func New(client client.Client, version, modulesDir, globalDir string, dc dependency.Container, exts *extenders.ExtendersStack, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, conversionsStore *conversion.ConversionsStore, logger *log.Logger) *Loader {
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
		conversionsStore:     conversionsStore,
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
		if err = l.conversionsStore.Add(def.Name, filepath.Join(def.Path, "openapi", "conversions")); err != nil {
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
	modules := []string{}
	for _, module := range l.modules {
		if module.GetModuleDefinition().ExclusiveGroup == exclusiveGroup {
			modules = append(modules, module.GetBasicModule().Name)
		}
	}
	return modules
}

// LoadModulesFromFS parses and ensures modules from FS
func (l *Loader) LoadModulesFromFS(ctx context.Context) error {
	// load the 'global' module conversions
	if _, err := os.Stat(filepath.Join(l.globalDir, "openapi", "conversions")); err == nil {
		l.logger.Debug("conversions for the 'global' module found")
		if err = l.conversionsStore.Add("global", filepath.Join(l.globalDir, "openapi", "conversions")); err != nil {
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
		for _, def := range definitions {
			l.logger.Debug("process module definition from the dir", slog.String("name", def.Name), slog.String("path", dir))
			module, err := l.processModuleDefinition(ctx, def)
			if err != nil {
				return fmt.Errorf("process the '%s' module definition: %w", def.Name, err)
			}

			if _, ok := l.modules[def.Name]; ok {
				l.logger.Warn("module already exists, skip it from path", slog.String("name", def.Name), slog.String("path", def.Path))
				continue
			}

			l.logger.Debug("ensure module", slog.String("name", def.Name))
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

	moduleConfigs := new(v1alpha1.ModuleConfigList)
	if err := l.client.List(ctx, moduleConfigs); err != nil {
		return fmt.Errorf("list module configs: %w", err)
	}

	for _, module := range modulesList.Items {
		if module.IsEmbedded() && l.modules[module.Name] == nil {
			l.logger.Debug("delete embedded module", slog.String("name", module.Name))
			if err := l.client.Delete(ctx, &module); err != nil {
				return fmt.Errorf("delete the '%s' emebedded module: %w", module.Name, err)
			}
		}

		var found bool
		for _, config := range moduleConfigs.Items {
			if config.GetName() == module.Name {
				found = true
				break
			}
		}

		if !found {
			err := ctrlutils.UpdateStatusWithRetry(ctx, l.client, &module, func() error {
				module.SetConditionUnknown(v1alpha1.ModuleConditionEnabledByModuleConfig, "", "")

				return nil
			})
			if err != nil {
				return fmt.Errorf("update status for the '%s' module: %w", module.Name, err)
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
