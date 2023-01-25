/*
Copyright 2022 Flant JSC

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

package deckhouse_config

import (
	"fmt"
	"strconv"
	"strings"

	kcm "github.com/flant/addon-operator/pkg/kube_config_manager"
	"github.com/flant/addon-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
)

// configMapSection is a holder for one module section from the ConfigMap.
type configMapSection struct {
	name      string
	valuesKey string
	values    utils.Values
	enabled   *bool
}

// getValuesMap returns a values map or nil for module or global section.
func (s *configMapSection) getValuesMap() (map[string]interface{}, error) {
	untypedValues := s.values[s.valuesKey]

	isValidType := true
	switch v := untypedValues.(type) {
	case map[string]interface{}:
		// Module section is not empty, and it is a map.
		return v, nil
	case nil:
		// Module values are nil when ConfigMap has Enabled flag without module section.
		// Transform empty values to the empty 'configValues' field.
		return nil, nil
	case string:
		// Transform empty string to the empty 'configValues' field.
		if v != "" {
			isValidType = false
		}
	case []interface{}:
		// Array is not a valid module section, but it is ok if array is empty, just ignore it.
		if len(v) != 0 {
			isValidType = false
		}
	default:
		// Consider other types are not valid.
		isValidType = false
	}
	if !isValidType {
		return nil, fmt.Errorf("configmap section '%s' is not an object, need map[string]interface{}, got %T:(%+v)", s.valuesKey, untypedValues, untypedValues)
	}
	return nil, nil
}

// convertValues runs conversions on section values.
// Assume that values are from cm/deckhouse, so start conversions from the version 1.
func (s *configMapSection) convertValues() (int, map[string]interface{}, error) {
	// Values without conversion has version 1.
	latestVersion := 1
	latestValues, err := s.getValuesMap()
	if err != nil {
		return 0, nil, err
	}

	chain := conversion.Registry().Chain(s.name)
	latestVersion, latestValues, err = chain.ConvertToLatest(latestVersion, latestValues)
	if err != nil {
		return 0, nil, err
	}

	return latestVersion, latestValues, nil
}

// getModuleConfig constructs ModuleConfig object from ConfigMap's section.
// It converts section values to the latest version of module settings.
func (s *configMapSection) getModuleConfig() (*d8cfg_v1alpha1.ModuleConfig, string, error) {
	// Convert values to the latest schema if conversion chain is present.
	version, values, err := s.convertValues()
	if err != nil {
		return nil, "", err
	}

	cfg := &d8cfg_v1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleConfig",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: s.name,
		},
		Spec: d8cfg_v1alpha1.ModuleConfigSpec{},
	}

	msgs := make([]string, 0)

	if len(s.values) > 0 {
		msgs = append(msgs, "has values")
	} else {
		msgs = append(msgs, "no values")
	}

	if len(values) > 0 {
		msgs = append(msgs, fmt.Sprintf("values converted to version %d", version))
		cfg.Spec.Settings = values
		cfg.Spec.Version = version
	}

	if len(s.values) > 0 && len(values) == 0 {
		msgs = append(msgs, "converted to empty values")
	}

	// Enabled flag is not applicable for global section.
	if s.name != "global" {
		if s.enabled != nil {
			msgs = append(msgs, "has enabled flag")
			cfg.Spec.Enabled = s.enabled
		} else {
			msgs = append(msgs, "no enabled flag")
		}
	}

	if s.enabled == nil && len(values) == 0 {
		cfg = nil
		msgs = append(msgs, "ignore creating empty object")
	}

	msg := fmt.Sprintf("section '%s': %s", s.name, strings.Join(msgs, ", "))

	return cfg, msg, nil
}

// ToConfigMapData convert values to the latest schema and returns a map with 1 or 2 fields:
// camelCased module name - module settings
// enabled flag - if enabled is not nil.
// This method assumes values are from the cm/deckhouse, so schema version is 1.
//
// Example:
//
//	input:
//	  configMapSection{
//	   name: "module-one",
//	   valuesKey: "moduleOne",
//	   values: map[string]interface{}{"param1":"val1"}
//	}
//	output:
//	  map[string]string{
//	    "moduleOne": "param1:\n  val1\n"
//	  }
func (s *configMapSection) getConfigMapData() (map[string]string, error) {
	out := map[string]string{}

	// Convert non-empty values to YAML.
	valuesMap, err := s.getValuesMap()
	if err != nil {
		return nil, err
	}
	if len(valuesMap) > 0 {
		_, values, err := s.convertValues()
		if err != nil {
			return nil, err
		}
		sectionBytes, err := yaml.Marshal(values)
		if err != nil {
			return nil, err
		}
		out[s.valuesKey] = string(sectionBytes)
	}

	if s.enabled != nil {
		out[s.valuesKey+"Enabled"] = strconv.FormatBool(*s.enabled)
	}

	return out, nil
}

func kubeConfigToConfigMapSections(kubeCfg *kcm.KubeConfig) []*configMapSection {
	sections := make([]*configMapSection, 0)

	// Handle "global" key.
	globalSection := &configMapSection{
		name:      "global",
		valuesKey: "global",
		enabled:   nil,
	}
	if kubeCfg.Global != nil {
		globalSection.values = kubeCfg.Global.Values
	}
	sections = append(sections, globalSection)

	// Handle modules.
	for _, modCfg := range kubeCfg.Modules {
		sections = append(sections, &configMapSection{
			name:      modCfg.ModuleName,
			valuesKey: modCfg.ModuleConfigKey,
			values:    modCfg.Values,
			enabled:   modCfg.IsEnabled,
		})
	}
	return sections
}
