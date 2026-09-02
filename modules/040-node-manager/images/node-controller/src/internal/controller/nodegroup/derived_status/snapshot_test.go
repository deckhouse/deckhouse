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

package derived_status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// The snapshot is the package's whole input. Building it in one place is what makes the derive and
// the validate halves see the same world: they used to read the zones, the InstanceClass and the
// type catalog once each.
func TestBuildSnapshot_StaticNodeGroupReadsStaticConfigOnly(t *testing.T) {
	s := newTestService(t, testSecret(staticConfigSecretNamespace, staticConfigSecretName, map[string][]byte{
		staticConfigKey: []byte("internalNetworkCIDRs:\n- 10.0.0.0/24\n"),
	}))
	ng := &v1.NodeGroup{}
	ng.Name = "master"
	ng.Spec.NodeType = v1.NodeTypeStatic

	snap, err := s.BuildSnapshot(t.Context(), ng)

	require.NoError(t, err)
	require.NotNil(t, snap.StaticConfig)
	require.Empty(t, snap.Provider.Type)
	require.Nil(t, snap.InstanceClass)
	require.Nil(t, snap.DefaultZones, "zones belong to the cloud overlay a Static NodeGroup never gets")
}

// An unpublished version leaves the InstanceClass unread rather than read at a guessed version: a
// guess goes through the provider's conversion webhook, changes the spec, and renames the immutable
// machine template the checksum points at.
func TestBuildSnapshot_CloudEphemeralWithoutPublishedVersionSkipsInstanceClass(t *testing.T) {
	s := newTestService(t, testSecret(cloudProviderSecretNamespace, cloudProviderSecretName, map[string][]byte{
		"type":              []byte(`aws`),
		"instanceClassKind": []byte(`AWSInstanceClass`),
	}))
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	snap, err := s.BuildSnapshot(t.Context(), ng)

	require.NoError(t, err)
	require.Empty(t, snap.Provider.InstanceClassAPIVersion)
	require.Nil(t, snap.InstanceClass)
	require.Nil(t, snap.KnownClassNames)
}

// A cluster with no cloud provider at all yields an empty snapshot, not an error: that is a static
// cluster, and every Static NodeGroup in it must still resolve.
func TestBuildSnapshot_NoCloudProviderIsNotAnError(t *testing.T) {
	s := newTestService(t)
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral

	snap, err := s.BuildSnapshot(t.Context(), ng)

	require.NoError(t, err)
	require.Empty(t, snap.Provider.InstanceClassKind)
	require.Nil(t, snap.InstanceClass)
}

// An InstanceClass can be deleted between the List that check #2 uses and the Get that reads its
// spec, inside a single pass. Both checks then see the stale name and pass, so unless the snapshot
// records the failure the NodeGroup is declared processed and publishes instanceClass: null —
// dropping a real class from the element and shifting the checksum on its nodes.
func TestBuildSnapshot_ClassDeletedMidPassIsRecorded(t *testing.T) {
	const kind = "AWSInstanceClass"

	scheme := newTestScheme(t)
	gvk := schema.GroupVersionKind{Group: instanceClassGroup, Version: "v1", Kind: kind}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(kind+"List"), &unstructured.UnstructuredList{})

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(gvk)
	existing.SetName("worker")

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existing, testSecret(cloudProviderSecretNamespace, cloudProviderSecretName, map[string][]byte{
			"type":                    []byte(`aws`),
			"instanceClassKind":       []byte(kind),
			"instanceClassAPIVersion": []byte(`v1`),
		})).
		WithInterceptorFuncs(interceptor.Funcs{
			// The List still returns it; the Get no longer does.
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if u, ok := obj.(*unstructured.Unstructured); ok && u.GroupVersionKind().Kind == kind {
					return apierrors.NewNotFound(schema.GroupResource{Resource: "awsinstanceclasses"}, key.Name)
				}
				return cl.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	s := &Service{Client: c}

	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		ClassReference: v1.ClassReference{Kind: kind, Name: "worker"},
		MinPerZone:     0,
		MaxPerZone:     3,
	}

	snap, err := s.BuildSnapshot(t.Context(), ng)
	require.NoError(t, err)
	require.Nil(t, snap.InstanceClass)
	require.Error(t, snap.CapacityErr, "a class that vanished mid-pass must be recorded")

	check := Validate(ng, snap)
	require.False(t, check.Processed, "an unreadable class must not be published as processed")
	require.NotEmpty(t, check.Error)
}

// The autoscaler needs a template NodeInfo for every node group it discovers. A group with
// minPerZone above zero never scales from zero, but it still spends time with no registered Node —
// while its only Node boots, or mid-churn — and with no template the clusterapi provider fails
// ResourcesLeft with "No node info for: <group>", which aborts scale-up for every other group in
// the cluster. So TemplateCapacity is resolved regardless of minPerZone.
//
// What must not follow is publishing it: nodeCapacity is a top-level key of the NodeGroup element,
// and bashible-apiserver's AddToChecksum excludes only the replica counters under cloudInstances —
// so a newly published key changes configurationChecksum and re-runs node configuration fleet-wide.
func TestBuildSnapshot_TemplateCapacityIsResolvedAboveZeroMinButNotPublished(t *testing.T) {
	for _, tc := range []struct {
		name          string
		minPerZone    int32
		wantPublished bool
	}{
		{name: "scale from zero publishes", minPerZone: 0, wantPublished: true},
		{name: "fixed size does not publish", minPerZone: 1, wantPublished: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newDVPTestService(t, "worker", 4, "8Gi")

			ng := &v1.NodeGroup{}
			ng.Name = "worker"
			ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
			ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
				ClassReference: v1.ClassReference{Kind: "DVPInstanceClass", Name: "worker"},
				MinPerZone:     tc.minPerZone,
				MaxPerZone:     3,
			}

			snap, err := s.BuildSnapshot(t.Context(), ng)
			require.NoError(t, err)
			require.NoError(t, snap.CapacityErr)

			require.NotNil(t, snap.TemplateCapacity, "the MachineDeployment annotations have no other source")
			require.Equal(t, "4", snap.TemplateCapacity.CPU.String())
			require.Equal(t, "8Gi", snap.TemplateCapacity.Memory.String())

			result, err := Derive(t.Context(), ng, snap)
			require.NoError(t, err)
			require.NotNil(t, result.TemplateCapacity)

			element := ResolveNodeGroup(ResolveInput{
				Name:           ng.Name,
				NodeType:       ng.Spec.NodeType,
				Spec:           ng.Spec,
				CloudProcessed: true,
			}, result).ToMap()

			if tc.wantPublished {
				require.NotNil(t, snap.Capacity)
				require.Contains(t, element, "nodeCapacity", "scale-from-zero has always published it")
			} else {
				require.Nil(t, snap.Capacity, "only scale-from-zero publishes the value")
				require.Nil(t, result.NodeCapacity)
				require.NotContains(t, element, "nodeCapacity",
					"publishing it here changes configurationChecksum and re-runs bashible on every node")
			}
		})
	}
}

// newDVPTestService serves one DVPInstanceClass whose spec carries its own capacity, so the test
// does not depend on the InstanceTypesCatalog being present.
func newDVPTestService(t *testing.T, className string, cores int64, memory string) *Service {
	t.Helper()

	const kind = "DVPInstanceClass"

	scheme := newTestScheme(t)
	gvk := schema.GroupVersionKind{Group: instanceClassGroup, Version: "v1", Kind: kind}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(kind+"List"), &unstructured.UnstructuredList{})

	ic := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"virtualMachine": map[string]interface{}{
				"cpu":    map[string]interface{}{"cores": cores},
				"memory": map[string]interface{}{"size": memory},
			},
		},
	}}
	ic.SetGroupVersionKind(gvk)
	ic.SetName(className)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ic, testSecret(cloudProviderSecretNamespace, cloudProviderSecretName, map[string][]byte{
			"type":                    []byte(`dvp`),
			"instanceClassKind":       []byte(kind),
			"instanceClassAPIVersion": []byte(`v1`),
		})).
		Build()

	return &Service{Client: c}
}

// An unreadable source must abort the pass: an empty provider reads as "no cloud", which drops
// instanceClass from the published element and re-runs bashible on every node.
func TestBuildSnapshot_UnreadableProviderAborts(t *testing.T) {
	s := newDeniedSecretService(t, cloudProviderSecretName)
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral

	_, err := s.BuildSnapshot(t.Context(), ng)

	require.ErrorContains(t, err, "read cloud provider secret")
}
