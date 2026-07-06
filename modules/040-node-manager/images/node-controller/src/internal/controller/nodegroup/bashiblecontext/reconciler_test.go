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
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

// newReconciler builds a Reconciler backed by a single fake client whose scheme
// knows the deckhouse v1 types (so Assemble can List NodeGroups) plus corev1 /
// discoveryv1 for the source objects the blob is assembled from.
func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, discoveryv1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Reconciler{
		Client:        c,
		Context:       &Service{Client: c},
		DerivedStatus: &derived_status.Service{Client: c},
	}
}

// readAssembledNodeGroups parses the produced Secret's input.yaml and returns
// its nodeGroups list.
func readAssembledNodeGroups(t *testing.T, c client.Client) []interface{} {
	t.Helper()
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: secretNamespace, Name: secretName}, secret))
	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(secret.Data[secretInputKey], &parsed))
	ngs, _ := parsed["nodeGroups"].([]interface{})
	return ngs
}

func staticNodeGroup(name string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
}

// TestAssemble_SortsAndWritesAllNodeGroups proves Assemble builds one element per
// NodeGroup and emits them name-sorted for a deterministic payload.
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

// TestAssemble_PreservesPriorOnValidationFailure verifies get_crds preserve-prior:
// a NodeGroup that fails validation keeps its previously-stored blob element.
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
		secret(kubeSystemNS, cloudProviderSecretName, map[string][]byte{
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
	assert.Equal(t, "worker", el["name"])
	assert.Equal(t, "kept-from-prior", el["marker"], "failed NG must reuse the prior element")
}

// TestAssemble_OmitsFailingNodeGroupWithoutPrior verifies a validation failure
// with no prior element drops the NodeGroup entirely (get_crds `continue`).
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
		secret(kubeSystemNS, cloudProviderSecretName, map[string][]byte{
			"instanceClassKind": []byte(`"YandexInstanceClass"`),
		}),
	)

	require.NoError(t, r.Assemble(context.Background()))

	assert.Empty(t, readAssembledNodeGroups(t, r.Client))
}
