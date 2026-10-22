/*
Copyright 2022 Flant JSC

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

package kinds

import (
	"context"
	"testing"
	"time"

	"github.com/flant/kube-client/fake"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/require"

	"k8s.io/client-go/restmapper"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicate(t *testing.T) {
	con := gatekeeper.Constraint{Spec: gatekeeper.ConstraintSpec{Match: gatekeeper.Match{
		Kinds: []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"networking.k8s.io", "extensions"},
				Kinds:     []string{"Ingress"},
			},
		},
	}}}

	con2 := gatekeeper.Constraint{Spec: gatekeeper.ConstraintSpec{Match: gatekeeper.Match{
		Kinds: []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"extensions", "networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
		},
	}}}

	res, _ := deduplicateKinds([]gatekeeper.Constraint{con, con2})

	assert.Len(t, res, 2)
	assert.Equal(t, res[":Pod"], gatekeeper.MatchKind{APIGroups: []string{""}, Kinds: []string{"Pod"}})
	assert.Equal(t, res["extensions,networking.k8s.io:Ingress"], gatekeeper.MatchKind{APIGroups: []string{"extensions", "networking.k8s.io"}, Kinds: []string{"Ingress"}})
}

func TestResourceMapper(t *testing.T) {
	cl := fake.NewFakeCluster(fake.ClusterVersionV121)

	apiRes, err := restmapper.GetAPIGroupResources(cl.Discovery)
	require.NoError(t, err)

	rmatch := resourceMatcher{
		apiGroupResources: apiRes,
		mapper:            restmapper.NewDiscoveryRESTMapper(apiRes),
	}

	t.Run("deduplicate objects with wildcard and empty apiGroup", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{"*"},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
`, string(data))
	})

	t.Run("dedup equal resources", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{"extensions", "networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - extensions
  - networking.k8s.io
  resources:
  - ingresses
`, string(data))
	})

	t.Run("keep different resources", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
			{
				APIGroups: []string{"apps"},
				Kinds:     []string{"Deployment"},
			},
			{
				APIGroups: []string{""},
				Kinds:     []string{"PodExecOptions"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
- apiGroups:
  - apps
  resources:
  - deployments
- apiGroups:
  - ""
  resources:
  - pods/attach
  - pods/exec
  operations:
  - CONNECT
`, string(data))
	})

	t.Run("wrong kind", func(t *testing.T) {
		kinds := []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"apps/v1"},
				Kinds:     []string{"Deployment"},
			},
		}

		data, err := rmatch.convertKindsToResource(kinds)
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
`, string(data))
	})
}

func mutationWithOps(kinds []gatekeeper.MatchKind, ops ...string) gatekeeper.Mutation {
	m := gatekeeper.Mutation{Spec: gatekeeper.MutationSpec{Match: gatekeeper.Match{Kinds: kinds}}}
	if len(ops) > 0 {
		m.Spec.ApplyTo = []gatekeeper.MutationApplyTo{{Operations: ops}}
	}
	return m
}

func TestDeduplicateMutationKinds(t *testing.T) {
	pods := []gatekeeper.MatchKind{{APIGroups: []string{""}, Kinds: []string{"Pod"}}}
	deployments := []gatekeeper.MatchKind{{APIGroups: []string{"apps"}, Kinds: []string{"Deployment"}}}

	t.Run("operations of mutations with the same kinds are united", func(t *testing.T) {
		res, _ := deduplicateMutationKinds([]gatekeeper.Mutation{
			mutationWithOps(pods, "CREATE"),
			mutationWithOps(pods, "UPDATE"),
			mutationWithOps(deployments, "CREATE"),
		})

		assert.Equal(t, []mutationKind{
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{""}, Kinds: []string{"Pod"}}, Operations: []string{"CREATE", "UPDATE"}},
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{"apps"}, Kinds: []string{"Deployment"}}, Operations: []string{"CREATE"}},
		}, res)
	})

	t.Run("checksum changes with operations", func(t *testing.T) {
		_, createOnly := deduplicateMutationKinds([]gatekeeper.Mutation{mutationWithOps(pods, "CREATE")})
		_, all := deduplicateMutationKinds([]gatekeeper.Mutation{mutationWithOps(pods)})
		_, allAgain := deduplicateMutationKinds([]gatekeeper.Mutation{mutationWithOps(pods)})

		assert.NotEqual(t, createOnly, all)
		assert.Equal(t, all, allAgain)
	})

	t.Run("checksum does not depend on the order of mutations", func(t *testing.T) {
		_, first := deduplicateMutationKinds([]gatekeeper.Mutation{mutationWithOps(pods, "CREATE"), mutationWithOps(deployments)})
		_, second := deduplicateMutationKinds([]gatekeeper.Mutation{mutationWithOps(deployments), mutationWithOps(pods, "CREATE")})

		assert.Equal(t, first, second)
	})
}

func TestMutationResourceMapper(t *testing.T) {
	cl := fake.NewFakeCluster(fake.ClusterVersionV121)

	apiRes, err := restmapper.GetAPIGroupResources(cl.Discovery)
	require.NoError(t, err)

	rmatch := resourceMatcher{
		apiGroupResources: apiRes,
		mapper:            restmapper.NewDiscoveryRESTMapper(apiRes),
	}

	t.Run("resources of one group with the same operations share a rule", func(t *testing.T) {
		data, err := rmatch.convertMutationKindsToResource([]mutationKind{
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{"apps"}, Kinds: []string{"Deployment", "StatefulSet"}}, Operations: []string{"CREATE", "UPDATE"}},
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{""}, Kinds: []string{"Pod"}}, Operations: []string{"CREATE"}},
		})
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
  operations:
  - CREATE
- apiGroups:
  - apps
  resources:
  - deployments
  - statefulsets
  operations:
  - CREATE
  - UPDATE
`, string(data))
	})

	t.Run("operations of the same resource from different kinds entries are united", func(t *testing.T) {
		data, err := rmatch.convertMutationKindsToResource([]mutationKind{
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{"*"}, Kinds: []string{"Pod"}}, Operations: []string{"UPDATE"}},
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{""}, Kinds: []string{"Pod"}}, Operations: []string{"CREATE"}},
		})
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
  operations:
  - CREATE
  - UPDATE
`, string(data))
	})

	t.Run("groups are not crossed with resources of other groups", func(t *testing.T) {
		data, err := rmatch.convertMutationKindsToResource([]mutationKind{
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{"", "apps"}, Kinds: []string{"Pod", "Deployment"}}, Operations: []string{"CREATE", "UPDATE"}},
		})
		require.NoError(t, err)
		assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
  operations:
  - CREATE
  - UPDATE
- apiGroups:
  - apps
  resources:
  - deployments
  operations:
  - CREATE
  - UPDATE
`, string(data))
	})

	t.Run("unknown kinds are skipped", func(t *testing.T) {
		data, err := rmatch.convertMutationKindsToResource([]mutationKind{
			{MatchKind: gatekeeper.MatchKind{APIGroups: []string{"apps/v1"}, Kinds: []string{"Deployment"}}, Operations: []string{"CREATE"}},
		})
		require.NoError(t, err)
		assert.YAMLEq(t, `[]`, string(data))
	})
}

func TestUpdateTrackedObjectsWritesMutationOperations(t *testing.T) {
	cl := fake.NewFakeCluster(fake.ClusterVersionV121)
	kt := NewKindTracker(cl.Client, "d8-admission-policy-engine", "constraint-exporter")
	require.NoError(t, kt.FindInitialChecksum())

	createOnly := gatekeeper.Mutation{Spec: gatekeeper.MutationSpec{
		Match:   gatekeeper.Match{Kinds: []gatekeeper.MatchKind{{APIGroups: []string{"apps"}, Kinds: []string{"Deployment"}}}},
		ApplyTo: []gatekeeper.MutationApplyTo{{Operations: []string{"CREATE"}}},
	}}
	metadata := gatekeeper.Mutation{Spec: gatekeeper.MutationSpec{
		Match: gatekeeper.Match{Kinds: []gatekeeper.MatchKind{{APIGroups: []string{""}, Kinds: []string{"Pod"}}}},
	}}

	readMutateData := func() string {
		cm, err := cl.Client.CoreV1().ConfigMaps("d8-admission-policy-engine").Get(context.Background(), "constraint-exporter", metav1.GetOptions{})
		require.NoError(t, err)
		return cm.Data["mutate-resources.yaml"]
	}

	kt.UpdateTrackedObjects(nil, []gatekeeper.Mutation{createOnly, metadata})
	assert.YAMLEq(t, `
- apiGroups:
  - ""
  resources:
  - pods
  operations:
  - CREATE
  - UPDATE
- apiGroups:
  - apps
  resources:
  - deployments
  operations:
  - CREATE
`, readMutateData())

	// A change of the operations alone must rewrite the configmap.
	createOnly.Spec.ApplyTo[0].Operations = []string{"UPDATE"}
	kt.UpdateTrackedObjects(nil, []gatekeeper.Mutation{createOnly, metadata})
	assert.Contains(t, readMutateData(), "- apps\n  operations:\n  - UPDATE\n")
}

func TestUpdateTrackedObjectsConfigMapRetryOnConflict(t *testing.T) {
	client := k8sfake.NewSimpleClientset()

	_, err := client.CoreV1().ConfigMaps("d8-admission-policy-engine").Create(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "constraint-exporter",
			Namespace: "d8-admission-policy-engine",
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	var updateAttempts int
	client.PrependReactor("update", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		if updateAttempts == 1 {
			return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "", Resource: "configmaps"}, "constraint-exporter", assert.AnError)
		}

		obj := action.(k8stesting.UpdateAction).GetObject()
		return false, obj, nil
	})

	kt := NewKindTracker(client, "d8-admission-policy-engine", "constraint-exporter")

	err = kt.updateTrackedObjectsConfigMap("constraints", "mutations", "validate-data", "mutate-data")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, updateAttempts, 2)

	require.Eventually(t, func() bool {
		cm, getErr := client.CoreV1().ConfigMaps("d8-admission-policy-engine").Get(context.Background(), "constraint-exporter", metav1.GetOptions{})
		if getErr != nil {
			return false
		}

		return cm.Annotations[constraintChecksumAnnotation] == "constraints" &&
			cm.Annotations[mutationChecksumAnnotation] == "mutations" &&
			cm.Data["validate-resources.yaml"] == "validate-data" &&
			cm.Data["mutate-resources.yaml"] == "mutate-data"
	}, 5*time.Second, 50*time.Millisecond)
}
