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

package models

const (
	ModuleDefinitionFile = "module.yaml"
)

type DeckhouseModuleDefinition struct {
	Name        string   `yaml:"name"`
	Weight      uint32   `yaml:"weight,omitempty"`
	Tags        []string `yaml:"tags"`
	Stage       string   `yaml:"stage"`
	Description string   `yaml:"description"`

	Path string `yaml:"-"`
}
