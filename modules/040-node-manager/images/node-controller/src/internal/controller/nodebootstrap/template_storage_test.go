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
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
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

// Intended, not incidental: the template leaves the machine's network and storage
// out whole rather than as empty objects. The operator writes them in, and a node
// reads an absent section and an empty one alike.
func TestNodeConfigTemplateLeavesTheMachineSectionsOut(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	))

	object, err := storage.Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.NoError(t, err)
	raw, err := json.Marshal(object)
	require.NoError(t, err)

	var template struct {
		Spec map[string]json.RawMessage `json:"spec"`
	}
	require.NoError(t, json.Unmarshal(raw, &template))
	require.NotContains(t, template.Spec, "network", "the operator writes the network in")
	require.NotContains(t, template.Spec, "storage", "the operator writes the disk in")
	require.Contains(t, template.Spec, "kubelet", "the cluster's half stays")
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

// The render reads only what the cluster publishes itself, so an input that is not
// there yet is "not yet" for the client to retry, with the cause, rather than a 500.
func TestNodeConfigTemplateIsUnavailableUntilTheClusterInputsAreThere(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	cluster := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	).Build()

	_, err := NewTemplateStorage(cluster).Get(t.Context(), testTemplateNodeGroup, &metav1.GetOptions{})
	require.True(t, apierrors.IsServiceUnavailable(err), "expected ServiceUnavailable, got %v", err)
	require.ErrorContains(t, err, testTemplateNodeGroup)
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
	// The render lists NodeExtensionRequests, and the fake client answers a kind
	// its scheme does not know with an error rather than an empty list.
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(append(clusterInputs(), objects...)...).
		Build()
}

func clusterInputs() []client.Object {
	return []client.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: "bashible-apiserver-files"},
			Data:       map[string]string{"images_digests.json": testenv.ImagesDigestsJSON(testenv.TestContainerdDigest)},
		},
		testenv.PlatformTemplatesConfigMap(),
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

// A node bashible manages fetches its document here before bashible runs: its render
// without a node name or the tokens a hand-installed Engine machine needs. A group naming
// no type is bashible's too, and List keeps serving hand-installed Engine groups only.
func TestNodeConfigTemplateServesTheDocumentOfABashibleGroup(t *testing.T) {
	tests := []struct {
		name       string
		systemType deckhousev1.SystemType
	}{
		{name: "a Mutable group", systemType: deckhousev1.SystemTypeMutable},
		{name: "a group that names no type", systemType: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewTemplateStorage(templateCluster(t, bashibleNodeGroup("worker", tt.systemType)))

			object, err := storage.Get(callerIn(t, "system:nodes"), "worker", &metav1.GetOptions{})
			require.NoError(t, err)
			template, ok := object.(*templatesv1alpha1.NodeConfigTemplate)
			require.True(t, ok, "storage returned %T", object)

			require.Equal(t, "worker", template.Name)
			require.Equal(t, internalv1alpha1.SystemTypeMutable, template.Spec.SystemType)
			require.Empty(t, template.Spec.NodeName)
			require.Empty(t, template.Spec.Kubelet.BootstrapToken)
			require.Empty(t, template.Spec.StatusToken)
			require.Equal(t, []string{"https://192.168.199.10:6443"}, template.Spec.APIServerEndpoints)
			require.Equal(t, []byte("proxy-token"), template.Spec.RegistryPackagesProxy.AccessToken)
			require.Equal(t, map[string]bool{"static-pods": true, "extensions": true}, template.Spec.FeatureGates)
			names := make([]string, 0, len(template.Spec.Extensions))
			for _, ext := range template.Spec.Extensions {
				names = append(names, ext.Name)
				require.Nil(t, ext.RequestedBy, "a platform package names no requester: %s", ext.Name)
			}
			// The list itself is candi/nodelet/nodeconfig-mutable.yaml.tpl's: the agent first.
			require.Len(t, names, len(testenv.TestPlatformPackageDigests)+1)
			require.Equal(t, []string{"nodelet", "d8-ca-updater", "jq"}, names[:3])
			require.Equal(t, testenv.TestNodeletPackageDigest, template.Spec.Extensions[0].Digest)
			require.Equal(t, testenv.TestPlatformPackageDigests["jq"], template.Spec.Extensions[2].Digest)

			list, err := storage.List(t.Context(), &metainternalversion.ListOptions{})
			require.NoError(t, err)
			require.Empty(t, list.(*templatesv1alpha1.NodeConfigTemplateList).Items)
		})
	}
}

// A new node installs the registry packages meant for it before kubelet starts, so its
// group's template carries them: selected by the labels the node will register with, filed
// by the kind the requester declared.
func TestTheTemplateOfABashibleGroupCarriesItsPackageRequests(t *testing.T) {
	group := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:     deckhousev1.NodeTypeStatic,
			SystemType:   deckhousev1.SystemTypeMutable,
			NodeTemplate: &deckhousev1.NodeTemplate{Labels: map[string]string{"storage": "drbd"}},
		},
	}
	request := func(name, digest string, kind deckhousev1alpha1.ExtensionKind, labels map[string]string) client.Object {
		return &deckhousev1alpha1.NodeExtensionRequest{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
				Extension:    deckhousev1alpha1.NodeExtension{Name: name, Digest: digest, Kind: kind},
				NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: labels},
			},
		}
	}
	storage := NewTemplateStorage(templateCluster(t, group,
		request("sds-utils", "sha256:"+strings.Repeat("e", 64), deckhousev1alpha1.ExtensionKindPackage, map[string]string{"storage": "drbd"}),
		request("gpu-tools", "sha256:"+strings.Repeat("b", 64), deckhousev1alpha1.ExtensionKindPackage, map[string]string{"gpu": "true"}),
		request("drbd", "sha256:"+strings.Repeat("d", 64), deckhousev1alpha1.ExtensionKindSystemExtension, map[string]string{"storage": "drbd"}),
	))

	object, err := storage.Get(callerIn(t, "system:nodes"), "worker", &metav1.GetOptions{})
	require.NoError(t, err)
	template, ok := object.(*templatesv1alpha1.NodeConfigTemplate)
	require.True(t, ok, "storage returned %T", object)

	last := template.Spec.Extensions[len(template.Spec.Extensions)-1]
	require.Equal(t, "sds-utils", last.Name, "a request comes after the platform packages")
	for _, ext := range template.Spec.Extensions {
		require.NotEqual(t, "gpu-tools", ext.Name, "the group's nodes do not carry its label")
		require.NotEqual(t, "drbd", ext.Name, "a bashible node installs no system extension")
	}
}

// The role the agent reads templates with covers every group, and the template of
// a hand-installed Engine group carries a live bootstrap token: no node identity
// takes one home. Everyone else is answered as before.
func TestNodeConfigTemplateRefusesANodeTheTemplateOfAHandInstalledGroup(t *testing.T) {
	storage := NewTemplateStorage(templateCluster(t,
		immutableStaticNodeGroup(testTemplateNodeGroup),
		bootstrapTokenSecret(testTemplateNodeGroup),
	))

	for _, group := range []string{"system:nodes", "system:bootstrappers:d8-node-manager"} {
		t.Run(group, func(t *testing.T) {
			_, err := storage.Get(callerIn(t, group), testTemplateNodeGroup, &metav1.GetOptions{})
			require.True(t, apierrors.IsForbidden(err), "expected Forbidden, got %v", err)
		})
	}

	t.Run("an operator", func(t *testing.T) {
		_, err := storage.Get(callerIn(t, "system:masters"), testTemplateNodeGroup, &metav1.GetOptions{})
		require.NoError(t, err)
	})
}

// Every edition builds the agent, so a release without its package is broken: the
// template of a bashible group fails on it by name instead of reading as absent.
func TestNodeConfigTemplateOfABashibleGroupFailsWithoutTheAgentPackage(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))
	cluster := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		bashibleNodeGroup("worker", deckhousev1.SystemTypeMutable),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: "bashible-apiserver-files"},
			Data:       map[string]string{"images_digests.json": `{"registrypackages":{}}`},
		},
		testenv.PlatformTemplatesConfigMap(),
	).Build()

	_, err := NewTemplateStorage(cluster).Get(callerIn(t, "system:nodes"), "worker", &metav1.GetOptions{})
	require.ErrorContains(t, err, `"nodelet"`)
	require.False(t, apierrors.IsNotFound(err), "a broken release reads as a group without a template: %v", err)
}

func bashibleNodeGroup(name string, systemType deckhousev1.SystemType) client.Object {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic, SystemType: systemType},
	}
}

// callerIn is a request the API server authenticated as a member of the groups.
func callerIn(t *testing.T, groups ...string) context.Context {
	t.Helper()
	return request.WithUser(t.Context(), &user.DefaultInfo{
		Name:   "caller",
		Groups: append(groups, "system:authenticated"),
	})
}
