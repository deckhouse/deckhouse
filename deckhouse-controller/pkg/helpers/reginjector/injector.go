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

package reginjector

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func InjectRegistryToModuleValues(moduleVersionPath string, moduleSource *v1alpha1.ModuleSource) error {
	valuesFile := path.Join(moduleVersionPath, "openapi", "values.yaml")

	valuesData, err := os.ReadFile(valuesFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(valuesFile), 0o775)
		valuesData = bytes.TrimSpace([]byte("type: object"))
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
	reg.SetDockercfg(moduleSource.Spec.Registry.DockerCFG)
	reg.SetScheme(moduleSource.Spec.Registry.Scheme)
	reg.SetCA(moduleSource.Spec.Registry.CA)

	var yamlData injectedValues

	if err := yaml.Unmarshal(sourceValuesData, &yamlData); err != nil {
		return nil, err
	}

	yamlData.Properties.Registry = reg

	buf := bytes.NewBuffer(nil)

	yamlEncoder := yaml.NewEncoder(buf)
	yamlEncoder.SetIndent(2)
	err := yamlEncoder.Encode(yamlData)

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
		Dockercfg struct {
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
	rsv.Properties.Dockercfg.Type = "string"
	rsv.Properties.Scheme.Type = "string"
	rsv.Properties.CA.Type = "string"
	rsv.Type = "object"
}

func (rsv *registrySchemaForValues) SetBase(registryBase string) {
	rsv.fillTypes()

	rsv.Properties.Base.Default = registryBase
}

func (rsv *registrySchemaForValues) SetDockercfg(dockercfg string) {
	rsv.Properties.Dockercfg.Default = DockerCFGForModules(rsv.Properties.Base.Default, dockercfg)
}

func (rsv *registrySchemaForValues) SetScheme(scheme string) {
	rsv.Properties.Scheme.Default = scheme
}

func (rsv *registrySchemaForValues) SetCA(ca string) {
	rsv.Properties.CA.Default = ca
}

// DockerCFGForModules
// according to the deckhouse docs, for anonymous registry access we must have the value:
// {"auths": { "<PROXY_REGISTRY>": {}}}
// but it could be empty for a ModuleSource.
// modules are not ready to catch empty string, so we have to fill it with the default value
func DockerCFGForModules(repo, dockercfg string) string {
	if len(dockercfg) != 0 {
		return dockercfg
	}

	index := strings.Index(repo, "/")
	var registry string
	if index != -1 {
		registry = repo[:index]
	} else {
		registry = repo
	}

	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"auths": {"%s": {}}}`, registry)))
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
