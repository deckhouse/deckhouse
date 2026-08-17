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
	"fmt"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	init_config "github.com/deckhouse/deckhouse/go_lib/registry/models/initconfig"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

func errUnsupportedCRI(cri constant.CRIType) error {
	return fmt.Errorf(
		"registry module cannot be started with defaultCRI '%s'. "+
			"Please either configure registry in 'initConfiguration.deckhouse', "+
			"or use a supported defaultCRI type with the existing configuration in "+
			"'moduleConfig/deckhouse.spec.settings.registry'. Supported CRI types: %v",
		cri,
		constant.SupportedCRI,
	)
}

func errNonStaticClusterMode(mode constant.ModeType) error {
	return fmt.Errorf(
		"bootstrap with registry mode '%s' is supported only in a static cluster. "+
			"Please use one of the supported bootstrap modes for a non-static cluster: %v",
		mode,
		[]constant.ModeType{
			constant.ModeUnmanaged,
			constant.ModeDirect,
		},
	)
}

func NewConfigProvider(
	init *init_config.Config,
	deckhouseSettings *module_config.DeckhouseSettings,
	opts ...ProviderOption,
) *ConfigProvider {
	provider := &ConfigProvider{
		initConfig:        init,
		deckhouseSettings: deckhouseSettings,
	}
	for _, opt := range opts {
		opt(provider)
	}

	return provider
}

type ConfigProvider struct {
	initConfig        *init_config.Config
	deckhouseSettings *module_config.DeckhouseSettings

	// bundleBootstrap records that Local was decided from the registry module's own configuration
	// rather than read from the deckhouse ModuleConfig.
	//
	// The distinction earns its keep in exactly one place — the static-cluster refusal in Config
	// below. That refusal guards the legacy modes, where it was a deliberate restriction; this path
	// is allowed in a cloud cluster, because what serves the images is a static pod on the host
	// network and neither of those depends on the cluster being static.
	bundleBootstrap bool

	// storeExpected records that the registry module was asked for a store of its own.
	storeExpected bool

	// agentOwnsRuntime records that the first master is installed with the node agent on it.
	// See ModeModel.AgentOwnsRuntime for why that is what makes the rest of the nodes possible.
	agentOwnsRuntime bool
}

// ProviderOption adjusts a ConfigProvider at construction.
type ProviderOption func(*ConfigProvider)

// WithBundleBootstrap marks this installation as taking its images from a bundle.
//
// Everything downstream then behaves as it does for the legacy Local mode, which is the point: the
// candi schemas, the provider plugins and the module images all come from the same local registry
// reached through the reverse tunnel, and none of that code needs to know how the mode was decided.
func WithBundleBootstrap() ProviderOption {
	return func(p *ConfigProvider) { p.bundleBootstrap = true }
}

// WithStore marks that the registry module was asked to run a store in the cluster.
//
// What it decides is what the installer waits for at the end. Until the rewrite, that wait read the
// previous implementation's state secret — and that secret is now never written on a cluster the
// current implementation owns, which is every cluster installed from now on. So the wait had to be
// told, from the configuration, whether there is a store to wait for at all: with one, the store's
// own status is the answer; without one, there is nothing in the cluster that reports on a registry
// and the installation is finished when Deckhouse is.
//
// Read from the registry ModuleConfig rather than inferred from the mode, because the mode does not
// carry it: `Managed` with an upstream and no cache is a cluster whose pull path the module owns and
// which has no store in it.
func WithStore() ProviderOption {
	return func(p *ConfigProvider) { p.storeExpected = true }
}

// WithAgent installs the first master with the node agent already on it.
//
// Passed for every installation the registry module owns, because the alternative is a cluster that
// cannot install an agent at all: the package it comes from is fetched through
// registry-packages-proxy, and that proxy reaches the registry through the agent. The installer is
// outside that circle — its own proxy serves packages over the dhctl tunnel from the upstream the
// configuration names — so this is the one moment at which the first agent can be delivered.
//
// After it, the proxy on that master serves every other node through the agent beside it, which is
// how a worker joins without ever touching the upstream itself.
func WithAgent() ProviderOption {
	return func(p *ConfigProvider) { p.agentOwnsRuntime = true }
}

// IsLocal returns true when the bootstrap registry mode is Local.
// It is used only for preliminary registry information retrieval.
// When both initConfig and deckhouseSettings are provided, deckhouse
// ModuleConfig takes precedence over initConfiguration.
func (p *ConfigProvider) IsLocal() (bool, error) {
	if p.deckhouseSettings != nil {
		return p.deckhouseSettings.Mode == constant.ModeLocal, nil
	}
	return false, nil
}

// RemoteData returns the remote registry Data derived from the provided configuration.
// It is used only for preliminary registry information retrieval.
func (p *ConfigProvider) RemoteData() (Data, error) {
	var config Config

	switch {
	case p.deckhouseSettings != nil:
		if err := config.useDeckhouseSettings(*p.deckhouseSettings); err != nil {
			return Data{}, fmt.Errorf("get registry settings from 'moduleConfig/deckhouse': %w", err)
		}

	case p.initConfig != nil:
		if err := config.useInitConfig(*p.initConfig); err != nil {
			return Data{}, fmt.Errorf("get registry settings from 'initConfiguration': %w", err)
		}

	default:
		// criSupported=false selects legacy Unmanaged mode with default registry parameters.
		if err := config.useDefault(false); err != nil {
			return Data{}, fmt.Errorf("get default registry settings: %w", err)
		}
	}
	return config.Settings.RemoteData, nil
}

// Config builds a full registry Config from the provided configuration sources.
func (p *ConfigProvider) Config(defaultCRI constant.CRIType, isStatic bool) (Config, error) {
	var config Config

	criSupported := constant.IsCRISupported(defaultCRI)

	switch {
	case p.deckhouseSettings != nil:
		if !criSupported {
			return Config{}, errUnsupportedCRI(defaultCRI)
		}

		switch p.deckhouseSettings.Mode {
		case constant.ModeProxy, constant.ModeLocal:
			// The restriction belongs to the legacy modes, and it stays exactly as it was for them.
			// An installation from a bundle is allowed in a cloud cluster: what serves the images
			// during it is a static pod on the host network, which the cloud has no say in, and the
			// cluster's own store serves everything afterwards — already measured on the air-gapped
			// cloud variants of the test matrix.
			if !isStatic && !p.bundleBootstrap {
				return Config{}, errNonStaticClusterMode(p.deckhouseSettings.Mode)
			}
		}

		if err := config.useDeckhouseSettings(*p.deckhouseSettings); err != nil {
			return Config{}, fmt.Errorf("get registry settings from 'moduleConfig/deckhouse': %w", err)
		}

		// After useDeckhouseSettings, which replaces the whole struct.
		config.BundleBootstrap = p.bundleBootstrap
		config.StoreExpected = p.storeExpected
		config.AgentOwnsRuntime = p.agentOwnsRuntime

	case p.initConfig != nil:
		if err := config.useInitConfig(*p.initConfig); err != nil {
			return Config{}, fmt.Errorf("get registry settings from 'initConfiguration': %w", err)
		}

	default:
		if err := config.useDefault(criSupported); err != nil {
			return Config{}, fmt.Errorf("get default registry settings: %w", err)
		}
	}

	return config, nil
}

type Config struct {
	Settings          ModeSettings
	DeckhouseSettings module_config.DeckhouseSettings
	LegacyMode        bool

	// BundleBootstrap records that the images come from a bundle, which is a fact about this
	// installation that the mode alone does not carry: on the node the mode is Local, exactly as it is
	// for a legacy Local cluster, while what owns the registry afterwards is a different
	// implementation entirely.
	BundleBootstrap bool

	// StoreExpected records that the registry module runs a store in this cluster, and so that the
	// installation is not finished until that store reports itself ready. See WithStore.
	StoreExpected bool

	// AgentOwnsRuntime records that the first master comes up with the node agent on it, installed
	// from the installer's own packages proxy. See WithAgent.
	AgentOwnsRuntime bool
}

// useDefault configures the registry with default CE settings.
// When no registry configuration is provided:
// - If Direct mode is supported, uses Direct mode
// - Otherwise, falls back to Unmanaged mode
// - All parameters are populated with default values for the CE registry
func (c *Config) useDefault(criSupported bool) error {
	var settings module_config.DeckhouseSettings

	if criSupported {
		settings = module_config.New(constant.ModeDirect)
	} else {
		settings = module_config.New(constant.ModeUnmanaged)
	}
	return c.Process(settings, !criSupported)
}

// useInitConfig configures registry using legacy initConfiguration.
// Note: This method maintains backward compatibility and only supports Unmanaged legacy mode.
func (c *Config) useInitConfig(userConfig init_config.Config) error {
	// Prepare config
	initConfig := init_config.
		New().
		Merge(&userConfig)

	// Convert to registry settings
	registrySettings, err := initConfig.ToRegistrySettings()
	if err != nil {
		return fmt.Errorf("get registry settings: %w", err)
	}

	settings := module_config.
		New(constant.ModeUnmanaged).
		Merge(&module_config.DeckhouseSettings{
			Mode:      constant.ModeUnmanaged,
			Unmanaged: &registrySettings,
		})
	return c.Process(settings, true)
}

// useDeckhouseSettings configures registry using deckhouse ModuleConfig settings.
// The operation mode (Direct/Unmanaged) is determined from the user configuration.
func (c *Config) useDeckhouseSettings(userSettings module_config.DeckhouseSettings) error {
	settings := module_config.
		New(userSettings.Mode).
		Merge(&userSettings)
	return c.Process(settings, false)
}

func (c *Config) Process(deckhouseSettings module_config.DeckhouseSettings, legacyMode bool) error {
	// Validate
	if err := deckhouseSettings.Validate(); err != nil {
		return fmt.Errorf("validate registry settings: %w", err)
	}

	// This error checks whether the registry can be started in legacy mode.
	// The error is needed to check the tests of the UseInitConfig and UseDefault methods.
	if legacyMode && constant.ModuleRequired(deckhouseSettings.Mode) {
		return fmt.Errorf(
			"internal error: cannot run registry in legacy mode with registry mode: '%s'.",
			deckhouseSettings.Mode,
		)
	}

	// Prepare mode settings
	modeSettings, err := newModeSettings(deckhouseSettings)
	if err != nil {
		return fmt.Errorf("get registry mode settings: %w", err)
	}

	*c = Config{
		Settings:          modeSettings,
		DeckhouseSettings: deckhouseSettings,
		LegacyMode:        legacyMode,
	}

	return nil
}

func (c *Config) IsLocal() bool {
	return c.Settings.Mode == constant.ModeLocal
}

// Manifest creates a ManifestBuilder instance for generating configuration manifests.
func (c *Config) Manifest() *ManifestBuilder {
	model := c.Settings.ToModel()
	model.AgentOwnsRuntime = c.AgentOwnsRuntime
	return newManifestBuilder(model, c.LegacyMode, c.BundleBootstrap)
}

// DeepCopyInto copies the receiver into out.
func (c *Config) DeepCopyInto(out *Config) {
	*out = *c
	c.Settings.DeepCopyInto(&out.Settings)
	c.DeckhouseSettings.DeepCopyInto(&out.DeckhouseSettings)
}

// DeepCopy returns a deep copy of the receiver.
func (c *Config) DeepCopy() *Config {
	if c == nil {
		return nil
	}
	out := new(Config)
	c.DeepCopyInto(out)
	return out
}
