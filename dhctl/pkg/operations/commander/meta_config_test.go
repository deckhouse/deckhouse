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

package commander

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

const (
	clusterConfigNoPrefixYAML = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
`
	clusterConfigWithPrefixYAML = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: ccprefix
`
	yandexProviderConfigYAML = `
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
sshPublicKey: ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCsCkOVy6z7SPO+NYZyz15XTFSRGYqhaw3QVAoRuUkG6J1xCK7yCXtZoDYJM5uSdk58cQhd3/+Dto7saNa3NNEm+WW3vnZ6ArLl4U/YHmpHu0pUgDaoQsaRvNHW5jG/YsBter0G88ZqChRP4adhaMHK4x7JM+Yml+dTEecAROzcl9cIjMTPjUK/3ZJdbckpTQXiqX7re+Mzer2wdAT0YtwX2Ai++nrP/GIFzO+HMTd6lLdtP+uGWL+zNnHq2KTbP1v9BumZQXJNGLVXrI8V63TW7cKICr+8ASdF+hw9DDqyIJBeRE/LNm1tj2VIfnwPaGs9G5gdP0k5FUsvq8qwS6GDd6Ro/iGfhMhOhnLBSzlobGPO0I+kb7r250eyhwpJEGPvTR3koA/5KyFKtYctgbYkaBEJzCMhtgU9CzbFHimS7Y2/XIPLcLbuWYaknCqnny++kmvxzc4G7Qj6mf8gsr1NH273Qf/dlkkwPhGxIA+OJDK9OOjEu2ZjZyM+lJOgJQ0= root@11605a4d8b81
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: cloudId
  folderID: folderId
  serviceAccountJSON: "{}"
`
)

func seedModuleConfig(t *testing.T, kubeCl *client.KubernetesClient, name string, version int, settings map[string]interface{}) {
	t.Helper()
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]interface{}{"name": name},
		"spec":       map[string]interface{}{"version": float64(version), "settings": settings},
	}}
	_, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Create(t.Context(), obj, metav1.CreateOptions{})
	require.NoError(t, err)
}

func newStateWithUUID(t *testing.T) *cache.TestCache {
	t.Helper()
	stateCache := cache.NewTestCache()
	require.NoError(t, stateCache.Save(t.Context(), "uuid", []byte("9d7a0f7e-4b58-4d0c-a1a6-54f6d2b3c001")))
	return stateCache
}

func getterFor(kubeCl *client.KubernetesClient) config.KubeClientGetter {
	return func(context.Context) (*client.KubernetesClient, error) { return kubeCl, nil }
}

// The source tree has no candi/cloud-providers/yandex: a stub bundle in the download dir stands in for
// the one the dhctl image carries, so nothing is downloaded.
func yandexBundleOptions(t *testing.T) *options.GlobalOptions {
	t.Helper()
	downloadDir := t.TempDir()
	tests.StubDeliveredProviderBundle(t, downloadDir, "yandex")
	return &options.GlobalOptions{DownloadDir: downloadDir}
}

func noAPIGetter(context.Context) (*client.KubernetesClient, error) {
	return nil, errors.New("kubernetes API is unreachable")
}

// The retry after the masters are gone has no API: it must resolve everything from the state.
func TestParseMetaConfig_DestroyReadsModuleConfigsFromClusterAndCachesThem(t *testing.T) {
	ctx := t.Context()
	globalOptions := yandexBundleOptions(t)
	kubeCl := client.NewFakeKubernetesClient()
	seedModuleConfig(t, kubeCl, "global", 2, map[string]interface{}{"prefix": "mcprefix"})
	seedModuleConfig(t, kubeCl, "control-plane-manager", 3, map[string]interface{}{
		"kubernetesVersion": "1.33",
		"network": map[string]interface{}{
			"podSubnetCIDR":     "10.11.0.0/16",
			"serviceSubnetCIDR": "10.22.0.0/16",
			"clusterDomain":     "mc.local",
		},
	})
	stateCache := newStateWithUUID(t)
	params := NewCommanderModeParams([]byte(clusterConfigNoPrefixYAML), []byte(yandexProviderConfigYAML))

	first, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, getterFor(kubeCl), globalOptions)
	require.NoError(t, err)
	require.Equal(t, providerdir.DigestDir(globalOptions.DownloadDir, "yandex", tests.StubBundleDigest()), first.ProviderBundleDir,
		"the bundle the cluster runs travels with the MetaConfig")
	require.Equal(t, "mcprefix", first.ClusterPrefix)
	require.Equal(t, "10.11.0.0/16", first.Network().PodSubnetCIDR)
	require.Equal(t, "10.22.0.0/16", first.Network().ServiceSubnetCIDR)
	require.Equal(t, "mc.local", first.ClusterDomainResolved())
	clusterConfigMap, err := first.ClusterConfigMap()
	require.NoError(t, err)
	require.Equal(t, "1.33", clusterConfigMap["kubernetesVersion"])

	// The retry runs on the bundle already delivered. An installer that names no bundle of its own
	// proves the parse did not resolve one again.
	t.Setenv(digests.ImagesDigestsFileEnv, filepath.Join(t.TempDir(), "no-digests.json"))
	second, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, noAPIGetter, globalOptions)
	require.NoError(t, err)
	require.Equal(t, "mcprefix", second.ClusterPrefix)
	require.Equal(t, "10.11.0.0/16", second.Network().PodSubnetCIDR)
}

// With the cluster gone, destroy goes on with the bundle whose digest an earlier run recorded in
// the state of this very cluster, not with whatever bundle was delivered here last.
func TestParseMetaConfig_DestroyUsesTheBundleRecordedInTheState(t *testing.T) {
	ctx := t.Context()
	globalOptions := yandexBundleOptions(t)
	kubeCl := client.NewFakeKubernetesClient()
	seedModuleConfig(t, kubeCl, "global", 2, map[string]interface{}{"prefix": "mcprefix"})
	stateCache := newStateWithUUID(t)
	params := NewCommanderModeParams([]byte(clusterConfigNoPrefixYAML), []byte(yandexProviderConfigYAML))

	first, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, getterFor(kubeCl), globalOptions)
	require.NoError(t, err)

	recorded, err := stateCache.Load(ctx, providerBundleDigestCacheKey)
	require.NoError(t, err)
	require.Equal(t, providerdir.DigestFromDir(first.ProviderBundleDir), string(recorded))

	second, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, noAPIGetter, globalOptions)
	require.NoError(t, err)
	require.Equal(t, first.ProviderBundleDir, second.ProviderBundleDir)
}

// A recorded bundle that is not on this machine cannot stand in, and the operator needs both reasons.
func TestParseMetaConfig_RecordedBundleNotOnThisMachineKeepsTheOriginalError(t *testing.T) {
	globalOptions := yandexBundleOptions(t)
	stateCache := newStateWithUUID(t)
	const elsewhere = "sha256:0000000000000000000000000000000000000000000000000000000000000eee"
	require.NoError(t, stateCache.Save(t.Context(), providerBundleDigestCacheKey, []byte(elsewhere)))
	params := NewCommanderModeParams([]byte(clusterConfigNoPrefixYAML), []byte(yandexProviderConfigYAML))

	_, err := ParseMetaConfig(t.Context(), stateCache, params, infrastructureprovider.DhctlOperationDestroy, noAPIGetter, globalOptions)
	require.ErrorContains(t, err, "kubernetes API is unreachable")
	require.ErrorContains(t, err, "use provider bundle recorded in state cache")
	require.ErrorContains(t, err, providerdir.DigestDir(globalOptions.DownloadDir, "yandex", elsewhere))
}

// Destroy goes on with the digest recorded here, and the run that records it is usually a converge
// or a check (which parses as a converge) long before the destroy.
func TestParseMetaConfig_ConvergeRecordsTheBundleDigest(t *testing.T) {
	ctx := t.Context()
	globalOptions := yandexBundleOptions(t)
	kubeCl := client.NewFakeKubernetesClient()
	seedModuleConfig(t, kubeCl, "global", 2, map[string]interface{}{"prefix": "mcprefix"})
	stateCache := newStateWithUUID(t)
	params := NewCommanderModeParams([]byte(clusterConfigNoPrefixYAML), []byte(yandexProviderConfigYAML))

	mc, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationConverge, getterFor(kubeCl), globalOptions)
	require.NoError(t, err)

	recorded, err := stateCache.Load(ctx, providerBundleDigestCacheKey)
	require.NoError(t, err)
	require.Equal(t, tests.StubBundleDigest(), string(recorded))
	require.Equal(t, providerdir.DigestFromDir(mc.ProviderBundleDir), string(recorded))
}

func TestParseMetaConfig_NoRecordedBundleMeansTheOriginalError(t *testing.T) {
	globalOptions := yandexBundleOptions(t)
	params := NewCommanderModeParams([]byte(clusterConfigNoPrefixYAML), []byte(yandexProviderConfigYAML))

	_, err := ParseMetaConfig(t.Context(), newStateWithUUID(t), params, infrastructureprovider.DhctlOperationDestroy, noAPIGetter, globalOptions)
	require.ErrorContains(t, err, "kubernetes API is unreachable")
	require.ErrorContains(t, err, "ensure provider bundle from cluster", "the ModuleConfigs read fails the same way, so pin the step")
}

// Only destroy may go on with a recorded bundle. Converge with a stale bundle would change the cluster.
func TestParseMetaConfig_ConvergeNeverUsesTheRecordedBundle(t *testing.T) {
	ctx := t.Context()
	globalOptions := yandexBundleOptions(t)
	kubeCl := client.NewFakeKubernetesClient()
	seedModuleConfig(t, kubeCl, "global", 2, map[string]interface{}{"prefix": "mcprefix"})
	stateCache := newStateWithUUID(t)
	params := NewCommanderModeParams([]byte(clusterConfigNoPrefixYAML), []byte(yandexProviderConfigYAML))

	_, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, getterFor(kubeCl), globalOptions)
	require.NoError(t, err)

	_, err = ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationConverge, noAPIGetter, globalOptions)
	require.ErrorContains(t, err, "kubernetes API is unreachable")
	require.ErrorContains(t, err, "ensure provider bundle from cluster", "the ModuleConfigs read fails the same way, so pin the step")
}

// A cluster without the ModuleConfigs is cached as such, so the retry keeps the ClusterConfiguration prefix.
func TestParseMetaConfig_DestroyCachesAbsentModuleConfigs(t *testing.T) {
	ctx := t.Context()
	globalOptions := yandexBundleOptions(t)
	stateCache := newStateWithUUID(t)
	params := NewCommanderModeParams([]byte(clusterConfigWithPrefixYAML), []byte(yandexProviderConfigYAML))

	_, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, getterFor(client.NewFakeKubernetesClient()), globalOptions)
	require.NoError(t, err)

	second, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, noAPIGetter, globalOptions)
	require.NoError(t, err)
	require.Equal(t, "ccprefix", second.ClusterPrefix)
}

// Only destroy caches: check and converge must read fresh values every time.
func TestParseMetaConfig_ConvergeDoesNotCache(t *testing.T) {
	ctx := t.Context()
	globalOptions := yandexBundleOptions(t)
	kubeCl := client.NewFakeKubernetesClient()
	seedModuleConfig(t, kubeCl, "global", 2, map[string]interface{}{"prefix": "mcprefix"})
	stateCache := newStateWithUUID(t)
	params := NewCommanderModeParams([]byte(clusterConfigNoPrefixYAML), []byte(yandexProviderConfigYAML))

	mc, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationConverge, getterFor(kubeCl), globalOptions)
	require.NoError(t, err)
	require.Equal(t, "mcprefix", mc.ClusterPrefix)

	inCache, err := stateCache.InCache(ctx, clusterSettingsModuleConfigsCacheKey)
	require.NoError(t, err)
	require.False(t, inCache)
}

// A parse that fails must leave nothing behind: a cached ModuleConfig could not be corrected in the
// cluster afterwards, the retry would read the cache.
func TestParseMetaConfig_DestroyDoesNotCacheWhenTheParseFails(t *testing.T) {
	ctx := t.Context()
	globalOptions := yandexBundleOptions(t)
	kubeCl := client.NewFakeKubernetesClient()
	seedModuleConfig(t, kubeCl, "global", 2, map[string]interface{}{"prefix": "mcprefix"})
	stateCache := newStateWithUUID(t)
	params := NewCommanderModeParams([]byte(clusterConfigWithPrefixYAML), []byte(yandexProviderConfigYAML))

	_, err := ParseMetaConfig(ctx, stateCache, params, infrastructureprovider.DhctlOperationDestroy, getterFor(kubeCl), globalOptions)
	require.ErrorContains(t, err, "the cluster prefix is set twice")

	inCache, err := stateCache.InCache(ctx, clusterSettingsModuleConfigsCacheKey)
	require.NoError(t, err)
	require.False(t, inCache)
}

const providerResourcesClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: DVP
  prefix: pcc
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: Automatic
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/deckhouse/ce
`

// providerResources is the new provider format as Commander sends it: the provider ModuleConfig,
// the credentials, the instance class and the NodeGroups travel as resources, not as a
// ProviderClusterConfiguration.
const providerResources = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 2
  settings:
    provider:
      parameters:
        namespace: team
    storage:
      parameters: {}
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: kubeconfig
  secret: kubeconfig
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master-class
spec:
  rootDisk:
    size: 50Gi
    storageClass: replicated
    image:
      kind: ClusterVirtualImage
      name: ubuntu
  etcdDisk:
    size: 10Gi
    storageClass: replicated
  virtualMachine:
    cpu:
      cores: 4
      coreFraction: 20%
    memory:
      size: 8Gi
    virtualMachineClassName: generic
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    minPerZone: 3
    maxPerZone: 3
    classReference:
      kind: DVPInstanceClass
      name: master-class
`

// providerResourcesWithoutMaster keeps the provider configuration and drops the master NodeGroup,
// the shape a lost or truncated group would produce.
const providerResourcesWithoutMaster = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 2
  settings:
    provider:
      parameters:
        namespace: team
    storage:
      parameters: {}
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example
`

func parseMetaConfig(t *testing.T, resources, operation string) (*config.MetaConfig, error) {
	t.Helper()

	// The provider schema of the cluster configuration is delivered by the provider bundle;
	// the stub places the real schema where the parser looks for it.
	tests.StubDeliveredProviderBundle(t, options.DefaultTmpDir(), "dvp")

	stateCache := cache.NewTestCache()
	require.NoError(t, stateCache.Save(t.Context(), "uuid", []byte("cluster-uuid")))

	// The provider configuration slot is empty: the provider lives in the resources.
	params := NewCommanderModeParams([]byte(providerResourcesClusterConfiguration), []byte{})
	if resources != "" {
		params.WithResources([]byte(resources))
	}

	globalOptions := options.New().Global
	globalOptions.CandiDir = tests.RequireCandiDir(t)

	// The cluster settings ModuleConfigs are read from the cluster: an empty one has none.
	return ParseMetaConfig(t.Context(), stateCache, params, operation, getterFor(client.NewFakeKubernetesClient()), &globalOptions)
}

func TestParseMetaConfigDerivesTheClusterFromResources(t *testing.T) {
	metaConfig, err := parseMetaConfig(t, providerResources, infrastructureprovider.DhctlOperationConverge)
	require.NoError(t, err)

	require.False(t, metaConfig.HasLegacyProviderConfig(), "the provider is configured through resources")
	require.True(t, metaConfig.HasProviderModuleConfig())
	require.Equal(t, "cluster-uuid", metaConfig.UUID)

	// The count of control-plane nodes comes from the master NodeGroup among the resources.
	require.Equal(t, 3, metaConfig.MasterNodeGroupSpec.Replicas)

	require.NotNil(t, metaConfig.CloudProviderVars)
	require.Contains(t, metaConfig.CloudProviderVars.NodeGroups, "master")
	require.Contains(t, metaConfig.CloudProviderVars.InstanceClasses, "master-class")
	require.NotEmpty(t, metaConfig.CloudProviderVars.Secrets, "the credentials Secret has to reach the infrastructure utility")
}

// The resources join the configuration after the cluster configuration, so documents split by a
// separator at either end, or by several separators, must still parse.
func TestParseMetaConfigJoinsResourcesWithTheConfiguration(t *testing.T) {
	for name, resources := range map[string]string{
		"leading separator":  "---\n" + providerResources,
		"trailing separator": providerResources + "\n---\n",
		"blank lines around": "\n\n" + providerResources + "\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			metaConfig, err := parseMetaConfig(t, resources, infrastructureprovider.DhctlOperationConverge)
			require.NoError(t, err)
			require.Equal(t, 3, metaConfig.MasterNodeGroupSpec.Replicas)
			require.Contains(t, metaConfig.CloudProviderVars.NodeGroups, "master")
		})
	}
}

// A cloud cluster that keeps its provider in resources and lost the master NodeGroup must not
// reach check or converge: the count of control-plane nodes would read as zero.
func TestParseMetaConfigRefusesResourcesWithoutMaster(t *testing.T) {
	for name, resources := range map[string]string{
		"no resources at all":      "",
		"resources without master": providerResourcesWithoutMaster,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseMetaConfig(t, resources, infrastructureprovider.DhctlOperationConverge)
			require.ErrorContains(t, err, "NodeGroup \"master\"")
			require.ErrorContains(t, err, "minPerZone")
		})
	}
}

// Destroy removes the nodes whatever their number, so it must stay possible for a cluster whose
// resources lost the master.
func TestParseMetaConfigDestroyDoesNotNeedTheMaster(t *testing.T) {
	_, err := parseMetaConfig(t, providerResourcesWithoutMaster, infrastructureprovider.DhctlOperationDestroy)
	require.NoError(t, err)
}

func TestRequireControlPlaneSpec(t *testing.T) {
	cloud := func(mutate func(m *config.MetaConfig)) *config.MetaConfig {
		m := &config.MetaConfig{ClusterType: config.CloudClusterType}
		mutate(m)
		return m
	}

	tests := map[string]struct {
		metaConfig *config.MetaConfig
		operation  string
		wantErr    bool
	}{
		"cloud, resources, master known": {
			metaConfig: cloud(func(m *config.MetaConfig) { m.MasterNodeGroupSpec.Replicas = 1 }),
			operation:  infrastructureprovider.DhctlOperationConverge,
		},
		"cloud, resources, master unknown": {
			metaConfig: cloud(func(*config.MetaConfig) {}),
			operation:  infrastructureprovider.DhctlOperationConverge,
			wantErr:    true,
		},
		"cloud, legacy provider configuration": {
			metaConfig: cloud(func(m *config.MetaConfig) {
				m.ProviderClusterConfig = map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)}
			}),
			operation: infrastructureprovider.DhctlOperationConverge,
		},
		"cloud, master unknown, destroy": {
			metaConfig: cloud(func(*config.MetaConfig) {}),
			operation:  infrastructureprovider.DhctlOperationDestroy,
		},
		"static cluster": {
			metaConfig: &config.MetaConfig{ClusterType: config.StaticClusterType},
			operation:  infrastructureprovider.DhctlOperationConverge,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := requireControlPlaneSpec(tt.metaConfig, tt.operation)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
