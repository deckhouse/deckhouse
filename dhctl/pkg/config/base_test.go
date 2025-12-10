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
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func TestParseConfigFromData(t *testing.T) {
	clusterConfig := `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.30"
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
				metaConfig, err := ParseConfigFromData(context.TODO(), initConfig, DummyPreparatorProvider())
				require.NoError(t, err)
				require.Equal(t, metaConfig.Registry.LegacyMode, true)
				require.Equal(t, metaConfig.Registry.Settings.Mode, registry_const.ModeUnmanaged)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, registry.ImagesRepo, "test")
				require.Equal(t, registry.Scheme, registry_const.SchemeHTTPS)
				require.Equal(t, registry.Username, "")
				require.Equal(t, registry.Password, "")
				require.Equal(t, registry.CA, "")
			})
			t.Run("With CRI (module enable)", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(context.TODO(), initConfig+clusterConfig, DummyPreparatorProvider())
				require.NoError(t, err)
				require.Equal(t, metaConfig.Registry.LegacyMode, true)
				require.Equal(t, metaConfig.Registry.Settings.Mode, registry_const.ModeUnmanaged)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, registry.ImagesRepo, "test")
				require.Equal(t, registry.Scheme, registry_const.SchemeHTTPS)
				require.Equal(t, registry.Username, "")
				require.Equal(t, registry.Password, "")
				require.Equal(t, registry.CA, "")
			})
		})
		t.Run("Default -> CE edition registry", func(t *testing.T) {
			t.Run("Without CRI (module disable) -> unmanaged && legacy", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(context.TODO(), "", DummyPreparatorProvider())
				require.NoError(t, err)
				require.Equal(t, metaConfig.Registry.LegacyMode, true)
				require.Equal(t, metaConfig.Registry.Settings.Mode, registry_const.ModeUnmanaged)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, registry.ImagesRepo, "registry.deckhouse.io/deckhouse/ce")
				require.Equal(t, registry.Scheme, registry_const.SchemeHTTPS)
				require.Equal(t, registry.Username, "")
				require.Equal(t, registry.Password, "")
				require.Equal(t, registry.CA, "")
			})
			t.Run("With CRI (module enable) -> direct && not legacy", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(context.TODO(), ""+clusterConfig, DummyPreparatorProvider())
				require.NoError(t, err)
				require.Equal(t, metaConfig.Registry.LegacyMode, false)
				require.Equal(t, metaConfig.Registry.Settings.Mode, registry_const.ModeDirect)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, registry.ImagesRepo, "registry.deckhouse.io/deckhouse/ce")
				require.Equal(t, registry.Scheme, registry_const.SchemeHTTPS)
				require.Equal(t, registry.Username, "")
				require.Equal(t, registry.Password, "")
				require.Equal(t, registry.CA, "")
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
        imagesRepo: r.example.com/test/
        username: test-user
        password: test-password
        scheme: HTTPS
        ca: "-----BEGIN CERTIFICATE-----"
  version: 1
`
			t.Run("Without CRI (module disable) -> error", func(t *testing.T) {
				_, err := ParseConfigFromData(context.TODO(), moduleConfigDeckhouse, DummyPreparatorProvider())
				require.Error(t, err)
			})
			t.Run("With CRI (module enable) -> from moduleConfig && not legacy", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(context.TODO(), moduleConfigDeckhouse+clusterConfig, DummyPreparatorProvider())
				require.NoError(t, err)
				require.Equal(t, metaConfig.Registry.LegacyMode, false)
				require.Equal(t, metaConfig.Registry.Settings.Mode, registry_const.ModeUnmanaged)
				registry := metaConfig.Registry.Settings.RemoteData
				require.Equal(t, registry.ImagesRepo, "r.example.com/test")
				require.Equal(t, registry.Scheme, registry_const.SchemeHTTPS)
				require.Equal(t, registry.Username, "test-user")
				require.Equal(t, registry.Password, "test-password")
				require.Equal(t, registry.CA, "-----BEGIN CERTIFICATE-----")
			})
		})
	})

	t.Run("Standard Static", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig, DummyPreparatorProvider())
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
		metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig, DummyPreparatorProvider())
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
			metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+moduleConfigGlobalValid, DummyPreparatorProvider())
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Global invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+moduleConfigGlobalInvalid, DummyPreparatorProvider())
			require.Error(t, err)
		})

		t.Run("Module valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+moduleConfigCommonValid, DummyPreparatorProvider())

			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Module invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+moduleConfigCommonInvalid, DummyPreparatorProvider())
			require.Error(t, err)
		})

		t.Run("Module without enabled field", func(t *testing.T) {
			_, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+moduleConfigCommonWithoutEnabled, DummyPreparatorProvider())
			require.Error(t, err)
		})

		t.Run("Module without settings", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+moduleConfigCommonWithoutSettings, DummyPreparatorProvider())
			require.NoError(t, err)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Unknown module should move into resources", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+unknownModuleConfig, DummyPreparatorProvider())
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)
		})
	})

	t.Run("Config with another k8s resources eg configMap", func(t *testing.T) {
		t.Run("Should move another resources into resourcesYAML", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+configMapAndInstanceClass, DummyPreparatorProvider())
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
			metaConfig, err := ParseConfigFromData(context.TODO(), clusterConfig+initConfig+staticConfig+ngWithTemplating, DummyPreparatorProvider())
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
	imagesDigestsJSON = "./mocks/images_digests.json"
	app.VersionFile = "./mocks/version"
	t.Run("parse wildcard", func(t *testing.T) {
		metaConfig, err := LoadConfigFromFile(context.TODO(), []string{"./mocks/*.yml", "./mocks/3-ModuleConfig.yaml"}, DummyPreparatorProvider())
		require.NoError(t, err)
		require.Equal(t, "Static", metaConfig.ClusterType)

		t.Run("Registry CE edition config", func(t *testing.T) {
			registry := metaConfig.Registry.Settings.RemoteData
			require.Equal(t, registry.ImagesRepo, "registry.deckhouse.io/deckhouse/ce")
			require.Equal(t, registry.Scheme, registry_const.SchemeHTTPS)
			require.Equal(t, registry.Username, "")
			require.Equal(t, registry.Password, "")
			require.Equal(t, registry.CA, "")
		})

		require.Len(t, metaConfig.ModuleConfigs, 3)
	})
}

func TestParseConfigFromCluster(t *testing.T) {
	doParseFromClusterNoError := func(t *testing.T, tst *testParseConfigFromCluster) *MetaConfig {
		metaConfig, err := parseConfigFromCluster(context.TODO(), tst.kubeCl, tst.preparatorProvider)

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
		metaConfig, err := parseConfigFromCluster(context.TODO(), tst.kubeCl, tst.preparatorProvider)

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
	})

}

type testParseConfigFromClusterParams struct {
	clusterConfig string
	clusterType   string
}

type testParseConfigFromCluster struct {
	testParseConfigFromClusterParams

	kubeCl             *client.KubernetesClient
	preparatorProvider MetaConfigPreparatorProvider
}

func createTestParseConfigFromCluster(t *testing.T, p testParseConfigFromClusterParams) *testParseConfigFromCluster {
	kubeCl := client.NewFakeKubernetesClient()

	if p.clusterConfig != "" {
		testCreateKubeSystemSecret(t, kubeCl, "d8-cluster-configuration", map[string][]byte{
			"cluster-configuration.yaml": []byte(p.clusterConfig),
		})
	}

	return &testParseConfigFromCluster{
		testParseConfigFromClusterParams: p,

		kubeCl:             kubeCl,
		preparatorProvider: DummyPreparatorProvider(),
	}
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

	_, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).Create(context.TODO(), secret, metav1.CreateOptions{})
	require.NoError(t, err)
}
