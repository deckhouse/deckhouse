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

package destroy

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestInitStateLoader(t *testing.T) {
	createKubeProvider := func() kube.ClientProviderWithCleanup {
		kubeCl := testCreateFakeKubeClient()
		return newFakeKubeClientProvider(kubeCl)
	}

	clusterUUID := uuid.Must(uuid.NewRandom()).String()

	noBeforeFunc := func(t *testing.T, tst *testInitStateLoader) {}
	fillCommanderStateBeforeFunc := func(t *testing.T, tst *testInitStateLoader) {
		testAddCloudStatesToCache(t, tst.stateCache, tst.clusterUUID)
	}

	noAssertFunc := func(t *testing.T, tst *testInitStateLoader) {}
	assertEmptyCacheFunc := func(t *testing.T, tst *testInitStateLoader) {
		tst.assertStateCacheIsEmpty(t)
	}

	t.Run("no commander", func(t *testing.T) {
		noCommanderHappyCaseKubeProvider := createKubeProvider()
		noCommanderTests := []*testInitStateLoader{
			newTestInitStateLoader(&testInitStateLoader{
				name: "happy case with state in kube",
				params: &Params{
					SkipResources:       false,
					CommanderMode:       false,
					CommanderModeParams: nil,
				},
				kubeProvider: noCommanderHappyCaseKubeProvider,
				before:       testCreateMetaConfigForInitLoaderTestInCluster,
				assertBefore: assertEmptyCacheFunc,
				assertLoader: assertFromClusterKeysInCacheAfterLoad,
				clusterUUID:  clusterUUID,

				expectedStateLoaderType:  &infrastructurestate.LazyTerraStateLoader{},
				expectedKubeProviderType: noCommanderHappyCaseKubeProvider,
				hasInitError:             false,
				kubeProviderAsPassed:     true,
				hasLoadMetaConfigError:   false,
				hasLoadStateError:        false,
			}),

			newTestInitStateLoader(&testInitStateLoader{
				name: "skip resources: keys not in cache",
				params: &Params{
					SkipResources:       true,
					CommanderMode:       false,
					CommanderModeParams: nil,
				},
				kubeProvider: createKubeProvider(),
				before:       noBeforeFunc,
				assertBefore: assertEmptyCacheFunc,
				assertLoader: assertFromClusterKeysInCacheAfterLoad,

				clusterUUID: clusterUUID,

				expectedStateLoaderType:  &infrastructurestate.LazyTerraStateLoader{},
				expectedKubeProviderType: &kubeClientErrorProvider{},
				hasInitError:             false,
				kubeProviderAsPassed:     false,
				hasLoadMetaConfigError:   true,
				hasLoadStateError:        true,
			}),

			newTestInitStateLoader(&testInitStateLoader{
				name: "skip resources: keys in cache",
				params: &Params{
					SkipResources:       true,
					CommanderMode:       false,
					CommanderModeParams: nil,
				},
				kubeProvider: createKubeProvider(),
				before: func(t *testing.T, tst *testInitStateLoader) {
					testCreateMetaConfigForInitLoaderTestInCluster(t, tst)
					loader := infrastructurestate.NewCachedTerraStateLoader(tst.kubeProvider, tst.params.StateCache, tst.params.LoggerProvider())
					ctx := context.TODO()
					_, err := loader.PopulateMetaConfig(ctx)
					require.NoError(t, err, "populate metaconfig before test")
					_, _, err = loader.PopulateClusterState(ctx)
					require.NoError(t, err, "populate state before test")
				},
				assertBefore: assertFromClusterKeysInCacheAfterLoad,
				assertLoader: assertFromClusterKeysInCacheAfterLoad,

				clusterUUID: clusterUUID,

				expectedStateLoaderType:  &infrastructurestate.LazyTerraStateLoader{},
				expectedKubeProviderType: &kubeClientErrorProvider{},
				hasInitError:             false,
				kubeProviderAsPassed:     false,
				hasLoadMetaConfigError:   false,
				hasLoadStateError:        false,
			}),
		}

		for _, tst := range noCommanderTests {
			t.Run(tst.name, func(t *testing.T) {
				tst.do(t)
			})
		}
	})

	t.Run("in commander", func(t *testing.T) {
		assertFileKeysInCacheAfterLoad := func(t *testing.T, tst *testInitStateLoader) {
			tst.assertFileKeysInCacheAfterLoad(t)
		}

		commanderKubeProvider := createKubeProvider()
		commanderTests := []*testInitStateLoader{
			newTestInitStateLoader(&testInitStateLoader{
				name: "happy case with state in kube",
				params: &Params{
					SkipResources: false,
					CommanderMode: true,
					CommanderModeParams: commander.NewCommanderModeParams(
						[]byte(cloudClusterGenericConfigYAML),
						[]byte(providerConfigYAML),
					),
				},
				kubeProvider: commanderKubeProvider,
				before:       fillCommanderStateBeforeFunc,
				assertBefore: noAssertFunc,
				assertLoader: assertFileKeysInCacheAfterLoad,
				clusterUUID:  clusterUUID,

				expectedStateLoaderType:  &infrastructurestate.FileTerraStateLoader{},
				expectedKubeProviderType: commanderKubeProvider,
				hasInitError:             false,
				kubeProviderAsPassed:     true,
				hasLoadMetaConfigError:   false,
				hasLoadStateError:        false,
			}),

			newTestInitStateLoader(&testInitStateLoader{
				name: "skip resources does not matter",
				params: &Params{
					SkipResources: true,
					CommanderMode: true,
					CommanderModeParams: commander.NewCommanderModeParams(
						[]byte(cloudClusterGenericConfigYAML),
						[]byte(providerConfigYAML),
					),
				},
				kubeProvider: commanderKubeProvider,
				before:       fillCommanderStateBeforeFunc,
				assertBefore: noAssertFunc,
				assertLoader: assertFileKeysInCacheAfterLoad,
				clusterUUID:  clusterUUID,

				expectedStateLoaderType:  &infrastructurestate.FileTerraStateLoader{},
				expectedKubeProviderType: commanderKubeProvider,
				hasInitError:             false,
				kubeProviderAsPassed:     true,
				hasLoadMetaConfigError:   false,
				hasLoadStateError:        false,
			}),

			newTestInitStateLoader(&testInitStateLoader{
				name: "state cache is empty",
				params: &Params{
					SkipResources: true,
					CommanderMode: true,
					CommanderModeParams: commander.NewCommanderModeParams(
						[]byte(cloudClusterGenericConfigYAML),
						[]byte(providerConfigYAML),
					),
				},
				kubeProvider: commanderKubeProvider,
				before:       noBeforeFunc,
				assertBefore: noAssertFunc,
				assertLoader: noAssertFunc,
				clusterUUID:  clusterUUID,

				expectedStateLoaderType:  nil,
				expectedKubeProviderType: nil,
				hasInitError:             true,
				kubeProviderAsPassed:     true,
				hasLoadMetaConfigError:   true,
				hasLoadStateError:        true,
			}),

			newTestInitStateLoader(&testInitStateLoader{
				name: "incorrect config",
				params: &Params{
					SkipResources: true,
					CommanderMode: true,
					CommanderModeParams: commander.NewCommanderModeParams(
						[]byte(`{"a": "b"}`),
						[]byte(`{"c": "d"}`),
					),
				},
				kubeProvider: commanderKubeProvider,
				before:       fillCommanderStateBeforeFunc,
				assertBefore: noAssertFunc,
				assertLoader: assertFileKeysInCacheAfterLoad,
				clusterUUID:  clusterUUID,

				expectedStateLoaderType:  nil,
				expectedKubeProviderType: nil,
				hasInitError:             true,
				kubeProviderAsPassed:     false,
				hasLoadMetaConfigError:   true,
				hasLoadStateError:        true,
			}),
		}

		for _, tst := range commanderTests {
			t.Run(tst.name, func(t *testing.T) {
				tst.do(t)
			})
		}
	})
}

type testInitStateLoader struct {
	*baseTest

	name         string
	params       *Params
	kubeProvider kube.ClientProviderWithCleanup
	before       func(t *testing.T, tst *testInitStateLoader)
	assertBefore func(t *testing.T, tst *testInitStateLoader)
	assertLoader func(t *testing.T, tst *testInitStateLoader)
	clusterUUID  string

	expectedStateLoaderType  controller.StateLoader
	expectedKubeProviderType kube.ClientProviderWithCleanup
	kubeProviderAsPassed     bool
	hasInitError             bool
	hasLoadStateError        bool
	hasLoadMetaConfigError   bool
}

func newTestInitStateLoader(tst *testInitStateLoader) *testInitStateLoader {
	stateCache := cache.NewTestCache()
	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())

	tst.baseTest = &baseTest{
		stateCache:   stateCache,
		tmpDir:       "",
		logger:       logger,
		kubeProvider: tst.kubeProvider,
	}

	tst.params.StateCache = stateCache
	tst.params.LoggerProvider = log.SimpleLoggerProvider(logger)

	return tst
}

func (ts *testInitStateLoader) do(t *testing.T) {
	ts.before(t, ts)

	ctx := context.TODO()

	initParams := ts.params.getStateLoaderParams()
	require.False(t, govalue.IsNil(initParams.logger))
	require.False(t, initParams.forceFromCache)
	initParams.forceFromCache = true

	stateLoader, provider, err := initStateLoader(ctx, initParams, ts.kubeProvider)

	createAssertError(ts.hasInitError, "should init no error", "should init error")(t, err)

	ts.assertBefore(t, ts)

	if ts.kubeProviderAsPassed {
		require.Equal(t, ts.expectedKubeProviderType, provider, "kube provider returned as passed")
	}

	require.IsType(t, ts.expectedKubeProviderType, provider, "incorrect kube provider type")
	require.IsType(t, ts.expectedStateLoaderType, stateLoader, "incorrect state loader type")

	if ts.hasInitError && govalue.IsNil(stateLoader) {
		log.SafeProvideLogger(ts.params.LoggerProvider).LogInfoLn("Has init error and state loader is nil. Skip")
		return
	}

	metaConfig, err := stateLoader.PopulateMetaConfig(ctx)
	createAssertError(ts.hasLoadMetaConfigError, "should load metaconfig", "should not load metaconfig")(t, err)

	if !ts.hasLoadMetaConfigError {
		require.Equal(t, ts.clusterUUID, metaConfig.UUID, "should valid cluster UUID")
		require.NotEmpty(t, metaConfig.ClusterConfig, "cluster config should not be empty")
		require.NotEmpty(t, metaConfig.ProviderClusterConfig, "provider cluster config should not be empty")
	}

	clusterState, nodesStates, err := stateLoader.PopulateClusterState(ctx)
	createAssertError(ts.hasLoadStateError, "should load states", "should not load states")(t, err)

	if !ts.hasLoadStateError {
		require.NotEmpty(t, clusterState, "cluster state should not be empty")
		require.NotEmpty(t, nodesStates, "nodes states should not be empty")
		require.Contains(t, nodesStates, global.MasterNodeGroupName, "nodes state should have master node group")
		require.NotEmpty(t, nodesStates[global.MasterNodeGroupName], "nodes state for master ng should not be empty")

		ts.assertLoader(t, ts)
	}
}

func assertFromClusterKeysInCacheAfterLoad(t *testing.T, tst *testInitStateLoader) {
	stateKeys := tst.stateCacheKeys(t)
	expectedKeys := []string{
		metaConfigKey,
		clusterStateKey,
		nodesStateKey,
	}

	for _, key := range expectedKeys {
		require.Contains(t, stateKeys, key, "state cache should contain key", key)
	}
}

func testCreateMetaConfigForInitLoaderTestInCluster(t *testing.T, tst *testInitStateLoader) {
	require.False(t, govalue.IsNil(tst.kubeProvider))
	require.NotEmpty(t, tst.clusterUUID, "cluster UUID should not be empty")

	ctx := context.TODO()

	client, err := tst.kubeProvider.KubeClientCtx(ctx)
	require.NoError(t, err, "kube client should returned")

	testCreateProviderClusterConfigSecret(t, client, providerConfigYAML)

	testCreateClusterConfigSecret(t, client, cloudClusterGenericConfigYAML)

	testCreateClusterUUIDCM(t, client, tst.clusterUUID)

	testCreateSystemSecret(t, client, manifests.InfrastructureClusterStateName, map[string][]byte{
		"cluster-tf-state.json": []byte(`{}`),
	})

	masterSecret := manifests.SecretWithNodeInfrastructureState(
		"master-0",
		global.MasterNodeGroupName,
		[]byte(`{}`),
		nil,
	)

	_, err = client.CoreV1().Secrets(global.D8SystemNamespace).Create(ctx, masterSecret, metav1.CreateOptions{})
	require.NoError(t, err, "master secret should create")
}
