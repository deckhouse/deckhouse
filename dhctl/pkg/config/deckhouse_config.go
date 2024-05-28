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
	"github.com/google/uuid"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	initConfigurationError = `%s fields in InitConfiguration are deprecated.
Please use ModuleConfig 'deckhouse' section in configuration. Example:
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
...
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
...
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  settings:
    %s
`
	configOverridesWarn = `
Config overrides are deprecated. Please use module config:
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
...
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
...
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: false
    modules:
      publicDomainTemplate: '%s.example.com'
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-flannel
spec:
  enabled: true
---
...
`

	DefaultBundle   = "Default"
	DefaultLogLevel = "Info"
)

type DeckhouseInstaller struct {
	Registry              RegistryData
	LogLevel              string
	Bundle                string
	DevBranch             string
	UUID                  string
	KubeDNSAddress        string
	ClusterConfig         []byte
	ProviderClusterConfig []byte
	StaticClusterConfig   []byte
	TerraformState        []byte
	NodesTerraformState   map[string][]byte
	CloudDiscovery        []byte
	ModuleConfigs         []*ModuleConfig

	KubeadmBootstrap   bool
	MasterNodeSelector bool

	ReleaseChannel   string
	InstallerVersion string

	CommanderMode bool
	CommanderUUID uuid.UUID
}

func (c *DeckhouseInstaller) GetImage(forceVersionTag bool) string {
	registryNameTemplate := "%s%s:%s"
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

	return fmt.Sprintf(registryNameTemplate, c.Registry.Address, c.Registry.Path, tag)
}

func (c *DeckhouseInstaller) IsRegistryAccessRequired() bool {
	return c.Registry.DockerCfg != ""
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

	releaseChannel := ""

	// todo after release 1.55 remove it and from openapi schema
	deprecatedFields := make([]string, 0, 3)
	deprecatedFieldsExamples := make([]string, 0, 3)
	if metaConfig.DeckhouseConfig.ReleaseChannel != "" {
		releaseChannel = metaConfig.DeckhouseConfig.ReleaseChannel
		deprecatedFields = append(deprecatedFields, "releaseChannel")
		deprecatedFieldsExamples = append(deprecatedFieldsExamples, "releaseChannel: Stable")
	}

	if metaConfig.DeckhouseConfig.Bundle != bundle {
		bundle = metaConfig.DeckhouseConfig.Bundle
		deprecatedFields = append(deprecatedFields, "bundle")
		deprecatedFieldsExamples = append(deprecatedFieldsExamples, "bundle: Default")
	}

	if metaConfig.DeckhouseConfig.LogLevel != logLevel {
		logLevel = metaConfig.DeckhouseConfig.LogLevel
		deprecatedFields = append(deprecatedFields, "logLevel")
		deprecatedFieldsExamples = append(deprecatedFieldsExamples, "logLevel: Info")
	}

	if len(deprecatedFields) > 0 {
		log.WarnF(initConfigurationError, strings.Join(deprecatedFields, ","), strings.Join(deprecatedFieldsExamples, "\n    "))
	}

	schemasStore := NewSchemaStore()

	if len(metaConfig.DeckhouseConfig.ConfigOverrides) > 0 {
		log.WarnLn(configOverridesWarn)
		if len(metaConfig.ModuleConfigs) > 0 {
			return nil, fmt.Errorf("Cannot use ModuleConfig's and configOverrides at the same time. Please use ModuleConfig's")
		}

		mcs, err := ConvertInitConfigurationToModuleConfigs(metaConfig, schemasStore, bundle, logLevel)
		if err != nil {
			return nil, err
		}

		metaConfig.ModuleConfigs = mcs
	}

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
	}

	if deckhouseCm == nil {
		deckhouseCm, err = buildModuleConfigWithOverrides(schemasStore, "deckhouse", true, map[string]any{
			"bundle":   bundle,
			"logLevel": logLevel,
		})
		if err != nil {
			return nil, fmt.Errorf("Cannot create ModuleConfig deckhouse: %s", err)
		}
		metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, deckhouseCm)
	} else {
		releaseChannelRaw, hasReleaseChannelKey := deckhouseCm.Spec.Settings["releaseChannel"]
		if rc, ok := releaseChannelRaw.(string); hasReleaseChannelKey && ok {
			// we need set releaseChannel after bootstrapping process done
			// to prevent update during bootstrap
			delete(deckhouseCm.Spec.Settings, "releaseChannel")
			releaseChannel = rc
		}
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
		ReleaseChannel:        releaseChannel,
		InstallerVersion:      metaConfig.InstallerVersion,
	}

	return &installConfig, nil
}
