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

func generateMetaConfigForDeckhouseConfigTest(t *testing.T, data map[string]interface{}) *MetaConfig {
	return generateMetaConfig(t, configOverridesTemplate, data, false)
}

func generateMetaConfigForDeckhouseConfigTestWithErr(t *testing.T, data map[string]interface{}) *MetaConfig {
	return generateMetaConfig(t, configOverridesTemplate, data, true)
}

func TestDeckhouseReleaseChannelDeprecated(t *testing.T) {
	metaConfig := generateMetaConfig(t, `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.28"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  devBranch: aaaa
  releaseChannel: Beta
---
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 192.168.199.0/24

`, map[string]interface{}{}, false)
	iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
	require.NoError(t, err)
	require.Equal(t, iCfg.ReleaseChannel, "Beta")
}

func TestModuleDeckhouseConfigOverridesAndMc(t *testing.T) {
	t.Run("Fail whe module config and config overrides", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
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
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-11-22T09:12:26Z"
  generation: 1
  name: common
  resourceVersion: "826312837"
  uid: b275a253-dcb5-4321-b0ef-8881fdc8a2a8
spec:
  enabled: false
`,
		})

		_, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.Error(t, err)
	})

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

	t.Run("Remove releaseChannel from module config and set to installer cfg for adding after bootstrap", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    releaseChannel: RockSolid
  version: 1
`,
		})

		iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)

		require.Equal(t, iCfg.LogLevel, "Info")
		require.Equal(t, iCfg.Bundle, "Default")

		require.Len(t, iCfg.ModuleConfigs, 1)

		require.NotContains(t, iCfg.ModuleConfigs[0].Spec.Settings, "releaseChannel")
		require.Equal(t, iCfg.ReleaseChannel, "RockSolid")
	})

	t.Run("Use bundle and logLevel from module config", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
			"logLevel": "Debug",
			"bundle":   "Minimal",
		})

		iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)

		require.Equal(t, iCfg.LogLevel, "Debug")
		require.Equal(t, iCfg.Bundle, "Minimal")
	})

	t.Run("Convert config overrides to module config", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
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

		iCfg, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.NoError(t, err)

		require.Len(t, iCfg.ModuleConfigs, 5)
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
  name: operator-prometheus-crd
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
  name: operator-prometheus-crd
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
