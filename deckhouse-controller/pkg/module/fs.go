/*
Copyright 2024 Flant JSC

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

package module

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

type moduleOpenAPISpec struct {
	Properties struct {
		Registry struct {
			Properties struct {
				Base struct {
					Default string `yaml:"default"`
				} `yaml:"base"`
				DockerCFG struct {
					Default string `yaml:"default"`
				} `yaml:"dockercfg"`
				Scheme struct {
					Default string `yaml:"default"`
				} `yaml:"scheme"`
				CA struct {
					Default string `yaml:"default"`
				} `yaml:"ca"`
			} `yaml:"properties"`
		} `yaml:"registry,omitempty"`
	} `yaml:"properties,omitempty"`
}

// SyncRegistrySpec compares and updates current registry settings of a deployed module (in the ./openapi/values.yaml file)
// and the registry settings set in the related module source
func SyncRegistrySpec(downloadedModulesDir, moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource) error {
	var openAPISpec moduleOpenAPISpec

	openAPIFile, err := os.Open(filepath.Join(downloadedModulesDir, moduleName, moduleVersion, "openapi/values.yaml"))
	if err != nil {
		return fmt.Errorf("couldn't open the %s module's openapi values: %w", moduleName, err)
	}
	defer openAPIFile.Close()

	b, err := io.ReadAll(openAPIFile)
	if err != nil {
		return fmt.Errorf("couldn't read from the %s module's openapi values: %w", moduleName, err)
	}

	err = yaml.Unmarshal(b, &openAPISpec)
	if err != nil {
		return fmt.Errorf("couldn't unmarshal the %s module's registry spec: %w", moduleName, err)
	}

	registrySpec := openAPISpec.Properties.Registry.Properties

	if moduleSource.Spec.Registry.CA != registrySpec.CA.Default || moduleSource.Spec.Registry.DockerCFG != registrySpec.DockerCFG.Default || moduleSource.Spec.Registry.Repo != registrySpec.Base.Default || moduleSource.Spec.Registry.Scheme != registrySpec.Scheme.Default {
		err = InjectRegistryToModuleValues(filepath.Join(downloadedModulesDir, moduleName, moduleVersion), moduleSource)
	}

	return err
}

// Inject registry to module values

func InjectRegistryToModuleValues(moduleVersionPath string, moduleSource *v1alpha1.ModuleSource) error {
	valuesFile := filepath.Join(moduleVersionPath, "openapi", "values.yaml")

	valuesData, err := os.ReadFile(valuesFile)
	if err != nil {
		return err
	}

	valuesData, err = mutateOpenapiSchema(valuesData, moduleSource)
	if err != nil {
		return err
	}

	return os.WriteFile(valuesFile, valuesData, 0o666)
}

func mutateOpenapiSchema(sourceValuesData []byte, moduleSource *v1alpha1.ModuleSource) ([]byte, error) {
	reg := new(registrySchemaForValues)
	reg.SetBase(moduleSource.Spec.Registry.Repo)
	reg.SetDockerCfg(moduleSource.Spec.Registry.DockerCFG)
	reg.SetScheme(moduleSource.Spec.Registry.Scheme)
	reg.SetCA(moduleSource.Spec.Registry.CA)

	var yamlData injectedValues

	err := yaml.Unmarshal(sourceValuesData, &yamlData)
	if err != nil {
		return nil, err
	}

	yamlData.Properties.Registry = reg

	buf := bytes.NewBuffer(nil)

	yamlEncoder := yaml.NewEncoder(buf)
	yamlEncoder.SetIndent(2)
	err = yamlEncoder.Encode(yamlData)

	return buf.Bytes(), err
}

// part of openapi schema for injecting registry values
type registrySchemaForValues struct {
	Type       string   `yaml:"type"`
	Default    struct{} `yaml:"default"`
	Properties struct {
		Base struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default"`
		} `yaml:"base"`
		DockerCfg struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default,omitempty"`
		} `yaml:"dockercfg"`
		Scheme struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default"`
		} `yaml:"scheme"`
		CA struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default,omitempty"`
		} `yaml:"ca"`
	} `yaml:"properties"`
}

func (rsv *registrySchemaForValues) fillTypes() {
	rsv.Properties.Base.Type = "string"
	rsv.Properties.DockerCfg.Type = "string"
	rsv.Properties.Scheme.Type = "string"
	rsv.Properties.CA.Type = "string"
	rsv.Type = "object"
}

func (rsv *registrySchemaForValues) SetBase(registryBase string) {
	rsv.fillTypes()

	rsv.Properties.Base.Default = registryBase
}

func (rsv *registrySchemaForValues) SetDockerCfg(dockerCfg string) {
	if len(dockerCfg) == 0 {
		return
	}

	rsv.Properties.DockerCfg.Default = dockerCfg
}

func (rsv *registrySchemaForValues) SetScheme(scheme string) {
	rsv.Properties.Scheme.Default = scheme
}

func (rsv *registrySchemaForValues) SetCA(ca string) {
	rsv.Properties.CA.Default = ca
}

type injectedValues struct {
	Type       string                 `json:"type" yaml:"type"`
	Xextend    map[string]interface{} `json:"x-extend" yaml:"x-extend"`
	Properties struct {
		YYY      map[string]interface{}   `json:",inline" yaml:",inline"`
		Registry *registrySchemaForValues `json:"registry" yaml:"registry"`
	} `json:"properties" yaml:"properties"`
	XXX map[string]interface{} `json:",inline" yaml:",inline"`
}

func FindExistingSymlink(rootPath, moduleName string) (string, error) {
	var symlinkPath string

	moduleRegexp := regexp.MustCompile(`^(([0-9]+)-)?(` + moduleName + `)$`)
	walkDir := func(path string, d os.DirEntry, _ error) error {
		if !moduleRegexp.MatchString(d.Name()) {
			return nil
		}

		symlinkPath = path
		return filepath.SkipDir
	}

	err := filepath.WalkDir(rootPath, walkDir)

	return symlinkPath, err
}

func Enable(downloadedModulesDir, oldSymlinkPath, newSymlinkPath, modulePath string) error {
	if oldSymlinkPath != "" {
		if _, err := os.Lstat(oldSymlinkPath); err == nil {
			err = os.Remove(oldSymlinkPath)
			if err != nil {
				return err
			}
		}
	}

	if _, err := os.Lstat(newSymlinkPath); err == nil {
		err = os.Remove(newSymlinkPath)
		if err != nil {
			return err
		}
	}

	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(downloadedModulesDir, strings.TrimPrefix(modulePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(modulePath, newSymlinkPath)
}

// WipeSymlinks checks if there are symlinks for the module with different weight in the symlink folder
func WipeSymlinks(symlinksDir, moduleName string) error {
	// delete all module's symlinks in a loop
	for {
		anotherModuleSymlink, err := FindExistingSymlink(symlinksDir, moduleName)
		if err != nil {
			return fmt.Errorf("couldn't check if there are any other symlinks for module %v: %w", moduleName, err)
		}

		if len(anotherModuleSymlink) > 0 {
			if err := os.Remove(anotherModuleSymlink); err != nil {
				return fmt.Errorf("couldn't delete stale symlink %v for module %v: %w", anotherModuleSymlink, moduleName, err)
			}
			// go for another spin
			continue
		}

		// no more symlinks found
		break
	}
	return nil
}

func RestoreSymlink(downloadedModulesDir, symlinkPath, moduleRelativePath string) error {
	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(downloadedModulesDir, strings.TrimPrefix(moduleRelativePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(moduleRelativePath, symlinkPath)
}
