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

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	DefaultBundle   = "Default"
	DefaultLogLevel = "Info"
)

type DeckhouseInstaller struct {
	Registry                 registry.Config
	LogLevel                 string
	Bundle                   string
	DevBranch                string
	UUID                     string
	KubeDNSAddress           string
	ClusterConfig            []byte
	ProviderClusterConfig    []byte
	StaticClusterConfig      []byte
	InfrastructureState      []byte
	NodesInfrastructureState map[string][]byte
	CloudDiscovery           []byte
	ModuleConfigs            []*ModuleConfig

	KubeadmBootstrap   bool
	MasterNodeSelector bool

	InstallerVersion string

	CommanderMode bool
	CommanderUUID uuid.UUID
}

func (c *DeckhouseInstaller) GetImageTag(forceVersionTag bool) string {
	if tag, ok := os.LookupEnv("DHCTL_TEST_VERSION_TAG"); ok {
		return tag
	}
	tag := c.DevBranch
	if forceVersionTag {
		versionTag, foundValidTag := ReadVersionTagFromInstallerContainer()
		if foundValidTag {
			tag = versionTag
		}
	}

	if tag == "" {
		panic("You are probably using a development image. please use devBranch")
	}
	return tag
}

func (c *DeckhouseInstaller) GetImage(forceVersionTag bool) string {
	tag := c.GetImageTag(forceVersionTag)
	return fmt.Sprintf("%s:%s", c.Registry.InClusterImagesRepo(), tag)
}

func ReadVersionTagFromInstallerContainer() (string, bool) {
	rawFile, err := os.ReadFile(app.VersionFile)
	if err != nil {
		log.WarnF(
			"Could not read %s: %v\nWill fall back to installation from release channel or dev branch.",
			app.VersionFile, err,
		)
		return "", false
	}

	tag := strings.TrimSpace(string(rawFile))
	if _, err = semver.NewVersion(strings.TrimPrefix(tag, "v")); err != nil {
		return "", false
	}

	return tag, true
}

func PrepareDeckhouseInstallConfig(metaConfig *MetaConfig) (*DeckhouseInstaller, error) {
	if metaConfig == nil {
		return nil, fmt.Errorf("Internal error. Metaconfig is nil")
	}

	if len(metaConfig.DeckhouseConfig.ConfigOverrides) > 0 {
		return nil, fmt.Errorf("Support for 'configOverrides' was removed. Please use ModuleConfig's instead.")
	}

	if metaConfig.DeckhouseConfig.ReleaseChannel != "" {
		return nil, fmt.Errorf("Support for 'releaseChannel' was removed. Please use 'deckhouse' ModuleConfig's settings instead.")
	}

	if metaConfig.DeckhouseConfig.Bundle != "" {
		return nil, fmt.Errorf("Support for 'bundle' in InitConfiguration was removed. Please use 'deckhouse' ModuleConfig's settings instead.")
	}

	if metaConfig.DeckhouseConfig.LogLevel != "" {
		return nil, fmt.Errorf("Support for 'logLevel' in InitConfiguration was removed. Please use 'deckhouse' ModuleConfig's settings instead.")
	}

	clusterConfig, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Marshal cluster config failed: %v", err)
	}

	providerClusterConfig, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Marshal provider config failed: %v", err)
	}

	staticClusterConfig, err := metaConfig.StaticClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Marshal static config failed: %v", err)
	}

	bundle := DefaultBundle
	logLevel := DefaultLogLevel
	hasRegistrySettings, registrySettings, err := metaConfig.Registry.DeckhouseSettings()
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare registry settings for ModuleConfig deckhouse: %w", err)
	}

	schemasStore := NewSchemaStore()

	var deckhouseCm *ModuleConfig
	// find deckhouse module config for extract release
	for _, mc := range metaConfig.ModuleConfigs {
		if mc.GetName() != "deckhouse" {
			continue
		}

		deckhouseCm = mc

		logLevelRaw, ok := mc.Spec.Settings["logLevel"]
		if ok {
			logLevel = logLevelRaw.(string)
		}
		bundleRaw, ok := mc.Spec.Settings["bundle"]
		if ok {
			bundle = bundleRaw.(string)
		}
		if hasRegistrySettings {
			mc.Spec.Settings["registry"] = registrySettings
		} else {
			delete(mc.Spec.Settings, "registry")
		}
	}

	if deckhouseCm == nil {
		settings := map[string]any{
			"bundle":   bundle,
			"logLevel": logLevel,
		}
		if hasRegistrySettings {
			settings["registry"] = registrySettings
		}
		deckhouseCm, err = buildModuleConfig(schemasStore, "deckhouse", true, settings)
		if err != nil {
			return nil, fmt.Errorf("Cannot create ModuleConfig deckhouse: %s", err)
		}
		metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, deckhouseCm)
	}

	installConfig := DeckhouseInstaller{
		UUID:                  metaConfig.UUID,
		Registry:              metaConfig.Registry,
		DevBranch:             metaConfig.DeckhouseConfig.DevBranch,
		Bundle:                bundle,
		LogLevel:              logLevel,
		KubeDNSAddress:        metaConfig.ClusterDNSAddress,
		ProviderClusterConfig: providerClusterConfig,
		StaticClusterConfig:   staticClusterConfig,
		ClusterConfig:         clusterConfig,
		ModuleConfigs:         metaConfig.ModuleConfigs,
		InstallerVersion:      metaConfig.InstallerVersion,
	}

	return &installConfig, nil
}
