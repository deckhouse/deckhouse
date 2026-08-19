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
	"github.com/deckhouse/node-controller/internal/cloudprovider"
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

	snap, err := s.BuildSnapshot(t.Context(), ng, testRegistry(t, s))

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
	s := newTestService(t, providerSecret(map[string][]byte{
		"type":              []byte(`aws`),
		"instanceClassKind": []byte(`AWSInstanceClass`),
	}))
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	snap, err := s.BuildSnapshot(t.Context(), ng, testRegistry(t, s))

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

	snap, err := s.BuildSnapshot(t.Context(), ng, testRegistry(t, s))

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
		WithObjects(existing, providerSecret(map[string][]byte{
			"type":                    []byte(`aws`),
			"instanceClassKind":       []byte(kind),
			"instanceClassAPIVersion": []byte(`v1`),
		}), cloudClusterConfig("AWS")).
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

	snap, err := s.BuildSnapshot(t.Context(), ng, testRegistry(t, s))
	require.NoError(t, err)
	require.Nil(t, snap.InstanceClass)
	require.Error(t, snap.CapacityErr, "a class that vanished mid-pass must be recorded")

	check := Validate(ng, snap)
	require.False(t, check.Processed, "an unreadable class must not be published as processed")
	require.NotEmpty(t, check.Error)
}

// An unreadable source must abort the pass: an empty provider reads as "no cloud", which drops
// instanceClass from the published element and re-runs bashible on every node. The provider half
// of that guard now sits at the load boundary — see cloudprovider.TestLoad_ForbiddenListIsAnError;
// here the cluster UUID stands for the sources this package still reads itself.
func TestBuildSnapshot_UnreadableSourceAborts(t *testing.T) {
	s := newDeniedSecretService(t, clusterUUIDConfigMapName)
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral

	_, err := s.BuildSnapshot(t.Context(), ng, cloudprovider.Providers{})

	require.ErrorContains(t, err, "read cluster uuid configmap")
}
