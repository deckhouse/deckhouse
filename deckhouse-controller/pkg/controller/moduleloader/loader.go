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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/loader"
	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/pkgsync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/module/installer"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	d8utils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	defaultModuleWeight = 900

	// definitionLabelPrefix marks the labels the module definition owns on the Module: the
	// tags and the cni/cloud-provider markers.
	definitionLabelPrefix = "module.deckhouse.io/"

	moduleOrderIdx = 2
	moduleNameIdx  = 3
)

var (
	// some ephemeral modules, which we even don't want to load
	excludeModules = map[string]struct{}{
		"000-common":           {},
		"007-registrypackages": {},
	}

	// validModuleNameRe defines a valid module name. It may have a number prefix: it is an order of the module.
	validModuleNameRe = regexp.MustCompile(`^(([0-9]+)-)?(.+)$`)

	ErrModuleIsNotFound              = errors.New("module is not found")
	ErrConversionsDirectoryPathEmpty = errors.New("conversions directory path is empty")
)

var _ loader.ModuleLoader = &Loader{}

// Installer abstracts *installer.Installer so tests can inject a mock and assert
// which installation path a module took (Restore vs StageFromRegistry) without
// touching the registry or filesystem. It mirrors the public method set of
// *installer.Installer that the loader and its getter expose to other controllers.
type Installer interface {
	SetClusterUUID(id string)
	GetInstalled() (map[string]struct{}, error)
	GetImageDigest(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error)
	Download(ctx context.Context, source *v1alpha1.ModuleSource, moduleName, version string) (string, error)
	Install(ctx context.Context, module, version, tempModulePath string) error
	Stage(ctx context.Context, module, version, tempModulePath string) error
	StageFromRegistry(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error
	Restore(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error
	IsEmbeddedPresent(module string) bool
	Uninstall(ctx context.Context, module string) error
}

type Loader struct {
	client client.Client
	// reader bypasses the manager cache: the package versions the loader completes are cached
	// only when the module packages feature is on, and the loader runs everywhere.
	reader         client.Reader
	logger         *log.Logger
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer
	modules        map[string]*moduletypes.Module
	version        string
	modulesDirs    []string
	// global module dir
	globalDir string

	installer Installer

	registries map[string]*addonmodules.Registry

	dependencyContainer dependency.Container
	exts                extenders.IExtendersStack

	downloadedModulesDir string
	symlinksDir          string
	conversionsStore     *conversion.ConversionsStore
}

func New(client client.Client, reader client.Reader, version, modulesDir, globalDir string, dc dependency.Container, exts extenders.IExtendersStack, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, conversionsStore *conversion.ConversionsStore, logger *log.Logger) *Loader {
	return &Loader{
		client:               client,
		reader:               reader,
		logger:               logger,
		modulesDirs:          addonutils.SplitToPaths(modulesDir),
		globalDir:            globalDir,
		downloadedModulesDir: app.DownloadedModulesDir(),
		installer:            installer.New(dc, logger),
		symlinksDir:          filepath.Join(app.DownloadedModulesDir(), "modules"),
		modules:              make(map[string]*moduletypes.Module),
		registries:           make(map[string]*addonmodules.Registry),
		embeddedPolicy:       embeddedPolicy,
		version:              version,
		dependencyContainer:  dc,
		exts:                 exts,
		conversionsStore:     conversionsStore,
	}
}

// Sync syncs fs and cluster, restores or deletes modules
func (l *Loader) Sync(ctx context.Context) error {
	l.installer.SetClusterUUID(d8utils.GetClusterUUID(ctx, l.client))

	l.logger.Debug("init module loader")

	l.logger.Debug("delete orphan modules")
	if err := l.deleteOrphanModules(ctx); err != nil {
		return fmt.Errorf("delete orphan modules: %w", err)
	}

	l.logger.Debug("restore modules by overrides")
	if err := l.restoreModulesByOverrides(ctx); err != nil {
		return fmt.Errorf("restore modules by overrides: %w", err)
	}

	l.logger.Debug("restore modules by releases")
	if err := l.restoreModulesByReleases(ctx); err != nil {
		return fmt.Errorf("restore modules by releases: %w", err)
	}

	go l.runDeleteStaleModuleReleasesLoop(ctx)

	l.logger.Debug("module loader initialized")

	return nil
}

// Installer returns installer instance
func (l *Loader) Installer() Installer {
	return l.installer
}

// LoadModules implements the module loader interface from addon-operator, used for registering modules in addon-operator
func (l *Loader) LoadModules() ([]*addonmodules.BasicModule, error) {
	result := make([]*addonmodules.BasicModule, 0, len(l.modules))

	for _, module := range l.modules {
		result = append(result, module.GetBasicModule())
	}

	return result, nil
}

// LoadModule implements the module loader interface from addon-operator, it reads single directory and returns BasicModule
// modulePath is in the following format: /deckhouse-controller/downloaded/<module_name>/<module_version>
func (l *Loader) LoadModule(_, modulePath string) (*addonmodules.BasicModule, error) {
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
	moduleStaticValues, err := addonutils.LoadValuesFileFromDir(def.Path, app.StrictModeEnabled())
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

	// inject registry value
	if reg, ok := l.registries[def.Name]; ok {
		module.GetBasicModule().InjectRegistryValue(reg)
	}

	// load conversions
	conversionsDir := filepath.Join(def.Path, "openapi", "conversions")
	var conversions []v1alpha1.ModuleSettingsConversion
	if _, err = os.Stat(conversionsDir); err == nil {
		l.logger.Debug("conversions for the module found", slog.String("name", def.Name))
		if err = l.conversionsStore.Add(def.Name, filepath.Join(def.Path, "openapi", "conversions")); err != nil {
			return nil, fmt.Errorf("load conversions for the %q module: %w", def.Name, err)
		}

		// load conversions for settings
		conversions, err = l.loadConversions(conversionsDir)
		if err != nil {
			if errors.Is(err, ErrConversionsDirectoryPathEmpty) {
				return nil, fmt.Errorf("conversions directory path is empty for the %q module", def.Name)
			}
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
	if err = l.ensureModuleSettings(ctx, def.Name, rawConfig, conversions); err != nil {
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

	l.logger.Info("LoadModulesFromFS started")

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

			embedded := strings.HasPrefix(def.Path, app.EmbeddedModulesDir)

			l.logger.Debug("ensure module", slog.String("name", def.Name))
			if err = l.ensureModule(ctx, def, embedded); err != nil {
				return fmt.Errorf("ensure the '%s' module: %w", def.Name, err)
			}

			if !embedded {
				if err = l.ensurePackageVersion(ctx, def); err != nil {
					return fmt.Errorf("ensure the '%s' module package version: %w", def.Name, err)
				}
			}

			l.modules[def.Name] = module
		}
	}

	// Run cleanup synchronously to avoid race with module-config-controller:
	// async cleanup could set EnabledByModuleConfig to Unknown while the controller is processing a config.
	if err := l.cleanupDeletedModules(ctx); err != nil {
		return fmt.Errorf("cleanup deleted modules: %w", err)
	}

	return nil
}

// cleanupDeletedModules marks the modules without a module config and hands every module its
// version. The modules the image stopped shipping are deleted by the package sync at start.
func (l *Loader) cleanupDeletedModules(ctx context.Context) error {
	ctx, span := otel.Tracer("module-loader").Start(ctx, "cleanupDeletedModules")
	defer span.End()

	ctx, listSpan := otel.Tracer("module-loader").Start(ctx, "listModules")

	modulesList := new(v1alpha2.ModuleList)
	if err := l.client.List(ctx, modulesList); err != nil {
		listSpan.RecordError(err)
		listSpan.End()

		return fmt.Errorf("list all modules: %w", err)
	}

	listSpan.SetAttributes(attribute.Int("modules.count", len(modulesList.Items)))
	listSpan.End()

	ctx, configListSpan := otel.Tracer("module-loader").Start(ctx, "listModuleConfigs")
	moduleConfigs := new(v1alpha1.ModuleConfigList)
	if err := l.client.List(ctx, moduleConfigs); err != nil {
		configListSpan.RecordError(err)
		configListSpan.End()

		return fmt.Errorf("list module configs: %w", err)
	}

	configListSpan.SetAttributes(attribute.Int("moduleConfigs.count", len(moduleConfigs.Items)))
	configListSpan.End()

	ctx, processSpan := otel.Tracer("module-loader").Start(ctx, "processModules")
	defer processSpan.End()

	configured := make(map[string]struct{}, len(moduleConfigs.Items))
	for _, config := range moduleConfigs.Items {
		configured[config.Name] = struct{}{}
	}

	statusUpdatedCount := 0

	for _, module := range modulesList.Items {
		if _, found := configured[module.Name]; found {
			continue
		}

		ctx, statusUpdateSpan := otel.Tracer("module-loader").Start(ctx, "updateModuleStatus")
		statusUpdateSpan.SetAttributes(attribute.String("module.name", module.Name))

		err := ctrlutils.UpdateStatusWithRetry(ctx, l.client, &module, func() error {
			module.SetConditionUnknown(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonUnknown, "")
			return nil
		})
		if err != nil {
			statusUpdateSpan.RecordError(err)
			statusUpdateSpan.End()
			return fmt.Errorf("update status for the '%s' module: %w", module.Name, err)
		}
		statusUpdatedCount++
		statusUpdateSpan.End()
	}

	processSpan.SetAttributes(
		attribute.Int("total_modules", len(modulesList.Items)),
		attribute.Int("status_updated_modules", statusUpdatedCount),
	)
	span.SetAttributes(
		attribute.Int("total_modules", len(modulesList.Items)),
		attribute.Int("status_updated_modules", statusUpdatedCount),
	)

	l.syncModuleVersions(modulesList.Items)

	return nil
}

// syncModuleVersions propagates the module version into each BasicModule. An embedded module
// reports the running Deckhouse version as is, which kubeall expects, not the reduced package
// version its Module carries.
func (l *Loader) syncModuleVersions(modules []v1alpha2.Module) {
	for i := range modules {
		m := &modules[i]

		mod, ok := l.modules[m.Name]
		if !ok {
			continue
		}

		version := m.Spec.PackageVersion
		if m.IsEmbedded() {
			version = l.version
		}

		if version == "" {
			continue
		}

		mod.GetBasicModule().SetVersion(version)
	}
}

// ensureModule writes what the module files say about the module onto its object: the
// description annotations, the tag labels and, for an embedded module, the release channel of
// the embedded policy. The object itself is placed by the package sync before the loader runs,
// so a module without one is skipped.
func (l *Loader) ensureModule(ctx context.Context, def *moduletypes.Definition, embedded bool) error {
	module := new(v1alpha2.Module)
	if err := l.client.Get(ctx, client.ObjectKey{Name: def.Name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the %q module: %w", def.Name, err)
		}

		l.logger.Warn("module does not exist, skip it", slog.String("name", def.Name))

		return nil
	}

	patch := client.MergeFrom(module.DeepCopy())

	applyDefinition(module, def)

	if embedded {
		module.Spec.ReleaseChannel = l.embeddedPolicy.Get().ReleaseChannel
	}

	data, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build patch for the %q module: %w", def.Name, err)
	}

	if string(data) == "{}" {
		return nil
	}

	if err := l.client.Patch(ctx, module, client.RawPatch(patch.Type(), data)); err != nil {
		return fmt.Errorf("patch the %q module: %w", def.Name, err)
	}

	return nil
}

// applyDefinition replaces the labels and annotations the definition owns with the current
// ones, so a tag or a description dropped from the module files leaves the object too. Every
// other key belongs to another writer and stays.
func applyDefinition(module *v1alpha2.Module, def *moduletypes.Definition) {
	for key := range module.Labels {
		if strings.HasPrefix(key, definitionLabelPrefix) {
			delete(module.Labels, key)
		}
	}

	for key, value := range def.Labels() {
		if module.Labels == nil {
			module.Labels = make(map[string]string)
		}

		module.Labels[key] = value
	}

	delete(module.Annotations, v1alpha1.ModuleAnnotationDescriptionRu)
	delete(module.Annotations, v1alpha1.ModuleAnnotationDescriptionEn)

	for key, value := range def.Annotations() {
		if module.Annotations == nil {
			module.Annotations = make(map[string]string)
		}

		module.Annotations[key] = value
	}
}

// ensurePackageVersion completes the package version of a downloaded module from its files,
// which are certainly on disk once the loader parsed them. A dev module follows a tag no
// version describes, and a module without a package triple names no version.
func (l *Loader) ensurePackageVersion(ctx context.Context, def *moduletypes.Definition) error {
	module := new(v1alpha2.Module)
	if err := l.client.Get(ctx, client.ObjectKey{Name: def.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("get the %q module: %w", def.Name, err)
	}

	if module.IsDev() || module.IsEmbedded() || module.Spec.PackageRepositoryName == "" || module.Spec.PackageVersion == "" {
		return nil
	}

	spec := v1alpha1.ModulePackageVersionSpec{
		PackageName:           def.Name,
		PackageRepositoryName: module.Spec.PackageRepositoryName,
		PackageVersion:        module.Spec.PackageVersion,
	}

	return pkgsync.EnsureModulePackageVersion(ctx, l.reader, l.client, l.dependencyContainer, spec, def.Path, l.logger)
}

func (l *Loader) ensureModuleSettings(ctx context.Context, module string, rawConfig []byte, conversions []v1alpha1.ModuleSettingsConversion) error {
	settings := new(v1alpha1.ModuleSettingsDefinition)
	if err := l.client.Get(ctx, client.ObjectKey{Name: module}, settings); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("get the '%s' module settings: %w", module, err)
	}

	if err := settings.SetVersion(rawConfig, conversions); err != nil {
		return fmt.Errorf("set the module settings: %w", err)
	}

	// settings not found
	if settings.UID == "" {
		settings.Name = module
		settings.Labels = map[string]string{"heritage": "deckhouse"}
		if err := l.client.Create(ctx, settings); err != nil {
			return fmt.Errorf("create: %w", err)
		}
		return nil
	}

	if err := l.client.Update(ctx, settings); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
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
			return nil, fmt.Errorf("parse module definition '%s' from dir '%s': %w", name, absPath, err)
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
		return "", fmt.Errorf("info: %w", err)
	}

	targetDirPath, isTargetDir, err := addonutils.SymlinkInfo(filepath.Join(dirPath, info.Name()), info)
	if err != nil {
		return "", fmt.Errorf("symlink info: %w", err)
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
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	def := new(moduletypes.Definition)
	if err = yaml.NewDecoder(f).Decode(def); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
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

// loadConversions loads all conversion rules from the module's conversions directory
func (l *Loader) loadConversions(conversionsDir string) ([]v1alpha1.ModuleSettingsConversion, error) {
	if conversionsDir == "" {
		return nil, ErrConversionsDirectoryPathEmpty
	}

	// Read all files from conversions directory
	files, err := os.ReadDir(conversionsDir)
	if err != nil {
		return nil, fmt.Errorf("read conversions directory: %w", err)
	}

	// Regex to match version files like v1.yaml, v2.yaml, etc.
	versionFileRe := regexp.MustCompile(`^v(\d+)\.yaml$`)

	var allConversions []v1alpha1.ModuleSettingsConversion

	// Process each version file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := versionFileRe.FindStringSubmatch(file.Name())
		if matches == nil {
			continue // Skip non-version files
		}

		// Read and parse the conversion file
		filePath := filepath.Join(conversionsDir, file.Name())
		conversion, err := l.readConversionFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read conversion file %s: %w", file.Name(), err)
		}

		if conversion != nil {
			allConversions = append(allConversions, *conversion)
		}
	}

	return allConversions, nil
}

// readConversionFile reads a single conversion file and extracts conversions and description
func (l *Loader) readConversionFile(filePath string) (*v1alpha1.ModuleSettingsConversion, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Parse YAML directly into a temporary struct
	var fileContent struct {
		Conversions []string                                       `yaml:"conversions"`
		Description *v1alpha1.ModuleSettingsConversionDescriptions `yaml:"description"`
	}

	if err := yaml.Unmarshal(data, &fileContent); err != nil { //nolint:musttag
		return nil, fmt.Errorf("unmarshal conversion file: %w", err)
	}

	return &v1alpha1.ModuleSettingsConversion{
		Expr:         fileContent.Conversions,
		Descriptions: fileContent.Description,
	}, nil
}
