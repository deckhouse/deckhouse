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

package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

func schemaGroupResource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: deckhousev1alpha2.GroupVersion.Group, Resource: resource}
}

func staticNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{nodecommon.NodeTypeLabel: string(deckhousev1.NodeTypeStatic)},
		},
	}
}

func TestReconcileNodeEnsuresInstanceForStaticNode(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().
		WithScheme(newNodeTestScheme(t)).
		WithStatusSubresource(&deckhousev1alpha2.Instance{}).
		WithObjects(staticNode("static-a")).
		Build()

	require.NoError(t, ReconcileNode(context.Background(), c, "static-a"))

	instance := &deckhousev1alpha2.Instance{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "static-a"}, instance))
	require.Equal(t, "static-a", instance.Spec.NodeRef.Name)
	require.Equal(t, deckhousev1alpha2.InstancePhaseRunning, instance.Status.Phase)
}

func TestReconcileNodeSkipsNonStaticNode(t *testing.T) {
	t.Parallel()

	cloudNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cloud-a",
			Labels: map[string]string{nodecommon.NodeTypeLabel: "CloudEphemeral"},
		},
	}
	c := newNodeTestClient(t, cloudNode)

	require.NoError(t, ReconcileNode(context.Background(), c, "cloud-a"))

	instance := &deckhousev1alpha2.Instance{}
	err := c.Get(context.Background(), types.NamespacedName{Name: "cloud-a"}, instance)
	require.True(t, apierrors.IsNotFound(err), "no instance must be created for a non-static node")
}

func TestReconcileNodeMissingNodeDeletesNodeBasedInstance(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ghost",
			Finalizers: []string{instancecommon.InstanceControllerFinalizer},
		},
		Spec: deckhousev1alpha2.InstanceSpec{
			NodeRef: deckhousev1alpha2.NodeRef{Name: "ghost"},
		},
	}
	c := newNodeTestClient(t, instance)

	require.NoError(t, ReconcileNode(context.Background(), c, "ghost"))

	persisted := &deckhousev1alpha2.Instance{}
	err := c.Get(context.Background(), types.NamespacedName{Name: "ghost"}, persisted)
	require.True(t, apierrors.IsNotFound(err))
}

func TestReconcileNodeMissingNodeNoInstanceIsNoop(t *testing.T) {
	t.Parallel()

	c := newNodeTestClient(t)
	require.NoError(t, ReconcileNode(context.Background(), c, "nothing-here"))
}

func TestDeleteNodeBasedInstanceIfExistsFinalizerRemovalNotFound(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "node-a",
			Finalizers: []string{instancecommon.InstanceControllerFinalizer},
		},
		Spec: deckhousev1alpha2.InstanceSpec{
			NodeRef: deckhousev1alpha2.NodeRef{Name: "node-a"},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(newNodeTestScheme(t)).
		WithObjects(instance.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				return apierrors.NewNotFound(schemaGroupResource("instances"), "node-a")
			},
		}).
		Build()

	result, err := deleteNodeBasedInstanceIfExists(context.Background(), c, "node-a")
	require.NoError(t, err)
	require.False(t, result.InstanceDeleted)
}

func TestDeleteNodeBasedInstanceIfExistsDeleteNotFound(t *testing.T) {
	t.Parallel()

	instance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
		Spec: deckhousev1alpha2.InstanceSpec{
			NodeRef: deckhousev1alpha2.NodeRef{Name: "node-a"},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(newNodeTestScheme(t)).
		WithObjects(instance.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
				return apierrors.NewNotFound(schemaGroupResource("instances"), "node-a")
			},
		}).
		Build()

	result, err := deleteNodeBasedInstanceIfExists(context.Background(), c, "node-a")
	require.NoError(t, err)
	require.False(t, result.InstanceDeleted)
}

func TestDeleteNodeBasedInstanceIfExistsMissingInstance(t *testing.T) {
	t.Parallel()

	c := newNodeTestClient(t)

	result, err := deleteNodeBasedInstanceIfExists(context.Background(), c, "absent")
	require.NoError(t, err)
	require.False(t, result.InstanceDeleted)
}
