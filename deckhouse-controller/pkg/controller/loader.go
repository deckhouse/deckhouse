package controller

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"

	"gopkg.in/yaml.v3"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/utils"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// some ephemeral modules, which we even don't want to load
	excludeModules = map[string]struct{}{
		"000-common":           {},
		"007-registrypackages": {},
	}
)

func (dml *DeckhouseController) LoadModules() ([]*modules.BasicModule, error) {
	result := make([]*modules.BasicModule, 0, len(dml.deckhouseModules))

	for _, m := range dml.deckhouseModules {
		result = append(result, m.GetBasicModule())
	}

	return result, nil
}

func (dml *DeckhouseController) searchAndLoadDeckhouseModules() error {
	for _, dir := range dml.dirs {
		definitions, err := dml.findModulesInDir(dir)
		if err != nil {
			return err
		}

		for _, def := range definitions {
			err = validateModuleName(def.Name)
			if err != nil {
				return err
			}

			// load values for module
			valuesModuleName := utils.ModuleNameToValuesKey(def.Name)
			// 1. from static values.yaml inside the module
			moduleStaticValues, err := utils.LoadValuesFileFromDir(def.Path)
			if err != nil {
				return err
			}

			if moduleStaticValues.HasKey(valuesModuleName) {
				moduleStaticValues = moduleStaticValues.GetKeySection(valuesModuleName)
			}

			// 2. from openapi defaults
			cb, vb, err := utils.ReadOpenAPIFiles(filepath.Join(def.Path, "openapi"))
			if err != nil {
				return err
			}

			if cb != nil && vb != nil {
				err = dml.valuesValidator.SchemaStorage.AddModuleValuesSchemas(valuesModuleName, cb, vb)
				if err != nil {
					return err
				}
			}

			if _, ok := dml.deckhouseModules[def.Name]; ok {
				log.Warnf("Module %q is already exists. Skipping module from %q", def.Name, def.Path)
				continue
			}

			dm := models.NewDeckhouseModule(def, moduleStaticValues, dml.valuesValidator)
			dml.deckhouseModules[def.Name] = dm
		}
	}

	return nil
}

func (dml *DeckhouseController) findModulesInDir(modulesDir string) ([]models.DeckhouseModuleDefinition, error) {
	dirEntries, err := os.ReadDir(modulesDir)
	if err != nil && os.IsNotExist(err) {
		return nil, fmt.Errorf("path '%s' does not exist", modulesDir)
	}
	if err != nil {
		return nil, fmt.Errorf("listing modules directory '%s': %s", modulesDir, err)
	}

	definitions := make([]models.DeckhouseModuleDefinition, 0)
	for _, dirEntry := range dirEntries {
		name, absPath, err := resolveDirEntry(modulesDir, dirEntry)
		if err != nil {
			return nil, err
		}
		// Skip non-directories.
		if name == "" {
			continue
		}

		if _, ok := excludeModules[name]; ok {
			continue
		}

		definition, err := dml.moduleFromFile(absPath)
		if err != nil {
			return nil, err
		}

		if definition == nil {
			log.Debugf("module.yaml for module %q does not exist", name)
			definition, err = dml.moduleFromDirName(name, absPath)
			if err != nil {
				return nil, err
			}
		}

		definitions = append(definitions, *definition)
	}

	return definitions, nil
}

// validModuleNameRe defines a valid module name. It may have a number prefix: it is an order of the module.
var validModuleNameRe = regexp.MustCompile(`^(([0-9]+)-)?(.+)$`)

const (
	ModuleOrderIdx = 2
	ModuleNameIdx  = 3
)

func (dml *DeckhouseController) moduleFromFile(absPath string) (*models.DeckhouseModuleDefinition, error) {
	mFilePath := filepath.Join(absPath, models.ModuleDefinitionFile)
	if _, err := os.Stat(mFilePath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	f, err := os.Open(mFilePath)
	if err != nil {
		return nil, err
	}

	var def models.DeckhouseModuleDefinition

	err = yaml.NewDecoder(f).Decode(&def)
	if err != nil {
		return nil, err
	}

	if def.Name == "" || def.Weight == 0 {
		return nil, nil
	}

	def.Path = absPath

	return &def, nil
}

// moduleFromDirName returns Module instance filled with name, order and its absolute path.
func (dml *DeckhouseController) moduleFromDirName(dirName string, absPath string) (*models.DeckhouseModuleDefinition, error) {
	matchRes := validModuleNameRe.FindStringSubmatch(dirName)
	if matchRes == nil {
		return nil, fmt.Errorf("'%s' is invalid name for module: should match regex '%s'", dirName, validModuleNameRe.String())
	}

	return &models.DeckhouseModuleDefinition{
		Name:   matchRes[ModuleNameIdx],
		Path:   absPath,
		Weight: parseUintOrDefault(matchRes[ModuleOrderIdx], 100),
	}, nil
}

func parseUintOrDefault(num string, defaultValue uint32) uint32 {
	val, err := strconv.ParseUint(num, 10, 31)
	if err != nil {
		return defaultValue
	}
	return uint32(val)
}

func (dml *DeckhouseController) RestoreAbsentSourceModules() error {
	externalModulesDir := os.Getenv("EXTERNAL_MODULES_DIR")
	if externalModulesDir == "" {
		log.Warn("EXTERNAL_MODULES_DIR is not set")
		return nil
	}
	// directory for symlinks will actual versions to all external-modules
	symlinksDir := filepath.Join(externalModulesDir, "modules")

	releaseList, err := dml.kubeClient.DeckhouseV1alpha1().ModuleReleases().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// TODO: add labels to list only Deployed releases
	for _, item := range releaseList.Items {
		if item.Status.Phase != "Deployed" {
			continue
		}

		dml.sourceModules[item.Spec.ModuleName] = item.Labels["source"]

		moduleDir := filepath.Join(symlinksDir, fmt.Sprintf("%d-%s", item.Spec.Weight, item.Spec.ModuleName))
		_, err = os.Stat(moduleDir)
		if err != nil && os.IsNotExist(err) {
			log.Infof("Module %q is absent on file system. Restoring it from source %q", item.Spec.ModuleName, item.Labels["source"])
			moduleVersion := "v" + item.Spec.Version.String()
			moduleName := item.Spec.ModuleName
			moduleVersionPath := path.Join(externalModulesDir, moduleName, moduleVersion)

			err = dml.downloadModule(moduleName, moduleVersion, item.Labels["source"], moduleVersionPath)
			if err != nil {
				log.Warnf("Download module %q with version %s failed: %s. Skipping", moduleName, moduleVersion, err)
				continue
			}

			// restore symlink
			moduleRelativePath := filepath.Join("../", moduleName, moduleVersion)
			symlinkPath := filepath.Join(symlinksDir, fmt.Sprintf("%d-%s", item.Spec.Weight, moduleName))
			err = restoreModuleSymlink(externalModulesDir, symlinkPath, moduleRelativePath)
			if err != nil {
				log.Warnf("Create symlink for module %q failed: %s", moduleName, err)
				continue
			}

			log.Infof("Module %s:%s restored", moduleName, moduleVersion)
		}
	}

	return nil
}

func (dml *DeckhouseController) downloadModule(moduleName, moduleVersion, moduleSource, modulePath string) error {
	if moduleSource == "" {
		return nil
	}

	ms, err := dml.kubeClient.DeckhouseV1alpha1().ModuleSources().Get(context.TODO(), moduleSource, metav1.GetOptions{})
	if err != nil {
		return err
	}

	repo := ms.Spec.Registry.Repo

	opts := make([]cr.Option, 0)

	if ms.Spec.Registry.Scheme == "HTTP" {
		opts = append(opts, cr.WithInsecureSchema(true))
	}

	if ms.Spec.Registry.CA != "" {
		opts = append(opts, cr.WithCA(ms.Spec.Registry.CA))
	}

	if ms.Spec.Registry.DockerCFG != "" {
		opts = append(opts, cr.WithAuth(ms.Spec.Registry.DockerCFG))
	}

	regClient, err := cr.NewClient(path.Join(repo, moduleName), opts...)
	if err != nil {
		return err
	}

	img, err := regClient.Image(moduleVersion)
	if err != nil {
		return fmt.Errorf("fetch module version error: %v", err)
	}

	return copyModuleToFS(modulePath, img)
}

func copyModuleToFS(rootPath string, img regv1.Image) error {
	rc := mutate.Extract(img)
	defer rc.Close()

	err := copyLayersToFS(rootPath, rc)
	if err != nil {
		return fmt.Errorf("copy tar to fs: %w", err)
	}

	return nil
}

func copyLayersToFS(rootPath string, rc io.ReadCloser) error {
	if err := os.MkdirAll(rootPath, 0700); err != nil {
		return fmt.Errorf("mkdir root path: %w", err)
	}

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar reader next: %w", err)
		}

		if strings.Contains(hdr.Name, "..") {
			// CWE-22 check, prevents path traversal
			return fmt.Errorf("path traversal detected in the module archive: malicious path %v", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path.Join(rootPath, hdr.Name), 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(rootPath, hdr.Name))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("copy: %w", err)
			}
			outFile.Close()

			err = os.Chmod(outFile.Name(), os.FileMode(hdr.Mode)&0700) // remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err != nil {
				return fmt.Errorf("chmod: %w", err)
			}
		case tar.TypeSymlink:
			link := path.Join(rootPath, hdr.Name)
			if isRel(hdr.Linkname, link) && isRel(hdr.Name, link) {
				if err := os.Symlink(hdr.Linkname, link); err != nil {
					return fmt.Errorf("create symlink: %w", err)
				}
			}
		case tar.TypeLink:
			err := os.Link(path.Join(rootPath, hdr.Linkname), path.Join(rootPath, hdr.Name))
			if err != nil {
				return fmt.Errorf("create hardlink: %w", err)
			}

		default:
			return errors.New("unknown tar type")
		}
	}
}

func isRel(candidate, target string) bool {
	// GOOD: resolves all symbolic links before checking
	// that `candidate` does not escape from `target`
	if filepath.IsAbs(candidate) {
		return false
	}
	realpath, err := filepath.EvalSymlinks(filepath.Join(target, candidate))
	if err != nil {
		return false
	}
	relpath, err := filepath.Rel(target, realpath)
	return err == nil && !strings.HasPrefix(filepath.Clean(relpath), "..")
}

func restoreModuleSymlink(externalModulesDir, symlinkPath, moduleRelativePath string) error {
	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(externalModulesDir, strings.TrimPrefix(moduleRelativePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(moduleRelativePath, symlinkPath)
}

func resolveDirEntry(dirPath string, entry os.DirEntry) (string, string, error) {
	name := entry.Name()
	absPath := filepath.Join(dirPath, name)

	if entry.IsDir() {
		return name, absPath, nil
	}
	// Check if entry is a symlink to a directory.
	targetPath, err := resolveSymlinkToDir(dirPath, entry)
	if err != nil {
		if e, ok := err.(*fs.PathError); ok {
			if e.Err.Error() == "no such file or directory" {
				log.Warnf("Symlink target %q does not exist. Ignoring module", dirPath)
				return "", "", nil
			}
		}

		return "", "", fmt.Errorf("resolve '%s' as a possible symlink: %v", absPath, err)
	}

	if targetPath != "" {
		return name, targetPath, nil
	}

	if name != utils.ValuesFileName {
		log.Warnf("Ignore '%s' while searching for modules", absPath)
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

func validateModuleName(name string) error {
	// Check if name is consistent for conversions between kebab-case and camelCase.
	valuesKey := utils.ModuleNameToValuesKey(name)
	restoredName := utils.ModuleNameFromValuesKey(valuesKey)

	if name != restoredName {
		return fmt.Errorf("'%s' name should be in kebab-case and be restorable from camelCase: consider renaming to '%s'", name, restoredName)
	}

	return nil
}
