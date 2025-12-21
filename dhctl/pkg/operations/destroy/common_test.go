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
	"testing"

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
	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1alpha1"
	sapcloud "github.com/deckhouse/deckhouse/dhctl/pkg/apis/sapcloudio/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const (
	metaConfigKey = "cluster-config"

	cloudClusterGenericConfigYAML = `
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
	providerConfigYAML = `
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
	clusterStateKey = "cluster-state"
	nodesStateKey   = "nodes-state"
	uuidKey         = "uuid"
	baseInfraKey    = "base-infrastructure"
)

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

func assertResources(t *testing.T, kubeCl *client.KubernetesClient, resources []testCreatedResource, shouldDeleted bool) {
	if shouldDeleted {
		assertResourcesDeleted(t, kubeCl, resources)
		return
	}

	assertResourceExists(t, kubeCl, resources)
}

func assertResourcesDeleted(t *testing.T, kubeCl *client.KubernetesClient, resources []testCreatedResource) {
	ctx := context.TODO()
	for _, r := range resources {
		err := r.getFunc(t, ctx, kubeCl)
		if r.shouldExists {
			require.NoError(t, err, r.Name(), "resource should not delete", r.Name())
			continue
		}

		require.Error(t, err, r.Name(), "resource should delete", r.Name())
		require.True(t, k8errors.IsNotFound(err), "resource should not delete", r.Name(), err)
	}
}

func assertResourceExists(t *testing.T, kubeCl *client.KubernetesClient, resources []testCreatedResource) {
	ctx := context.TODO()
	for _, r := range resources {
		err := r.getFunc(t, ctx, kubeCl)
		require.NoError(t, err, r.Name(), "resource should not delete", r.Name())
	}
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

func testCreateSystemSecret(t *testing.T, kubeCl *client.KubernetesClient, name string, data map[string][]byte) {
	t.Helper()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: global.D8SystemNamespace,
		},
		Data: data,
	}

	_, err := kubeCl.CoreV1().Secrets(global.D8SystemNamespace).Create(context.TODO(), secret, metav1.CreateOptions{})
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

func createAssertError(shouldReturnError bool, noErrorMSg, errorMsg string) func(t *testing.T, err error) {
	errorAssert := require.NoError
	errorAssertMsg := noErrorMSg
	if shouldReturnError {
		errorAssert = require.Error
		errorAssertMsg = errorMsg
	}

	return func(t *testing.T, err error) {
		errorAssert(t, err, errorAssertMsg)
	}
}

func assertClusterDestroyError(t *testing.T, shouldReturnError bool, err error) {
	errorAssert := createAssertError(shouldReturnError, "should not destroyed", "should destroyed")
	errorAssert(t, err)
}

type childTest interface {
	getStateCache() dhctlstate.Cache
}

type baseTest struct {
	childTest childTest
}

func (ts *baseTest) stateCacheKeys(t *testing.T) []string {
	stateCache := ts.childTest.getStateCache()
	require.False(t, govalue.IsNil(stateCache))

	keys := make([]string, 0)

	err := stateCache.Iterate(func(k string, _ []byte) error {
		keys = append(keys, k)
		return nil
	})
	require.NoError(t, err, "state cache keys getting")

	return keys
}

func (ts *baseTest) assertStateCacheIsEmpty(t *testing.T) {
	keys := ts.stateCacheKeys(t)
	require.Empty(t, keys, fmt.Sprintf("has keys %v", keys))
}

func (ts *baseTest) assertStateCacheNotEmpty(t *testing.T) {
	keys := ts.stateCacheKeys(t)
	require.NotEmpty(t, keys, "has not keys")
}

func (ts *baseTest) assertStateCache(t *testing.T, empty bool) {
	if empty {
		ts.assertStateCacheIsEmpty(t)
		return
	}

	ts.assertStateCacheNotEmpty(t)
}

const (
	nodeStateKey       = "test-master-0.tfstate"
	nodeBackupStateKey = "test-master-0.tfstate.backup"
)

func testAddCloudStatesToCache(t *testing.T, stateCache dhctlstate.Cache, uuid string) {
	require.False(t, govalue.IsNil(stateCache))

	err := stateCache.Save(uuidKey, []byte(uuid))
	require.NoError(t, err, "uuid should save")

	err = stateCache.Save(baseInfraKey, []byte(`{}`))
	require.NoError(t, err, "base infra should save")

	err = stateCache.Save(nodeBackupStateKey, []byte(`{"a": "b"}`))
	require.NoError(t, err, "backup should save")

	err = stateCache.Save(nodeStateKey, []byte(`{}`))
	require.NoError(t, err, "master state should save")
}
