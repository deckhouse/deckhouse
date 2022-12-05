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

	kcm "github.com/flant/addon-operator/pkg/kube_config_manager"
	"github.com/flant/addon-operator/pkg/utils"
	"sigs.k8s.io/yaml"

	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

type Transformer struct {
	// Possible names for ModuleConfig objects (known modules + "global").
	possibleNames set.Set
}

func NewTransformer(possibleNames set.Set) *Transformer {
	return &Transformer{
		possibleNames: possibleNames,
	}
}

// ModuleConfigListToConfigMap creates new Data for ConfigMap from existing ModuleConfig objects.
// It creates module sections for known modules using a cached set of possible names.
func (t *Transformer) ModuleConfigListToConfigMap(allConfigs []*d8cfg_v1alpha1.ModuleConfig) (map[string]string, error) {
	data := make(map[string]string)

	// Note: possibleNames are kebab-cased, cfg.Name should also be kebab-cased.
	for _, cfg := range allConfigs {
		name := cfg.GetName()

		// Ignore unknown module names.
		if !t.possibleNames.Has(name) {
			continue
		}

		// Put module section to ConfigMap if ModuleConfig object has at least one field in values.
		valuesKey := utils.ModuleNameToValuesKey(name)
		cfgValues := cfg.Spec.Settings
		if len(cfgValues) > 0 {
			sectionBytes, err := yaml.Marshal(cfg.Spec.Settings)
			if err != nil {
				return nil, err
			}
			data[valuesKey] = string(sectionBytes)
		}

		// Prevent useless 'globalEnabled' key.
		if name == "global" {
			continue
		}

		// Put '*Enabled' key if 'enabled' field is present in the ModuleConfig resource.
		if cfg.Spec.Enabled != nil {
			enabledKey := valuesKey + "Enabled"
			data[enabledKey] = strconv.FormatBool(*cfg.Spec.Enabled)
		}
	}

	return data, nil
}

// ConfigMapToModuleConfigList returns a list of ModuleConfig objects.
// It transforms 'global' section and all modules sections in ConfigMap/deckhouse.
// Conversion chain is triggered for each section to convert values to the latest
// version. If module has no conversions, 'version: 1' is used.
// It ignores sections with unknown names.
func (t *Transformer) ConfigMapToModuleConfigList(cmData map[string]string) ([]*d8cfg_v1alpha1.ModuleConfig, []string, error) {
	// Messages to log.
	msgs := make([]string, 0)

	// Use ConfigMap parser from addon-operator.
	cfg, err := kcm.ParseConfigMapData(cmData)
	if err != nil {
		return nil, msgs, fmt.Errorf("parse cm/deckhouse data: %v", err)
	}

	// Construct list of sections from *KubeConfig objects.
	sections := kubeConfigToConfigMapSections(cfg)

	// Transform ConfigMap sections to ModuleConfig objects.
	cfgList := make([]*d8cfg_v1alpha1.ModuleConfig, 0)
	for _, section := range sections {
		// Note: possibleNames items and modCfg.ModuleName keys are kebab-cased, modCfg.ModuleConfigKey is camelCased.
		// Ignore unknown module names.
		if !t.possibleNames.Has(section.name) {
			msgs = append(msgs, fmt.Sprintf("migrate '%s': module unknown, ignore", section.name))
			continue
		}

		cfg, msg, err := section.getModuleConfig()
		if err != nil {
			return nil, nil, err
		}
		msgs = append(msgs, msg)
		if cfg != nil {
			cfgList = append(cfgList, cfg)
		}
	}

	return cfgList, msgs, nil
}
