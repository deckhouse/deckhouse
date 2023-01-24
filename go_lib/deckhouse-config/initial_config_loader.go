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
	"github.com/flant/kube-client/client"
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	log "github.com/sirupsen/logrus"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	AnnoMigrationInProgress = "deckhouse.io/migration-in-progress"
)

// InitialConfigLoader runs conversions on module sections in the ConfigMap
// or for settings in all ModuleConfig resources to make a KubeConfig
// with values conform to the latest OpenAPI schemas.
// It is used at start to provide a valid config to the AddonOperator instance.
type InitialConfigLoader struct {
	KubeClient client.Client
}

func NewInitialConfigLoader(kubeClient client.Client) *InitialConfigLoader {
	return &InitialConfigLoader{
		kubeClient,
	}
}

// GetInitialKubeConfig runs conversions on settings to feed valid 'KubeConfig'
// to the AddonOperator. Otherwise, Deckhouse will stuck trying to validate old settings
// with new OpenAPI schemas.
//
// There are 2 cases:
//  1. cm/deckhouse is in use or cm/deckhouse-generated-do-no-edit was just copied (it has annotation).
//     There are no ModuleConfig resources, so module sections are treated as settings with version 0.
//  2. cm/deckhouse-generated-do-no-edit has no annotation.
//     ConfigMap has no versions, so ModuleConfig resources are used to create initial config.
//     Also, there is no ModuleManager instance, only module names from the ConfigMap are used.
func (l *InitialConfigLoader) GetInitialKubeConfig(cmName string) (*kcm.KubeConfig, error) {
	if cmName != DeckhouseConfigMapName && cmName != GeneratedConfigMapName {
		return nil, fmt.Errorf("load initial config: unknown ConfigMap/%s", cmName)
	}

	// Init Kubernetes client if it was not specified.
	err := l.initKubeClient()
	if err != nil {
		return nil, fmt.Errorf("init default Kubernetes client: %v", err)
	}

	// Get ConfigMap. Return nil if the ConfigMap is not exists or contains no settings.
	// This situation will be handled later by the 'startup_sync.go' global hook.
	cm, err := GetConfigMap(l.KubeClient, DeckhouseNS, cmName)
	if err != nil {
		if k8errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load cm/%s: %v", cmName, err)
	}

	// Check if the ConfigMap/deckhouse-generated-config-do-not-edit is just a copy of ConfigMap/deckhouse.
	migrationInProgress := false
	if len(cm.GetAnnotations()) > 0 {
		_, migrationInProgress = cm.GetAnnotations()[AnnoMigrationInProgress]
	}

	if cmName == DeckhouseConfigMapName || migrationInProgress {
		return l.LegacyConfigMapToInitialConfig(cm.Data)
	}

	possibleNames := set.New()
	hasValues := false
	for k := range cm.Data {
		valuesKey := strings.TrimSuffix(k, "Enabled")
		if valuesKey == k {
			hasValues = true
		}
		possibleNames.Add(utils.ModuleNameFromValuesKey(valuesKey))
	}

	// No conversions needed if there are no settings in the ConfigMap.
	// KubeConfigManager will be absolutely happy about this situation.
	if !hasValues {
		return nil, nil
	}

	// Create initial config from ModuleConfig resources using module names from the ConfigMap.
	cfgList, err := GetAllConfigs(l.KubeClient)
	if err != nil {
		return nil, fmt.Errorf("load initial config from ModuleConfig resources: %v", err)
	}
	return l.ModuleConfigListToInitialConfig(cfgList, possibleNames)
}

func (l *InitialConfigLoader) initKubeClient() error {
	if l.KubeClient != nil {
		return nil
	}

	// Mute logger to prevent non-formatted message.
	lvl := log.GetLevel()
	log.SetLevel(log.FatalLevel)
	defer func() {
		log.SetLevel(lvl)
	}()

	kubeClient := shell_operator.DefaultMainKubeClient(nil, nil)
	err := kubeClient.Init()
	if err != nil {
		return err
	}
	l.KubeClient = kubeClient
	return nil
}

// ModuleConfigListToInitialConfig runs conversion for ModuleConfig resources to transforms settings and enabled flag to
// the ConfigMap content. Then parse resulting ConfigMap to the KubeConfig.
func (l *InitialConfigLoader) ModuleConfigListToInitialConfig(allConfigs []*d8cfg_v1alpha1.ModuleConfig, possibleNames set.Set) (*kcm.KubeConfig, error) {
	data := make(map[string]string)

	for _, cfg := range allConfigs {
		name := cfg.GetName()

		// No need to convert settings if it is not in the ConfigMap.
		if !possibleNames.Has(name) {
			continue
		}

		valuesKey := utils.ModuleNameToValuesKey(cfg.GetName())

		// Run registered conversions if spec.settings are not empty and
		// put module section to the ConfigMap data.
		if len(cfg.Spec.Settings) > 0 && cfg.Spec.Version > 0 {
			chain := conversion.Registry().Chain(cfg.GetName())

			_, latestSettings, err := chain.ConvertToLatest(cfg.Spec.Version, cfg.Spec.Settings)
			if err != nil {
				if chain.LatestVersion() != cfg.Spec.Version {
					return nil, fmt.Errorf("convert settings in ModuleConfig/%s from version %d to latest version %d: %v", cfg.GetName(), cfg.Spec.Version, chain.LatestVersion(), err)
				}
				return nil, fmt.Errorf("settings in ModuleConfig/%s with latest version %d: %v", cfg.GetName(), cfg.Spec.Version, err)
			}

			sectionBytes, err := yaml.Marshal(latestSettings)
			if err != nil {
				return nil, err
			}
			data[valuesKey] = string(sectionBytes)
		}

		// Prevent useless 'globalEnabled' key.
		if cfg.GetName() == "global" {
			continue
		}

		// Put '*Enabled' key if 'enabled' field is present in the ModuleConfig resource.
		if cfg.Spec.Enabled != nil {
			enabledKey := valuesKey + "Enabled"
			data[enabledKey] = strconv.FormatBool(*cfg.Spec.Enabled)
		}
	}

	return kcm.ParseConfigMapData(data)
}

// LegacyConfigMapToInitialConfig runs registered conversion for each 'module section'
// in the ConfigMap data. It assumes settings have version 0 (cm/deckhouse case).
func (l *InitialConfigLoader) LegacyConfigMapToInitialConfig(cmData map[string]string) (*kcm.KubeConfig, error) {
	// Parse data as KubeConfigManager will do.
	kubeCfg, err := kcm.ParseConfigMapData(cmData)
	if err != nil {
		return nil, fmt.Errorf("parse ConfigMap data: %v", err)
	}

	sections := kubeConfigToConfigMapSections(kubeCfg)
	newData := map[string]string{}
	for _, section := range sections {
		sData, err := section.getConfigMapData()
		if err != nil {
			return nil, fmt.Errorf("transform section '%s': %v", section.name, err)
		}
		for k, v := range sData {
			newData[k] = v
		}
	}

	// Parse new Data to have proper checksums.
	newCfg, err := kcm.ParseConfigMapData(newData)
	if err != nil {
		return nil, fmt.Errorf("prepare initial KubeConfig: %v", err)
	}
	return newCfg, nil
}
