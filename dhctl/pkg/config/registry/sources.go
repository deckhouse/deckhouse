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

package registry

import (
	"encoding/json"

	"sigs.k8s.io/yaml"

	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/initconfig"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

// deckhouseMC represents the relevant subset of a DeckhouseModuleConfig resource:
//
//	apiVersion: deckhouse.io/v1alpha1
//	kind: ModuleConfig
//	metadata:
//	  name: deckhouse
//	spec:
//	  settings:
//	    registry:
//	      mode: Unmanaged
//	      unmanaged:
//	        imagesRepo: ...
//	        scheme: ...
//	        ca: ...
//	        username: ...
//	        password: ...
type deckhouseMC struct {
	Spec struct {
		Settings struct {
			Registry *module_config.DeckhouseSettings `json:"registry" yaml:"registry"`
		} `json:"settings" yaml:"settings"`
	} `json:"spec" yaml:"spec"`
}

// initConfiguration represents the relevant subset of an InitConfiguration resource:
//
//	apiVersion: deckhouse.io/v1
//	kind: InitConfiguration
//	deckhouse:
//	  imagesRepo: ...
//	  registryDockerCfg: ...
//	  registryScheme: ...
//	  registryCA: ...
type initConfiguration struct {
	Deckhouse *init_config.Config `json:"deckhouse" yaml:"deckhouse"`
}

// ParsJSONInitConfig parses an InitConfiguration from JSON and returns its registry config.
// Returns nil if the input is empty or contains no registry fields.
func ParsJSONInitConfig(rawJSON []byte) (*init_config.Config, error) {
	if len(rawJSON) == 0 {
		return nil, nil
	}
	var config initConfiguration
	if err := json.Unmarshal(rawJSON, &config); err != nil {
		return nil, err
	}

	deckhouse := config.Deckhouse
	if deckhouse == nil || deckhouse.IsEmpty() {
		return nil, nil
	}
	return deckhouse, nil
}

// ParsYAMLInitConfig parses an InitConfiguration from YAML and returns its registry config.
// Returns nil if the input is empty or contains no registry fields.
func ParsYAMLInitConfig(rawYAML []byte) (*init_config.Config, error) {
	if len(rawYAML) == 0 {
		return nil, nil
	}
	var config initConfiguration
	if err := yaml.Unmarshal(rawYAML, &config); err != nil {
		return nil, err
	}

	deckhouse := config.Deckhouse
	if deckhouse == nil || deckhouse.IsEmpty() {
		return nil, nil
	}
	return deckhouse, nil
}

// ParsJSONDeckhouseMC parses a DeckhouseModuleConfig from JSON and returns its registry settings.
// Returns nil if the input is empty or the registry section is absent.
func ParsJSONDeckhouseMC(rawJSON []byte) (*module_config.DeckhouseSettings, error) {
	if len(rawJSON) == 0 {
		return nil, nil
	}
	var v deckhouseMC
	if err := json.Unmarshal(rawJSON, &v); err != nil {
		return nil, err
	}
	return v.Spec.Settings.Registry, nil
}

// ParsYAMLDeckhouseMC parses a DeckhouseModuleConfig from YAML and returns its registry settings.
// Returns nil if the input is empty or the registry section is absent.
func ParsYAMLDeckhouseMC(rawYAML []byte) (*module_config.DeckhouseSettings, error) {
	if len(rawYAML) == 0 {
		return nil, nil
	}
	var v deckhouseMC
	if err := yaml.Unmarshal(rawYAML, &v); err != nil {
		return nil, err
	}
	return v.Spec.Settings.Registry, nil
}
