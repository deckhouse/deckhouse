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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func TestParseConfigFromData(t *testing.T) {
	clusterConfig := `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.33"
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
				metaConfig, err := ParseConfigFromData(t.Context(), initConfig, DummyValidatorProvider(), nil)
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
				metaConfig, err := ParseConfigFromData(t.Context(), initConfig+clusterConfig, DummyValidatorProvider(), nil)
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
				metaConfig, err := ParseConfigFromData(t.Context(), "", DummyValidatorProvider(), nil)
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
				metaConfig, err := ParseConfigFromData(t.Context(), ""+clusterConfig, DummyValidatorProvider(), nil)
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
			// No ClusterConfiguration, and therefore no defaultCRI to check — which is not an error any
			// more. A cluster whose control plane dhctl did not create declares no runtime and dhctl
			// configures none of its nodes, so the runtime the registry module needs is not dhctl's to
			// validate; see `ConfigProvider.Config` and its `hasClusterConfiguration` parameter.
			//
			// This case used to require an error, from before that parameter existed. What it asserts now
			// is the contract as it stands: the configuration is read, and it comes from the ModuleConfig
			// rather than from the legacy contour.
			t.Run("Without a ClusterConfiguration the CRI is not dhctl's to check", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(t.Context(), moduleConfigDeckhouse, DummyValidatorProvider(), nil)
				require.NoError(t, err)
				require.False(t, metaConfig.Registry.LegacyMode)
				require.Equal(t, registry_const.ModeUnmanaged, metaConfig.Registry.Settings.Mode)
				require.Equal(t, "r.example.com/test", metaConfig.Registry.Settings.RemoteData.ImagesRepo)
			})
			t.Run("With CRI (module enable) -> from moduleConfig && not legacy", func(t *testing.T) {
				metaConfig, err := ParseConfigFromData(t.Context(), moduleConfigDeckhouse+clusterConfig, DummyValidatorProvider(), nil)
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
		metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig, DummyValidatorProvider(), nil)
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
		metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig, DummyValidatorProvider(), nil)
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
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigGlobalValid, DummyValidatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Global invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigGlobalInvalid, DummyValidatorProvider(), nil)
			require.Error(t, err)
		})

		t.Run("Module valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonValid, DummyValidatorProvider(), nil)

			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Module invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonInvalid, DummyValidatorProvider(), nil)
			require.Error(t, err)
		})

		t.Run("Module without enabled field", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonWithoutEnabled, DummyValidatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)
			require.Nil(t, metaConfig.ModuleConfigs[0].Spec.Enabled, "the field must reach the cluster as written")
		})

		t.Run("Module without settings", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+moduleConfigCommonWithoutSettings, DummyValidatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Unknown module should move into resources", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+unknownModuleConfig, DummyValidatorProvider(), nil)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)
		})
	})

	t.Run("Config with another k8s resources eg configMap", func(t *testing.T) {
		t.Run("Should move another resources into resourcesYAML", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+configMapAndInstanceClass, DummyValidatorProvider(), nil)
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
			metaConfig, err := ParseConfigFromData(t.Context(), clusterConfig+initConfig+staticConfig+ngWithTemplating, DummyValidatorProvider(), nil)
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
		metaConfig, err := LoadConfigFromFile(t.Context(), []string{"./mocks/*.yml", "./mocks/3-ModuleConfig.yaml"}, DummyValidatorProvider(), &options.GlobalOptions{})
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
	yandexCandiDir := tests.RequireProviderCandiDir(t, "yandex")

	// An empty GlobalOptions makes withDownloadDir fall back to options.DefaultTmpDir(), the
	// directory a real dhctl run downloads provider bundles into. The stubbed download below
	// would then leave a fake bundle there for the next real run to pick up, so pin the
	// download dir to a per-test directory the framework removes afterwards.
	downloadDir := t.TempDir()
	globalOptions := func() *options.GlobalOptions {
		return &options.GlobalOptions{DownloadDir: downloadDir}
	}

	doParseFromClusterNoError := func(t *testing.T, tst *testParseConfigFromCluster) *MetaConfig {
		metaConfig, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.validatorProvider, globalOptions(), "")

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
		metaConfig, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.validatorProvider, globalOptions(), "")

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
kubernetesVersion: "1.33"
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
kubernetesVersion: "1.33"
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
kubernetesVersion: "1.33"
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
kubernetesVersion: "1.33"
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
		// Yandex ships no schemas in candi, so a bundle has to be delivered. Stub the
		// resolve+download vars instead of hitting the registry, copying the real schema from
		// the provider's candi so these tests keep exercising actual
		// YandexClusterConfiguration validation.
		origDigest := resolveProviderBundleRef
		resolveProviderBundleRef = func(_ context.Context, _ string, _ providerModuleLookup, _ *options.GlobalOptions) (providerBundleRef, error) {
			return providerBundleRef{Digest: "sha256:test-yandex-digest"}, nil
		}
		t.Cleanup(func() { resolveProviderBundleRef = origDigest })

		origDownload := downloadProviderBundle
		downloadProviderBundle = func(_ context.Context, _, dest string, _ image.RegistryConfig, _ bool) error {
			schema, err := os.ReadFile(filepath.Join(yandexCandiDir, "openapi", "cluster_configuration.yaml"))
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Join(dest, "openapi"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dest, "openapi", "cluster_configuration.yaml"), schema, 0o644)
		}
		t.Cleanup(func() { downloadProviderBundle = origDownload })

		writeDeliveredYandexBundle := func(t *testing.T, downloadDir, digest string) {
			t.Helper()
			digestDir := providerdir.DigestDir(downloadDir, "yandex", digest)
			schema, err := os.ReadFile(filepath.Join(yandexCandiDir, "openapi", "cluster_configuration.yaml"))
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Join(digestDir, "openapi"), 0o755))
			require.NoError(t, os.WriteFile(providerdir.SchemaPath(digestDir), schema, 0o644))
		}

		clusterGenericConfig := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "test"
kubernetesVersion: "1.33"
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
				{Group: "deckhouse.io", Version: "v1", Resource: "yandexinstanceclasses"}: "YandexInstanceClassList",
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

			_, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.validatorProvider, globalOptions(), "")
			require.Error(t, err)
			require.Contains(t, err.Error(), "ModuleConfig")
			require.Contains(t, err.Error(), "d8-provider-cluster-configuration")
		})

		// The lazy provider-plugin and terraform-manager pulls read these off DeckhouseConfig, and
		// they happen long after the parse, so a cloud cluster needs them whether or not anything
		// was downloaded here.
		t.Run("cloud cluster reads registry data even when candi is already available", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)
			testCreateCloudProviderModuleConfig(t, tst.kubeCl, "yandex")

			opts := globalOptions()
			opts.EnsureCandiAvailable = false
			metaConfig, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.validatorProvider, opts, "")
			require.NoError(t, err)
			require.NotEmpty(t, metaConfig.DeckhouseConfig.RegistryDockerCfg)
			require.NotEmpty(t, metaConfig.DeckhouseConfig.ImagesRepo)
		})

		t.Run("the bundle dir of the resolved digest travels with the MetaConfig", func(t *testing.T) {
			tst := createTestParseConfigFromCluster(t, testParams)
			testCreateCloudProviderModuleConfig(t, tst.kubeCl, "yandex")

			opts := globalOptions()
			opts.EnsureCandiAvailable = false
			metaConfig, err := parseConfigFromCluster(t.Context(), tst.kubeCl, tst.validatorProvider, opts, "")
			require.NoError(t, err)
			require.Equal(t, providerdir.DigestDir(opts.DownloadDir, "yandex", "sha256:test-yandex-digest"), metaConfig.ProviderBundleDir)
		})

		t.Run("a registry secret that cannot be read fails every operation, destroy included", func(t *testing.T) {
			const digest = "sha256:registry-unreachable-ready"

			tst := createTestParseConfigFromCluster(t, testParams)
			testCreateCloudProviderModuleConfig(t, tst.kubeCl, "yandex")

			secret, err := tst.kubeCl.CoreV1().Secrets("d8-system").Get(t.Context(), "deckhouse-registry", metav1.GetOptions{})
			require.NoError(t, err)
			secret.Data[".dockerconfigjson"] = []byte("not-json")
			_, err = tst.kubeCl.CoreV1().Secrets("d8-system").Update(t.Context(), secret, metav1.UpdateOptions{})
			require.NoError(t, err)

			origDigest := resolveProviderBundleRef
			resolveProviderBundleRef = func(context.Context, string, providerModuleLookup, *options.GlobalOptions) (providerBundleRef, error) {
				return providerBundleRef{Digest: digest}, nil
			}
			t.Cleanup(func() { resolveProviderBundleRef = origDigest })

			opts := globalOptions()
			opts.DownloadDir = t.TempDir()
			writeDeliveredYandexBundle(t, opts.DownloadDir, digest)

			for _, operation := range []string{"converge", "destroy"} {
				_, err = parseConfigFromCluster(t.Context(), tst.kubeCl, tst.validatorProvider, opts, operation)
				require.ErrorContains(t, err, "read registry data from cluster", operation)
			}
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

	kubeCl            *client.KubernetesClient
	validatorProvider MetaConfigValidatorProvider
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

		kubeCl:            kubeCl,
		validatorProvider: DummyValidatorProvider(),
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

		_, err := ParseConfigFromData(t.Context(), configWithCommentedSeparator, DummyValidatorProvider(), nil)
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

		_, err := ParseConfigFromData(t.Context(), configWithoutSeparator, DummyValidatorProvider(), nil)
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

		metaConfig, err := ParseConfigFromData(t.Context(), validConfig, DummyValidatorProvider(), nil)
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

		metaConfig, err := ParseConfigFromData(t.Context(), configWithComment, DummyValidatorProvider(), nil)
		require.NoError(t, err)
		require.NotNil(t, metaConfig)
	})
}

func TestRegistryConfigProvider(t *testing.T) {
	t.Run("Parse mocks config paths with wildcard", func(t *testing.T) {
		docs, err := FetchDocuments(t.Context(), []string{"./mocks/*.yml", "./mocks/3-ModuleConfig.yaml"})
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

		docs, err := FetchDocuments(t.Context(), []string{"./mocks/1-Init*.yml"})
		require.NoError(t, err)
		require.Len(t, docs, 2)

		require.Equal(t, "", docs[0])
		require.Equal(t, initConfig, docs[1])
	})

	t.Run("Parse all yml config paths", func(t *testing.T) {
		docs, err := FetchDocuments(t.Context(), []string{"./mocks/*.yml", "./mocks/3-ModuleConfig.yaml"})
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

// TestRegistryConfigProviderBundleBootstrap covers installing a cluster whose images come from a
// bundle, recognised from the registry module's own ModuleConfig rather than from the deckhouse one.
//
// The third case is the reason this exists at all. Everything that serves images during such an
// installation — the temporary registry on the first master, the candi schemas, the provider plugins —
// comes from one local registry reached through a reverse tunnel, and none of that depends on the
// cluster being static. The refusal that used to stand in the way belongs to the legacy modes and is
// left untouched for them.
func TestRegistryConfigProviderBundleBootstrap(t *testing.T) {
	const mcRegistryCacheNoUpstream = `
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
    storage:
      cache: true
      size: 10Gi
`

	const mcRegistryWithUpstream = `
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
    storage:
      cache: true
      size: 10Gi
`

	t.Run("a cache with no upstream installs from the bundle", func(t *testing.T) {
		provider, err := RegistryConfigProvider([]string{mcRegistryCacheNoUpstream})
		require.NoError(t, err)

		isLocal, err := provider.IsLocal()
		require.NoError(t, err)
		require.True(t, isLocal, "a cache with nothing to fill it from over the network is a bundle install")

		remote, err := provider.RemoteData()
		require.NoError(t, err)
		require.Equal(t, registry_const.BundleImagesRepo, remote.ImagesRepo,
			"the images must be addressed at the local bundle registry, which is what the tunnel serves")
	})

	t.Run("a cache filled from an upstream is an ordinary install", func(t *testing.T) {
		provider, err := RegistryConfigProvider([]string{mcRegistryWithUpstream})
		require.NoError(t, err)

		isLocal, err := provider.IsLocal()
		require.NoError(t, err)
		require.False(t, isLocal)
	})

	t.Run("a cloud cluster is allowed to install from a bundle", func(t *testing.T) {
		provider, err := RegistryConfigProvider([]string{mcRegistryCacheNoUpstream})
		require.NoError(t, err)

		// isStatic=false is the whole point: this used to be refused outright with "supported only in
		// a static cluster", and every cluster this is tested on is a cloud one.
		_, err = provider.Config(registry_const.CRIContainerdV2, false, true)
		require.NoError(t, err, "installing from a bundle must not require a static cluster")
	})

	t.Run("the deckhouse ModuleConfig keeps precedence", func(t *testing.T) {
		const mcDeckhouseUnmanaged = `
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
        imagesRepo: r.example.com/test
        scheme: HTTPS
`

		provider, err := RegistryConfigProvider([]string{mcDeckhouseUnmanaged, mcRegistryCacheNoUpstream})
		require.NoError(t, err)

		isLocal, err := provider.IsLocal()
		require.NoError(t, err)
		require.False(t, isLocal, "an explicit legacy configuration must not be overridden by inference")
	})
}

// TestAConfigurationThatNamesNoRegistryOutsideTheModule is the shape the air-gap test actually uses,
// and the reason it is honest.
//
// `InitConfiguration.deckhouse` keeps only the version to install; the registry is stated once, in the
// object that owns the pull path. Before the installer could read it there, this configuration
// resolved to the public CE registry — so a test that removed those fields to prove the module stands
// on its own proved the opposite of what it claimed, quietly.
func TestAConfigurationThatNamesNoRegistryOutsideTheModule(t *testing.T) {
	const initWithoutRegistry = `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  devBranch: "pr21788"
`
	const bundleModule = `
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
    storage:
      cache: true
      size: 20Gi
      source:
        bundleRef: d8-mirror-bundle
        expectedDigests: 556
`

	provider, err := RegistryConfigProvider([]string{initWithoutRegistry, bundleModule})
	require.NoError(t, err)

	isLocal, err := provider.IsLocal()
	require.NoError(t, err)
	require.True(t, isLocal, "a cache with nothing to fill it from over the network is a bundle install")

	remote, err := provider.RemoteData()
	require.NoError(t, err)
	assert.Equal(t, registry_const.BundleImagesRepo, remote.ImagesRepo,
		"the images come from the bundle the installer serves, not from a registry nobody named")

	// And a cloud cluster is allowed to install this way, which every variant of the test matrix is.
	_, err = provider.Config(registry_const.CRIContainerdV2, false, true)
	require.NoError(t, err)
}

// TestTheRegistryIsTakenFromTheModuleWhenAnUpstreamIsStated is the same rule for every other variant
// where the module is enabled: the upstream stated in the module's own configuration is what the
// installer uses.
func TestTheRegistryIsTakenFromTheModuleWhenAnUpstreamIsStated(t *testing.T) {
	const initWithoutRegistry = `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  devBranch: "pr21788"
`
	const managedModule = `
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
        host: dev-registry.deckhouse.io
        path: /sys/deckhouse-oss
        # Under auth, which is where the module's own schema puts them. This fixture stated them
        # one level up, and it passed for as long as nothing read them.
        auth:
          username: someone
          password: secret
    storage:
      cache: true
      size: 10Gi
`

	provider, err := RegistryConfigProvider([]string{initWithoutRegistry, managedModule})
	require.NoError(t, err)

	remote, err := provider.RemoteData()
	require.NoError(t, err)
	assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss", remote.ImagesRepo,
		"the installer pulls from where the module says, not from the public registry")
	assert.Equal(t, "someone", remote.Username)
	assert.Equal(t, "secret", remote.Password)
}

// The mc-flow bootstrap path in one piece: NodeGroup documents have no schema,
// so they land in ResourcesYAML, from where Prepare must derive the typed node
// group fields. Without them dhctl bootstraps a single master and no workers.
func TestParseConfigFromDataCloudDerivesNodeGroups(t *testing.T) {
	config := `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: DVP
  prefix: test
kubernetesVersion: "Automatic"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
   imagesRepo: test
   devBranch: test
   # {"auths": { "test": {}}}
   registryDockerCfg: eyJhdXRocyI6IHsgInRlc3QiOiB7fX19
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    minPerZone: 1
    classReference:
      kind: DVPInstanceClass
      name: master-dvp
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudPermanent
  cloudInstances:
    minPerZone: 1
    classReference:
      kind: DVPInstanceClass
      name: worker-dvp
`

	metaConfig, err := ParseConfigFromData(t.Context(), config, DummyValidatorProvider(), nil)
	require.NoError(t, err)

	require.Equal(t, 1, metaConfig.MasterNodeGroupSpec.Replicas)
	require.Len(t, metaConfig.TerraNodeGroupSpecs, 1)
	require.Equal(t, "worker", metaConfig.TerraNodeGroupSpecs[0].Name)
	require.Equal(t, 1, metaConfig.TerraNodeGroupSpecs[0].Replicas)
}

func TestApplyRegistryToDeckhouseConfigPopulatesDecodableDockerCfg(t *testing.T) {
	// Commander cold-pod ops carry registry creds in a separate registryConfig
	// doc, not in the cluster config; the lazy provider-plugin download then
	// decodes DeckhouseConfig.RegistryDockerCfg. An empty value used to crash
	// with "unmarshaling dockerconfig JSON: unexpected end of JSON input".
	metaConfig := &MetaConfig{}
	require.NoError(t, applyRegistryToDeckhouseConfig(metaConfig, []string{ensureRegistryMCDoc}))

	require.NotEmpty(t, metaConfig.DeckhouseConfig.RegistryDockerCfg, "dockercfg must be populated from registry MC")
	require.Equal(t, "r.example.com/test", metaConfig.DeckhouseConfig.ImagesRepo)
	require.True(t, strings.EqualFold("HTTPS", metaConfig.DeckhouseConfig.RegistryScheme))

	// The exact round-trip the lazy image download performs must succeed.
	dc, err := image.DecodeDockerConfig(metaConfig.DeckhouseConfig.RegistryDockerCfg)
	require.NoError(t, err)
	rc, err := image.RegistryConfigFromDockerConfig(dc, "HTTPS", metaConfig.DeckhouseConfig.ImagesRepo)
	require.NoError(t, err)
	require.NotNil(t, rc)
}

func TestApplyRegistryToDeckhouseConfigKeepsExisting(t *testing.T) {
	metaConfig := &MetaConfig{}
	metaConfig.DeckhouseConfig.RegistryDockerCfg = "preset"
	require.NoError(t, applyRegistryToDeckhouseConfig(metaConfig, []string{ensureRegistryMCDoc}))
	require.Equal(t, "preset", metaConfig.DeckhouseConfig.RegistryDockerCfg, "must not clobber dockercfg already supplied by configData")
}

// candi left in the download dir by another dhctl version must not be read by this one, and the
// templates of the candi it does unpack must name the directory they end up in.
func TestPrepareCandiDirUnpacksTheCandiOfThisInstaller(t *testing.T) {
	const digest = "sha256:4444444444444444444444444444444444444444444444444444444444444444"

	stubEmbeddedDigests(t, `{"common": {"candi": "`+digest+`"}}`)

	downloadDir := t.TempDir()
	oldMarker := filepath.Join(downloadDir, "deckhouse", "candi", "marker")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldMarker), 0o755))
	require.NoError(t, os.WriteFile(oldMarker, []byte("old"), 0o644))

	var downloads atomic.Int32
	orig := downloadCandi
	downloadCandi = func(_ context.Context, _, dir string, _ image.RegistryConfig, _ bool) error {
		downloads.Add(1)
		bashible := filepath.Join(dir, "deckhouse", "candi", "bashible")
		if err := os.MkdirAll(bashible, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "deckhouse", "candi", "marker"), []byte("new"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(bashible, "step.sh.tpl"), []byte("source deckhouse/candi/bashible/lib.sh\n"), 0o644)
	}
	t.Cleanup(func() { downloadCandi = orig })

	globalOptions := &options.GlobalOptions{DownloadDir: downloadDir}
	options.ResolveAndApplyPaths(globalOptions)

	conf, err := image.NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
	require.NoError(t, err)

	require.NoError(t, prepareCandiDir(t.Context(), conf, globalOptions))
	require.NoError(t, prepareCandiDir(t.Context(), conf, globalOptions))
	require.Equal(t, int32(1), downloads.Load(), "an unpacked candi digest is not downloaded again")

	marker, err := os.ReadFile(filepath.Join(globalOptions.CandiDir, "marker"))
	require.NoError(t, err)
	require.Equal(t, "new", string(marker))

	tpl, err := os.ReadFile(filepath.Join(globalOptions.CandiDir, "bashible", "step.sh.tpl"))
	require.NoError(t, err)
	require.Equal(t, "source "+filepath.Join(globalOptions.CandiDir, "bashible")+"/lib.sh\n", string(tpl))
}

func TestLoadInstallerVersionFallsBackToTheUnpackedCandi(t *testing.T) {
	const digest = "sha256:5555555555555555555555555555555555555555555555555555555555555555"

	stubEmbeddedDigests(t, `{"common": {"candi": "`+digest+`"}}`)

	downloadDir := t.TempDir()
	versionFile := filepath.Join(downloadDir, "candi@"+digest, "deckhouse", "version")
	require.NoError(t, os.MkdirAll(filepath.Dir(versionFile), 0o755))
	require.NoError(t, os.WriteFile(versionFile, []byte("v1.77.0\n"), 0o644))

	m := &MetaConfig{VersionFilePath: filepath.Join(t.TempDir(), "absent"), DownloadRootDir: downloadDir}
	require.NoError(t, m.LoadInstallerVersion())
	require.Equal(t, "v1.77.0", m.InstallerVersion)
}

// Paths rooted anywhere but the candi digest directory are refused before anything is pulled.
func TestPrepareCandiDirRefusesMisrootedPathsBeforeDownload(t *testing.T) {
	const digest = "sha256:6666666666666666666666666666666666666666666666666666666666666666"

	stubEmbeddedDigests(t, `{"common": {"candi": "`+digest+`"}}`)

	var downloads atomic.Int32
	orig := downloadCandi
	downloadCandi = func(_ context.Context, _, dir string, _ image.RegistryConfig, _ bool) error {
		downloads.Add(1)
		return os.MkdirAll(filepath.Join(dir, "deckhouse"), 0o755)
	}
	t.Cleanup(func() { downloadCandi = orig })

	downloadDir := t.TempDir()
	globalOptions := &options.GlobalOptions{DownloadDir: downloadDir}
	options.SetPaths(downloadDir, globalOptions)

	conf, err := image.NewRegistryConfig("HTTPS", "r.example.com/test", "u", "p", "")
	require.NoError(t, err)

	err = prepareCandiDir(t.Context(), conf, globalOptions)
	require.ErrorContains(t, err, "but the install paths are rooted at "+filepath.Join(downloadDir, "deckhouse")+": resolve the paths again after changing the download dir")
	require.Equal(t, int32(0), downloads.Load())
}

// ParseConfig is the whole parse of "bootstrap-phase install-deckhouse" and "deckhouse
// create-deployment", and both read the installer version off the MetaConfig it returns.
func TestParseConfigLocatesTheInstallerVersion(t *testing.T) {
	deckhouseDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(deckhouseDir, "version"), []byte("v1.77.0\n"), 0o644))

	path := writeConfig(t, `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.33"
clusterDomain: cluster.local
`)

	metaConfig, err := ParseConfig(t.Context(), []string{path}, DummyValidatorProvider(), &options.GlobalOptions{DeckhouseDir: deckhouseDir})
	require.NoError(t, err)

	require.NoError(t, metaConfig.LoadInstallerVersion())
	require.Equal(t, "v1.77.0", metaConfig.InstallerVersion)
	require.Equal(t, options.DefaultTmpDir(), metaConfig.DownloadRootDir)
}

// Without a candi digest there is no fallback path to name: the error carries the cause of each read.
func TestLoadInstallerVersionErrorCarriesEachRead(t *testing.T) {
	stubEmbeddedDigests(t, `{"something": {"app": "sha256:x"}}`)

	absent := filepath.Join(t.TempDir(), "absent")
	m := &MetaConfig{VersionFilePath: absent, DownloadRootDir: t.TempDir()}

	err := m.LoadInstallerVersion()
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.EqualError(t, err, "read installer version: open "+absent+": no such file or directory\n"+
		"get candi image digest: Image digests section 'common' not found or empty")
}

const yandexCompleteConfig = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "test"
kubernetesVersion: "1.33"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
clusterDomain: cluster.local
podSubnetNodeCIDRPrefix: "24"
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: test
  # {"auths": { "test": {}}}
  registryDockerCfg: eyJhdXRocyI6IHsgInRlc3QiOiB7fX19
---
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

// Every consumer of the bundle (validator, cni-bootstrap, layouts, plugin) reads it off the MetaConfig.
func TestParseConfigHandsTheDeliveredBundleDirToTheMetaConfig(t *testing.T) {
	downloadDir := t.TempDir()
	bundleDir := tests.StubDeliveredProviderBundle(t, downloadDir, "yandex")

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(yandexCompleteConfig), 0o644))

	metaConfig, err := ParseConfig(t.Context(), []string{path}, DummyValidatorProvider(), &options.GlobalOptions{DownloadDir: downloadDir})
	require.NoError(t, err)
	require.Equal(t, bundleDir, metaConfig.ProviderBundleDir)
}

// The stdin path of "config parse cluster-configuration" hands no bundle dir in, so the parse
// prepares the bundle itself.
func TestParseConfigFromDataEnsureProviderHandsTheBundleDirToTheMetaConfig(t *testing.T) {
	downloadDir := t.TempDir()
	bundleDir := tests.StubDeliveredProviderBundle(t, downloadDir, "yandex")

	metaConfig, err := ParseConfigFromDataEnsureProvider(t.Context(), yandexCompleteConfig, DummyValidatorProvider(), &options.GlobalOptions{DownloadDir: downloadDir})
	require.NoError(t, err)
	require.Equal(t, bundleDir, metaConfig.ProviderBundleDir)
}
