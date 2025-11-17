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

package check

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestCheckClusterConfig(t *testing.T) {
	const (
		k8sVersionOld    = "1.32"
		k8sVersionNew    = "1.33"
		staticClusterFmt = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "%s"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
clusterDomain: cluster.local
podSubnetNodeCIDRPrefix: "24"
`
		staticClusterConfigFmt = `
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.0.0/24
%s
`
		cloudConfigFmt = `
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
clusterDomain: "%s"
podSubnetNodeCIDRPrefix: "24"
`
		cloudClusterConfigFmt = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
masterNodeGroup:
  replicas: 1
  instanceClass:
    etcdDiskSizeGb: 10
    platform: standard-v2
    cores: %s
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
		clusterDomainOld = "cluster.local"
		clusterDomainNew = "new.local"
	)

	// need for prevent use same k8s version to avoid changing in drop k8s version support
	require.NotEqual(t, k8sVersionOld, k8sVersionNew)

	clusterUUIDOld, err := uuid.NewUUID()
	require.NoError(t, err)
	clusterUUIDNew, err := uuid.NewUUID()
	require.NoError(t, err)

	tests := []testCheckClusterConfigParams{
		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster without static cluster config equal",
				expectedSyncStatus: CheckStatusInSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDOld.String(),
		},

		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster with static cluster config equal",
				expectedSyncStatus: CheckStatusInSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),

			commanderStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "")),
			inClusterStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "")),

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDOld.String(),
		},

		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster without static cluster config not equal uuid",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDNew.String(),
		},

		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster without static cluster config not equal uuid and cluster",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionNew),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDNew.String(),
		},

		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster with static cluster config not equal uuid and cluster and conf",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionNew),

			commanderStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "")),
			inClusterStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "- 10.10.0.0/24")),

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDNew.String(),
		},

		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster with static cluster config not equal cluster and conf",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionNew),

			commanderStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "")),
			inClusterStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "- 10.10.0.0/24")),

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDOld.String(),
		},

		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster with static cluster config in commander but not in cluster not sync",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),

			commanderStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "")),
			inClusterStaticConfig: nil,

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDOld.String(),
		},

		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "static cluster without static cluster config in commander but has in cluster not sync",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.StaticClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),
			inClusterClusterConfig: fmt.Sprintf(staticClusterFmt, k8sVersionOld),

			commanderStaticConfig: nil,
			inClusterStaticConfig: pointer.String(fmt.Sprintf(staticClusterConfigFmt, "")),

			commanderCloudConfig: "",
			inClusterCloudConfig: "",

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDOld.String(),
		},
		// --- cloud ---
		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "cloud cluster sync",
				expectedSyncStatus: CheckStatusInSync,
				isError:            false,
				clusterType:        config.CloudClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),
			inClusterClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			// cores in master
			commanderCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),
			inClusterCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDOld.String(),
		},
		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "cloud cluster not sync with different cluster uuid",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.CloudClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),
			inClusterClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			// cores in master
			commanderCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),
			inClusterCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDNew.String(),
		},
		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "cloud cluster not sync with different cluster uuid and cluster conf",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.CloudClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),
			inClusterClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainNew),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			// cores in master
			commanderCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),
			inClusterCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDNew.String(),
		},
		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "cloud cluster not sync with different cluster uuid and cluster conf and provider",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.CloudClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),
			inClusterClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainNew),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			// cores in master
			commanderCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),
			inClusterCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "3"),

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDNew.String(),
		},
		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "cloud cluster not sync with different cluster conf and provider",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.CloudClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),
			inClusterClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainNew),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			// cores in master
			commanderCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),
			inClusterCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "3"),

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDOld.String(),
		},
		{
			testCheckClusterConfigBase: testCheckClusterConfigBase{
				testName:           "cloud cluster not sync with different uuid and provider",
				expectedSyncStatus: CheckStatusOutOfSync,
				isError:            false,
				clusterType:        config.CloudClusterType,
			},

			commanderClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),
			inClusterClusterConfig: fmt.Sprintf(cloudConfigFmt, clusterDomainOld),

			commanderStaticConfig: nil,
			inClusterStaticConfig: nil,

			// cores in master
			commanderCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "1"),
			inClusterCloudConfig: fmt.Sprintf(cloudClusterConfigFmt, "3"),

			commanderClusterUUID: clusterUUIDOld.String(),
			inClusterClusterUUID: clusterUUIDNew.String(),
		},
	}

	for _, params := range tests {
		tst := createTestCheckClusterConfig(t, params)
		t.Run(tst.testName, func(t *testing.T) {
			syncStatus, err := tst.checker.checkConfiguration(context.TODO(), tst.kubeCl, tst.commanderMetaConfig)

			if tst.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tst.expectedSyncStatus, syncStatus)
		})
	}
}

type testCheckClusterConfigBase struct {
	expectedSyncStatus CheckStatus
	isError            bool
	testName           string
	clusterType        string
}

type testCheckClusterConfigParams struct {
	testCheckClusterConfigBase

	commanderClusterConfig string
	inClusterClusterConfig string

	commanderStaticConfig *string
	inClusterStaticConfig *string

	commanderCloudConfig string
	inClusterCloudConfig string

	commanderClusterUUID string
	inClusterClusterUUID string
}

type testCheckClusterConfig struct {
	testCheckClusterConfigBase

	kubeCl              *client.KubernetesClient
	commanderMetaConfig *config.MetaConfig
	checker             *Checker
	logger              *log.InMemoryLogger
}

func createTestCheckClusterConfig(t *testing.T, p testCheckClusterConfigParams) *testCheckClusterConfig {
	t.Helper()

	require.NotEmpty(t, p.testName)
	require.NotEmpty(t, p.expectedSyncStatus, p.testName)
	require.NotEmpty(t, p.clusterType, p.testName)

	kubeCl := client.NewFakeKubernetesClient()
	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())

	commanderMetaConfig := &config.MetaConfig{}
	commanderMetaConfig.ClusterType = p.clusterType

	commanderMetaConfig.UUID = p.commanderClusterUUID
	if p.inClusterClusterUUID != "" {
		testCreateKubeSystemCM(t, kubeCl, "d8-cluster-uuid", map[string]string{
			"cluster-uuid": p.inClusterClusterUUID,
		})
	}

	commanderMetaConfig.ClusterConfig = testMarshalConfig(t, p.commanderClusterConfig)
	if p.inClusterClusterConfig != "" {
		testCreateKubeSystemSecret(t, kubeCl, "d8-cluster-configuration", map[string][]byte{
			"cluster-configuration.yaml": []byte(p.inClusterClusterConfig),
		})
	}

	_, err := config.DoByClusterType(context.TODO(), commanderMetaConfig, &testCheckSpecificClusterFiller{
		params: p,
		t:      t,
		kubeCl: kubeCl,
	})
	require.NoError(t, err, p.testName)

	commanderUUID, err := uuid.NewUUID()
	require.NoError(t, err, p.testName)

	return &testCheckClusterConfig{
		testCheckClusterConfigBase: p.testCheckClusterConfigBase,
		commanderMetaConfig:        commanderMetaConfig,
		kubeCl:                     kubeCl,
		logger:                     logger,
		checker: NewChecker(&Params{
			Logger:        logger,
			CommanderMode: true,
			IsDebug:       false,
			KubeClient:    kubeCl,
			CommanderUUID: commanderUUID,
		}),
	}
}

func testMarshalConfig(t *testing.T, config string) map[string]json.RawMessage {
	t.Helper()

	if config == "" {
		return nil
	}

	var result map[string]json.RawMessage
	err := yaml.Unmarshal([]byte(config), &result)
	require.NoError(t, err)

	return result
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

func testCreateKubeSystemCM(t *testing.T, kubeCl *client.KubernetesClient, name string, data map[string]string) {
	t.Helper()

	cm := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: global.ConfigsNS,
		},
		Data: data,
	}

	_, err := kubeCl.CoreV1().ConfigMaps(global.ConfigsNS).Create(context.TODO(), cm, metav1.CreateOptions{})
	require.NoError(t, err)
}

type nilType *struct{}

type testCheckSpecificClusterFiller struct {
	params testCheckClusterConfigParams
	t      *testing.T
	kubeCl *client.KubernetesClient
}

func (f *testCheckSpecificClusterFiller) Cloud(_ context.Context, metaConfig *config.MetaConfig) (nilType, error) {
	metaConfig.ProviderClusterConfig = testMarshalConfig(f.t, f.params.commanderCloudConfig)

	if f.params.inClusterCloudConfig != "" {
		data := map[string][]byte{
			"cloud-provider-cluster-configuration.yaml": []byte(f.params.inClusterCloudConfig),
			"cloud-provider-discovery-data.json":        []byte(`{"a": "b"}`),
		}
		testCreateKubeSystemSecret(f.t, f.kubeCl, "d8-provider-cluster-configuration", data)
	}

	return nil, nil
}

func (f *testCheckSpecificClusterFiller) Static(_ context.Context, metaConfig *config.MetaConfig) (nilType, error) {
	if f.params.commanderStaticConfig != nil {
		metaConfig.StaticClusterConfig = testMarshalConfig(f.t, *f.params.commanderStaticConfig)
	} else {
		metaConfig.StaticClusterConfig = nil
	}

	if f.params.inClusterStaticConfig != nil {
		data := make(map[string][]byte)
		if *f.params.inClusterStaticConfig != "" {
			data["static-cluster-configuration.yaml"] = []byte(*f.params.inClusterStaticConfig)
		}

		testCreateKubeSystemSecret(f.t, f.kubeCl, "d8-static-cluster-configuration", data)
	}

	return nil, nil
}

func (f *testCheckSpecificClusterFiller) Incorrect(_ context.Context, metaConfig *config.MetaConfig) (nilType, error) {
	return nil, config.UnsupportedClusterTypeErr(metaConfig)
}
