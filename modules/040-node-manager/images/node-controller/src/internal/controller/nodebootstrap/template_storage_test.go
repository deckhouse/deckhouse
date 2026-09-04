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
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
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

// The node name is the operator's to pick, and a NodeConfig carries it twice:
// crds/nodeconfig.yaml requires spec.nodeName, and the own-node-only policy
// refuses a write whose metadata.name differs from it. Neither may be guessed.
func TestNodeConfigTemplateNamesNoNode(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	))

	object, err := storage.Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.NoError(t, err)

	template, ok := object.(*templatesv1alpha1.NodeConfigTemplate)
	require.True(t, ok, "storage returned %T", object)

	require.Empty(t, template.Spec.NodeName, "one template serves every machine of the group")
	require.Equal(t, testTemplateNodeGroup, template.Name,
		"a served object carries the name it was asked for, or no client can key it")
}

// A list of nameless items is one item as far as any client keyed by name is
// concerned, and metadata.name is a field this storage already filters on.
func TestNodeConfigTemplateListNamesEveryTemplate(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup("aaa-wanted"),
		immutableStaticNodeGroup("zzz-other"),
		bootstrapTokenSecret("aaa-wanted"),
		bootstrapTokenSecret("zzz-other"),
	))

	object, err := storage.List(t.Context(), &metainternalversion.ListOptions{})
	require.NoError(t, err)

	list, ok := object.(*templatesv1alpha1.NodeConfigTemplateList)
	require.True(t, ok, "storage returned %T", object)

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}
	require.ElementsMatch(t, []string{"aaa-wanted", "zzz-other"}, names)
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

// Every template carries a live bootstrap token and the registry credential of
// its group, and each one costs a full render. A client that asked for one group
// must be given that group and nothing else.
func TestNodeConfigTemplateListHonoursTheRequest(t *testing.T) {
	// Only the first group has a token, so rendering the second one fails: an
	// answer without an error is proof the second was never rendered — a
	// template names no group of its own.
	cluster := templateCluster(t,
		immutableStaticNodeGroup("aaa-wanted"),
		immutableStaticNodeGroup("zzz-other"),
		bootstrapTokenSecret("aaa-wanted"),
	)

	tests := []struct {
		name    string
		options metainternalversion.ListOptions
		exp     int
	}{
		{
			name:    "a name selector answers for that group alone",
			options: metainternalversion.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", "aaa-wanted")},
			exp:     1,
		},
		{
			name:    "a label selector matches nothing: a template carries no labels",
			options: metainternalversion.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"x": "y"})},
			exp:     0,
		},
		{
			name:    "a limit is not exceeded",
			options: metainternalversion.ListOptions{Limit: 1},
			exp:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			object, err := NewTemplateStorage(cluster).List(t.Context(), &tt.options)
			require.NoError(t, err)

			list, ok := object.(*templatesv1alpha1.NodeConfigTemplateList)
			require.True(t, ok, "storage returned %T", object)
			require.Len(t, list.Items, tt.exp, "the answer must hold exactly the templates that were asked for")
		})
	}
}

// A field this storage cannot filter on must be refused, not silently ignored:
// ignoring it answers with every group's joinable credentials.
func TestNodeConfigTemplateListRefusesAFieldItCannotFilterOn(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	))

	_, err := storage.List(t.Context(), &metainternalversion.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", "worker-0"),
	})
	require.True(t, apierrors.IsBadRequest(err), "expected BadRequest, got %v", err)
}

// A group whose token has not been issued yet is a state the cluster leaves on
// its own. Answering 500 tells the operator their request was broken; a client
// can act on "not yet".
func TestNodeConfigTemplateIsUnavailableWithoutAToken(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t, immutableStaticNodeGroup(testTemplateNodeGroup)))

	_, err := storage.Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.True(t, apierrors.IsServiceUnavailable(err), "expected ServiceUnavailable, got %v", err)

	_, err = storage.List(t.Context(), &metainternalversion.ListOptions{})
	require.True(t, apierrors.IsServiceUnavailable(err), "expected ServiceUnavailable, got %v", err)
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
			Name:      "bootstrap-token-" + ngName,
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

// A machine installed by hand has no config to keep a status token from, so the
// template mints one per read; and it has to be told which networks the cluster
// runs on, or a multi-homed machine registers on whichever NIC it likes.
func TestNodeConfigTemplateCarriesAStatusTokenAndTheClusterNetworks(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	))

	first := templateOf(t, storage)
	require.Equal(t, []string{"192.168.199.0/24"}, first.Spec.InternalNetworkCIDRs)
	require.Len(t, first.Spec.StatusToken, 2*statusTokenBytes, "32 random bytes, hex-encoded")

	// Two machines handed the same token could each read the other's status.
	second := templateOf(t, storage)
	require.NotEqual(t, first.Spec.StatusToken, second.Spec.StatusToken)
}

func templateOf(t *testing.T, storage *TemplateStorage) *templatesv1alpha1.NodeConfigTemplate {
	t.Helper()

	object, err := storage.Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.NoError(t, err)
	template, ok := object.(*templatesv1alpha1.NodeConfigTemplate)
	require.True(t, ok, "storage returned %T", object)
	return template
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
	digests := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q,"nodeletSysext":%q},"nodeManager":{"engine":%q},"common":{"pause":%q}}`,
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
		// The networks a static cluster addresses its nodes in; the template
		// carries them like every other cluster-decided field.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.KubeSystemNamespace, Name: "d8-static-cluster-configuration"},
			Data: map[string][]byte{
				"static-cluster-configuration.yaml": []byte("apiVersion: deckhouse.io/v1\nkind: StaticClusterConfiguration\n" +
					"internalNetworkCIDRs:\n- 192.168.199.0/24\n"),
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
