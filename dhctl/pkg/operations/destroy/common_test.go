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
	"os"
	"strings"
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/testssh"
)

const (
	staticClusterGeneralConfigYAML = `
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
sshPublicKey: ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCsCkOVy6z7SPO+NYZyz15XTFSRGYqhaw3QVAoRuUkG6J1xCK7yCXtZoDYJM5uSdk58cQhd3/+Dto7saNa3NNEm+WW3vnZ6ArLl4U/YHmpHu0pUgDaoQsaRvNHW5jG/YsBter0G88ZqChRP4adhaMHK4x7JM+Yml+dTEecAROzcl9cIjMTPjUK/3ZJdbckpTQXiqX7re+Mzer2wdAT0YtwX2Ai++nrP/GIFzO+HMTd6lLdtP+uGWL+zNnHq2KTbP1v9BumZQXJNGLVXrI8V63TW7cKICr+8ASdF+hw9DDqyIJBeRE/LNm1tj2VIfnwPaGs9G5gdP0k5FUsvq8qwS6GDd6Ro/iGfhMhOhnLBSzlobGPO0I+kb7r250eyhwpJEGPvTR3koA/5KyFKtYctgbYkaBEJzCMhtgU9CzbFHimS7Y2/XIPLcLbuWYaknCqnny++kmvxzc4G7Qj6mf8gsr1NH273Qf/dlkkwPhGxIA+OJDK9OOjEu2ZjZyM+lJOgJQ0= root@11605a4d8b81
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: cloudId
  folderID: folderId
  serviceAccountJSON: "{}"
`
	metaConfigKey   = "cluster-config"
	clusterStateKey = "cluster-state"
	nodesStateKey   = "nodes-state"
	uuidKey         = "uuid"
	baseInfraKey    = "base-infrastructure"

	bastionHost = "127.0.0.1"
	bastionUser = "notexistsb"
	bastionPort = "23"
	inputPort   = "22"
	inputUser   = "notexists"
)

var (
	inputPrivateKeys = []string{"/tmp/fake_ssh/input_private_key_1", "/tmp/fake_ssh/input_private_key_2"}
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

func testAddDeckhouseStorageClass(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient, gvr schema.GroupVersionResource, resourceYAML string) testCreatedResource {
	sc := testYAMLToUnstructured(t, resourceYAML)
	name := sc.GetName()

	_, err := kubeCl.Dynamic().Resource(gvr).Create(ctx, sc, metav1.CreateOptions{})
	require.NoError(t, err)
	return testCreatedResource{
		name: name,
		ns:   sc.GetNamespace(),
		kind: sc.GetKind(),
		getFunc: func(t *testing.T, ctx context.Context, kubeCl *client.KubernetesClient) error {
			_, err := kubeCl.Dynamic().Resource(gvr).Get(ctx, name, metav1.GetOptions{})
			return err
		},
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
		if err != nil {
			require.True(t, k8errors.IsAlreadyExists(err), "failed to create namespace")
		}
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

	deckhouseSC := []struct {
		gvr          schema.GroupVersionResource
		resourceYAML string
	}{
		{
			gvr: v1alpha1.LocalStorageClassGVR,
			resourceYAML: `
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
`,
		},
		{
			gvr: v1alpha1.ReplicatedStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStorageClass
metadata:
  name: replicated-storage-class
spec:
  storagePool: thick-pool
  reclaimPolicy: Delete
  topology: Ignored
  replication: ConsistencyAndAvailability
`,
		},
		{
			gvr: v1alpha1.NFSStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    host: 10.223.187.3
    share: /
    nfsVersion: "4.1"
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
  workloadNodes:
    nodeSelector:
      matchLabels:
        storage: "true"
`,
		},
		{
			gvr: v1alpha1.CephStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-fs-sc
spec:
  clusterConnectionName: ceph-cluster-1
  reclaimPolicy: Delete
  type: CephFS
  cephFS:
    fsName: cephfs
`,
		},
		{
			gvr: v1alpha1.SCSIStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSIStorageClass
metadata:
  name: scsi-storage-class
spec:
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
`,
		},
		{
			gvr: v1alpha1.S3StorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: S3StorageClass
metadata:
  name: s3-storage-class
spec:
  reclaimPolicy: Delete
  volumeBindingMode: Immediate
`,
		},
		{
			gvr: v1alpha1.YadroTatlinUnifiedStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageClass
metadata:
  name: yadro-tatlin-unified-storage-class
spec:
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
`,
		},
		{
			gvr: v1alpha1.NetappStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: NetappStorageClass
metadata:
  name: netapp-storage-class
spec:
  reclaimPolicy: Delete
  volumeBindingMode: Immediate
`,
		},
		{
			gvr: v1alpha1.HuaweiStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: HuaweiStorageClass
metadata:
  name: huawei-storage-class
spec:
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
`,
		},
		{
			gvr: v1alpha1.HPEStorageClassGVR,
			resourceYAML: `
apiVersion: storage.deckhouse.io/v1alpha1
kind: HPEStorageClass
metadata:
  name: hpe-storage-class
spec:
  reclaimPolicy: Delete
  volumeBindingMode: Immediate
`,
		},
	}

	for _, sc := range deckhouseSC {
		createdResources = append(createdResources, testAddDeckhouseStorageClass(t, ctx, kubeCl, sc.gvr, sc.resourceYAML))
	}

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

func testCreateProviderClusterConfigSecret(t *testing.T, kubeCl *client.KubernetesClient, configYAML string) {
	testCreateKubeSystemSecret(t, kubeCl, "d8-provider-cluster-configuration", map[string][]byte{
		"cloud-provider-cluster-configuration.yaml": []byte(configYAML),
		"cloud-provider-discovery-data.json":        []byte(`{"a": "b"}`),
	})
}

func testCreateClusterConfigSecret(t *testing.T, kubeCl *client.KubernetesClient, configYAML string) {
	testCreateKubeSystemSecret(t, kubeCl, "d8-cluster-configuration", map[string][]byte{
		"cluster-configuration.yaml": []byte(configYAML),
	})
}

func testCreateClusterUUIDCM(t *testing.T, kubeCl *client.KubernetesClient, clusterUUID string) {
	testCreateKubeSystemCM(t, kubeCl, "d8-cluster-uuid", map[string]string{
		"cluster-uuid": clusterUUID,
	})
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

type baseTest struct {
	stateCache   dhctlstate.Cache
	tmpDir       string
	logger       *log.InMemoryLogger
	kubeProvider kube.ClientProviderWithCleanup
	metaConfig   *config.MetaConfig
}

func (ts *baseTest) stateCacheKeys(t *testing.T) []string {
	require.False(t, govalue.IsNil(ts.stateCache))

	keys := make([]string, 0)

	err := ts.stateCache.Iterate(func(k string, _ []byte) error {
		keys = append(keys, k)
		return nil
	})
	require.NoError(t, err, "state cache keys getting")

	return keys
}

func (ts *baseTest) clean(t *testing.T) {
	tmpDir := ts.tmpDir
	logger := ts.logger

	require.NotEmpty(t, tmpDir)
	require.False(t, govalue.IsNil(logger))

	err := os.RemoveAll(tmpDir)
	if err != nil {
		logger.LogErrorF("Couldn't remove tmp dir '%s': %v\n", tmpDir, err)
		return
	}

	logger.LogInfoF("tmp dir '%s' removed\n", tmpDir)
}

func (ts *baseTest) setResourcesDestroyed(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))

	err := deckhouse.NewState(ts.stateCache).SetResourcesDestroyed()
	require.NoError(t, err, "resources destroyed should save in cache")
}

func (ts *baseTest) saveMetaConfigToCache(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))
	require.False(t, govalue.IsNil(ts.metaConfig))

	err := ts.stateCache.SaveStruct(metaConfigKey, ts.metaConfig)
	require.NoError(t, err, "metaconfig should be saved in cache")
}

func (ts *baseTest) assertHasMetaConfigInCache(t *testing.T, saved bool) {
	require.False(t, govalue.IsNil(ts.stateCache))

	inCache, err := ts.stateCache.InCache(metaConfigKey)

	if !saved {
		require.NoError(t, err, "metaconfig should not save in cache")
		return
	}

	require.NoError(t, err, "metaconfig should in cache")
	require.True(t, inCache, "metaconfig should in cache")
}

func (ts *baseTest) assertResourcesSetDestroyedInCache(t *testing.T, destroyed bool) {
	require.False(t, govalue.IsNil(ts.stateCache))

	destroyedInCache, err := deckhouse.NewState(ts.stateCache).IsResourcesDestroyed()
	require.NoError(t, err, "resources destroyed flag should be set")
	require.Equal(t, destroyed, destroyedInCache, "resources destroyed should be set correct flag")
}

func (ts *baseTest) assertKubeProviderCleaned(t *testing.T, cleaned bool, shouldStop bool) {
	require.False(t, govalue.IsNil(ts.kubeProvider))

	kubeProvider, ok := ts.kubeProvider.(*fakeKubeClientProvider)
	if !cleaned && !ok {
		return
	}
	require.True(t, ok, "correct kube provider")
	require.Equal(t, cleaned, kubeProvider.cleaned, "kube provider should cleaned")
	require.Equal(t, shouldStop, kubeProvider.stopSSH, "kube provider ssh should or not stop")
}

func (ts *baseTest) assertKubeProviderIsErrorProvider(t *testing.T) {
	require.False(t, govalue.IsNil(ts.kubeProvider))
	require.IsType(t, &kubeClientErrorProvider{}, ts.kubeProvider, "kube provider should be error provider")
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

func (ts *baseTest) assertFileKeysInCacheAfterLoad(t *testing.T) {
	stateKeys := ts.stateCacheKeys(t)
	expectedKeys := []string{
		uuidKey,
		baseInfraKey,
		nodeStateKey,
		nodeBackupStateKey,
	}

	require.Len(t, stateKeys, len(expectedKeys), "state cache should contain keys")

	for _, key := range expectedKeys {
		require.Contains(t, stateKeys, key, "state cache should contain key", key)
	}
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

func testCreateDefaultTestSSHProvider(destroyOverHost session.Host, overBastion bool) *testssh.SSHProvider {
	initKeys := make([]session.AgentPrivateKey, 0, len(inputPrivateKeys))
	for _, key := range inputPrivateKeys {
		initKeys = append(initKeys, session.AgentPrivateKey{
			Key: key,
		})
	}

	input := session.Input{
		User:       inputUser,
		Port:       inputPort,
		BecomePass: "",
		AvailableHosts: []session.Host{
			destroyOverHost,
		},
	}

	if overBastion {
		input.BastionHost = bastionHost
		input.BastionUser = bastionUser
		input.BastionPort = bastionPort
	}

	return testssh.NewSSHProvider(session.NewSession(input), true).WithInitPrivateKeys(initKeys)
}

func assertOverDefaultBastion(t *testing.T, overBastion bool, bastion testssh.Bastion, tp string) {
	require.False(t, bastion.NoSession, "bastion should have session")

	assert := func(t *testing.T, expected, actual string) {
		require.Empty(t, actual, fmt.Sprintf("call '%s' should not over bastion", tp))
	}

	if overBastion {
		assert = func(t *testing.T, expected, actual string) {
			require.Equal(t, expected, actual, fmt.Sprintf("call '%s' should over bastion", tp))
		}
	}

	assert(t, bastionHost, bastion.Host)
	assert(t, bastionPort, bastion.Port)
	assert(t, bastionUser, bastion.User)
}

func testIsCleanCommand(scriptPath string) bool {
	return strings.HasPrefix(scriptPath, "test -f /var/lib/bashible/cleanup_static_node.sh")
}

func assertStringSliceContainsUniqVals(t *testing.T, list []string, msg string) {
	uniq := make(map[string]struct{})
	for _, v := range list {
		uniq[v] = struct{}{}
	}
	require.Len(t, uniq, len(list), msg)
}

type cloudInfraDestroyer struct {
	err error
}

func newCloudInfraDestroyer(err error) *cloudInfraDestroyer {
	return &cloudInfraDestroyer{
		err: err,
	}
}

func (d *cloudInfraDestroyer) DestroyCluster(context.Context, bool) error {
	return d.err
}
