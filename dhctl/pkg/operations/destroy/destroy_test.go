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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis"
	capi "github.com/deckhouse/deckhouse/dhctl/pkg/apis/capi/v1beta1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	sapcloud "github.com/deckhouse/deckhouse/dhctl/pkg/apis/sapcloudio/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/static"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/testssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	rootTmpDirStatic = path.Join(os.TempDir(), "dhctl-test-static-destroy")
)

func TestStaticDestroy(t *testing.T) {
	defer func() {
		logger := log.GetDefaultLogger()
		if err := os.RemoveAll(rootTmpDirStatic); err != nil {
			logger.LogErrorF("Couldn't remove temp dir '%s': %v\n", rootTmpDirStatic, err)
			return
		}
		logger.LogInfoF("Tmp dir '%s' removed\n", rootTmpDirStatic)
	}()

	t.Run("skip resources returns errors because metaconfig not in cache", func(t *testing.T) {
		hosts := []session.Host{
			{Host: "127.0.0.2", Name: "master-1"},
		}
		params := testStaticDestroyTestParams{
			skipResources:   true,
			commanderMode:   false,
			commanderParams: nil,
			destroyOverHost: hosts[0],
			hosts:           hosts,
		}

		tst := createTestStaticDestroyTest(t, params)
		defer tst.clean(t)

		testCreateNodes(t, tst.kubeCl, hosts)
		resources := testCreateResourcesForStatic(t, tst.kubeCl)

		err := tst.destroyer.DestroyCluster(context.TODO(), true)
		require.Error(t, err)
		tst.assertStateCacheIsEmpty(t)
		// skip deleting
		assertResourceExists(t, tst.kubeCl, resources)
	})

	assertStateHasMetaconfigAndResourcesDestroyed := func(t *testing.T, tst *testStaticDestroyTest) {
		destroyed, err := tst.d8State.IsResourcesDestroyed()
		require.NoError(t, err)
		require.True(t, destroyed)

		tst.assertHasMetaConfigInCache(t)
	}

	t.Run("one node", func(t *testing.T) {
		noStaticOneBeforeFunc := func(t *testing.T, tst *testStaticDestroyTest) {}

		oneNodeTests := []struct {
			name string

			sshOut string
			sshErr error

			stateCacheEmpty       bool
			skipResources         bool
			skipResourcesCreating bool

			before func(t *testing.T, tst *testStaticDestroyTest)
			assert func(t *testing.T, tst *testStaticDestroyTest, err error, resources []testCreatedResource)
		}{
			{
				name: "happy case",

				sshOut: "ok",
				sshErr: nil,

				stateCacheEmpty: true,

				before: noStaticOneBeforeFunc,
				assert: func(t *testing.T, tst *testStaticDestroyTest, err error, resources []testCreatedResource) {
					require.NoError(t, err)
					assertResourceDeleted(t, tst.kubeCl, resources)
					tst.assertKubeProviderCleaned(t)
				},
			},

			{
				name: "resources already deleted",

				sshOut: "ok",
				sshErr: nil,

				stateCacheEmpty: true,

				before: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.setResourcesDestroyed(t)
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest, err error, resources []testCreatedResource) {
					require.NoError(t, err)
					// skip deleting
					assertResourceExists(t, tst.kubeCl, resources)
					tst.assertKubeProviderCleaned(t)
				},
			},

			{
				name: "metaconfig in cache and skip resources but ips not in cache",

				sshOut: "ok",
				sshErr: nil,

				stateCacheEmpty:       false,
				skipResources:         true,
				skipResourcesCreating: true,

				before: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.setResourcesDestroyed(t)
					tst.saveMetaConfigToCache(t)
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest, err error, resources []testCreatedResource) {
					require.Error(t, err)
					assertStateHasMetaconfigAndResourcesDestroyed(t, tst)
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},

			{
				name: "metaconfig in cache and skip resources and ips in cache",

				sshOut: "ok",
				sshErr: nil,

				stateCacheEmpty:       true,
				skipResources:         true,
				skipResourcesCreating: true,

				before: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.saveNodeUser(t, tst.params.hosts, nil)
					tst.setResourcesDestroyed(t)
					tst.saveMetaConfigToCache(t)
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest, err error, resources []testCreatedResource) {
					require.NoError(t, err)
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},

			{
				name: "clean script returns error",

				sshOut: "error!",
				sshErr: errors.New("error"),

				stateCacheEmpty: false,

				before: noStaticOneBeforeFunc,
				assert: func(t *testing.T, tst *testStaticDestroyTest, err error, resources []testCreatedResource) {
					require.Error(t, err)
					assertStateHasMetaconfigAndResourcesDestroyed(t, tst)
					assertResourceDeleted(t, tst.kubeCl, resources)
					tst.assertKubeProviderCleaned(t)
				},
			},
		}

		for _, tt := range oneNodeTests {
			t.Run(tt.name, func(t *testing.T) {
				hosts := []session.Host{
					{Host: "127.0.0.2", Name: "master-1"},
				}
				params := testStaticDestroyTestParams{
					skipResources:   tt.skipResources,
					commanderMode:   false,
					commanderParams: nil,
					destroyOverHost: hosts[0],
					hosts:           hosts,
				}

				tst := createTestStaticDestroyTest(t, params)
				defer tst.clean(t)

				testCreateNodes(t, tst.kubeCl, hosts)

				var resources []testCreatedResource
				if !tt.skipResourcesCreating {
					resources = testCreateResourcesForStatic(t, tst.kubeCl)
				}

				tt.before(t, tst)

				testAddCleanCommand(tst.sshProvider, hosts[0], tt.sshOut, tt.sshErr, tst.logger)

				err := tst.destroyer.DestroyCluster(context.TODO(), true)

				tst.assertNodeUserDidNotCreate(t)
				tst.assertStateCache(t, tt.stateCacheEmpty)

				tt.assert(t, tst, err, resources)
			})
		}
	})

}

type testStaticDestroyTestParams struct {
	skipResources bool

	commanderMode   bool
	commanderParams *commander.CommanderModeParams

	destroyOverHost session.Host

	hosts []session.Host
}

type testWaiter struct {
	mu  sync.Mutex
	wg  *sync.WaitGroup
	err error
}

func newTestWaiter() *testWaiter {
	return &testWaiter{
		wg: new(sync.WaitGroup),
	}
}

func (w *testWaiter) setErr(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.err = err
}

func (w *testWaiter) getErr() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.err
}

type testStaticDestroyTest struct {
	params testStaticDestroyTestParams

	destroyer *ClusterDestroyer
	logger    *log.InMemoryLogger

	kubeCl *client.KubernetesClient

	sshProvider  *testssh.SSHProvider
	kubeProvider kube.ClientProviderWithCleanup

	stateCache dhctlstate.Cache
	d8State    *deckhouse.State
	metaConfig *config.MetaConfig

	tmpDir string
}

func (ts *testStaticDestroyTest) clean(t *testing.T) {
	require.NotEmpty(t, ts.tmpDir)
	require.False(t, govalue.IsNil(ts.logger))

	err := os.RemoveAll(ts.tmpDir)
	if err != nil {
		ts.logger.LogErrorF("Couldn't remove tmp dir '%s': %v\n", ts.tmpDir, err)
		return
	}

	ts.logger.LogInfoF("tmp dir '%s' removed\n", ts.tmpDir)
}

func (ts *testStaticDestroyTest) stateCacheKeys(t *testing.T) []string {
	require.False(t, govalue.IsNil(ts.stateCache))

	keys := make([]string, 0)

	err := ts.stateCache.Iterate(func(k string, _ []byte) error {
		keys = append(keys, k)
		return nil
	})
	require.NoError(t, err)

	return keys
}

func (ts *testStaticDestroyTest) setResourcesDestroyed(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))

	err := ts.d8State.SetResourcesDestroyed()
	require.NoError(t, err)
}

func (ts *testStaticDestroyTest) saveNodeUser(t *testing.T, hosts []session.Host, credentials *v1.NodeUserCredentials) {
	require.False(t, govalue.IsNil(ts.stateCache))

	ips := make([]entity.NodeIP, 0, len(hosts))
	for _, h := range hosts {
		ips = append(ips, entity.NodeIP{
			InternalIP: h.Host,
		})
	}

	credsToSave := credentials
	if credsToSave == nil {
		credsToSave = &v1.NodeUserCredentials{}
	}

	err := static.NewDestroyState(ts.stateCache).SaveNodeUser(&static.NodesWithCredentials{
		IPs:                 ips,
		NodeUserCredentials: credsToSave,
	})

	require.NoError(t, err)
}

const metaConfigKey = "cluster-config"

func (ts *testStaticDestroyTest) saveMetaConfigToCache(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))
	require.False(t, govalue.IsNil(ts.metaConfig))

	err := ts.stateCache.SaveStruct(metaConfigKey, ts.metaConfig)
	require.NoError(t, err)
}

func (ts *testStaticDestroyTest) assertHasMetaConfigInCache(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))

	inCache, err := ts.stateCache.InCache(metaConfigKey)
	require.NoError(t, err)
	require.True(t, inCache)
}

func (ts *testStaticDestroyTest) assertStateCache(t *testing.T, empty bool) {
	if empty {
		ts.assertStateCacheIsEmpty(t)
		return
	}

	ts.assertStateCacheNotEmpty(t)
}

func (ts *testStaticDestroyTest) assertStateCacheIsEmpty(t *testing.T) {
	keys := ts.stateCacheKeys(t)
	require.Empty(t, keys, fmt.Sprintf("has keys %v", keys))
}

func (ts *testStaticDestroyTest) assertStateCacheNotEmpty(t *testing.T) {
	keys := ts.stateCacheKeys(t)
	require.NotEmpty(t, keys, "has not keys")
}

func (ts *testStaticDestroyTest) assertNodeUserDidNotCreate(t *testing.T) {
	require.False(t, govalue.IsNil(ts.kubeCl))

	_, err := ts.kubeCl.Dynamic().Resource(v1.NodeUserGVR).Get(context.TODO(), global.ConvergeNodeUserName, metav1.GetOptions{})
	require.Error(t, err)
	require.True(t, k8errors.IsNotFound(err))
}

func (ts *testStaticDestroyTest) assertKubeProviderCleaned(t *testing.T) {
	require.False(t, govalue.IsNil(ts.kubeProvider))

	kubeProvider, ok := ts.kubeProvider.(*fakeKubeClientProvider)
	require.True(t, ok)
	require.True(t, kubeProvider.cleaned)
	require.False(t, kubeProvider.stopSSH)
}

func (ts *testStaticDestroyTest) assertKubeProviderIsErrorProvider(t *testing.T) {
	require.False(t, govalue.IsNil(ts.kubeProvider))
	require.IsType(t, &kubeClientErrorProvider{}, ts.kubeProvider)
}

func createTestStaticDestroyTest(t *testing.T, params testStaticDestroyTestParams) *testStaticDestroyTest {
	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())

	stateCache := cache.NewTestCache()

	kubeCl := testCreateFakeKubeClient()
	kubeClProvider := newFakeKubeClientProvider(kubeCl)

	ctx := context.TODO()

	if params.commanderMode {
		require.NotNil(t, params.commanderParams)
	}

	clusterUUID := uuid.Must(uuid.NewRandom())

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
	testCreateKubeSystemSecret(t, kubeCl, "d8-cluster-configuration", map[string][]byte{
		"cluster-configuration.yaml": []byte(clusterGenericConfig),
	})

	testCreateKubeSystemCM(t, kubeCl, "d8-cluster-uuid", map[string]string{
		"cluster-uuid": clusterUUID.String(),
	})

	metaConfig, err := config.ParseConfigFromCluster(context.TODO(), kubeCl, config.DummyPreparatorProvider())
	require.NoError(t, err)

	commanderUUID := uuid.Must(uuid.NewRandom())

	loaderParams := &stateLoaderParams{
		commanderMode:   params.commanderMode,
		commanderParams: params.commanderParams,
		stateCache:      stateCache,
		logger:          logger,
		skipResources:   params.skipResources,
		forceFromCache:  true,
	}

	loader, kubeProviderForInfraDestroyer, err := initStateLoader(ctx, loaderParams, kubeClProvider)
	require.NoError(t, err)

	loggerProvider := log.SimpleLoggerProvider(logger)
	pipeline := phases.NewDummyDefaultPipelineProviderOpts(
		phases.WithPipelineName("static destroy"),
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

	sshProvider := testssh.NewSSHProvider(session.NewSession(session.Input{
		User:        "notexists",
		Port:        "22",
		BastionHost: "127.0.0.1",
		BastionUser: "notexists",
		BastionPort: "22",
		BecomePass:  "",
		AvailableHosts: []session.Host{
			params.destroyOverHost,
		},
	}), true)

	i := rand.New(rand.NewSource(time.Now().UnixNano()))

	tmpDir, err := fs.RandomTmpDirWith10Runes(rootTmpDirStatic, fmt.Sprintf("%d", i), 15)
	require.NoError(t, err)

	logger.LogInfoF("Tmp dir: '%s'\n", tmpDir)

	infraProvider := &infraDestroyerProvider{
		stateCache:           stateCache,
		loggerProvider:       loggerProvider,
		kubeProvider:         kubeProviderForInfraDestroyer,
		phasesActionProvider: phaseActionProvider,
		commanderMode:        params.commanderMode,
		skipResources:        params.skipResources,
		cloudStateProvider:   nil,
		sshClientProvider:    sshProvider,
		tmpDir:               tmpDir,
		staticLoopsParams: static.LoopsParams{
			NodeUserLoopParams: retry.NewEmptyParams(
				retry.WithWait(2*time.Second),
				retry.WithAttempts(5),
			),
			DestroyMasterLoopParams: retry.NewEmptyParams(
				retry.WithWait(1*time.Second),
				retry.WithAttempts(1),
			),
			GetMastersIPsLoopParams: retry.NewEmptyParams(
				retry.WithWait(1*time.Second),
				retry.WithAttempts(2),
			),
		},
	}

	destroyer := &ClusterDestroyer{
		stateCache:       stateCache,
		configPreparator: loader,

		pipeline: pipeline,

		d8Destroyer:   d8Destroyer,
		infraProvider: infraProvider,
	}

	return &testStaticDestroyTest{
		params: params,

		destroyer: destroyer,
		logger:    logger,

		stateCache: stateCache,
		d8State:    d8State,
		metaConfig: metaConfig,

		kubeCl: kubeCl,

		sshProvider:  sshProvider,
		kubeProvider: kubeProviderForInfraDestroyer,

		tmpDir: tmpDir,
	}
}

type testCreatedResource struct {
	name         string
	ns           string
	kind         string
	getFunc      func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error
	shouldExists bool
}

func (t testCreatedResource) Name() string {
	return fmt.Sprintf("%s: %s/%s", t.kind, t.ns, t.name)
}

func assertResourceDeleted(t *testing.T, kubeCl *client.KubernetesClient, resources []testCreatedResource) {
	ctx := context.TODO()
	for _, r := range resources {
		err := r.getFunc(t, ctx, kubeCl)
		if r.shouldExists {
			require.NoError(t, err, r.Name())
			continue
		}

		require.Error(t, err, r.Name())
		require.True(t, k8errors.IsNotFound(err), r.Name(), err)
	}
}

func assertResourceExists(t *testing.T, kubeCl *client.KubernetesClient, resources []testCreatedResource) {
	ctx := context.TODO()
	for _, r := range resources {
		err := r.getFunc(t, ctx, kubeCl)
		require.NoError(t, err, r.Name())
	}
}

func testCreateResourcesForStatic(t *testing.T, kubeCl *client.KubernetesClient) []testCreatedResource {
	return append(testCreateResourcesGeneral(t, kubeCl), testCreateCAPIResources(t, kubeCl)...)
}

func testCreateResourcesGeneral(t *testing.T, kubeCl *client.KubernetesClient) []testCreatedResource {
	ctx := context.TODO()

	createdResources := make([]testCreatedResource, 0)

	deckhouseDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deckhouse",
			Namespace: "d8-system",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deckhouse",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}

	nss := []string{
		"d8-system",
		"d8-cloud-instance-manager",
		"test",
	}

	for _, ns := range nss {
		obj := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		}
		_, err := kubeCl.CoreV1().Namespaces().Create(ctx, &obj, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	_, err := kubeCl.AppsV1().Deployments(deckhouseDeployment.GetNamespace()).Create(ctx, deckhouseDeployment, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: deckhouseDeployment.GetName(),
		ns:   deckhouseDeployment.GetNamespace(),
		kind: "Deployment",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.AppsV1().Deployments(deckhouseDeployment.GetNamespace()).Get(ctx, deckhouseDeployment.GetName(), metav1.GetOptions{})
			return err
		},
	})

	minAvailable := intstr.FromString("25%")
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			MaxUnavailable: &minAvailable,
		},
	}
	_, err = kubeCl.PolicyV1().PodDisruptionBudgets(pdb.GetNamespace()).Create(ctx, &pdb, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: pdb.GetName(),
		ns:   pdb.GetNamespace(),
		kind: "PodDisruptionBudget",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.PolicyV1().PodDisruptionBudgets(pdb.GetNamespace()).Get(ctx, pdb.GetName(), metav1.GetOptions{})
			return err
		},
	})

	svcLb := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-svc",
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80},
			},
			Selector: map[string]string{
				"app": "test",
			},
			ClusterIP: corev1.ClusterIPNone,
			Type:      corev1.ServiceTypeLoadBalancer,
		},
	}

	svcCluster := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "d8-system",
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.0.0.1",
		},
	}

	for _, svc := range []corev1.Service{svcLb, svcCluster} {
		_, err := kubeCl.CoreV1().Services(svc.Namespace).Create(ctx, &svc, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	createdResources = append(createdResources, testCreatedResource{
		name: svcLb.GetName(),
		ns:   svcLb.GetNamespace(),
		kind: "Service",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.CoreV1().Services(svcLb.GetNamespace()).Get(ctx, svcLb.GetName(), metav1.GetOptions{})
			return err
		},
	})

	createdResources = append(createdResources, testCreatedResource{
		name: svcCluster.GetName(),
		ns:   svcCluster.GetNamespace(),
		kind: "Service",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.CoreV1().Services(svcCluster.GetNamespace()).Get(ctx, svcCluster.GetName(), metav1.GetOptions{})
			return err
		},
		shouldExists: true,
	})

	scLocal := testYAMLToUnstructured(t, `
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class
spec:
  lvm:
    lvmVolumeGroups:
    - name: vg-1-on-worker-0
      thin:
        poolName: thin-1
    type: Thin
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
`)

	_, err = kubeCl.Dynamic().Resource(v1alpha1.LocalStorageClassGRV).Create(ctx, scLocal, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: scLocal.GetName(),
		ns:   scLocal.GetNamespace(),
		kind: "LocalStorageClass",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.Dynamic().Resource(v1alpha1.LocalStorageClassGRV).Get(ctx, scLocal.GetName(), metav1.GetOptions{})
			return err
		},
	})

	reclame := corev1.PersistentVolumeReclaimDelete
	scDefault := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		AllowVolumeExpansion: pointer.Bool(true),
		Provisioner:          "test.csi.example.org",
		Parameters: map[string]string{
			"type": "__DEFAULT__",
		},
		ReclaimPolicy: &reclame,
	}

	_, err = kubeCl.StorageV1().StorageClasses().Create(ctx, &scDefault, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: scDefault.GetName(),
		ns:   scDefault.GetNamespace(),
		kind: "StorageClass",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.StorageV1().StorageClasses().Get(ctx, scDefault.GetName(), metav1.GetOptions{})
			return err
		},
	})

	pvcs := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upmeter",
				Namespace: "d8-system",
			},
		},
	}

	for _, pvc := range pvcs {
		pvc.Spec = corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Selector:         &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			StorageClassName: &scDefault.Name,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}

		_, err = kubeCl.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, &pvc, metav1.CreateOptions{})
		require.NoError(t, err)

		createdResources = append(createdResources, testCreatedResource{
			name: pvc.GetName(),
			ns:   pvc.GetNamespace(),
			kind: "PersistentVolumeClaim",
			getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
				_, err := kubeCl.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.GetName(), metav1.GetOptions{})
				return err
			},
		})
	}

	podWithoutVolumes := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "staus",
			Namespace: "d8-system",
		},
		Spec: corev1.PodSpec{
			NodeName:   "test",
			Containers: make([]corev1.Container, 0),
		},
	}

	_, err = kubeCl.CoreV1().Pods(podWithoutVolumes.GetNamespace()).Create(ctx, &podWithoutVolumes, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: podWithoutVolumes.GetName(),
		ns:   podWithoutVolumes.GetNamespace(),
		kind: "Pod",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.CoreV1().Pods(podWithoutVolumes.GetNamespace()).Get(ctx, podWithoutVolumes.GetName(), metav1.GetOptions{})
			return err
		},
		shouldExists: true,
	})

	podWithVolumes := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: corev1.PodSpec{
			NodeName:   "test",
			Containers: make([]corev1.Container, 0),
			Volumes: []corev1.Volume{
				{
					Name: "test",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "test",
						},
					},
				},
				{
					Name: "test2",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
	_, err = kubeCl.CoreV1().Pods(podWithVolumes.GetNamespace()).Create(ctx, &podWithVolumes, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: podWithVolumes.GetName(),
		ns:   podWithVolumes.GetNamespace(),
		kind: "Pod",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.CoreV1().Pods(podWithVolumes.GetNamespace()).Get(ctx, podWithVolumes.GetName(), metav1.GetOptions{})
			return err
		},
	})

	return createdResources
}

func testCreateCAPIResources(t *testing.T, kubeCl *client.KubernetesClient) []testCreatedResource {
	createdResources := make([]testCreatedResource, 0)

	md := testYAMLToUnstructured(t, `
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  annotations:
    machinedeployment.clusters.x-k8s.io/revision: "1"
  labels:
    node-group: worker
  name: test-worker-9bfeb8f2
  namespace: d8-cloud-instance-manager
  ownerReferences:
  - apiVersion: cluster.x-k8s.io/v1beta1
    kind: Cluster
    name: test
    uid: 1f63df99-2a20-4460-877e-d8bc69001052
spec:
  clusterName: test
  minReadySeconds: 0
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: test
      cluster.x-k8s.io/deployment-name: test-worker-9bfeb8f2
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  template:
    metadata:
      labels:
        cluster.x-k8s.io/cluster-name: test
        cluster.x-k8s.io/deployment-name: test-worker-9bfeb8f2
        node-group: worker
    spec:
      bootstrap:
        dataSecretName: worker-9e1e0bbc
      clusterName: test
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: MachineTemplate
        name: worker-9e1e0bbc
        namespace: d8-cloud-instance-manager
      nodeDeletionTimeout: 10m0s
      nodeDrainTimeout: 10m0s
      nodeVolumeDetachTimeout: 10m0s
`)
	_, err := kubeCl.Dynamic().Resource(capi.MachineDeploymentGVR).Namespace(md.GetNamespace()).Create(context.TODO(), md, metav1.CreateOptions{})
	require.NoError(t, err)
	createdResources = append(createdResources, testCreatedResource{
		name: md.GetName(),
		ns:   md.GetNamespace(),
		kind: "CAPIMachineDeployment",
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.Dynamic().Resource(capi.MachineDeploymentGVR).Namespace(md.GetNamespace()).Get(ctx, md.GetName(), metav1.GetOptions{})
			return err
		},
	})

	return createdResources
}

func testYAMLToUnstructured(t *testing.T, r string) *unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(r), &obj)
	require.NoError(t, err)
	return &obj
}

func testCreateFakeKubeClient() *client.KubernetesClient {
	kinds := map[schema.GroupVersionResource]string{
		v1.NodeUserGVR: v1.NodeUserList,
	}

	apisToAdd := []apis.ListKindToGVR{
		v1alpha1.D8StoragesListsGVRs(),
		capi.ListsGVRs(),
		sapcloud.ListsGVRs(),
	}

	for _, apiGVRs := range apisToAdd {
		for listKind, gvr := range apiGVRs {
			kinds[gvr] = listKind
		}
	}

	kubeCl := client.NewFakeKubernetesClientWithListGVR(kinds)

	discovery := kubeCl.Discovery().(*fakediscovery.FakeDiscovery)
	discovery.Resources = append(discovery.Resources, sapcloud.APIResourcesList(), capi.APIResourcesList())

	return kubeCl
}

type fakeKubeClientProvider struct {
	kubeCl *client.KubernetesClient

	cleaned bool
	stopSSH bool
}

func newFakeKubeClientProvider(kubeCl *client.KubernetesClient) *fakeKubeClientProvider {
	return &fakeKubeClientProvider{
		kubeCl: kubeCl,
	}
}
func (p *fakeKubeClientProvider) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	if p.cleaned {
		return nil, fmt.Errorf("already cleaned")
	}

	return p.kubeCl, nil
}
func (p *fakeKubeClientProvider) Cleanup(stopSSH bool) {
	p.cleaned = true
	p.stopSSH = stopSSH
}

func testCreateNodes(t *testing.T, kubeCl *client.KubernetesClient, hosts []session.Host) {
	names := make(map[string]struct{})
	ips := make(map[string]struct{})
	for _, host := range hosts {
		names[host.Name] = struct{}{}
		ips[host.Host] = struct{}{}
	}

	require.Len(t, names, len(hosts), hosts)
	require.Len(t, ips, len(hosts), hosts)

	for _, host := range hosts {
		obj := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: host.Name,
				Labels: map[string]string{
					"node.deckhouse.io/group": "master",
				},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Address: host.Host, Type: corev1.NodeInternalIP},
					{Address: host.Name, Type: corev1.NodeHostName},
				},
			},
		}

		_, err := kubeCl.CoreV1().Nodes().Create(context.TODO(), &obj, metav1.CreateOptions{})
		require.NoError(t, err, host.Name)
	}

	nodes, err := kubeCl.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, hosts)
	require.Len(t, nodes.Items, len(hosts))
	for _, node := range nodes.Items {
		require.Len(t, node.Status.Addresses, 2)
	}
}

func testAddCleanCommand(sshProvider *testssh.SSHProvider, forHost session.Host, out string, err error, logger log.Logger) {
	provider := sshProvider.CommandProvider()
	sshProvider.WithCommandProvider(func(host string, scriptPath string, args ...string) *testssh.Command {
		if !govalue.IsNil(provider) {
			cmd := provider(host, scriptPath, args...)
			if !govalue.IsNil(cmd) {
				return cmd
			}
		}
		if host != forHost.Host {
			return nil
		}

		if !strings.HasPrefix(scriptPath, "test -f /var/lib/bashible/cleanup_static_node.sh") {
			return nil
		}

		cmd := testssh.NewCommand([]byte(out))
		if err != nil {
			cmd.WithErr(err).WithRun(func() {
				logger.LogWarnLn("Clean command failed")
			})

			return cmd
		}

		return cmd.WithErr(nil).WithRun(func() {
			logger.LogInfoLn("Clean command success")
		})
	})
}

func testAddNodeUserCreated(ctx context.Context, kubeCl *client.KubernetesClient, hosts []session.Host, waiter *testWaiter) {
	waiter.wg.Add(1)

	go func() {
		defer waiter.wg.Done()
		err := retry.NewLoop("wait node user", 20, 500*time.Millisecond).RunContext(ctx, func() error {
			_, err := kubeCl.Dynamic().Resource(v1.NodeUserGVR).Get(ctx, global.ConvergeNodeUserName, metav1.GetOptions{})
			return err
		})
		if err != nil {
			waiter.setErr(err)
			return
		}

		for _, host := range hosts {
			node, err := kubeCl.CoreV1().Nodes().Get(ctx, host.Name, metav1.GetOptions{})
			if err != nil {
				waiter.setErr(err)
				return
			}

			if len(node.Annotations) == 0 {
				node.Annotations = make(map[string]string)
			}

			node.Annotations[global.ConvergerNodeUserAnnotation] = "true"
			_, err = kubeCl.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			if err != nil {
				waiter.setErr(err)
				return
			}
		}

		waiter.setErr(nil)
	}()
}

func testCreateKubeSystemSecret(t *testing.T, kubeCl *client.KubernetesClient, name string, data map[string][]byte) {
	t.Helper()

	secret := &corev1.Secret{
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

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: global.ConfigsNS,
		},
		Data: data,
	}

	_, err := kubeCl.CoreV1().ConfigMaps(global.ConfigsNS).Create(context.TODO(), cm, metav1.CreateOptions{})
	require.NoError(t, err)
}
