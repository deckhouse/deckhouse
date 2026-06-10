// Copyright 2021 Flant JSC
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
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func TestParseConfigFromData(t *testing.T) {
	clusterConfig := `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.31"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
`
	initConfig := `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
   imagesRepo: test
   devBranch: test
   # {"auths": { "test": {}}}
   registryDockerCfg: eyJhdXRocyI6IHsgInRlc3QiOiB7fX19
`
	staticConfig := `
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.0.0/24
`
	moduleConfigGlobalValid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: false
    modules:
      publicDomainTemplate: '%s.domain.example.com'
  version: 1
`
	moduleConfigGlobalInvalid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: "wswswswss"
    modules:
      publicDomainTemplate: 'domain.example.com'
  version: 1
`

	moduleConfigCommonInvalid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  enabled: true
  settings:
    testString: true
    testArray: 1
    testEnum: c
  version: 1
`
	moduleConfigCommonValid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  enabled: false
  settings:
    testString: "aaaaa"
    testArray: ["1", "2"]
    testEnum: Aa
  version: 1
`

	moduleConfigCommonWithoutEnabled := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  settings:
    testString: "aaaaa"
    testArray: ["1", "2"]
    testEnum: Aa
  version: 1
`

	moduleConfigCommonWithoutSettings := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  enabled: false
`
	unknownModuleConfig := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: unknown
spec:
  enabled: true
`

	configMapAndInstanceClass := `
---
apiVersion: v1
data:
  isUpdating: "false"
  notified: "false"
kind: ConfigMap
metadata:
  labels:
    heritage: deckhouse
  name: d8-release-data
  namespace: d8-system
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
spec:
  cores: 4
  memory: 8192
`

	ngWithTemplating := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
    maxPerZone: 1
    minPerZone: 1
    zones:
    - ru-central1-a
    additionalSubnets:
    - '{{ index .cloudDiscovery.zoneToSubnetIdMap "ru-central1-a" }}'
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: CloudEphemeral
`
	// Registry
	t.Run("Registry", func(t *testing.T) {
		t.Run("InitConfiguration -> always unmanaged && legacy", func(t *testing.T) {
			t.Run("Without CRI (module disable)", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(t.Context(), initConfig, DummyPreparatorProvider(), nil)
				require.NoError(t, err)
				require.Equal(t, true, metaConfig.Registry.LegacyMode)
				require.Equal(t, registry_const.ModeUnmanaged, metaConfig.Registry.Settings.Mode)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, "test", registry.ImagesRepo)
				require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
				require.Equal(t, "", registry.Username)
				require.Equal(t, "", registry.Password)
				require.Equal(t, "", registry.CA)
			})
			t.Run("With CRI (module enable)", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(t.Context(), initConfig+clusterConfig, DummyPreparatorProvider(), nil)
				require.NoError(t, err)
				require.Equal(t, true, metaConfig.Registry.LegacyMode)
				require.Equal(t, registry_const.ModeUnmanaged, metaConfig.Registry.Settings.Mode)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, "test", registry.ImagesRepo)
				require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
				require.Equal(t, "", registry.Username)
				require.Equal(t, "", registry.Password)
				require.Equal(t, "", registry.CA)
			})
		})
		t.Run("Default -> CE edition registry", func(t *testing.T) {
			t.Run("Without CRI (module disable) -> unmanaged && legacy", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(t.Context(), "", DummyPreparatorProvider(), nil)
				require.NoError(t, err)
				require.Equal(t, true, metaConfig.Registry.LegacyMode)
				require.Equal(t, registry_const.ModeUnmanaged, metaConfig.Registry.Settings.Mode)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, "registry.deckhouse.io/deckhouse/ce", registry.ImagesRepo)
				require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
				require.Equal(t, "", registry.Username)
				require.Equal(t, "", registry.Password)
				require.Equal(t, "", registry.CA)
			})
			t.Run("With CRI (module enable) -> direct && not legacy", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(t.Context(), ""+clusterConfig, DummyPreparatorProvider(), nil)
				require.NoError(t, err)
				require.Equal(t, false, metaConfig.Registry.LegacyMode)
				require.Equal(t, registry_const.ModeDirect, metaConfig.Registry.Settings.Mode)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, "registry.deckhouse.io/deckhouse/ce", registry.ImagesRepo)
				require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
				require.Equal(t, "", registry.Username)
				require.Equal(t, "", registry.Password)
				require.Equal(t, "", registry.CA)
			})
		})
		t.Run("ModuleConfig Deckhouse", func(t *testing.T) {
			moduleConfigDeckhouse := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: r.example.com/test
        username: test-user
        password: test-password
        scheme: HTTPS
        ca: "-----BEGIN CERTIFICATE-----"
  version: 1
`
			t.Run("Without CRI (module disable) -> error", func(t *testing.T) {
				_, err := ParseConfigFromData(t.Context(), moduleConfigDeckhouse, DummyPreparatorProvider(), nil)
				require.Error(t, err)
			})
			t.Run("With CRI (module enable) -> from moduleConfig && not legacy", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(t.Context(), moduleConfigDeckhouse+clusterConfig, DummyPreparatorProvider(), nil)
				require.NoError(t, err)
				require.Equal(t, false, metaConfig.Registry.LegacyMode)
				require.Equal(t, registry_const.ModeUnmanaged, metaConfig.Registry.Settings.Mode)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, "r.example.com/test", registry.ImagesRepo)
				require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
				require.Equal(t, "test-user", registry.Username)
				require.Equal(t, "test-password", registry.Password)
				require.Equal(t, "-----BEGIN CERTIFICATE-----", registry.CA)
			})
		})
	})

	t.Run("Standard Static", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig, DummyPreparatorProvider(), nil)
		require.NoError(t, err)

		parsedStaticConfig, err := metaConfig.StaticClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedStaticConfig))

		parsedProviderConfig, err := metaConfig.ProviderClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedProviderConfig))

		require.Equal(t, "10.111.0.10", metaConfig.ClusterDNSAddress)
		require.Equal(t, "Static", metaConfig.ClusterType)
		require.Len(t, metaConfig.ResourcesYAML, 0)
	})

	t.Run("Static with StaticClusterConfig", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig, DummyPreparatorProvider(), nil)
		require.NoError(t, err)

		parsedStaticConfig, err := metaConfig.StaticClusterConfigYAML()
		require.NoError(t, err)
		require.YAMLEq(t, staticConfig, string(parsedStaticConfig))

		parsedProviderConfig, err := metaConfig.ProviderClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedProviderConfig))

		require.Equal(t, "10.111.0.10", metaConfig.ClusterDNSAddress)
		require.Equal(t, "Static", metaConfig.ClusterType)

		require.Len(t, metaConfig.ResourcesYAML, 0)
	})

	t.Run("Module config", func(t *testing.T) {
		t.Run("Global valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigGlobalValid, DummyPreparatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Global invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigGlobalInvalid, DummyPreparatorProvider(), nil)
			require.Error(t, err)
		})

		t.Run("Module valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonValid, DummyPreparatorProvider(), nil)

			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Module invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonInvalid, DummyPreparatorProvider(), nil)
			require.Error(t, err)
		})

		t.Run("Module without enabled field", func(t *testing.T) {
			_, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonWithoutEnabled, DummyPreparatorProvider(), nil)
			require.Error(t, err)
		})

		t.Run("Module without settings", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonWithoutSettings, DummyPreparatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Unknown module should move into resources", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+unknownModuleConfig, DummyPreparatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)
		})
	})

	t.Run("Config with another k8s resources eg configMap", func(t *testing.T) {
		t.Run("Should move another resources into resourcesYAML", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+configMapAndInstanceClass, DummyPreparatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)

			bigFileTmp := strings.TrimSpace(metaConfig.ResourcesYAML)
			docs := input.YAMLSplitRegexp.Split(bigFileTmp, -1)

			configMapFound := false
			instanceClassFound := false

			for _, doc := range docs {
				var index SchemaIndex
				err := yaml.Unmarshal([]byte(doc), &index)

				require.NoError(t, err)
				require.True(t, index.IsValid())
				switch index.Kind {
				case "ConfigMap":
					configMapFound = true
				case "YandexInstanceClass":
					instanceClassFound = true
				}
			}

			require.True(t, configMapFound)
			require.True(t, instanceClassFound)
		})

		t.Run("Should move resourcesYAML", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+ngWithTemplating, DummyPreparatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)

			bigFileTmp := strings.TrimSpace(metaConfig.ResourcesYAML)

			var index SchemaIndex
			err = yaml.Unmarshal([]byte(bigFileTmp), &index)

			require.NoError(t, err)
			require.True(t, index.IsValid())

			require.Equal(t, index.Kind, "NodeGroup")
		})
	})
}

func TestParseConfigFromFiles(t *testing.T) {
	t.Run("parse wildcard", func(t *testing.T) {
		err := os.WriteFile("/deckhouse/version", []byte("dev"), 0o666)
		if err != nil {
			panic(err)
		}

		defer func() {
			os.Remove("/deckhouse/version")
		}()
		metaConfig, err := LoadConfigFromFile(t.Context(), []string{"./mocks/*.yml", "./mocks/3-ModuleConfig.yaml"}, DummyPreparatorProvider(), &options.GlobalOptions{})
		require.NoError(t, err)
		require.Equal(t, "Static", metaConfig.ClusterType)

		t.Run("Registry CE edition config", func(t *testing.T) {
			registry := metaConfig.Registry.Settings.RemoteData
			require.Equal(t, "registry.deckhouse.io/deckhouse/ce", registry.ImagesRepo)
			require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
			require.Equal(t, "", registry.Username)
			require.Equal(t, "", registry.Password)
			require.Equal(t, "", registry.CA)
		})

		require.Len(t, metaConfig.ModuleConfigs, 3)
	})
}

func TestParseConfigFromCluster(t *testing.T) {
	tests.RequireDir(t, "/deckhouse/candi/cloud-providers", "werf bundles cloud-providers from modules/030-cloud-provider-* at CI time")
	doParseFromClusterNoError := func(t *testing.T, tst *testParseConfigFromCluster) *MetaConfig {
		metaConfig, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.preparatorProvider, &options.GlobalOptions{}, "")

		require.NoError(t, err)
		require.NotNil(t, metaConfig)
		require.NotEmpty(t, metaConfig.ClusterType)
		require.Equal(t, metaConfig.ClusterType, tst.clusterType)
		require.NotEmpty(t, metaConfig.ClusterConfig)
		cfg, err := metaConfig.ClusterConfigYAML()
		require.NoError(t, err)
		require.YAMLEq(t, tst.clusterConfig, string(cfg))

		return metaConfig
	}

	doParseFromClusterWithError := func(t *testing.T, tst *testParseConfigFromCluster) {
		metaConfig, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.preparatorProvider, &options.GlobalOptions{}, "")

		require.Error(t, err)
		require.Nil(t, metaConfig)
	}

	t.Run("Invalid cluster", func(t *testing.T) {
		type test struct {
			name   string
			params testParseConfigFromClusterParams
		}

		tests := []test{
			{
				name: "no secret",
				params: testParseConfigFromClusterParams{
					clusterConfig: "",
					clusterType:   StaticClusterType,
				},
			},
			{
				name: "invalid secret",
				params: testParseConfigFromClusterParams{
					clusterConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
domain: cluster.local
podSubnetNodeCIDRPrefix: "24"
`,
					clusterType: StaticClusterType,
				},
			},
			{
				name: "empty cluster type",
				params: testParseConfigFromClusterParams{
					clusterConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: ""
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
clusterDomain: cluster.local
podSubnetNodeCIDRPrefix: "24"
`,
					clusterType: StaticClusterType,
				},
			},
			{
				name: "invalid cluster type",
				params: testParseConfigFromClusterParams{
					clusterConfig: `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: "invalid"
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
clusterDomain: cluster.local
podSubnetNodeCIDRPrefix: "24"
`,
					clusterType: StaticClusterType,
				},
			},
			{
				name: "invalid yaml",
				params: testParseConfigFromClusterParams{
					clusterConfig: `:a""vrgrg`,
					clusterType:   StaticClusterType,
				},
			},
		}

		for _, tst := range tests {
			t.Run(tst.name, func(t *testing.T) {
				tt := createTestParseConfigFromCluster(t, tst.params)

				doParseFromClusterWithError(t, tt)
			})
		}
	})

	t.Run("Static cluster", func(t *testing.T) {
		clusterGenericConfig := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
clusterDomain: cluster.local
podSubnetNodeCIDRPrefix: "24"
`
		testParams := testParseConfigFromClusterParams{
			clusterConfig: clusterGenericConfig,
			clusterType:   StaticClusterType,
		}

		createStaticConfigSecret := func(t *testing.T, tst *testParseConfigFromCluster, config *string) {
			t.Helper()

			data := make(map[string][]byte)
			if config != nil {
				data["static-cluster-configuration.yaml"] = []byte(*config)
			}

			testCreateKubeSystemSecret(t, tst.kubeCl, "d8-static-cluster-configuration", data)
		}

		assertStaticConfigEmpty := func(t *testing.T, metaConfig *MetaConfig) {
			require.Nil(t, metaConfig.StaticClusterConfig)
			cfg, err := metaConfig.StaticClusterConfigYAML()
			require.NoError(t, err)
			require.Empty(t, cfg)
		}

		createAndAssertStaticConfigEmpty := func(t *testing.T, tst *testParseConfigFromCluster, config *string) {
			createStaticConfigSecret(t, tst, config)
			metaConfig := doParseFromClusterNoError(t, tst)
			assertStaticConfigEmpty(t, metaConfig)
		}

		t.Run("no secret", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)

			metaConfig := doParseFromClusterNoError(t, tst)

			assertStaticConfigEmpty(t, metaConfig)
		})

		t.Run("empty data", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)

			createAndAssertStaticConfigEmpty(t, tst, nil)
		})

		t.Run("empty config", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)

			createAndAssertStaticConfigEmpty(t, tst, pointer.String(""))
		})

		t.Run("valid config", func(t *testing.T) {
			const staticConfig = `
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.0.0/24
`
			tst := createTestParseConfigFromCluster(t, testParams)

			createStaticConfigSecret(t, tst, pointer.String(staticConfig))
			metaConfig := doParseFromClusterNoError(t, tst)

			require.NotEmpty(t, metaConfig.StaticClusterConfig)

			staticConfigFromMetaConfig, err := metaConfig.StaticClusterConfigYAML()
			require.NoError(t, err)
			require.YAMLEq(t, staticConfig, string(staticConfigFromMetaConfig))
		})

		t.Run("invalid config", func(t *testing.T) {
			const staticConfig = `
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
  tst: "string"
`
			tst := createTestParseConfigFromCluster(t, testParams)

			createStaticConfigSecret(t, tst, pointer.String(staticConfig))
			doParseFromClusterWithError(t, tst)
		})

		t.Run("invalid yaml", func(t *testing.T) {
			const staticConfig = `: ""aa`
			tst := createTestParseConfigFromCluster(t, testParams)

			createStaticConfigSecret(t, tst, pointer.String(staticConfig))
			doParseFromClusterWithError(t, tst)
		})
	})

	t.Run("Cloud cluster", func(t *testing.T) {
		clusterGenericConfig := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "test"
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
clusterDomain: cluster.local
podSubnetNodeCIDRPrefix: "24"
`
		testParams := testParseConfigFromClusterParams{
			clusterConfig: clusterGenericConfig,
			clusterType:   CloudClusterType,
			extraGVRs: map[schema.GroupVersionResource]string{
				{Group: instanceClassAPIGroup, Version: "v1", Resource: "yandexinstanceclasses"}: "YandexInstanceClassList",
			},
		}

		createCloudConfigSecret := func(t *testing.T, tst *testParseConfigFromCluster, config *string) {
			t.Helper()

			data := make(map[string][]byte)
			if config != nil {
				data["cloud-provider-cluster-configuration.yaml"] = []byte(*config)
				data["cloud-provider-discovery-data.json"] = []byte(`{"a": "b"}`)
			}

			testCreateKubeSystemSecret(t, tst.kubeCl, "d8-provider-cluster-configuration", data)
		}

		createAndAssertCloudConfigEmptyOrInvalidError := func(t *testing.T, tst *testParseConfigFromCluster, config *string) {
			createCloudConfigSecret(t, tst, config)
			doParseFromClusterWithError(t, tst)
		}

		t.Run("no secret", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)

			doParseFromClusterWithError(t, tst)
		})

		t.Run("empty data", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)

			createAndAssertCloudConfigEmptyOrInvalidError(t, tst, nil)
		})

		t.Run("empty config", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)

			createAndAssertCloudConfigEmptyOrInvalidError(t, tst, pointer.String(""))
		})

		t.Run("valid config", func(t *testing.T) {
			const cloudConfig = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
masterNodeGroup:
  replicas: 1
  instanceClass:
    etcdDiskSizeGb: 10
    platform: standard-v2
    cores: 4
    memory: 8192
    imageID: imageId
    externalIPAddresses:
      - Auto
sshPublicKey: ssh-rsa AAAAB3NzaC
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: cloudId
  folderID: folderId
  serviceAccountJSON: "{}"
`
			tst := createTestParseConfigFromCluster(t, testParams)

			createCloudConfigSecret(t, tst, pointer.String(cloudConfig))
			metaConfig := doParseFromClusterNoError(t, tst)

			require.NotEmpty(t, metaConfig.ProviderClusterConfig)

			cloudConfigFromMetaConfig, err := metaConfig.ProviderClusterConfigYAML()
			require.NoError(t, err)
			require.YAMLEq(t, cloudConfig, string(cloudConfigFromMetaConfig))
		})

		t.Run("invalid config", func(t *testing.T) {
			const cloudConfig = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNATT
sshPublicKey: ssh-rsa AAAAB3NzaC
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: cloudId
  folderID: folderId
  serviceAccountJSON: "{}"
`
			tst := createTestParseConfigFromCluster(t, testParams)

			createAndAssertCloudConfigEmptyOrInvalidError(t, tst, pointer.String(cloudConfig))
		})

		t.Run("invalid yaml", func(t *testing.T) {
			const cloudConfig = `:a""n`
			tst := createTestParseConfigFromCluster(t, testParams)

			createAndAssertCloudConfigEmptyOrInvalidError(t, tst, pointer.String(cloudConfig))
		})

		t.Run("mc-flow: only ModuleConfig, no PCC Secret", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)
			testCreateCloudProviderModuleConfig(t, tst.kubeCl, "yandex")

			metaConfig := doParseFromClusterNoError(t, tst)

			require.Empty(t, metaConfig.ProviderClusterConfig, "PCC must remain unset in mc-flow")
			require.Len(t, metaConfig.ModuleConfigs, 1)
			require.Equal(t, "cloud-provider-yandex", metaConfig.ModuleConfigs[0].GetName())
		})

		t.Run("mc-flow and legacy: both markers loaded, PCC kept for typed fields", func(t *testing.T) {
			// A cluster mid-migration carries both markers. The ModuleConfig
			// is often a stub without settings while the legacy PCC still
			// holds the real layout/master sizing, so Cloud() loads both:
			// extractProviderClusterFields gives PCC priority for typed
			// fields, with the ModuleConfig filling whatever is left. Ignoring
			// the PCC here would zero out Layout on such clusters
			// (the "Empty Layout" converge regression).
			tst := createTestParseConfigFromCluster(t, testParams)
			testCreateCloudProviderModuleConfig(t, tst.kubeCl, "yandex")
			createCloudConfigSecret(t, tst, pointer.String(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
masterNodeGroup:
  replicas: 1
  instanceClass:
    etcdDiskSizeGb: 10
    platform: standard-v2
    cores: 4
    memory: 8192
    imageID: imageId
sshPublicKey: ssh-rsa AAAAB3NzaC
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: cloudId
  folderID: folderId
  serviceAccountJSON: "{}"
`))

			metaConfig := doParseFromClusterNoError(t, tst)

			require.NotEmpty(t, metaConfig.ProviderClusterConfig, "legacy PCC must be loaded alongside the MC")
			require.Len(t, metaConfig.ModuleConfigs, 1)
			require.Equal(t, "without-nat", metaConfig.Layout, "Layout must come from PCC, not the stub MC")
		})

		t.Run("neither marker present", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)

			_, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.preparatorProvider, &options.GlobalOptions{}, "")
			require.Error(t, err)
			require.Contains(t, err.Error(), "ModuleConfig")
			require.Contains(t, err.Error(), "d8-provider-cluster-configuration")
		})

		t.Run("cloud cluster loads registry-fields even when EnsureCandiAvailable=false", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)
			testCreateCloudProviderModuleConfig(t, tst.kubeCl, "yandex")
			// deckhouse-registry Secret is already seeded by createTestParseConfigFromCluster.

			metaConfig, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.preparatorProvider, &options.GlobalOptions{EnsureCandiAvailable: false}, "")
			require.NoError(t, err)
			require.NotEmpty(t, metaConfig.DeckhouseConfig.RegistryDockerCfg, "registry docker cfg must be populated for cloud cluster")
			require.NotEmpty(t, metaConfig.DeckhouseConfig.ImagesRepo)
		})
	})
}

func testCreateCloudProviderModuleConfig(t *testing.T, kubeCl *client.KubernetesClient, providerName string) {
	t.Helper()

	// Real cloud-provider-<name> ModuleConfig schemas vary per provider
	// (yandex exposes additionalExternalNetworkIDs/storageClass, not
	// nodes.parameters.layout). These tests don't exercise
	// applyCloudProviderModuleSettings — they only need the MC to exist as
	// a marker — so seed an empty-settings spec that validates under any
	// provider's schema.
	mc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]interface{}{"name": "cloud-provider-" + providerName},
		"spec": map[string]interface{}{
			"version":  float64(2),
			"enabled":  true,
			"settings": map[string]interface{}{},
		},
	}}

	_, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Create(t.Context(), mc, metav1.CreateOptions{})
	require.NoError(t, err)
}

type testParseConfigFromClusterParams struct {
	clusterConfig string
	clusterType   string
	extraGVRs     map[schema.GroupVersionResource]string
}

type testParseConfigFromCluster struct {
	testParseConfigFromClusterParams

	kubeCl             *client.KubernetesClient
	preparatorProvider MetaConfigPreparatorProvider
}

func createTestParseConfigFromCluster(t *testing.T, p testParseConfigFromClusterParams) *testParseConfigFromCluster {
	gvrs := map[schema.GroupVersionResource]string{
		nodeGroupGVR:    "NodeGroupList",
		ModuleConfigGVR: "ModuleConfigList",
	}
	for gvr, kind := range p.extraGVRs {
		gvrs[gvr] = kind
	}
	kubeCl := client.NewFakeKubernetesClientWithListGVR(gvrs)

	if p.clusterConfig != "" {
		testCreateKubeSystemSecret(t, kubeCl, "d8-cluster-configuration", map[string][]byte{
			"cluster-configuration.yaml": []byte(p.clusterConfig),
		})
	}

	// parseConfigFromCluster fetches the d8-system/deckhouse-registry Secret
	// for every Cloud cluster (base.go: needRegistryData = ... ||
	// clusterType == CloudClusterType). Without this seed registrydata.
	// GetRegistryData retry-loops for 45 × 5 s and the test trips the 600 s
	// go-test timeout.
	testCreateDeckhouseRegistrySecret(t, kubeCl)

	return &testParseConfigFromCluster{
		testParseConfigFromClusterParams: p,

		kubeCl:             kubeCl,
		preparatorProvider: DummyPreparatorProvider(),
	}
}

func TestParseConfigFromData_MergedDocuments(t *testing.T) {
	t.Run("Should detect missing separator between InitConfiguration and ModuleConfig", func(t *testing.T) {
		// This reproduces the issue from https://github.com/deckhouse/deckhouse/issues/14009
		// When --- separator is commented out, documents get merged
		configWithCommentedSeparator := `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: test:EE
  registryDockerCfg: test
# ---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    releaseChannel: Alpha
    logLevel: Info
    update:
      mode: Manual
---
`

		_, err := ParseConfigFromData(t.Context(), configWithCommentedSeparator, DummyPreparatorProvider(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing '---' separator")
		require.Contains(t, err.Error(), "InitConfiguration")
		require.Contains(t, err.Error(), "ModuleConfig")
	})

	t.Run("Should detect missing separator with multiple apiVersion fields", func(t *testing.T) {
		configWithoutSeparator := `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: test:EE
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
---
`

		_, err := ParseConfigFromData(t.Context(), configWithoutSeparator, DummyPreparatorProvider(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing '---' separator")
	})

	t.Run("Should allow valid config with proper separators", func(t *testing.T) {
		validConfig := `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ee
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
---
`

		metaConfig, err := ParseConfigFromData(t.Context(), validConfig, DummyPreparatorProvider(), nil)
		require.NoError(t, err)
		require.NotNil(t, metaConfig)
		require.NotEmpty(t, metaConfig.InitClusterConfig)
	})

	t.Run("Should allow comments with kind in them", func(t *testing.T) {
		configWithComment := `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  # This is a comment with kind: something
---
`

		metaConfig, err := ParseConfigFromData(t.Context(), configWithComment, DummyPreparatorProvider(), nil)
		require.NoError(t, err)
		require.NotNil(t, metaConfig)
	})
}

func TestRegistryConfigProvider(t *testing.T) {
	t.Run("Parse mocks config paths with wildcard", func(t *testing.T) {
		docs, err := FetchDocuments([]string{"./mocks/*.yml", "./mocks/3-ModuleConfig.yaml"})
		require.NoError(t, err)
		provider, err := RegistryConfigProvider(docs)
		require.NoError(t, err)

		remote, err := provider.RemoteData()
		require.NoError(t, err)
		require.Equal(t, "registry.deckhouse.io/deckhouse/ce", remote.ImagesRepo)
		require.Equal(t, registry_const.SchemeHTTPS, remote.Scheme)
		require.Equal(t, "", remote.Username)
		require.Equal(t, "", remote.Password)
		require.Equal(t, "", remote.CA)

		isLocal, err := provider.IsLocal()
		require.NoError(t, err)
		require.Equal(t, false, isLocal)
	})

	t.Run("Parse raw Deckhouse ModuleConfig", func(t *testing.T) {
		mcDeckhouse := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: r.example.com/test
        username: test-user
        password: test-password
        scheme: HTTPS
        ca: "-----BEGIN CERTIFICATE-----"
  version: 1
`

		provider, err := RegistryConfigProvider([]string{mcDeckhouse})
		require.NoError(t, err)

		remote, err := provider.RemoteData()
		require.NoError(t, err)
		require.Equal(t, "r.example.com/test", remote.ImagesRepo)
		require.Equal(t, registry_const.SchemeHTTPS, remote.Scheme)
		require.Equal(t, "test-user", remote.Username)
		require.Equal(t, "test-password", remote.Password)
		require.Equal(t, "-----BEGIN CERTIFICATE-----", remote.CA)

		isLocal, err := provider.IsLocal()
		require.NoError(t, err)
		require.Equal(t, false, isLocal)
	})

	t.Run("Parse raw InitConfig", func(t *testing.T) {
		initConfig := `
---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
deckhouse:
  imagesRepo: r.example.com/test
  registryScheme: HTTPS
  registryCA: "-----BEGIN CERTIFICATE-----"
`

		provider, err := RegistryConfigProvider([]string{initConfig})
		require.NoError(t, err)

		remote, err := provider.RemoteData()
		require.NoError(t, err)
		require.Equal(t, "r.example.com/test", remote.ImagesRepo)
		require.Equal(t, registry_const.SchemeHTTPS, remote.Scheme)
		require.Equal(t, "", remote.Username)
		require.Equal(t, "", remote.Password)
		require.Equal(t, "-----BEGIN CERTIFICATE-----", remote.CA)

		isLocal, err := provider.IsLocal()
		require.NoError(t, err)
		require.Equal(t, false, isLocal)
	})
}

func TestFetchDocuments(t *testing.T) {
	t.Run("Parse init config path", func(t *testing.T) {
		initConfig := `---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce`

		docs, err := FetchDocuments([]string{"./mocks/1-Init*.yml"})
		require.NoError(t, err)
		require.Len(t, docs, 2)

		require.Equal(t, "", docs[0])
		require.Equal(t, initConfig, docs[1])
	})

	t.Run("Parse all yml config paths", func(t *testing.T) {
		docs, err := FetchDocuments([]string{"./mocks/*.yml", "./mocks/3-ModuleConfig.yaml"})
		require.NoError(t, err)
		require.Len(t, docs, 6)
		require.Equal(t, "", docs[0])
	})
}

func testCreateKubeSystemSecret(t *testing.T, kubeCl *client.KubernetesClient, name string, data map[string][]byte) {
	t.Helper()

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: global.ConfigsNS,
		},
		Data: data,
	}

	_, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).Create(t.Context(), secret, metav1.CreateOptions{})
	require.NoError(t, err)
}

// testCreateDeckhouseRegistrySecret seeds the d8-system/deckhouse-registry
// Secret that registrydata.GetRegistryData looks up unconditionally for
// Cloud clusters. Tests that hit parseConfigFromCluster on a Cloud
// ClusterConfiguration must call this helper, otherwise the test hangs on
// the retry-loop until the go-test timeout fires.
func testCreateDeckhouseRegistrySecret(t *testing.T, kubeCl *client.KubernetesClient) {
	t.Helper()

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deckhouse-registry",
			Namespace: "d8-system",
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
			"imagesRegistry":    []byte("registry.example.com/deckhouse"),
			"scheme":            []byte("HTTPS"),
		},
	}

	_, err := kubeCl.CoreV1().Secrets("d8-system").Create(t.Context(), secret, metav1.CreateOptions{})
	require.NoError(t, err)
}
