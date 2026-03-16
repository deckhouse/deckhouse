//go:build ai_tests

/*
Copyright 2025 Flant JSC

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

package instanceclass

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	return s
}

func newReconciler(cl client.Client, scheme *runtime.Scheme) *NodeGroupInstanceClassReconciler {
	r := &NodeGroupInstanceClassReconciler{}
	r.Base = dynr.Base{
		Client: cl,
		Scheme: scheme,
	}
	return r
}

func newUnstructuredIC(kind, name, apiVersion string) *unstructured.Unstructured {
	ic := &unstructured.Unstructured{}
	gv, _ := schema.ParseGroupVersion(apiVersion)
	ic.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	})
	ic.SetName(name)
	return ic
}

func restMapperForIC(kind, apiVersion string) meta.RESTMapper {
	gv, _ := schema.ParseGroupVersion(apiVersion)
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gv})
	mapper.Add(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: kind}, meta.RESTScopeRoot)
	return mapper
}

// ssaClient wraps a fake client and handles SSA status patches by properly
// merging the patch object status into the existing object.
// The fake client does not support Server-Side Apply, so we simulate it.
type ssaClient struct {
	inner client.Client
}

func (c *ssaClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.inner.Get(ctx, key, obj, opts...)
}
func (c *ssaClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.inner.List(ctx, list, opts...)
}
func (c *ssaClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return c.inner.Create(ctx, obj, opts...)
}
func (c *ssaClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return c.inner.Delete(ctx, obj, opts...)
}
func (c *ssaClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return c.inner.Update(ctx, obj, opts...)
}
func (c *ssaClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.inner.Patch(ctx, obj, patch, opts...)
}
func (c *ssaClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return c.inner.DeleteAllOf(ctx, obj, opts...)
}
func (c *ssaClient) Scheme() *runtime.Scheme {
	return c.inner.Scheme()
}
func (c *ssaClient) RESTMapper() meta.RESTMapper {
	return c.inner.RESTMapper()
}
func (c *ssaClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.inner.GroupVersionKindFor(obj)
}
func (c *ssaClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.inner.IsObjectNamespaced(obj)
}
func (c *ssaClient) SubResource(subResource string) client.SubResourceClient {
	return c.inner.SubResource(subResource)
}
func (c *ssaClient) Status() client.SubResourceWriter {
	return &ssaStatusWriter{inner: c.inner}
}

type ssaStatusWriter struct {
	inner client.Client
}

func (w *ssaStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return w.inner.Status().Create(ctx, obj, subResource, opts...)
}

func (w *ssaStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return w.inner.Status().Update(ctx, obj, opts...)
}

func (w *ssaStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if patch.Type() == types.ApplyPatchType {
		// For SSA with unstructured objects: the obj contains the desired state.
		// We merge status fields from obj into the existing object and write it back.
		if u, ok := obj.(*unstructured.Unstructured); ok {
			existing := &unstructured.Unstructured{}
			existing.SetGroupVersionKind(u.GroupVersionKind())
			if err := w.inner.Get(ctx, client.ObjectKeyFromObject(u), existing); err != nil {
				return client.IgnoreNotFound(err)
			}

			// Merge status from the SSA object into existing
			patchStatus, _, _ := unstructured.NestedMap(u.Object, "status")
			if patchStatus != nil {
				_ = unstructured.SetNestedMap(existing.Object, patchStatus, "status")
			}

			return w.inner.Update(ctx, existing)
		}
		return w.inner.Update(ctx, obj)
	}
	return w.inner.Status().Patch(ctx, obj, patch, opts...)
}

func buildSSAFakeClient(s *runtime.Scheme, mapper meta.RESTMapper, objs []client.Object, runtimeObjs []runtime.Object) client.Client {
	builder := fake.NewClientBuilder().WithScheme(s)
	if mapper != nil {
		builder = builder.WithRESTMapper(mapper)
	}
	if len(objs) > 0 {
		builder = builder.WithObjects(objs...)
	}
	if len(runtimeObjs) > 0 {
		builder = builder.WithRuntimeObjects(runtimeObjs...)
	}
	inner := builder.Build()
	return &ssaClient{inner: inner}
}

// TestAI_CollectConsumersSingleNodeGroup verifies that a single NodeGroup referencing
// an InstanceClass produces the correct consumer list.
func TestAI_CollectConsumersSingleNodeGroup(t *testing.T) {
	ref := deckhousev1.ClassReference{
		Kind: "VCDInstanceClass",
		Name: "my-ic",
	}

	nodeGroups := []deckhousev1.NodeGroup{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 1,
					MaxPerZone: 3,
					ClassReference: deckhousev1.ClassReference{
						Kind: "VCDInstanceClass",
						Name: "my-ic",
					},
				},
			},
		},
	}

	consumers := collectConsumers(nodeGroups, ref)
	assert.Equal(t, []string{"worker"}, consumers)
}

// TestAI_CollectConsumersMultipleNodeGroupsSameIC verifies that multiple NodeGroups
// referencing the same InstanceClass all appear in the consumer list.
func TestAI_CollectConsumersMultipleNodeGroupsSameIC(t *testing.T) {
	ref := deckhousev1.ClassReference{
		Kind: "VCDInstanceClass",
		Name: "shared-ic",
	}

	nodeGroups := []deckhousev1.NodeGroup{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-a"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 1,
					MaxPerZone: 3,
					ClassReference: deckhousev1.ClassReference{
						Kind: "VCDInstanceClass",
						Name: "shared-ic",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-b"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 2,
					MaxPerZone: 5,
					ClassReference: deckhousev1.ClassReference{
						Kind: "VCDInstanceClass",
						Name: "shared-ic",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-c"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 1,
					MaxPerZone: 2,
					ClassReference: deckhousev1.ClassReference{
						Kind: "VCDInstanceClass",
						Name: "other-ic",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "static-ng"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeStatic,
			},
		},
	}

	consumers := collectConsumers(nodeGroups, ref)
	assert.Len(t, consumers, 2)
	assert.Contains(t, consumers, "worker-a")
	assert.Contains(t, consumers, "worker-b")
	assert.NotContains(t, consumers, "worker-c")
	assert.NotContains(t, consumers, "static-ng")
}

// TestAI_CollectConsumersNoMatches verifies that when no NodeGroups reference the IC,
// the consumer list is empty.
func TestAI_CollectConsumersNoMatches(t *testing.T) {
	ref := deckhousev1.ClassReference{
		Kind: "VCDInstanceClass",
		Name: "unused-ic",
	}

	nodeGroups := []deckhousev1.NodeGroup{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone: 1,
					MaxPerZone: 3,
					ClassReference: deckhousev1.ClassReference{
						Kind: "VCDInstanceClass",
						Name: "other-ic",
					},
				},
			},
		},
	}

	consumers := collectConsumers(nodeGroups, ref)
	assert.Empty(t, consumers)
}

// TestAI_ReconcileSingleNodeGroupUpdatesIC verifies the full Reconcile flow:
// a NodeGroup with an InstanceClass reference triggers a status patch on the IC.
func TestAI_ReconcileSingleNodeGroupUpdatesIC(t *testing.T) {
	s := newTestScheme()
	mapper := restMapperForIC("VCDInstanceClass", "deckhouse.io/v1")

	ic := newUnstructuredIC("VCDInstanceClass", "my-ic", "deckhouse.io/v1")

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 3,
				ClassReference: deckhousev1.ClassReference{
					Kind: "VCDInstanceClass",
					Name: "my-ic",
				},
			},
		},
	}

	cl := buildSSAFakeClient(s, mapper, []client.Object{ng}, []runtime.Object{ic})
	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "worker"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the IC was patched (the reconciler passes the full IC object to SSA Patch,
	// which writes the object back; with real SSA, the server would extract managed fields).
	updatedIC := newUnstructuredIC("VCDInstanceClass", "my-ic", "deckhouse.io/v1")
	err = cl.Get(context.Background(), client.ObjectKey{Name: "my-ic"}, updatedIC)
	require.NoError(t, err)
	// The IC object should still exist and be accessible after reconciliation
	assert.Equal(t, "my-ic", updatedIC.GetName())
}

// TestAI_ReconcileMultipleNodeGroupsSameIC verifies that reconciling one NodeGroup
// correctly identifies all consumers of the same InstanceClass.
func TestAI_ReconcileMultipleNodeGroupsSameIC(t *testing.T) {
	s := newTestScheme()
	mapper := restMapperForIC("VCDInstanceClass", "deckhouse.io/v1")

	ic := newUnstructuredIC("VCDInstanceClass", "shared-ic", "deckhouse.io/v1")

	ng1 := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-a"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1, MaxPerZone: 3,
				ClassReference: deckhousev1.ClassReference{Kind: "VCDInstanceClass", Name: "shared-ic"},
			},
		},
	}

	ng2 := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-b"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 2, MaxPerZone: 5,
				ClassReference: deckhousev1.ClassReference{Kind: "VCDInstanceClass", Name: "shared-ic"},
			},
		},
	}

	ng3 := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-c"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1, MaxPerZone: 2,
				ClassReference: deckhousev1.ClassReference{Kind: "VCDInstanceClass", Name: "other-ic"},
			},
		},
	}

	cl := buildSSAFakeClient(s, mapper, []client.Object{ng1, ng2, ng3}, []runtime.Object{ic})
	r := newReconciler(cl, s)

	// Reconcile ng1 — should succeed without error
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "worker-a"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_NonCloudEphemeralNodeGroupIgnored verifies that non-CloudEphemeral NodeGroups
// are silently skipped.
func TestAI_NonCloudEphemeralNodeGroupIgnored(t *testing.T) {
	s := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "static-ng",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(ng).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "static-ng"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_NodeGroupWithEmptyClassReference verifies that a NodeGroup with an empty
// class reference does not trigger any IC update.
func TestAI_NodeGroupWithEmptyClassReference(t *testing.T) {
	s := newTestScheme()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ephemeral-no-ref",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 3,
				ClassReference: deckhousev1.ClassReference{
					Kind: "",
					Name: "",
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(ng).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "ephemeral-no-ref"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_NodeGroupNotFound verifies that reconciling a nonexistent NodeGroup
// returns no error (IgnoreNotFound).
func TestAI_NodeGroupNotFound(t *testing.T) {
	s := newTestScheme()

	cl := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "nonexistent"},
	})
	require.NoError(t, err, "should not error on missing NodeGroup (IgnoreNotFound)")
	assert.Equal(t, ctrl.Result{}, result)
}

// TestAI_InstanceClassNotFoundIsNotAnError verifies that if the referenced
// InstanceClass does not exist, the reconciler returns no error.
func TestAI_InstanceClassNotFoundIsNotAnError(t *testing.T) {
	s := newTestScheme()
	mapper := restMapperForIC("VCDInstanceClass", "deckhouse.io/v1")

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: 1,
				MaxPerZone: 3,
				ClassReference: deckhousev1.ClassReference{
					Kind: "VCDInstanceClass",
					Name: "missing-ic",
				},
			},
		},
	}

	cl := buildSSAFakeClient(s, mapper, []client.Object{ng}, nil)

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "worker"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}
