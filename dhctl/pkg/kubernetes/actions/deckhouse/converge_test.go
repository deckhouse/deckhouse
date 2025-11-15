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

package deckhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
)

const (
	kubeVersionBefore             = "1.32"
	kubeVersionAfter              = "1.33"
	clusterConfigurationStaticTmp = `
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
	clusterConfigurationYandexTmp = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: "test"
kubernetesVersion: "%s"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
encryptionAlgorithm: RSA-2048
defaultCRI: Containerd
clusterDomain: cluster.local
podSubnetNodeCIDRPrefix: "24"
`
	clusterConfigurationYandexBeforeValid = `
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
)

var (
	yandexProviderClusterDataDiscovery = []byte(`{"a": "b"}`)
)

func TestStaticClusterClusterManifestConverge(t *testing.T) {
	// need for prevent set equal versions during add new k8s version
	require.NotEqual(t, kubeVersionBefore, kubeVersionAfter)

	paramsBefore := commander.CommanderModeParams{
		ClusterConfigurationData: []byte(fmt.Sprintf(clusterConfigurationStaticTmp, kubeVersionBefore)),
		ProviderClusterConfigurationData: []byte(`
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.0.0/24
`),
	}
	paramsAfter := commander.CommanderModeParams{
		ClusterConfigurationData: []byte(fmt.Sprintf(clusterConfigurationStaticTmp, kubeVersionAfter)),
		ProviderClusterConfigurationData: []byte(`
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.0.0/24
- 10.10.0.0/24
`),
	}

	testUpdateWithoutError := func(t *testing.T, params testConvergeManifestsParams) {
		test := testCreateConvergeManifestTest(t, params)

		testCreateSecret(
			t,
			test.kubeCl,
			manifests.SecretWithStaticClusterConfig(params.commanderStateBefore.ProviderClusterConfigurationData),
		)

		test.secretsToAssert = append(
			test.secretsToAssert,
			manifests.SecretWithStaticClusterConfig(params.commanderStateAfter.ProviderClusterConfigurationData),
		)

		test.do(t)
	}

	testUpdateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore: paramsBefore,
		commanderStateAfter:  paramsAfter,
		testName:             "static: normal update",
	})

	paramsAfterWithEmptyConfig := paramsAfter
	paramsAfterWithEmptyConfig.ProviderClusterConfigurationData = make([]byte, 0)
	testUpdateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore: paramsBefore,
		commanderStateAfter:  paramsAfterWithEmptyConfig,
		testName:             "static: with empty static configuration no fault and rewrite with empty data",
	})

	testUpdateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore: paramsBefore,
		commanderStateAfter:  paramsBefore,
		testName:             "static: no update",
	})

	testUpdateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore:   paramsBefore,
		commanderStateAfter:    paramsAfter,
		doNotHaveCommanderUUID: true,
		testName:               "static: without commander uuid",
	})

	testUpdateAndCreateWithoutError := func(t *testing.T, params testConvergeManifestsParams) {
		test := testCreateConvergeManifestTest(t, params)

		test.secretsToAssert = append(
			test.secretsToAssert,
			manifests.SecretWithStaticClusterConfig(params.commanderStateAfter.ProviderClusterConfigurationData),
		)

		test.do(t)
	}

	testUpdateAndCreateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore: paramsBefore,
		commanderStateAfter:  paramsAfter,
		testName:             "static: create static configuration if need",
	})
}

func TestCloudClusterManifestConverge(t *testing.T) {
	// need for prevent set equal versions during add new k8s version
	require.NotEqual(t, kubeVersionBefore, kubeVersionAfter)

	paramsBefore := commander.CommanderModeParams{
		ClusterConfigurationData:         []byte(fmt.Sprintf(clusterConfigurationYandexTmp, kubeVersionBefore)),
		ProviderClusterConfigurationData: []byte(clusterConfigurationYandexBeforeValid),
	}
	paramsAfter := commander.CommanderModeParams{
		ClusterConfigurationData: []byte(fmt.Sprintf(clusterConfigurationYandexTmp, kubeVersionAfter)),
		ProviderClusterConfigurationData: []byte(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
masterNodeGroup:
  replicas: 3
  instanceClass:
    etcdDiskSizeGb: 10
    platform: standard-v2
    cores: 4
    memory: 8192
    imageID: imageId
    externalIPAddresses:
      - Auto
nodeGroups:
- name: worker
  replicas: 1
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: imageId
    coreFraction: 50
    platform: standard-v2
  zones:
  - ru-central1-a
sshPublicKey: ssh-rsa AAAAB3NzaC
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: cloudId
  folderID: folderId
  serviceAccountJSON: "{}"
`),
	}

	testUpdateWithoutError := func(t *testing.T, params testConvergeManifestsParams) {
		test := testCreateConvergeManifestTest(t, params)

		testCreateSecret(
			t,
			test.kubeCl,
			manifests.SecretWithProviderClusterConfig(
				params.commanderStateBefore.ProviderClusterConfigurationData,
				yandexProviderClusterDataDiscovery,
			),
		)

		test.secretsToAssert = append(
			test.secretsToAssert,
			manifests.SecretWithProviderClusterConfig(
				params.commanderStateAfter.ProviderClusterConfigurationData,
				yandexProviderClusterDataDiscovery,
			),
		)

		test.do(t)
	}

	testUpdateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore: paramsBefore,
		commanderStateAfter:  paramsAfter,
		testName:             "provider: normal update",
	})

	testUpdateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore: paramsBefore,
		commanderStateAfter:  paramsBefore,
		testName:             "provider: no update",
	})

	testUpdateWithoutError(t, testConvergeManifestsParams{
		commanderStateBefore:   paramsBefore,
		commanderStateAfter:    paramsAfter,
		doNotHaveCommanderUUID: true,
		testName:               "provider: no commander uuid",
	})
}

func TestErrorConvergeManifests(t *testing.T) {
	// need for prevent set equal versions during add new k8s version
	require.NotEqual(t, kubeVersionBefore, kubeVersionAfter)

	type beforeTest func(t *testing.T, params testConvergeManifestsParams, test *testConvergeManifests)

	errorTest := func(t *testing.T, params testConvergeManifestsParams, before beforeTest) {
		tst := testCreateConvergeManifestTest(t, params)

		before(t, params, tst)

		tst.doWithError(t)
	}

	staticClusterConvergeParams := testConvergeManifestsParams{
		commanderStateBefore: commander.CommanderModeParams{
			ClusterConfigurationData: []byte(fmt.Sprintf(clusterConfigurationStaticTmp, kubeVersionBefore)),
		},
		commanderStateAfter: commander.CommanderModeParams{
			ClusterConfigurationData: []byte(fmt.Sprintf(clusterConfigurationStaticTmp, kubeVersionBefore)),
		},
	}

	createEmptyStaticConfigurationSecret := func(t *testing.T, tst *testConvergeManifests) {
		testCreateSecret(t, tst.kubeCl, manifests.SecretWithStaticClusterConfig(nil))
		tst.secretsToAssert = append(tst.secretsToAssert, manifests.SecretWithStaticClusterConfig(nil))
	}

	yandexClusterConvergeParams := testConvergeManifestsParams{
		commanderStateBefore: commander.CommanderModeParams{
			ClusterConfigurationData:         []byte(fmt.Sprintf(clusterConfigurationYandexTmp, kubeVersionBefore)),
			ProviderClusterConfigurationData: []byte(clusterConfigurationYandexBeforeValid),
		},
		commanderStateAfter: commander.CommanderModeParams{
			ClusterConfigurationData:         []byte(fmt.Sprintf(clusterConfigurationYandexTmp, kubeVersionBefore)),
			ProviderClusterConfigurationData: []byte(clusterConfigurationYandexBeforeValid),
		},
	}

	createYandexConfigurationSecret := func(t *testing.T, tst *testConvergeManifests, params testConvergeManifestsParams) {
		testCreateSecret(
			t,
			tst.kubeCl,
			manifests.SecretWithProviderClusterConfig(
				params.commanderStateBefore.ProviderClusterConfigurationData,
				yandexProviderClusterDataDiscovery,
			),
		)
		tst.secretsToAssert = append(
			tst.secretsToAssert,
			manifests.SecretWithProviderClusterConfig(
				params.commanderStateBefore.ProviderClusterConfigurationData,
				yandexProviderClusterDataDiscovery,
			),
		)
	}

	errorTest(t, staticClusterConvergeParams.CopyWithName("no cluster uuid"), func(t *testing.T, params testConvergeManifestsParams, tst *testConvergeManifests) {
		createEmptyStaticConfigurationSecret(t, tst)
		tst.metaConfig.UUID = ""
	})

	// empty cluster configuration, because commander does not support managed clusters
	errorTest(t, staticClusterConvergeParams.CopyWithName("no cluster config"), func(t *testing.T, params testConvergeManifestsParams, tst *testConvergeManifests) {
		createEmptyStaticConfigurationSecret(t, tst)
		tst.metaConfig.ClusterConfig = nil
	})

	// empty cluster type, because commander does not support managed clusters
	errorTest(t, staticClusterConvergeParams.CopyWithName("empty cluster type"), func(t *testing.T, params testConvergeManifestsParams, tst *testConvergeManifests) {
		createEmptyStaticConfigurationSecret(t, tst)
		tst.metaConfig.ClusterType = ""
	})

	errorTest(t, staticClusterConvergeParams.CopyWithName("incorrect cluster type"), func(t *testing.T, params testConvergeManifestsParams, tst *testConvergeManifests) {
		createEmptyStaticConfigurationSecret(t, tst)
		tst.metaConfig.ClusterType = "incorrect"
	})

	errorTest(t, staticClusterConvergeParams.CopyWithName("incorrect static config"), func(t *testing.T, params testConvergeManifestsParams, tst *testConvergeManifests) {
		createEmptyStaticConfigurationSecret(t, tst)
		tst.metaConfig.StaticClusterConfig = map[string]json.RawMessage{
			"something": json.RawMessage(`{"a": "}`),
		}
	})

	errorTest(t, yandexClusterConvergeParams.CopyWithName("no provider config"), func(t *testing.T, params testConvergeManifestsParams, tst *testConvergeManifests) {
		createYandexConfigurationSecret(t, tst, params)
		tst.metaConfig.ProviderClusterConfig = nil
	})

	errorTest(t, yandexClusterConvergeParams.CopyWithName("incorrect provider config"), func(t *testing.T, params testConvergeManifestsParams, tst *testConvergeManifests) {
		createYandexConfigurationSecret(t, tst, params)
		tst.metaConfig.ProviderClusterConfig = map[string]json.RawMessage{
			"something": json.RawMessage(`{"a": "}`),
		}
	})
}

type testConvergeManifestsParams struct {
	commanderStateBefore   commander.CommanderModeParams
	commanderStateAfter    commander.CommanderModeParams
	doNotHaveCommanderUUID bool
	testName               string
}

func (p *testConvergeManifestsParams) CopyWithName(name string) testConvergeManifestsParams {
	return testConvergeManifestsParams{
		commanderStateBefore:   p.commanderStateBefore,
		commanderStateAfter:    p.commanderStateAfter,
		doNotHaveCommanderUUID: p.doNotHaveCommanderUUID,
		testName:               name,
	}
}

type testConvergeManifests struct {
	testConvergeManifestsParams

	metaConfig    *config.MetaConfig
	commanderUUID uuid.UUID
	kubeCl        *client.KubernetesClient

	configMapsToAssert []*corev1.ConfigMap
	secretsToAssert    []*corev1.Secret
}

func (tt *testConvergeManifests) assertGeneral(t *testing.T) {
	require.NotEmpty(t, tt.testName)

	require.NotNil(t, tt.metaConfig, tt.testName)
	require.NotNil(t, tt.kubeCl, tt.testName)

	require.NotEmpty(t, tt.configMapsToAssert, tt.testName)
	require.NotEmpty(t, tt.secretsToAssert, tt.testName)

	if tt.doNotHaveCommanderUUID {
		require.Empty(t, tt.commanderUUID, tt.testName)
	} else {
		require.NotEmpty(t, tt.commanderUUID, tt.testName)
	}
}

func (tt *testConvergeManifests) assertConfiguration(t *testing.T) {
	allSecrets, err := tt.kubeCl.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, allSecrets.Items, len(tt.secretsToAssert))

	for _, secretToAssert := range tt.secretsToAssert {
		assertSecret(t, tt.kubeCl, secretToAssert)
	}

	allCms, err := tt.kubeCl.CoreV1().ConfigMaps("").List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, allCms.Items, len(tt.configMapsToAssert))

	for _, cm := range tt.configMapsToAssert {
		assertConfigMap(t, tt.kubeCl, cm)
	}
}

func (tt *testConvergeManifests) do(t *testing.T) {
	t.Run(fmt.Sprintf("converge %s", tt.testName), func(t *testing.T) {
		tt.assertGeneral(t)

		tasks, err := getTasksForRunning(context.TODO(), tt.kubeCl, tt.commanderUUID, tt.metaConfig)
		require.NoError(t, err)

		tasksNames := make(map[string]struct{}, len(tasks))
		for _, task := range tasks {
			tasksNames[task.Name] = struct{}{}
		}

		assertContainsOrNotCommanderUUID := require.Contains
		expectedLen := len(tt.configMapsToAssert) + len(tt.secretsToAssert)

		if tt.doNotHaveCommanderUUID {
			expectedLen = expectedLen - 1
			assertContainsOrNotCommanderUUID = require.NotContains
		}

		require.Len(t, tasks, expectedLen)
		assertContainsOrNotCommanderUUID(t, tasksNames, `ConfigMap "d8-commander-uuid"`)

		for _, task := range tasks {
			err := task.CreateOrUpdate()
			require.NoError(t, err)
		}

		tt.assertConfiguration(t)
	})
}

func (tt *testConvergeManifests) doWithError(t *testing.T) {
	t.Run(fmt.Sprintf("has error %s", tt.testName), func(t *testing.T) {
		tt.assertGeneral(t)

		tasks, err := getTasksForRunning(context.TODO(), tt.kubeCl, tt.commanderUUID, tt.metaConfig)
		require.Len(t, tasks, 0)
		require.Error(t, err)

		tt.assertConfiguration(t)
	})
}

func testCreateConvergeManifestTest(t *testing.T, p testConvergeManifestsParams) *testConvergeManifests {
	require.NotEmpty(t, p.commanderStateBefore.ClusterConfigurationData, p.testName)
	require.NotEmpty(t, p.commanderStateAfter.ClusterConfigurationData, p.testName)

	clusterUUID, err := uuid.NewUUID()
	require.NoError(t, err, p.testName)

	commanderUUID, err := uuid.NewUUID()
	require.NoError(t, err, p.testName)

	clusterUUIDStr := clusterUUID.String()

	metaConfigToApply := testCreateMetaConfigForConvergeManifests(t, context.TODO(), p.commanderStateAfter, clusterUUIDStr)

	kubeCl := client.NewFakeKubernetesClient()

	notAffectedCm := []*corev1.ConfigMap{
		manifests.ClusterUUIDConfigMap(clusterUUIDStr),
		// if commander uuid is empty does not affect current
		manifests.CommanderUUIDConfigMap(commanderUUID.String()),
	}

	configMapsToAssert := make([]*corev1.ConfigMap, 0, len(notAffectedCm))

	for _, cm := range notAffectedCm {
		createdCm, err := kubeCl.CoreV1().ConfigMaps(cm.GetNamespace()).Create(context.TODO(), cm, metav1.CreateOptions{})
		require.NoError(t, err, cm.GetName())
		assertConfigMap(t, kubeCl, cm)

		configMapsToAssert = append(configMapsToAssert, createdCm)
	}

	testCreateSecret(t, kubeCl, manifests.SecretWithClusterConfig(p.commanderStateBefore.ClusterConfigurationData))

	secretsToAssert := append(
		[]*corev1.Secret{},
		manifests.SecretWithClusterConfig(p.commanderStateAfter.ClusterConfigurationData),
	)

	if p.doNotHaveCommanderUUID {
		commanderUUID = uuid.Nil
	}

	return &testConvergeManifests{
		testConvergeManifestsParams: p,
		metaConfig:                  metaConfigToApply,
		kubeCl:                      kubeCl,
		configMapsToAssert:          configMapsToAssert,
		secretsToAssert:             secretsToAssert,
		commanderUUID:               commanderUUID,
	}
}

func testCreateMetaConfigForConvergeManifests(t *testing.T, ctx context.Context, params commander.CommanderModeParams, clusterUUID string) *config.MetaConfig {
	configData := fmt.Sprintf("%s\n---\n%s", params.ClusterConfigurationData, params.ProviderClusterConfigurationData)
	metaConfig, err := config.ParseConfigFromData(
		ctx,
		configData,
		config.DummyPreparatorProvider(),
	)

	require.NoError(t, err)

	metaConfig.UUID = clusterUUID

	return metaConfig
}

func testCreateSecret(t *testing.T, kubeCl *client.KubernetesClient, secret *corev1.Secret) {
	_, err := kubeCl.CoreV1().Secrets(secret.GetNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{})
	require.NoError(t, err, secret.GetName())
	assertSecret(t, kubeCl, secret)
}

func assertSecret(t *testing.T, kubeCl *client.KubernetesClient, secret *corev1.Secret) {
	require.NotNil(t, secret)

	name := secret.GetName()
	ns := secret.GetNamespace()

	gotSecret, err := kubeCl.CoreV1().Secrets(ns).Get(context.TODO(), secret.GetName(), metav1.GetOptions{})
	require.NoError(t, err, name)

	require.Equal(t, name, gotSecret.GetName(), name)
	require.Equal(t, ns, gotSecret.GetNamespace(), name)

	require.Len(t, secret.Data, len(gotSecret.Data))
	for k, v := range secret.Data {
		require.Contains(t, gotSecret.Data, k)
		assertKV(t, k, v, gotSecret.Data[k])
	}
}

func assertConfigMap(t *testing.T, kubeCl *client.KubernetesClient, configMap *corev1.ConfigMap) {
	require.NotNil(t, configMap)

	name := configMap.GetName()
	ns := configMap.GetNamespace()

	gotCm, err := kubeCl.CoreV1().ConfigMaps(ns).Get(context.TODO(), name, metav1.GetOptions{})
	require.NoError(t, err, name)

	require.Equal(t, name, gotCm.GetName(), name)
	require.Equal(t, ns, gotCm.GetNamespace(), name)

	require.Len(t, configMap.Data, len(gotCm.Data))
	for k, v := range configMap.Data {
		require.Contains(t, gotCm.Data, k)
		assertKV(t, k, []byte(v), []byte(gotCm.Data[k]))
	}
}

func assertKV(t *testing.T, k string, expectedV []byte, v []byte) {
	var yamlV any
	err := yaml.Unmarshal(v, &yamlV)
	if err != nil {
		require.Equal(t, expectedV, v, k)
		return
	}
	require.YAMLEq(t, string(expectedV), string(v), k)
}
