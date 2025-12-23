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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sapcloud "github.com/deckhouse/deckhouse/dhctl/pkg/apis/sapcloudio/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

var (
	rootTmpDirCloud = path.Join(os.TempDir(), "dhctl-test-cloud-destroy")
)

func TestCloudDestroy(t *testing.T) {
	defer func() {
		logger := log.GetDefaultLogger()
		if err := os.RemoveAll(rootTmpDirCloud); err != nil {
			logger.LogErrorF("Couldn't remove temp dir '%s': %v\n", rootTmpDirCloud, err)
			return
		}
		logger.LogInfoF("Tmp dir '%s' removed\n", rootTmpDirCloud)
	}()

	t.Run("no commander", func(t *testing.T) {
		noBeforeFunc := func(t *testing.T, tst *testCloudDestroyTest) {}
		noAssertFunc := func(t *testing.T, tst *testCloudDestroyTest) {}

		noCommanderTest := func(tst testCloudDestroyTestParams) testCloudDestroyTestParams {
			tst.commanderMode = false
			tst.commanderModeParams = nil
			tst.commanderUUID = uuid.Nil
			tst.commanderUUIDInCluster = uuid.Nil

			return tst
		}

		setAllStatesInCache := func(t *testing.T, tst *testCloudDestroyTest) {
			tst.saveMetaConfigToCache(t)
			tst.saveInfraStateKeys(t)
			tst.setResourcesDestroyed(t)
			tst.setConvergeLock(t)
		}

		assertAllStateInCacheAfterDestroy := func(t *testing.T, tst *testCloudDestroyTest) {
			tst.assertHasMetaConfigInCache(t, true)
			tst.assertConvergeLockSetInCache(t, true)
			tst.assertResourcesDestroyed(t, true)
			tst.assertInfraStateInCache(t, true)
		}

		noCommanderTests := []struct {
			testCloudDestroyTestParams
			name string

			stateCacheShouldEmpty            bool
			resourcesShouldDeleted           bool
			destroyClusterShouldReturnsError bool
			kubeProviderShouldCleaned        bool
			lockShouldCreated                bool

			before func(t *testing.T, tst *testCloudDestroyTest)
			assert func(t *testing.T, tst *testCloudDestroyTest)
		}{
			{
				name: "happy case",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     false,
				}),

				stateCacheShouldEmpty:            true,
				resourcesShouldDeleted:           true,
				destroyClusterShouldReturnsError: false,
				kubeProviderShouldCleaned:        true,
				lockShouldCreated:                true,

				before: noBeforeFunc,
				assert: noAssertFunc,
			},

			{
				name: "happy case. already locked in cache",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     false,
				}),

				stateCacheShouldEmpty:            true,
				resourcesShouldDeleted:           true,
				destroyClusterShouldReturnsError: false,
				kubeProviderShouldCleaned:        true,
				// test that lock is not created if in cache
				lockShouldCreated: false,

				before: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.setConvergeLock(t)
				},
				assert: noAssertFunc,
			},

			{
				name: "restart. all saved in cache before destroy",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     false,
					// should not call kube api if all in cache
					errorKubeProvider: true,
				}),

				stateCacheShouldEmpty: true,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: false,
				// error provider here
				kubeProviderShouldCleaned: false,
				// test that lock is not created if in cache
				lockShouldCreated: false,

				before: setAllStatesInCache,
				assert: noAssertFunc,
			},

			{
				name: "lock returns error",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     false,
					// for returning error while lock
					errorKubeProvider: true,
				}),

				stateCacheShouldEmpty: false,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: true,
				// error provider here
				kubeProviderShouldCleaned: false,
				// test that lock is not created if in cache
				lockShouldCreated: false,

				before: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.saveMetaConfigToCache(t)
				},
				assert: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.assertHasMetaConfigInCache(t, true)
					tst.assertConvergeLockSetInCache(t, false)
				},
			},

			{
				name: "get states from cluster returns error",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     false,
					// for returning error while states
					errorKubeProvider: true,
				}),

				stateCacheShouldEmpty: false,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: true,
				// error provider here
				kubeProviderShouldCleaned: false,
				// test that lock is not created if in cache
				lockShouldCreated: false,

				before: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.saveMetaConfigToCache(t)
					tst.setConvergeLock(t)
					tst.setResourcesDestroyed(t)
				},
				assert: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.assertHasMetaConfigInCache(t, true)
					tst.assertConvergeLockSetInCache(t, true)
					tst.assertResourcesDestroyed(t, true)
					tst.assertInfraStateInCache(t, false)
				},
			},

			{
				name: "infra destroyer returns error",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: errors.New("error"),
					skipResources:     false,
					// should not call kube api if all in cache
					errorKubeProvider: true,
				}),

				stateCacheShouldEmpty: false,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: true,
				// error provider here
				kubeProviderShouldCleaned: false,
				// test that lock is not created if in cache
				lockShouldCreated: false,

				before: setAllStatesInCache,
				assert: assertAllStateInCacheAfterDestroy,
			},

			{
				name: "skip resources passed. should return error if no states in cache",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     true,
					errorKubeProvider: false,
				}),

				stateCacheShouldEmpty: true,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: true,
				kubeProviderShouldCleaned:        false,
				lockShouldCreated:                false,

				before: noBeforeFunc,
				assert: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.assertHasMetaConfigInCache(t, false)
					tst.assertConvergeLockSetInCache(t, false)
					tst.assertResourcesDestroyed(t, false)
					tst.assertInfraStateInCache(t, false)
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},

			{
				name: "skip resources passed. metaconfig in cache",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     true,
					errorKubeProvider: false,
				}),

				stateCacheShouldEmpty: false,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: true,
				kubeProviderShouldCleaned:        false,
				lockShouldCreated:                false,

				before: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.saveMetaConfigToCache(t)
				},
				assert: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.assertHasMetaConfigInCache(t, true)
					tst.assertConvergeLockSetInCache(t, false)
					tst.assertResourcesDestroyed(t, false)
					tst.assertInfraStateInCache(t, false)
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},

			{
				name: "skip resources passed. all in cache but infra destroyer returns error",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: errors.New("error"),
					skipResources:     true,
					errorKubeProvider: false,
				}),

				stateCacheShouldEmpty: false,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: true,
				kubeProviderShouldCleaned:        false,
				lockShouldCreated:                false,

				before: setAllStatesInCache,
				assert: func(t *testing.T, tst *testCloudDestroyTest) {
					assertAllStateInCacheAfterDestroy(t, tst)
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},

			{
				name: "skip resources passed. all in cache should destroy",

				testCloudDestroyTestParams: noCommanderTest(testCloudDestroyTestParams{
					infraDestroyerErr: nil,
					skipResources:     true,
					errorKubeProvider: false,
				}),

				stateCacheShouldEmpty: true,
				// test that we cannot call kube api for destroy resources
				resourcesShouldDeleted:           false,
				destroyClusterShouldReturnsError: false,
				kubeProviderShouldCleaned:        false,
				lockShouldCreated:                false,

				before: setAllStatesInCache,
				assert: func(t *testing.T, tst *testCloudDestroyTest) {
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},
		}

		for _, tt := range noCommanderTests {
			t.Run(tt.name, func(t *testing.T) {
				tst := createTestCloudDestroyTest(t, tt.testCloudDestroyTestParams)
				defer tst.clean(t)

				resources := testCreateResourcesForCloud(t, tst.kubeCl)

				tt.before(t, tst)

				err := tst.destroyer.DestroyCluster(context.TODO(), true)
				assertClusterDestroyError(t, tt.destroyClusterShouldReturnsError, err)

				tst.assertStateCache(t, tt.stateCacheShouldEmpty)
				assertResources(t, tst.kubeCl, resources, tt.resourcesShouldDeleted)

				tst.assertDestroyLocked(t, tt.lockShouldCreated)
				tst.assertSkipCheckCommanderUUID(t, true)
				tst.assertKubeProviderCleaned(t, tt.kubeProviderShouldCleaned, true)

				tt.assert(t, tst)
			})
		}
	})

}

type testCloudDestroyTestParams struct {
	commanderMode          bool
	commanderModeParams    *commander.CommanderModeParams
	commanderUUID          uuid.UUID
	commanderUUIDInCluster uuid.UUID

	errorKubeProvider bool
	infraDestroyerErr error

	skipResources bool
}

type testCloudDestroyTest struct {
	*baseTest

	params testCloudDestroyTestParams

	destroyer *ClusterDestroyer

	kubeCl       *client.KubernetesClient
	kubeProvider kube.ClientProviderWithCleanup

	d8State    *deckhouse.State
	metaConfig *config.MetaConfig
}

func (ts *testCloudDestroyTest) saveInfraStateKeys(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))

	err := ts.stateCache.Save(clusterStateKey, []byte(`{}`))
	require.NoError(t, err)

	nodesState := map[string]state.NodeGroupInfrastructureState{
		global.MasterNodeGroupName: {
			State: map[string][]byte{
				"test-master-0": []byte(`{}`),
			},
		},
	}

	err = ts.stateCache.SaveStruct(nodesStateKey, nodesState)
	require.NoError(t, err)
}

func (ts *testCloudDestroyTest) assertInfraStateInCache(t *testing.T, inCache bool) {
	require.False(t, govalue.IsNil(ts.stateCache))

	clusterState, err := ts.stateCache.Load(clusterStateKey)
	require.NoError(t, err)
	assert := require.Empty
	if inCache {
		assert = require.NotEmpty
	}
	assert(t, clusterState, "cluster state should or not in cache")

	var nodesState map[string]state.NodeGroupInfrastructureState

	err = ts.stateCache.LoadStruct(nodesStateKey, &nodesState)

	assertNodesState := require.Error
	if inCache {
		assertNodesState = require.NoError
	}
	assertNodesState(t, err, "cluster state should or not in cache")
}

func (ts *testCloudDestroyTest) setConvergeLock(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))

	err := cloud.NewDestroyState(ts.stateCache).SetConvergeLocked()
	require.NoError(t, err)
}

func (ts *testCloudDestroyTest) assertConvergeLockSetInCache(t *testing.T, locked bool) {
	require.False(t, govalue.IsNil(ts.stateCache))

	lockedInCache, err := cloud.NewDestroyState(ts.stateCache).IsConvergeLocked()
	require.NoError(t, err)

	require.Equal(t, locked, lockedInCache, "should be converge locked or not")
}

func (ts *testCloudDestroyTest) assertDestroyLocked(t *testing.T, locked bool) {
	require.False(t, govalue.IsNil(ts.kubeCl))

	lockConfig := lock.GetLockLeaseConfig("not necessary")
	lockedInCluster, err := lock.IsConvergeLocked(context.TODO(), kubernetes.NewSimpleKubeClientGetter(ts.kubeCl), lockConfig, false)
	require.NoError(t, err, "is locked should not be error")

	require.Equal(t, locked, lockedInCluster, "should be locked or not")
}

func (ts *testCloudDestroyTest) assertSkipCheckCommanderUUID(t *testing.T, skip bool) {
	require.False(t, govalue.IsNil(ts.logger))

	match, err := ts.logger.FirstMatch(&log.Match{
		Prefix: []string{"Check commander UUID skipped"},
	})
	require.NoError(t, err)

	assert := require.Empty
	if skip {
		assert = require.NotEmpty
	}

	assert(t, match, "should skip commander UUID or not")
}

func testCreateResourcesForCloud(t *testing.T, kubeCl *client.KubernetesClient) []testCreatedResource {
	resources := append(testCreateResourcesGeneral(t, kubeCl), testCreateCAPIResources(t, kubeCl)...)
	resources = append(resources, testCreateMCMResources(t, kubeCl)...)
	return resources
}

func testCreateMCMResources(t *testing.T, kubeCl *client.KubernetesClient) []testCreatedResource {
	createdResources := make([]testCreatedResource, 0)

	md := testYAMLToUnstructured(t, `
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "6"
    zone: nova
  creationTimestamp: "2021-09-07T11:14:39Z"
  generation: 34
  labels:
    heritage: deckhouse
    module: node-manager
    node-group: worker-big
  name: sandbox-worker-big-8ef4a622
  namespace: d8-cloud-instance-manager
  resourceVersion: "1497359242"
  uid: d584b32b-a2bc-4a4c-bcec-7ba7d9cd8e76
spec:
  minReadySeconds: 300
  replicas: 0
  selector:
    matchLabels:
      instance-group: worker-big-nova
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  template:
    metadata:
      annotations:
        checksum/machine-class: 2c04cd98ddb84562f6ada8afb8954b9ecdbc8156341819c4856ead0f758c9a55
      creationTimestamp: null
      labels:
        instance-group: worker-big-nova
    spec:
      class:
        kind: OpenStackMachineClass
        name: worker-big-8ef4a622
      drainTimeout: 600s
      maxEvictRetries: 30
      nodeTemplate:
        metadata:
          creationTimestamp: null
          labels:
            node-role.kubernetes.io/worker-big: ""
            node.deckhouse.io/group: worker-big
            node.deckhouse.io/type: CloudEphemeral
        spec: {}
`)
	_, err := kubeCl.Dynamic().Resource(sapcloud.MachineDeploymentGVR).Namespace(md.GetNamespace()).Create(context.TODO(), md, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: md.GetName(),
		ns:   md.GetNamespace(),
		kind: "MCMMachineDeployment",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.Dynamic().Resource(sapcloud.MachineDeploymentGVR).Namespace(md.GetNamespace()).Get(ctx, md.GetName(), metav1.GetOptions{})
			return err
		},
	})

	return createdResources
}

func createTestCloudDestroyTest(t *testing.T, params testCloudDestroyTestParams) *testCloudDestroyTest {
	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())

	stateCache := cache.NewTestCache()

	kubeCl := testCreateFakeKubeClient()
	kubeClProvider := newFakeKubeClientProvider(kubeCl)

	ctx := context.TODO()

	clusterUUID := uuid.Must(uuid.NewRandom()).String()

	commanderUUIDInCluster := params.commanderUUIDInCluster
	commanderUUID := params.commanderUUID
	if commanderUUIDInCluster == uuid.Nil && commanderUUID == uuid.Nil {
		oneUUIDForAll := uuid.Must(uuid.NewRandom())
		commanderUUID = oneUUIDForAll
		commanderUUIDInCluster = oneUUIDForAll
	}

	var metaConfig *config.MetaConfig

	if params.commanderMode {
		require.NotNil(t, params.commanderModeParams, "commanderModeParams should not be nil")
		uuidCM := manifests.CommanderUUIDConfigMap(commanderUUIDInCluster.String())
		_, err := kubeCl.CoreV1().ConfigMaps(uuidCM.GetNamespace()).Create(ctx, uuidCM, metav1.CreateOptions{})
		require.NoError(t, err, "commander uuid cm should create")
		testAddCloudStatesToCache(t, stateCache, clusterUUID)
		metaConfig, err = commander.ParseMetaConfig(ctx, stateCache, params.commanderModeParams, logger)
		require.NoError(t, err)
	} else {
		d8SystemNs := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "d8-system",
			},
		}

		_, err := kubeCl.CoreV1().Namespaces().Create(ctx, &d8SystemNs, metav1.CreateOptions{})
		require.NoError(t, err, "d8-system namespace should create")

		testCreateClusterConfigSecret(t, kubeCl, cloudClusterGenericConfigYAML)
		testCreateProviderClusterConfigSecret(t, kubeCl, providerConfigYAML)
		testCreateClusterUUIDCM(t, kubeCl, clusterUUID)
		metaConfig, err = config.ParseConfigFromCluster(ctx, kubeCl, config.DummyPreparatorProvider())
		require.NoError(t, err)
	}

	loaderParams := &stateLoaderParams{
		commanderMode:   params.commanderMode,
		commanderParams: params.commanderModeParams,
		stateCache:      stateCache,
		logger:          logger,
		skipResources:   params.skipResources,
		forceFromCache:  true,
	}

	errorKubeProvider := newKubeClientErrorProvider("does not call kube api during destroy")

	var kubeProviderForLoader kube.ClientProviderWithCleanup = kubeClProvider
	if params.errorKubeProvider {
		kubeProviderForLoader = errorKubeProvider
	}

	loader, kubeProviderForInfraDestroyer, err := initStateLoader(ctx, loaderParams, kubeProviderForLoader)
	require.NoError(t, err)

	if params.errorKubeProvider {
		kubeProviderForInfraDestroyer = errorKubeProvider
	}

	loggerProvider := log.SimpleLoggerProvider(logger)
	pipeline := phases.NewDummyDefaultPipelineProviderOpts(
		phases.WithPipelineName("cloud destroy"),
		phases.WithPipelineLoggerProvider(loggerProvider),
	)()

	phaseActionProvider := phases.NewPhaseActionProviderFromPipeline(pipeline)
	d8State := deckhouse.NewState(stateCache)

	d8Destroyer := deckhouse.NewDestroyer(deckhouse.DestroyerParams{
		CommanderMode: params.commanderMode,
		CommanderUUID: commanderUUID,
		SkipResources: params.skipResources,

		State: d8State,

		LoggerProvider:       loggerProvider,
		KubeProvider:         kubeClProvider,
		PhasedActionProvider: phaseActionProvider,
	})

	i := rand.New(rand.NewSource(time.Now().UnixNano()))
	tmpDir, err := fs.RandomTmpDirWithNRunes(rootTmpDirStatic, fmt.Sprintf("%d", i), 15)
	require.NoError(t, err)

	logger.LogInfoF("Tmp dir: '%s'\n", tmpDir)

	infraProvider := &infraDestroyerProvider{
		stateCache:           stateCache,
		loggerProvider:       loggerProvider,
		kubeProvider:         kubeProviderForInfraDestroyer,
		phasesActionProvider: phaseActionProvider,

		cloudStateProvider: func() (controller.StateLoader, cloud.ClusterInfraDestroyer, error) {
			return loader, newCloudInfraDestroyer(params.infraDestroyerErr), nil
		},
		commanderMode: params.commanderMode,
		skipResources: params.skipResources,

		tmpDir: tmpDir,
	}

	destroyer := &ClusterDestroyer{
		stateCache:       stateCache,
		configPreparator: loader,

		pipeline: pipeline,

		d8Destroyer:   d8Destroyer,
		infraProvider: infraProvider,
	}

	tst := &testCloudDestroyTest{
		baseTest: &baseTest{
			logger:       logger,
			tmpDir:       tmpDir,
			stateCache:   stateCache,
			kubeProvider: kubeProviderForInfraDestroyer,
			metaConfig:   metaConfig,
		},

		params: params,

		destroyer: destroyer,

		d8State:    d8State,
		metaConfig: metaConfig,

		kubeCl: kubeCl,

		kubeProvider: kubeProviderForInfraDestroyer,
	}

	return tst
}
