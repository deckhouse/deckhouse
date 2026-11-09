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
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/require"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

const defaultRegistryClusterConfig = `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.33"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
`

// What a user writes to install CE through the registry module. The default has to be exactly this.
const explicitCERegistryModuleConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Managed
    primary:
      upstream:
        scheme: HTTPS
        host: registry.deckhouse.io
        path: /deckhouse/ce
`

const defaultRegistryWarning = "No registry settings were given: installing Deckhouse CE from registry.deckhouse.io/deckhouse/ce"

func registryModuleConfigs(mcs []*ModuleConfig) []*ModuleConfig {
	var out []*ModuleConfig
	for _, mc := range mcs {
		if mc.GetName() == registryModuleName {
			out = append(out, mc)
		}
	}
	return out
}

// A configuration that names no registry installs CE through the registry module, and the cluster it
// builds is the one the user would have got by writing that ModuleConfig.
func TestDefaultRegistryIsTheRegistryModule(t *testing.T) {
	defaulted, err := ParseConfigFromData(t.Context(), defaultRegistryClusterConfig, DummyValidatorProvider(), nil)
	require.NoError(t, err)

	explicit, err := ParseConfigFromData(t.Context(), defaultRegistryClusterConfig+explicitCERegistryModuleConfig, DummyValidatorProvider(), nil)
	require.NoError(t, err)

	require.True(t, defaulted.RegistryDefaulted())
	require.False(t, explicit.RegistryDefaulted())

	// The ModuleConfig created in the cluster, so that the module manages it after the installation.
	mcs := registryModuleConfigs(defaulted.ModuleConfigs)
	require.Len(t, mcs, 1)
	assertModuleConfig(t, mcs[0], true, 1, defaultRegistrySettings)

	// And what the installer makes of it: the same as of the ModuleConfig written by hand.
	require.Equal(t, explicit.Registry, defaulted.Registry)

	cfg := defaulted.Registry
	require.False(t, cfg.LegacyMode)
	require.True(t, cfg.FromRegistryModule, "nothing goes into the deckhouse ModuleConfig")
	require.True(t, cfg.AgentOwnsRuntime, "the first master comes up with the node agent")
	require.False(t, cfg.StoreExpected, "no cache was asked for")
	require.False(t, cfg.BundleBootstrap)
	require.Equal(t, registry_const.ModeDirect, cfg.Settings.Mode)

	remote := cfg.Settings.RemoteData
	require.Equal(t, registry_const.DefaultImagesRepo, remote.ImagesRepo)
	require.Equal(t, registry_const.SchemeHTTPS, remote.Scheme)
	require.Empty(t, remote.Username)
	require.Empty(t, remote.Password)
	require.Empty(t, remote.CA)
}

func TestDefaultRegistryInstallConfig(t *testing.T) {
	t.Run("no registry settings: the registry ModuleConfig is installed and the default is announced", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(t.Context(), defaultRegistryClusterConfig, DummyValidatorProvider(), nil)
		require.NoError(t, err)

		var logged bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

		installConfig, err := PrepareDeckhouseInstallConfig(ctx, metaConfig, &options.New().Global)
		require.NoError(t, err)

		assertModuleConfig(t, moduleConfigNamed(t, installConfig.ModuleConfigs, "registry"), true, 1, defaultRegistrySettings)
		require.NotContains(t, moduleConfigNamed(t, installConfig.ModuleConfigs, "deckhouse").Spec.Settings, "registry")

		// dhctl deckhouse create-deployment shares PrepareDeckhouseInstallConfig and creates only the
		// Deployment, so the ModuleConfig the warning announces is not created there. The bootstrap
		// says it, after PrepareDeckhouseInstallConfig.
		require.NotContains(t, logged.String(), defaultRegistryWarning)

		WarnAboutDefaultRegistry(ctx, metaConfig)
		require.Contains(t, logged.String(), "level=WARN")
		require.Contains(t, logged.String(), defaultRegistryWarning)
	})

	t.Run("a registry ModuleConfig written by the user: nothing to announce", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(t.Context(), defaultRegistryClusterConfig+explicitCERegistryModuleConfig, DummyValidatorProvider(), nil)
		require.NoError(t, err)

		var logged bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

		installConfig, err := PrepareDeckhouseInstallConfig(ctx, metaConfig, &options.New().Global)
		require.NoError(t, err)

		require.Len(t, registryModuleConfigs(installConfig.ModuleConfigs), 1)
		WarnAboutDefaultRegistry(ctx, metaConfig)
		require.NotContains(t, logged.String(), defaultRegistryWarning)
	})
}

// Every configuration that says something about the registry, or that is not an installation the
// registry module can manage, keeps what it gets today.
func TestDefaultRegistryNotAdded(t *testing.T) {
	const initConfigWithRegistry = `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: r.example.com/deckhouse/ee
  registryScheme: HTTPS
`
	const deckhouseModuleConfigWithRegistry = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: r.example.com/deckhouse/ee
        scheme: HTTPS
`
	const unmanagedRegistryModuleConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Unmanaged
`
	const clusterConfigNotManagedCRI = `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.33"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
defaultCRI: NotManaged
`

	tests := []struct {
		name   string
		config string
		opts   []ValidateOption

		// registryModuleConfigs is how many registry ModuleConfigs the result carries: the user's own.
		registryModuleConfigs int
		mode                  registry_const.ModeType
		legacyMode            bool
	}{
		{
			// A managed cluster such as EKS: no ClusterConfiguration, the legacy Unmanaged CE default.
			name:       "no ClusterConfiguration",
			config:     "",
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
		},
		{
			name:       "a container runtime the module cannot configure",
			config:     clusterConfigNotManagedCRI,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
		},
		{
			name:       "registry fields in InitConfiguration",
			config:     defaultRegistryClusterConfig + initConfigWithRegistry,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
		},
		{
			name:   "a registry section in the deckhouse ModuleConfig",
			config: defaultRegistryClusterConfig + deckhouseModuleConfigWithRegistry,
			mode:   registry_const.ModeUnmanaged,
		},
		{
			// Without the InitConfiguration fields it is refused: see TestRefuseRegistryModuleWithoutRegistry.
			name:                  "an Unmanaged registry ModuleConfig of the user's own with registry fields in InitConfiguration",
			config:                defaultRegistryClusterConfig + initConfigWithRegistry + unmanagedRegistryModuleConfig,
			registryModuleConfigs: 1,
			mode:                  registry_const.ModeUnmanaged,
			legacyMode:            true,
		},
		{
			// A cluster that exists: whatever it was installed with is not the installer's to restate.
			name:   "converge",
			config: defaultRegistryClusterConfig,
			opts:   []ValidateOption{ValidateOptionOperation(string(validatev1.OperationConverge))},
			mode:   registry_const.ModeDirect,
		},
		{
			name:   "destroy",
			config: defaultRegistryClusterConfig,
			opts:   []ValidateOption{ValidateOptionOperation(string(validatev1.OperationDestroy))},
			mode:   registry_const.ModeDirect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), tt.config, DummyValidatorProvider(), nil, tt.opts...)
			require.NoError(t, err)

			require.False(t, metaConfig.RegistryDefaulted())
			require.Len(t, registryModuleConfigs(metaConfig.ModuleConfigs), tt.registryModuleConfigs)
			require.False(t, metaConfig.Registry.FromRegistryModule)
			if tt.mode != "" {
				require.Equal(t, tt.mode, metaConfig.Registry.Settings.Mode)
				require.Equal(t, tt.legacyMode, metaConfig.Registry.LegacyMode)
			}
		})
	}
}

// The node-manager defaultCRI is read before the ClusterConfiguration's. Its schema does not have the
// setting today, so no parsed configuration gets here; the ModuleConfig is built directly. A managed
// cluster stays out of the default whatever runtime it names.
func TestDefaultRegistryNeedsClusterConfiguration(t *testing.T) {
	nodeManager := &ModuleConfig{Spec: ModuleConfigSpec{Version: 1, Settings: SettingsValues{
		"defaultCRI": string(registry_const.CRIContainerdV2),
	}}}
	nodeManager.SetName("node-manager")

	m := &MetaConfig{ModuleConfigs: []*ModuleConfig{nodeManager}}
	require.Equal(t, string(registry_const.CRIContainerdV2), m.effectiveDefaultCRI())

	needed, err := m.needsDefaultRegistry()
	require.NoError(t, err)
	require.False(t, needed)
}

// A set of modules without the registry module, as the d8-cluster-configuration webhook could meet:
// the parse keeps the legacy default instead of failing.
func TestDefaultRegistryWithoutRegistryModule(t *testing.T) {
	globalOptions := &options.New().Global
	globalOptions.ModulesDir = t.TempDir()
	schemaStore := newSchemaStore(globalOptions, schemaStorePaths(globalOptions))

	m := &MetaConfig{ClusterConfig: map[string]json.RawMessage{"defaultCRI": json.RawMessage(`"ContainerdV2"`)}}
	require.NoError(t, m.addDefaultRegistryModuleConfig(t.Context(), schemaStore))
	require.False(t, m.RegistryDefaulted())
	require.Empty(t, registryModuleConfigs(m.ModuleConfigs))
}

// A registry ModuleConfig that leaves the pull path to the user, Unmanaged or disabled, with no registry
// named anywhere else is refused: the legacy default it used to get leaves the nodes nowhere to pull
// from. Everything else keeps what it gets today.
func TestRefuseRegistryModuleWithoutRegistry(t *testing.T) {
	const registryModuleConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
`
	const unmanaged = registryModuleConfig + `  enabled: true
  version: 1
  settings:
    mode: Unmanaged
`
	const unmanagedNoEnabled = registryModuleConfig + `  version: 1
  settings:
    mode: Unmanaged
`
	// No mode is the module's default, Unmanaged.
	const noSettings = registryModuleConfig + `  enabled: true
  version: 1
`
	const noMode = registryModuleConfig + `  enabled: true
  version: 1
  settings:
    ingressClass: nginx
`
	const disabled = registryModuleConfig + `  enabled: false
`
	const disabledManaged = registryModuleConfig + `  enabled: false
  version: 1
  settings:
    mode: Managed
    primary:
      upstream:
        host: r.example.com
        path: /deckhouse/ee
`
	const initConfigWithImagesRepo = `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: r.example.com/deckhouse/ee
  registryScheme: HTTPS
`
	// What commander sends: the registry as the deckhouse ModuleConfig's Direct section.
	const deckhouseModuleConfigDirect = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    registry:
      mode: Direct
      direct:
        imagesRepo: r.example.com/deckhouse/ee
        scheme: HTTPS
`
	const clusterConfigNotManagedCRI = `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.33"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
defaultCRI: NotManaged
`
	bootstrap := []ValidateOption{ValidateOptionOperation(string(validatev1.OperationBootstrap))}

	refused := []struct {
		name   string
		config string
		opts   []ValidateOption
		state  string
	}{
		{name: "Unmanaged", config: defaultRegistryClusterConfig + unmanaged, state: "Unmanaged"},
		{name: "Unmanaged, bootstrap", config: defaultRegistryClusterConfig + unmanaged, opts: bootstrap, state: "Unmanaged"},
		{name: "Unmanaged without spec.enabled", config: defaultRegistryClusterConfig + unmanagedNoEnabled, state: "Unmanaged"},
		{name: "enabled with no settings", config: defaultRegistryClusterConfig + noSettings, state: "Unmanaged"},
		{name: "enabled with no mode", config: defaultRegistryClusterConfig + noMode, state: "Unmanaged"},
		{name: "Unmanaged, bootstrap in commander mode", config: defaultRegistryClusterConfig + unmanaged, opts: append(bootstrap, ValidateOptionCommanderMode(true)), state: "Unmanaged"},
		{name: "disabled", config: defaultRegistryClusterConfig + disabled, state: "disabled"},
		{name: "disabled, bootstrap", config: defaultRegistryClusterConfig + disabled, opts: bootstrap, state: "disabled"},
		{name: "disabled whatever its settings say", config: defaultRegistryClusterConfig + disabledManaged, state: "disabled"},
	}
	for _, tt := range refused {
		t.Run("refused: "+tt.name, func(t *testing.T) {
			_, err := ParseConfigFromData(t.Context(), tt.config, DummyValidatorProvider(), nil, tt.opts...)
			require.ErrorIs(t, err, ErrNoRegistryToPullFrom)
			require.ErrorContains(t, err, "the registry module is "+tt.state+" in its ModuleConfig")
			require.ErrorContains(t, err, "InitConfiguration.deckhouse: imagesRepo")
			require.ErrorContains(t, err, "mode: Managed with primary.upstream")
			// A mirror filled with d8 mirror push is an upstream like any other.
			require.ErrorContains(t, err, "a private or mirror registry the images were pushed to with d8 mirror push")
			// The air-gapped installation has its own path, which a mirror-less operator cannot get to
			// through primary.upstream. It needs the module enabled too, or it is refused again.
			require.ErrorContains(t, err, "enabled: true and mode: Managed with storage.cache: true and storage.source and no primary.upstream")
			require.ErrorContains(t, err, "--img-bundle-path")
		})
	}

	accepted := []struct {
		name   string
		config string
		opts   []ValidateOption

		mode       registry_const.ModeType
		legacyMode bool
		imagesRepo string
	}{
		{
			name:       "Unmanaged with imagesRepo in InitConfiguration",
			config:     defaultRegistryClusterConfig + initConfigWithImagesRepo + unmanaged,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
			imagesRepo: "r.example.com/deckhouse/ee",
		},
		{
			name:       "disabled with imagesRepo in InitConfiguration",
			config:     defaultRegistryClusterConfig + initConfigWithImagesRepo + disabled,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
			imagesRepo: "r.example.com/deckhouse/ee",
		},
		{
			name:       "Unmanaged with commander's Direct section in the deckhouse ModuleConfig",
			config:     defaultRegistryClusterConfig + deckhouseModuleConfigDirect + unmanaged,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true, // the section is folded into InitConfiguration, as without the ModuleConfig
			imagesRepo: "r.example.com/deckhouse/ee",
		},
		{
			// A managed cluster such as EKS: no ClusterConfiguration, the legacy Unmanaged CE default.
			name:       "Unmanaged without a ClusterConfiguration",
			config:     unmanaged,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
			imagesRepo: registry_const.DefaultImagesRepo,
		},
		{
			name:       "disabled without a ClusterConfiguration",
			config:     disabled,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
			imagesRepo: registry_const.DefaultImagesRepo,
		},
		{
			name:       "Unmanaged on a container runtime the module cannot configure",
			config:     clusterConfigNotManagedCRI + unmanaged,
			mode:       registry_const.ModeUnmanaged,
			legacyMode: true,
			imagesRepo: registry_const.DefaultImagesRepo,
		},
		{
			// A cluster that exists: refusing what it was installed with would leave it without converge.
			name:       "Unmanaged, converge",
			config:     defaultRegistryClusterConfig + unmanaged,
			opts:       []ValidateOption{ValidateOptionOperation(string(validatev1.OperationConverge))},
			mode:       registry_const.ModeDirect,
			imagesRepo: registry_const.DefaultImagesRepo,
		},
		{
			// Commander's ValidateResources, ConfigExtender and cache-path parses: possibly a cluster that
			// exists. The legacy default, as before.
			name:       "Unmanaged, commander mode with no operation",
			config:     defaultRegistryClusterConfig + unmanaged,
			opts:       []ValidateOption{ValidateOptionCommanderMode(true)},
			mode:       registry_const.ModeDirect,
			imagesRepo: registry_const.DefaultImagesRepo,
		},
		{
			name:       "disabled, destroy",
			config:     defaultRegistryClusterConfig + disabled,
			opts:       []ValidateOption{ValidateOptionOperation(string(validatev1.OperationDestroy))},
			mode:       registry_const.ModeDirect,
			imagesRepo: registry_const.DefaultImagesRepo,
		},
	}
	for _, tt := range accepted {
		t.Run("accepted: "+tt.name, func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), tt.config, DummyValidatorProvider(), nil, tt.opts...)
			require.NoError(t, err)

			require.False(t, metaConfig.RegistryDefaulted())
			require.Len(t, registryModuleConfigs(metaConfig.ModuleConfigs), 1, "only the user's own")
			require.Equal(t, tt.mode, metaConfig.Registry.Settings.Mode)
			require.Equal(t, tt.legacyMode, metaConfig.Registry.LegacyMode)
			require.Equal(t, tt.imagesRepo, metaConfig.Registry.Settings.RemoteData.ImagesRepo)
		})
	}

	// primary or storage without mode: Managed is what the module's schema rejects, missing mode
	// included, and with or without a registry named elsewhere. It used to be read as Managed.
	const upstreamSettings = `    primary:
      upstream:
        host: r.example.com
        path: /deckhouse/ee
`
	const noModeUpstream = registryModuleConfig + `  enabled: true
  version: 1
  settings:
` + upstreamSettings
	const noModeCache = registryModuleConfig + `  enabled: true
  version: 1
  settings:
    storage:
      cache: true
`
	const unmanagedUpstream = unmanaged + upstreamSettings
	const disabledNoModeUpstream = registryModuleConfig + `  enabled: false
  version: 1
  settings:
` + upstreamSettings

	needManaged := []struct {
		name   string
		config string
		opts   []ValidateOption
	}{
		{name: "no mode with primary.upstream", config: defaultRegistryClusterConfig + noModeUpstream},
		{name: "no mode with primary.upstream, bootstrap", config: defaultRegistryClusterConfig + noModeUpstream, opts: bootstrap},
		{name: "no mode with storage.cache", config: defaultRegistryClusterConfig + noModeCache},
		{name: "Unmanaged with primary.upstream", config: defaultRegistryClusterConfig + unmanagedUpstream},
		{name: "Unmanaged with primary.upstream and imagesRepo in InitConfiguration", config: defaultRegistryClusterConfig + initConfigWithImagesRepo + unmanagedUpstream},
		{name: "disabled with no mode and primary.upstream", config: defaultRegistryClusterConfig + disabledNoModeUpstream},
	}
	for _, tt := range needManaged {
		t.Run("refused without mode: Managed: "+tt.name, func(t *testing.T) {
			_, err := ParseConfigFromData(t.Context(), tt.config, DummyValidatorProvider(), nil, tt.opts...)
			require.ErrorIs(t, err, ErrRegistrySettingsWithoutManaged)
			require.ErrorContains(t, err, "Set mode: Managed, or remove primary and storage")
		})
	}

	// Converge reads a cluster that exists, and the same scope as the refusal above applies.
	t.Run("accepted: no mode with primary.upstream, converge", func(t *testing.T) {
		_, err := ParseConfigFromData(t.Context(), defaultRegistryClusterConfig+noModeUpstream, DummyValidatorProvider(), nil,
			ValidateOptionOperation(string(validatev1.OperationConverge)))
		require.NoError(t, err)
	})

	// A configuration that names nothing at all still gets the default registry ModuleConfig.
	t.Run("accepted: nothing at all gets the default", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(t.Context(), defaultRegistryClusterConfig, DummyValidatorProvider(), nil)
		require.NoError(t, err)
		require.True(t, metaConfig.RegistryDefaulted())
	})
}
