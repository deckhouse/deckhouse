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

package bashiblecontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	providermock "github.com/deckhouse/node-controller/internal/cloudprovider/mock"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, discoveryv1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	// Every real cluster has the kube-dns Service and the cluster-configuration Secret; the
	// assembly refuses to publish a context without a DNS address or a cluster domain, so the
	// fixture must carry both.
	// derived_status also requires d8-cluster-kubernetes for the target Kubernetes version.
	objs = append(objs,
		endpointSlice([]string{"10.0.0.1"}, "https", 6443),
		kubeDNSService("10.222.0.10"),
		clusterConfigurationSecret("cluster.local"),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-kubernetes", Namespace: kubeSystemNS},
			Data:       map[string]string{"spec": "desiredVersion: \"1.32\"\nupdateMode: Manual\n"},
		},
	)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{
		Client:        c,
		Context:       &Service{Client: c},
		DerivedStatus: &derived_status.Service{Client: c},
	}
}

func readAssembledNodeGroups(t *testing.T, c client.Client) []interface{} {
	t.Helper()
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: secretNamespace, Name: secretName}, secret))
	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(secret.Data[secretInputKey], &parsed))
	ngs, _ := parsed["nodeGroups"].([]interface{})
	return ngs
}

func clusterConfigurationSecret(domain string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigSecretName, Namespace: kubeSystemNS},
		Data:       map[string][]byte{clusterConfigKey: []byte("clusterDomain: " + domain + "\n")},
	}
}

func kubeDNSService(clusterIP string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-dns",
			Namespace: kubeSystemNS,
			Labels:    map[string]string{dnsAppLabel: "kube-dns"},
		},
		Spec: corev1.ServiceSpec{ClusterIP: clusterIP},
	}
}

func staticNodeGroup(name string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
}

func TestAssemble_SortsAndWritesAllNodeGroups(t *testing.T) {
	r := newReconciler(t,
		staticNodeGroup("zzz"),
		staticNodeGroup("aaa"),
		secret("kube-system", "d8-static-cluster-configuration", map[string][]byte{
			"static-cluster-configuration.yaml": []byte("internalNetworkCIDRs:\n- 172.18.200.0/24\n"),
		}),
	)

	require.NoError(t, r.Assemble(context.Background()))

	ngs := readAssembledNodeGroups(t, r.Client)
	require.Len(t, ngs, 2)
	assert.Equal(t, "aaa", ngs[0].(map[string]interface{})["name"])
	assert.Equal(t, "zzz", ngs[1].(map[string]interface{})["name"])
}

func TestAssemble_PreservesPriorOnValidationFailure(t *testing.T) {
	priorInput, err := Marshal(map[string]interface{}{
		"nodeGroups": []interface{}{
			map[string]interface{}{"name": "worker", "marker": "kept-from-prior"},
		},
	})
	require.NoError(t, err)

	r := newReconciler(t,
		&v1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec: v1.NodeGroupSpec{
				NodeType: v1.NodeTypeCloudEphemeral,
				CloudInstances: &v1.CloudInstancesSpec{
					ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
				},
			},
		},
		providermock.DefaultRegistration(map[string][]byte{
			"type":                    []byte(`"yandex"`),
			"instanceClassKind":       []byte(`"YandexInstanceClass"`),
			"instanceClassAPIVersion": []byte("v1alpha1"),
		}),
		secret(secretNamespace, secretName, map[string][]byte{
			secretInputKey: priorInput,
		}),
	)

	require.NoError(t, r.Assemble(context.Background()))

	ngs := readAssembledNodeGroups(t, r.Client)
	require.Len(t, ngs, 1)
	el := ngs[0].(map[string]interface{})
	assert.Equal(t, "worker", el["name"])
	assert.Equal(t, "kept-from-prior", el["marker"], "failed NG must reuse the prior element")
}

// An entry written before cloudProviderType existed names no provider, and would render without
// cloud steps.
func TestAssemble_PriorEntryGetsTheProviderOfItsNodeGroup(t *testing.T) {
	priorInput, err := Marshal(map[string]interface{}{
		"nodeGroups": []interface{}{
			map[string]interface{}{"name": "worker", "marker": "kept-from-prior"},
		},
	})
	require.NoError(t, err)

	r := newReconciler(t,
		&v1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec: v1.NodeGroupSpec{
				NodeType: v1.NodeTypeCloudEphemeral,
				CloudInstances: &v1.CloudInstancesSpec{
					ClassReference: v1.ClassReference{Kind: "YandexInstanceClass", Name: "worker"},
				},
			},
		},
		// The kind resolves; the missing API version is what fails validation.
		providermock.DefaultRegistration(map[string][]byte{
			"type":              []byte(`"yandex"`),
			"instanceClassKind": []byte(`"YandexInstanceClass"`),
		}),
		secret(secretNamespace, secretName, map[string][]byte{
			secretInputKey: priorInput,
		}),
	)

	require.NoError(t, r.Assemble(context.Background()))

	ngs := readAssembledNodeGroups(t, r.Client)
	require.Len(t, ngs, 1)
	el := ngs[0].(map[string]interface{})
	assert.Equal(t, "kept-from-prior", el["marker"], "the prior element must still be reused")
	assert.Equal(t, "yandex", el["cloudProviderType"], "the prior element must name its provider")
}

func TestAssemble_OmitsFailingNodeGroupWithoutPrior(t *testing.T) {
	r := newReconciler(t,
		&v1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec: v1.NodeGroupSpec{
				NodeType: v1.NodeTypeCloudEphemeral,
				CloudInstances: &v1.CloudInstancesSpec{
					ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
				},
			},
		},
		providermock.DefaultRegistration(map[string][]byte{
			"type":                    []byte(`"yandex"`),
			"instanceClassKind":       []byte(`"YandexInstanceClass"`),
			"instanceClassAPIVersion": []byte("v1alpha1"),
		}),
	)

	require.NoError(t, r.Assemble(context.Background()))

	assert.Empty(t, readAssembledNodeGroups(t, r.Client))
}

// internalNetworkCIDRs disappearing from the static cluster configuration is either an accident or
// a decision nobody confirmed: the published CIDRs are kept for every static group — only that
// block, the rest of each entry stays fresh — the context is still written, and the pass fails so
// the condition is visible, the way the convert_static_cluster_configuration hook failed.
func TestAssemble_KeepsPublishedCIDRsAndReportsIt(t *testing.T) {
	priorInput, err := Marshal(map[string]interface{}{
		"nodeGroups": []interface{}{
			map[string]interface{}{
				"name":              "losing",
				"nodeType":          "Static",
				"marker":            "stale-from-prior",
				"kubernetesVersion": "1.29",
				"static":            map[string]interface{}{"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"}},
			},
			map[string]interface{}{"name": "intact", "nodeType": "Static", "marker": "stale-from-prior"},
		},
	})
	require.NoError(t, err)

	// No d8-static-cluster-configuration Secret: both groups derive an empty CIDR list.
	r := newReconciler(t,
		staticNodeGroup("losing"),
		staticNodeGroup("intact"),
		secret(secretNamespace, secretName, map[string][]byte{secretInputKey: priorInput}),
	)

	err = r.Assemble(context.Background())
	require.ErrorContains(t, err, "internalNetworkCIDRs")
	require.ErrorContains(t, err, "losing")

	byName := map[string]map[string]interface{}{}
	for _, ng := range readAssembledNodeGroups(t, r.Client) {
		el := ng.(map[string]interface{})
		byName[el["name"].(string)] = el
	}
	require.Len(t, byName, 2, "the context must still be published")

	want := map[string]interface{}{"internalNetworkCIDRs": []interface{}{"172.18.200.0/24"}}
	for _, name := range []string{"losing", "intact"} {
		entry := byName[name]
		assert.Equal(t, want, entry["static"], "%s: the CIDRs are one cluster-wide value", name)
		assert.NotContains(t, entry, "marker", "%s: only the static block comes from the prior entry", name)
		assert.Equal(t, "1.32", entry["kubernetesVersion"], "%s: everything else must be freshly derived", name)
	}
}

// The prior context is a fallback for NodeGroups that still exist, never a source of them: a
// deleted NodeGroup must leave the published context, or its nodes keep bootstrapping forever.
func TestAssemble_DropsNodeGroupsThatNoLongerExist(t *testing.T) {
	priorInput, err := Marshal(map[string]interface{}{
		"nodeGroups": []interface{}{
			map[string]interface{}{"name": "deleted", "nodeType": "Static", "marker": "stale-from-prior"},
		},
	})
	require.NoError(t, err)

	r := newReconciler(t,
		staticNodeGroup("alive"),
		secret(secretNamespace, secretName, map[string][]byte{secretInputKey: priorInput}),
	)

	require.NoError(t, r.Assemble(context.Background()))

	ngs := readAssembledNodeGroups(t, r.Client)
	require.Len(t, ngs, 1)
	assert.Equal(t, "alive", ngs[0].(map[string]interface{})["name"])
}
