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

package packages

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/apps"
)

const (
	definitionFile = "package.yaml"
)

type Definition struct {
	Name    string `yaml:"name" json:"name"`
	Type    string `yaml:"type" json:"type"`
	Version string `yaml:"version" json:"version"`
	Stage   string `yaml:"stage" json:"stage"`

	Descriptions *Descriptions `json:"descriptions,omitempty" yaml:"descriptions,omitempty"`
	Requirements *Requirements `yaml:"requirements,omitempty" json:"requirements,omitempty"`

	path string
}

type Descriptions struct {
	Ru string `json:"ru,omitempty" yaml:"ru,omitempty"`
	En string `json:"en,omitempty" yaml:"en,omitempty"`
}

type Requirements struct {
	Modules map[string]string `yaml:"modules" json:"modules"`
}

func LoadDefinition(packageDir string) (*Definition, error) {
	path := filepath.Join(packageDir, definitionFile)

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file '%s': %w", path, err)
	}

	def := new(Definition)
	if err = yaml.Unmarshal(content, def); err != nil {
		return nil, fmt.Errorf("unmarshal file '%s': %w", path, err)
	}

	def.path = path

	return def, nil
}

func (d *Definition) ToApplication(name, namespace string) *apps.Application {
	var deps map[string]string
	if d.Requirements != nil {
		deps = d.Requirements.Modules
	}

	// <namespace>-<instance>-<package>
	name = fmt.Sprintf("%s-%s-%s", namespace, name, d.Name)

	return apps.NewApplication(name, namespace, d.Name, d.Version, deps)
}

func (d *Definition) ToClusterApplication() *apps.ClusterApplication {
	var deps map[string]string
	if d.Requirements != nil {
		deps = d.Requirements.Modules
	}

	return apps.NewClusterApplication(d.path, d.Name, d.Version, deps)
}
