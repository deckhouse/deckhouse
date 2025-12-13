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
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis"
	capi "github.com/deckhouse/deckhouse/dhctl/pkg/apis/capi/v1beta1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	sapcloud "github.com/deckhouse/deckhouse/dhctl/pkg/apis/sapcloudio/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/testssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func TestStaticDestroy(t *testing.T) {
	app.IsDebug = true

	t.Run("skip resources returns errors because metaconfig not in cache", func(t *testing.T) {
		hosts := []session.Host{
			{Host: "127.0.0.2", Name: "master-1"},
		}
		params := testStaticDestroyTestParams{
			skipResources:   true,
			commanderMode:   false,
			commanderParams: nil,
			destroyOverHost: hosts[0],
		}

		tst := createTestStaticDestroyTest(t, params)
		defer tst.clean(t)

		testCreateNodes(t, tst.kubeCl, hosts)

		err := tst.destroyer.DestroyCluster(context.TODO(), true)
		require.Error(t, err)
	})

	t.Run("one master host", func(t *testing.T) {
		hosts := []session.Host{
			{Host: "127.0.0.2", Name: "master-1"},
		}
		params := testStaticDestroyTestParams{
			skipResources:   false,
			commanderMode:   false,
			commanderParams: nil,
			destroyOverHost: hosts[0],
		}

		tst := createTestStaticDestroyTest(t, params)
		defer tst.clean(t)

		testCreateNodes(t, tst.kubeCl, hosts)

		ctx := context.TODO()

		waiter := newTestWaiter()
		go testAddNodeUserCreated(ctx, tst.kubeCl, hosts, waiter)

		err := tst.destroyer.DestroyCluster(ctx, true)
		require.Error(t, err)

		waiter.wg.Wait()

		require.NoError(t, waiter.getErr())
	})
}

type testStaticDestroyTestParams struct {
	skipResources bool

	commanderMode   bool
	commanderParams *commander.CommanderModeParams

	destroyOverHost session.Host
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

	kubeCl      *client.KubernetesClient
	sshProvider *testssh.SSHProvider

	stateCache dhctlstate.Cache
	d8State    *deckhouse.State

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

func createTestStaticDestroyTest(t *testing.T, params testStaticDestroyTestParams) testStaticDestroyTest {
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

	commanderUUID := uuid.Must(uuid.NewRandom())

	loaderParams := &stateLoaderParams{
		commanderMode:   params.commanderMode,
		commanderParams: params.commanderParams,
		stateCache:      stateCache,
		logger:          logger,
		skipResources:   params.skipResources,
		forceFromCache:  true,
	}

	loader, err := initStateLoader(ctx, loaderParams, kubeClProvider)
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

	tmpDir, err := fs.RandomTmpDirWith10Runes(os.TempDir(), fmt.Sprintf("%d", i), 15)
	require.NoError(t, err)

	logger.LogInfoF("Tmp dir: '%s'\n", tmpDir)

	infraProvider := &infraDestroyerProvider{
		stateCache:           stateCache,
		loggerProvider:       loggerProvider,
		kubeProvider:         kubeClProvider,
		phasesActionProvider: phaseActionProvider,
		commanderMode:        params.commanderMode,
		skipResources:        params.skipResources,
		cloudStateProvider:   nil,
		sshClientProvider:    sshProvider,
		tmpDir:               tmpDir,
		nodeUserWaitParams: retry.NewEmptyParams(
			retry.WithWait(2*time.Second),
			retry.WithAttempts(5),
		),
	}

	destroyer := &ClusterDestroyer{
		stateCache:       stateCache,
		configPreparator: loader,
		loggerProvider:   loggerProvider,

		pipeline: pipeline,

		d8Destroyer:   d8Destroyer,
		infraProvider: infraProvider,
	}

	return testStaticDestroyTest{
		params: params,

		destroyer: destroyer,
		logger:    logger,

		stateCache: stateCache,
		d8State:    d8State,

		kubeCl:      kubeCl,
		sshProvider: sshProvider,

		tmpDir: tmpDir,
	}
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

type fakeKubeClient struct {
	kubeCl *client.KubernetesClient

	cleaned bool
	stopSSH bool
}

func newFakeKubeClientProvider(kubeCl *client.KubernetesClient) *fakeKubeClient {
	return &fakeKubeClient{
		kubeCl: kubeCl,
	}
}
func (p *fakeKubeClient) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	if p.cleaned {
		return nil, fmt.Errorf("already cleaned")
	}

	return p.kubeCl, nil
}
func (p *fakeKubeClient) Cleanup(stopSSH bool) {
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
					"node.deckhouse.io/group":               "master",
					"node-role.kubernetes.io/control-plane": "",
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
