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
	"testing"

	"github.com/stretchr/testify/require"
)

const configOverridesTemplate = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.30"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  devBranch: aaaa
{{- if .bundle }}
  bundle: {{ .bundle }}
{{- end }}

{{- if .releaseChannel }}
  releaseChannel: {{ .releaseChannel }}
{{- end }}

{{- if .logLevel }}
  logLevel: {{ .logLevel }}
{{- end }}

{{- if .configOverrides}}
{{- .configOverrides | nindent 2 }}
{{- end }}
---
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 192.168.199.0/24
---
{{- if .moduleConfigs}}
{{- .moduleConfigs }}
{{- end }}
`

func assertModuleConfig(t *testing.T, mc *ModuleConfig, enabled bool, version int, setting map[string]interface{}) {
	require.NotNil(t, mc.Spec.Enabled)
	require.Equal(t, *mc.Spec.Enabled, enabled)
	require.Equal(t, mc.Spec.Version, version)
	require.Equal(t, mc.Spec.Settings, SettingsValues(setting))
}

func generateMetaConfigForDeckhouseConfigTest(t *testing.T, data map[string]interface{}) *MetaConfig {
	return generateMetaConfig(t, configOverridesTemplate, data, false)
}

func generateMetaConfigForDeckhouseConfigTestWithErr(t *testing.T, data map[string]interface{}) *MetaConfig {
	return generateMetaConfig(t, configOverridesTemplate, data, true)
}

func TestModuleDeckhouseConfigRegistryOverrides(t *testing.T) {
	tpl := `
{{ with .enableCRI }}
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.30"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 192.168.199.0/24
{{- end }}
{{- with .manifests }}
	{{- range . }}
---
		{{- . }}
	{{- end }}
{{- end }}
`

	initConfig := `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: "r.example.com/test/"
  # registryDockerCfg: {"auths":{"r.example.com":{"username":"test-user","password":"test-password"}}}
  registryDockerCfg: eyJhdXRocyI6eyJyLmV4YW1wbGUuY29tIjp7InVzZXJuYW1lIjoidGVzdC11c2VyIiwicGFzc3dvcmQiOiJ0ZXN0LXBhc3N3b3JkIn19fQ==
  registryCA: "-----BEGIN CERTIFICATE-----"
  registryScheme: HTTPS
`
	moduleConfigDeckhouse := `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    bundle: Default
    logLevel: Info
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: r.example.com/test/
        username: test-user
        password: test-password
        scheme: HTTPS
        ca: "-----BEGIN CERTIFICATE-----"
  version: 1
`
	assert := func(t *testing.T, tplCtx map[string]any, expect map[string]any) {
		metaConfig := generateMetaConfig(t, tpl, tplCtx, false)
		installConfig, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)
		require.Len(t, installConfig.ModuleConfigs, 1)
		assertModuleConfig(t, installConfig.ModuleConfigs[0], true, 1, expect)
	}

	t.Run("InitConfiguration -> always empty", func(t *testing.T) {
		t.Run("Without CRI (module disable) -> empty", func(t *testing.T) {
			assert(t,
				map[string]any{
					"enableCRI": false,
					"manifests": []string{initConfig},
				},
				map[string]any{
					"bundle":   "Default",
					"logLevel": "Info",
				})
		})
		t.Run("With CRI (module enable) -> empty", func(t *testing.T) {
			assert(t,
				map[string]any{
					"enableCRI": true,
					"manifests": []string{initConfig},
				},
				map[string]any{
					"bundle":   "Default",
					"logLevel": "Info",
				})
		})
	})
	t.Run("Default -> CE edition registry", func(t *testing.T) {
		t.Run("Without CRI (module disable) -> empty", func(t *testing.T) {
			assert(t,
				map[string]any{
					"enableCRI": false,
				},
				map[string]any{
					"bundle":   "Default",
					"logLevel": "Info",
				})
		})
		t.Run("With CRI (module enable) -> direct", func(t *testing.T) {
			assert(t,
				map[string]any{
					"enableCRI": true,
				},
				map[string]any{
					"bundle":   "Default",
					"logLevel": "Info",
					"registry": map[string]any{
						"mode": "Direct",
						"direct": map[string]any{
							"imagesRepo": "registry.deckhouse.io/deckhouse/ce",
							"scheme":     "HTTPS",
						},
					},
				})
		})
	})
	t.Run("ModuleConfig Deckhouse", func(t *testing.T) {
		t.Run("Without CRI (module disable) -> error", func(t *testing.T) {
			tplCtx := map[string]any{
				"enableCRI": false,
				"manifests": []string{moduleConfigDeckhouse},
			}
			_ = generateMetaConfig(t, tpl, tplCtx, true)
		})
		t.Run("With CRI (module enable) -> from moduleConfig", func(t *testing.T) {
			assert(t,
				map[string]any{
					"enableCRI": true,
					"manifests": []string{moduleConfigDeckhouse},
				},
				map[string]any{
					"bundle":   "Default",
					"logLevel": "Info",
					"registry": map[string]any{
						"mode": "Unmanaged",
						"unmanaged": map[string]any{
							"imagesRepo": "r.example.com/test",
							"username":   "test-user",
							"password":   "test-password",
							"scheme":     "HTTPS",
							"ca":         "-----BEGIN CERTIFICATE-----",
						},
					},
				})
		})
	})
}

func TestModuleDeckhouseConfigOverridesAndMc(t *testing.T) {
	t.Run("Use default bundle and logLevel", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  enabled: false
`,
		})

		iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)

		require.Equal(t, iCfg.LogLevel, "Info")
		require.Equal(t, iCfg.Bundle, "Default")

		// helm and deckhouseCm
		require.Len(t, iCfg.ModuleConfigs, 2)

		require.Contains(t, iCfg.ModuleConfigs[1].Spec.Settings, "bundle")
		require.Equal(t, iCfg.ModuleConfigs[1].Spec.Settings["bundle"], "Default")

		require.Contains(t, iCfg.ModuleConfigs[1].Spec.Settings, "logLevel")
		require.Equal(t, iCfg.ModuleConfigs[1].Spec.Settings["logLevel"], "Info")
	})

	t.Run("Use bundle and logLevel from module config", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    bundle: Minimal
    logLevel: Debug
  version: 1
`,
		})

		iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)

		require.Equal(t, iCfg.LogLevel, "Debug")
		require.Equal(t, iCfg.Bundle, "Minimal")

		require.Len(t, iCfg.ModuleConfigs, 1)
	})

	t.Run("Forbid to use configOverrides", func(t *testing.T) {
		generateMetaConfigForDeckhouseConfigTestWithErr(t, map[string]interface{}{
			"configOverrides": `
configOverrides:
  istioEnabled: false
  global:
    modules:
      publicDomainTemplate: "%s.example.com"
  cniCiliumEnabled: true
  cniCilium:
    tunnelMode: VXLAN
  common:
    testString: aaaaa
`,
		})
	})

	t.Run("Forbid to use releaseChannel", func(t *testing.T) {
		generateMetaConfigForDeckhouseConfigTestWithErr(t, map[string]interface{}{
			"releaseChannel": "Beta",
		})
	})

	t.Run("Forbid to use bundle", func(t *testing.T) {
		generateMetaConfigForDeckhouseConfigTestWithErr(t, map[string]interface{}{
			"bundle": "Default",
		})
	})

	t.Run("Forbid to use logLevel", func(t *testing.T) {
		generateMetaConfigForDeckhouseConfigTestWithErr(t, map[string]interface{}{
			"logLevel": "Info",
		})
	})

	t.Run("Correct parse module configs", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    bundle: Minimal
    logLevel: Debug
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: istio
spec:
  enabled: false
`,
		})

		iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)

		require.Len(t, iCfg.ModuleConfigs, 2)

		assertModuleConfig(t, iCfg.ModuleConfigs[0], true, 1, map[string]interface{}{
			"bundle":   "Minimal",
			"logLevel": "Debug",
			"registry": map[string]interface{}{
				"mode": "Direct",
				"direct": map[string]interface{}{
					"imagesRepo": "registry.deckhouse.io/deckhouse/ce",
					"scheme":     "HTTPS",
				},
			},
		})

		assertModuleConfig(t, iCfg.ModuleConfigs[1], false, 0, nil)
	})

	t.Run("Fail settings without version", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTestWithErr(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    bundle: Minimal
    logLevel: Debug
---
`,
		})

		_, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.Error(t, err)
	})

	t.Run("Fail with incorrect settings", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTestWithErr(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    bundle: AAAAAAAAAAA
    logLevel: Debug
  version: 1
---
`,
		})

		_, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.Error(t, err)
	})

	t.Run("Module without spec file should ok without settings", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registrypackages
spec:
  enabled: true
`,
		})

		iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)

		require.Len(t, iCfg.ModuleConfigs, 2)

		assertModuleConfig(t, iCfg.ModuleConfigs[0], true, 0, nil)
	})

	t.Run("Module without spec file should fail with settings", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTestWithErr(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registrypackages
spec:
  enabled: true
  version: 1
  settings:
    invalid: true
`,
		})

		_, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.Error(t, err)
	})
}
