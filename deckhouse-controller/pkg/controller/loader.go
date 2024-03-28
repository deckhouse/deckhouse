/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
)

var (
	ErrModuleAlreadyExists = errors.New("module already exists")
	// some ephemeral modules, which we even don't want to load
	excludeModules = map[string]struct{}{
		"000-common":           {},
		"007-registrypackages": {},
	}
)

// reads single directory and returns BasicModule
func (dml *DeckhouseController) LoadModule(moduleSource, modulePath string) (*modules.BasicModule, error) {
	_, err := readDir(modulePath)
	if err != nil {
		return nil, err
	}

	def, err := dml.parseModuleDir(filepath.Base(modulePath), modulePath)
	if err != nil {
		return nil, err
	}

	dm, err := dml.processModuleDefinition(*def)
	if err != nil && !errors.Is(err, ErrModuleAlreadyExists) {
		return nil, err
	}

	dml.deckhouseModules[def.Name] = dm
	dml.sourceModules.SetSource(def.Name, moduleSource)
	return dm.GetBasicModule(), nil
}

func (dml *DeckhouseController) LoadModules() ([]*modules.BasicModule, error) {
	result := make([]*modules.BasicModule, 0, len(dml.deckhouseModules))

	for _, m := range dml.deckhouseModules {
		result = append(result, m.GetBasicModule())
	}

	return result, nil
}

func (dml *DeckhouseController) processModuleDefinition(def models.DeckhouseModuleDefinition) (*models.DeckhouseModule, error) {
	err := validateModuleName(def.Name)
	if err != nil {
		return nil, err
	}

	// load values for module
	valuesModuleName := utils.ModuleNameToValuesKey(def.Name)
	// 1. from static values.yaml inside the module
	moduleStaticValues, err := utils.LoadValuesFileFromDir(def.Path)
	if err != nil {
		return nil, err
	}

	if moduleStaticValues.HasKey(valuesModuleName) {
		moduleStaticValues = moduleStaticValues.GetKeySection(valuesModuleName)
	}

	// 2. from openapi defaults
	cb, vb, err := utils.ReadOpenAPIFiles(filepath.Join(def.Path, "openapi"))
	if err != nil {
		return nil, err
	}

	if cb != nil && vb != nil {
		log.Debugf("Add openapi schema for %q module", valuesModuleName)
		err = dml.mm.GetValuesValidator().SchemaStorage.AddModuleValuesSchemas(valuesModuleName, cb, vb)
		if err != nil {
			return nil, err
		}
	}

	dm := models.NewDeckhouseModule(def, moduleStaticValues, dml.mm.GetValuesValidator())

	if _, ok := dml.deckhouseModules[def.Name]; ok {
		return dm, ErrModuleAlreadyExists
	}

	return dm, nil
}

func (dml *DeckhouseController) searchAndLoadDeckhouseModules() error {
	for _, dir := range dml.dirs {
		definitions, err := dml.findModulesInDir(dir)
		if err != nil {
			return err
		}

		for _, def := range definitions {
			dm, err := dml.processModuleDefinition(def)
			if err != nil {
				if errors.Is(err, ErrModuleAlreadyExists) {
					log.Warnf("Module %q is already exists. Skipping module from %q", def.Name, def.Path)
					continue
				}
				return err
			}

			dml.deckhouseModules[def.Name] = dm
		}
	}

	return nil
}

// checks if dir exists and returns entries
func readDir(dir string) ([]os.DirEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path '%s' does not exist", dir)
		}
		return nil, fmt.Errorf("listing modules directory '%s': %s", dir, err)
	}
	return dirEntries, nil
}

// get module's definition out of a target dir
func (dml *DeckhouseController) parseModuleDir(moduleName, moduleDir string) (*models.DeckhouseModuleDefinition, error) {
	definition, err := dml.moduleFromFile(moduleDir)
	if err != nil {
		return nil, err
	}

	if definition == nil {
		log.Debugf("module.yaml for module %q does not exist", moduleName)
		definition, err = dml.moduleFromDirName(moduleName, moduleDir)
		if err != nil {
			return nil, err
		}
	}

	return definition, nil
}

func (dml *DeckhouseController) findModulesInDir(modulesDir string) ([]models.DeckhouseModuleDefinition, error) {
	dirEntries, err := readDir(modulesDir)
	if err != nil {
		return nil, err
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

		definition, err := dml.parseModuleDir(name, absPath)
		if err != nil {
			return nil, err
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

func resolveDirEntry(dirPath string, entry os.DirEntry) (string, string, error) {
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
