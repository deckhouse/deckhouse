/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodebootstrap

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	templatesv1alpha1 "github.com/deckhouse/node-controller/api/templates.internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const testTemplateNodeGroup = "immutable-static"

// A hand-installed machine is the one thing the cluster cannot describe: only
// the operator standing in front of it knows its interfaces and its disks. Both
// used to be rendered as a guess (eth0/DHCP, the first disk over 2Gi), and a
// guess handed over as a template is one nobody notices is wrong.
func TestNodeConfigTemplateLeavesTheMachineFieldsToTheOperator(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	))

	object, err := storage.Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.NoError(t, err)

	template, ok := object.(*templatesv1alpha1.NodeConfigTemplate)
	require.True(t, ok, "storage returned %T", object)

	require.NotEmpty(t, template.Spec.APIServerEndpoints, "the cluster half must be rendered")
	require.Empty(t, template.Spec.Network.Interfaces)
	require.Empty(t, template.Spec.Storage.Device)
	require.Nil(t, template.Spec.Storage.DiskSelector)
}

// Without the token kubelet has nothing to present on first contact and the
// machine never joins; the operator has nowhere else to take one from, and a
// stored template would carry an expired one.
func TestNodeConfigTemplateCarriesABootstrapToken(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	))

	object, err := storage.Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.NoError(t, err)

	template, ok := object.(*templatesv1alpha1.NodeConfigTemplate)
	require.True(t, ok, "storage returned %T", object)
	require.NotEmpty(t, template.Spec.Kubelet.BootstrapToken)
}

// Nobody installs a cloud node by hand: its config is rendered onto the machine
// CAPI creates. Answering for such a group would put a template in an
// operator's hands that no machine of that group will ever read.
func TestNodeConfigTemplateIsNotServedForACloudGroup(t *testing.T) {
	cloud := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: testTemplateNodeGroup},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:   deckhousev1.NodeTypeCloudPermanent,
			SystemType: deckhousev1.SystemTypeImmutable,
		},
	}
	storage := NewTemplateStorage(templateCluster(t, cloud, bootstrapTokenSecret(testTemplateNodeGroup)))

	_, err := storage.Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err), "expected NotFound, got %v", err)
}

func immutableStaticNodeGroup(name string) client.Object {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:   deckhousev1.NodeTypeStatic,
			SystemType: deckhousev1.SystemTypeImmutable,
		},
	}
}

func bootstrapTokenSecret(ngName string) client.Object {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nodecommon.KubeSystemNamespace,
			Name:      "bootstrap-token-abcdef",
			Labels:    map[string]string{nodecommon.BootstrapTokenNodeGroupLabel: ngName},
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			"token-id":     []byte("abcdef"),
			"token-secret": []byte("0123456789abcdef"),
			"expiration":   []byte(time.Now().Add(24 * time.Hour).Format(time.RFC3339)),
		},
	}
}

// templateCluster builds a cluster a NodeConfig can be rendered from: the same
// inputs testenv.EnsureClusterInputs creates for the envtest suite, as plain
// objects.
func templateCluster(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := k8sruntime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(append(clusterInputs(), objects...)...).
		Build()
}

func clusterInputs() []client.Object {
	digests := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q,"nodeletSysext":%q},"nodeManager":{"olcedar":%q},"common":{"pause":%q}}`,
		testenv.TestContainerdDigest, testenv.TestCNIDigest, testenv.TestKubeletDigest, testenv.TestNodeletDigest, testenv.TestOSImageDigest, testenv.TestPauseDigest)

	return []client.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: "bashible-apiserver-files"},
			Data:       map[string]string{"images_digests.json": digests},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: "registry-packages-proxy-token"},
			Data:       map[string][]byte{"token": []byte("proxy-token")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "d8-system", Name: "deckhouse-registry"},
			Data: map[string][]byte{
				"address":           []byte(testenv.TestRegistryAddress),
				"path":              []byte(testenv.TestRegistryPath),
				"scheme":            []byte("https"),
				"imagesRegistry":    []byte(testenv.TestRegistryAddress + testenv.TestRegistryPath),
				".dockerconfigjson": []byte(fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, testenv.TestRegistryAddress, testenv.TestRegistryAuth)),
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: "kube-root-ca.crt"},
			Data:       map[string]string{"ca.crt": testenv.TestClusterCA},
		},
		// derived_status reads the cluster's Kubernetes version from this
		// ConfigMap, not from ClusterConfiguration (deckhouse#21828).
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: "d8-cluster-kubernetes"},
			Data:       map[string]string{"spec": "desiredVersion: \"" + testenv.TestKubernetesVersion + ".6\"\n"},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: "d8-cluster-configuration"},
			Data: map[string][]byte{
				"cluster-configuration.yaml": []byte("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterDomain: cluster.local\n" +
					"podSubnetNodeCIDRPrefix: \"23\"\nkubernetesVersion: \"" + testenv.TestKubernetesVersion + ".6\"\n"),
			},
		},
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"},
			Ports: []discoveryv1.EndpointPort{{
				Name: ptr.To("https"),
				Port: ptr.To(int32(6443)),
			}},
			Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"192.168.199.10"}}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nodecommon.KubeSystemNamespace,
				Name:      "kube-dns",
				Labels:    map[string]string{"k8s-app": "kube-dns"},
			},
			Spec: corev1.ServiceSpec{ClusterIP: "10.222.0.10"},
		},
	}
}
