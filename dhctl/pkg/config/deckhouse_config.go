// Copyright 2026 Flant JSC
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

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/uuid"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
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
	ProviderName             string
	ModuleConfigs            []*ModuleConfig

	// ModuleConfigCRDPath is the path to the ModuleConfig CRD manifest shipped
	// in the installer image (or downloaded candi image). Empty means the file
	// is unavailable and the CRD will be installed by deckhouse-controller.
	ModuleConfigCRDPath string

	KubeadmBootstrap   bool
	MasterNodeSelector bool

	InstallerVersion string

	// VersionFilePath is the absolute path to the deckhouse version file
	// embedded in the installer image. DownloadDir is the directory where
	// the deckhouse image is unpacked (a fallback location for the version
	// file). Both are required for GetImageTag(forceVersionTag=true).
	VersionFilePath string
	DownloadDir     string

	CommanderMode bool
	CommanderUUID uuid.UUID
}

// HasProviderModuleConfig reports whether the installer carries a
// cloud-provider-<name> ModuleConfig (the mc-flow provider format). Mirrors
// MetaConfig.HasProviderModuleConfig.
func (c *DeckhouseInstaller) HasProviderModuleConfig() bool {
	if c == nil || c.ProviderName == "" {
		return false
	}
	target := CloudProviderModuleName(c.ProviderName)
	for _, mc := range c.ModuleConfigs {
		if mc.Name == target {
			return true
		}
	}
	return false
}

// HasLegacyProviderConfig reports whether the installer carries a non-empty
// d8-provider-cluster-configuration payload (the legacy provider format).
// Mirrors MetaConfig.HasLegacyProviderConfig.
func (c *DeckhouseInstaller) HasLegacyProviderConfig() bool {
	return c != nil && len(c.ProviderClusterConfig) > 0
}

func (c *DeckhouseInstaller) GetImageTag(ctx context.Context, forceVersionTag bool) (string, error) {
	if tag, ok := os.LookupEnv("DHCTL_TEST_VERSION_TAG"); ok {
		return tag, nil
	}

	tag := c.DevBranch
	if forceVersionTag {
		versionTag, foundValidTag := ReadVersionTagFromInstallerContainer(ctx, c.VersionFilePath, c.DownloadDir)
		if foundValidTag {
			tag = versionTag
		}
	}

	if tag == "" {
		return "", fmt.Errorf("cannot determine Deckhouse image tag: you are probably using a development image, please set devBranch")
	}
	return tag, nil
}

func (c *DeckhouseInstaller) GetInclusterImage(ctx context.Context, forceVersionTag bool) (string, error) {
	tag, err := c.GetImageTag(ctx, forceVersionTag)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", c.Registry.Settings.ToModel().InClusterImagesRepo, tag), nil
}

func (c *DeckhouseInstaller) GetRemoteImage(ctx context.Context, forceVersionTag bool) (string, error) {
	tag, err := c.GetImageTag(ctx, forceVersionTag)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", c.Registry.Settings.ToModel().RemoteImagesRepo, tag), nil
}

// ReadVersionTagFromInstallerContainer reads the installer image version tag.
// versionFile is the absolute path to the embedded version file; downloadDir
// is the directory where the deckhouse image is unpacked (used as a fallback
// location for the version file).
func ReadVersionTagFromInstallerContainer(ctx context.Context, versionFile, downloadDir string) (string, bool) {
	rawFile, err := os.ReadFile(versionFile)
	if err != nil {
		rawFile, err = os.ReadFile(filepath.Join(downloadDir, "deckhouse", "version"))
		if err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, strings.TrimRight(fmt.Sprintf(
				"Could not read %s: %v\nWill fall back to installation from release channel or dev branch.",
				versionFile, err,
			), "\n"))
			return "", false
		}
	}

	tag := strings.TrimSpace(string(rawFile))
	if _, err = semver.NewVersion(strings.TrimPrefix(tag, "v")); err != nil {
		return "", false
	}

	return tag, true
}

func PrepareDeckhouseInstallConfig(ctx context.Context, metaConfig *MetaConfig, globalOptions *options.GlobalOptions) (*DeckhouseInstaller, error) {
	_, span := telemetry.StartSpan(ctx, "PrepareDeckhouseInstallConfig")
	defer span.End()

	if metaConfig == nil {
		return nil, fmt.Errorf("Internal error. Metaconfig is nil")
	}

	if len(metaConfig.DeckhouseConfig.ConfigOverrides) > 0 {
		return nil, fmt.Errorf("Support for 'configOverrides' was removed. Please use ModuleConfig instead.")
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
		return nil, fmt.Errorf("Failed to marshal cluster config: %v", err)
	}

	providerClusterConfig, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal provider config: %v", err)
	}

	staticClusterConfig, err := metaConfig.StaticClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal static config: %v", err)
	}

	bundle := DefaultBundle
	logLevel := DefaultLogLevel
	registry := metaConfig.
		Registry.
		DeckhouseSettings.
		ToMap()

	schemasStore := NewSchemaStore(globalOptions)

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
		if !metaConfig.Registry.LegacyMode {
			mc.Spec.Settings["registry"] = registry
		}
	}

	if deckhouseCm == nil {
		settings := map[string]any{
			"bundle":   bundle,
			"logLevel": logLevel,
		}
		if !metaConfig.Registry.LegacyMode {
			settings["registry"] = registry
		}
		deckhouseCm, err = buildModuleConfig(schemasStore, "deckhouse", true, settings)
		if err != nil {
			return nil, fmt.Errorf("Cannot create ModuleConfig deckhouse: %s", err)
		}
		metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, deckhouseCm)
	}

	moduleConfigCRDPath := ""
	if globalOptions != nil {
		moduleConfigCRDPath = globalOptions.ModuleConfigCRDPath
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
		ProviderName:          metaConfig.ProviderName,
		ModuleConfigs:         metaConfig.ModuleConfigs,
		ModuleConfigCRDPath:   moduleConfigCRDPath,
		InstallerVersion:      metaConfig.InstallerVersion,
		VersionFilePath:       metaConfig.VersionFilePath,
		DownloadDir:           metaConfig.DownloadRootDir,
	}

	return &installConfig, nil
}
