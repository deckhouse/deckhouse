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

package migration

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

func project() *v1alpha3.Project {
	return &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "example", UID: "project"},
		Spec:       v1alpha3.ProjectSpec{Quota: corev1.ResourceList{"pods": resource.MustParse("10")}},
	}
}

func replacement(p *v1alpha3.Project) *v1alpha1.ProjectQuotaStatus {
	return &v1alpha1.ProjectQuotaStatus{
		ObjectMeta: metav1.ObjectMeta{Name: p.Name, UID: "replacement", OwnerReferences: []metav1.OwnerReference{{
			APIVersion: v1alpha3.SchemeGroupVersion.String(), Kind: "Project", Name: p.Name,
			UID: p.UID, Controller: ptr.To(true), BlockOwnerDeletion: ptr.To(true),
		}}},
		Spec: v1alpha1.ProjectQuotaStatusSpec{Hard: p.Spec.Quota.DeepCopy()},
	}
}

func legacy() *unstructured.Unstructured {
	old := new(unstructured.Unstructured)
	old.SetAPIVersion("deckhouse.io/v1alpha1")
	old.SetKind("ProjectQuota")
	old.SetName("example")
	old.SetUID("legacy")
	old.SetFinalizers([]string{v1alpha1.ProjectQuotaFinalizer})
	old.SetLabels(map[string]string{"projects.deckhouse.io/managed-by": "controller"})
	old.SetOwnerReferences(replacement(project()).OwnerReferences)
	return old
}

func oldReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: legacyQuotaGVK.GroupVersion().String(), Kind: legacyQuotaGVK.Kind,
		Name: "example", UID: "legacy", Controller: ptr.To(true), BlockOwnerDeletion: ptr.To(true),
	}
}

func resourceQuota(namespace string) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-project-quota", Namespace: namespace, UID: types.UID(namespace),
			OwnerReferences: []metav1.OwnerReference{oldReference()},
		},
		Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{"pods": resource.MustParse("10")}},
	}
}

func migrationClient(t *testing.T, funcs interceptor.Funcs, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha3.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	scheme.AddKnownTypeWithName(legacyQuotaGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(legacyQuotaGVK.GroupVersion().WithKind("ProjectQuotaList"), &unstructured.UnstructuredList{})
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(funcs).Build()
}

func assertGone(t *testing.T, c client.Client) {
	t.Helper()
	old := legacy()
	assert.True(t, apierrors.IsNotFound(c.Get(context.Background(), client.ObjectKeyFromObject(old), old)))
}

func TestAutomaticQuotaTransition(t *testing.T) {
	ctx := context.Background()
	p := project()
	status := replacement(p)
	rq := resourceQuota("example")
	other := metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "foreign", UID: "foreign"}
	sameKind := oldReference()
	sameKind.UID = "different-owner"
	sameUID := oldReference()
	sameUID.APIVersion = "deckhouse.io/v1alpha2"
	rq.OwnerReferences = append(rq.OwnerReferences, other, sameKind, sameUID)
	c := migrationClient(t, interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			quotas := new(corev1.ResourceQuotaList)
			require.NoError(t, c.List(ctx, quotas))
			for _, quota := range quotas.Items {
				for _, ref := range quota.OwnerReferences {
					assert.False(t, ref.UID == "legacy" && ref.Kind == "ProjectQuota" &&
						ref.APIVersion == "deckhouse.io/v1alpha1", "detach before deleting the owner")
				}
			}
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(status), new(v1alpha1.ProjectQuotaStatus)))
			options := new(client.DeleteOptions)
			options.ApplyOptions(opts)
			require.NotNil(t, options.Preconditions)
			assert.Equal(t, types.UID("legacy"), *options.Preconditions.UID)
			return c.Delete(ctx, obj, opts...)
		},
	}, p, legacy(), rq, resourceQuota("additional"))
	m := &ProjectQuota{Client: c, Logger: logr.Discard()}

	// The normal project controller has not made the new status yet.
	require.ErrorContains(t, m.Run(ctx), "wait for the replacement")
	got := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(rq), got))
	assert.Len(t, got.OwnerReferences, 4)

	require.NoError(t, c.Create(ctx, status))
	require.NoError(t, m.Run(ctx))
	assertGone(t, c)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(rq), got))
	assert.Equal(t, rq.UID, got.UID)
	assert.Equal(t, rq.Spec, got.Spec)
	assert.Equal(t, []metav1.OwnerReference{other, sameKind, sameUID}, got.OwnerReferences)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "additional", Name: rq.Name}, got))
	assert.Empty(t, got.OwnerReferences)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(status), new(v1alpha1.ProjectQuotaStatus)))
	require.NoError(t, m.Run(ctx))
}

func TestWaitsForCurrentReplacement(t *testing.T) {
	for _, state := range []string{"different limits", "different owner", "terminating"} {
		t.Run(state, func(t *testing.T) {
			p := project()
			status := replacement(p)
			switch state {
			case "different limits":
				status.Spec.Hard["pods"] = resource.MustParse("1")
			case "different owner":
				status.OwnerReferences[0].UID = "previous-project"
			case "terminating":
				now := metav1.Now()
				status.DeletionTimestamp = &now
				status.Finalizers = []string{v1alpha1.ProjectQuotaFinalizer}
			}
			c := migrationClient(t, interceptor.Funcs{}, p, status, legacy())
			m := &ProjectQuota{Client: c}
			require.ErrorContains(t, m.Run(context.Background()), "wait for the replacement")
			require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: p.Name}, legacy()))
		})
	}
}

func TestRetiresQuotaWithoutReplacement(t *testing.T) {
	for _, state := range []string{"quota cleared", "project gone", "project deleting", "legacy deleting"} {
		t.Run(state, func(t *testing.T) {
			p := project()
			old := legacy()
			objects := []client.Object{old, resourceQuota("example")}
			switch state {
			case "quota cleared":
				p.Spec.Quota = nil
				objects = append(objects, p)
			case "project deleting":
				now := metav1.Now()
				p.DeletionTimestamp = &now
				p.Finalizers = []string{"foregroundDeletion"}
				objects = append(objects, p)
			case "legacy deleting":
				now := metav1.Now()
				old.SetDeletionTimestamp(&now)
			}
			c := migrationClient(t, interceptor.Funcs{}, objects...)
			m := &ProjectQuota{Client: c}
			require.NoError(t, m.Run(context.Background()))
			assertGone(t, c)
			require.NoError(t, m.Run(context.Background()))
		})
	}
}

func TestRetryAfterPartialFailure(t *testing.T) {
	for _, step := range []string{"detach", "delete", "finalizer"} {
		t.Run(step, func(t *testing.T) {
			ctx := context.Background()
			fail := true
			p := project()
			c := migrationClient(t, interceptor.Funcs{
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					_, rq := obj.(*corev1.ResourceQuota)
					if fail && ((step == "detach" && rq) || (step == "finalizer" && !rq)) {
						return fmt.Errorf("injected patch failure")
					}
					return c.Patch(ctx, obj, patch, opts...)
				},
				Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if fail && step == "delete" {
						return fmt.Errorf("injected delete failure")
					}
					return c.Delete(ctx, obj, opts...)
				},
			}, p, replacement(p), legacy(), resourceQuota("example"))
			m := &ProjectQuota{Client: c}
			require.Error(t, m.Run(ctx))
			old := legacy()
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(old), old))
			assert.Contains(t, old.GetFinalizers(), v1alpha1.ProjectQuotaFinalizer)
			fail = false
			// A new runnable after a restart resumes from the API objects.
			require.NoError(t, (&ProjectQuota{Client: c}).Run(ctx))
			assertGone(t, c)
		})
	}
}

func TestOptionalLegacyAPI(t *testing.T) {
	for _, state := range []string{"no mapping", "not found", "forbidden"} {
		t.Run(state, func(t *testing.T) {
			c := migrationClient(t, interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
					switch state {
					case "no mapping":
						return &meta.NoKindMatchError{GroupKind: legacyQuotaGVK.GroupKind()}
					case "not found":
						return apierrors.NewNotFound(schema.GroupResource{Group: "deckhouse.io", Resource: "projectquotas"}, "")
					default:
						return apierrors.NewForbidden(schema.GroupResource{Group: "deckhouse.io", Resource: "projectquotas"}, "", fmt.Errorf("denied"))
					}
				},
			})
			err := (&ProjectQuota{Client: c}).Run(context.Background())
			if state == "forbidden" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOtherFinalizersArePreserved(t *testing.T) {
	old := legacy()
	old.SetFinalizers(append(old.GetFinalizers(), "example.com/hold"))
	c := migrationClient(t, interceptor.Funcs{}, old)
	require.ErrorContains(t, (&ProjectQuota{Client: c}).Run(context.Background()), "wait for legacy quota deletion")
	current := legacy()
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(old), current))
	assert.Equal(t, []string{"example.com/hold"}, current.GetFinalizers())
}

func TestRetriesReadFailures(t *testing.T) {
	for _, operation := range []string{"project", "replacement", "resourcequotas"} {
		t.Run(operation, func(t *testing.T) {
			p := project()
			fail := true
			c := migrationClient(t, interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					_, isProject := obj.(*v1alpha3.Project)
					_, isStatus := obj.(*v1alpha1.ProjectQuotaStatus)
					if fail && ((operation == "project" && isProject) || (operation == "replacement" && isStatus)) {
						return apierrors.NewServiceUnavailable("injected read failure")
					}
					return c.Get(ctx, key, obj, opts...)
				},
				List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*corev1.ResourceQuotaList); fail && ok && operation == "resourcequotas" {
						return apierrors.NewServiceUnavailable("injected list failure")
					}
					return c.List(ctx, list, opts...)
				},
			}, p, replacement(p), legacy(), resourceQuota("example"))
			m := &ProjectQuota{Client: c}
			require.Error(t, m.Run(context.Background()))
			require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: p.Name}, legacy()))
			fail = false
			require.NoError(t, m.Run(context.Background()))
			assertGone(t, c)
		})
	}
}

func TestDoesNotRemoveForeignLegacyObject(t *testing.T) {
	old := legacy()
	old.SetLabels(nil)
	c := migrationClient(t, interceptor.Funcs{}, old)
	require.ErrorContains(t, (&ProjectQuota{Client: c}).remove(context.Background(), old, nil), "not controller-managed")
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(old), legacy()))
}

func TestRetryOwnerReferenceConflict(t *testing.T) {
	ctx := context.Background()
	p := project()
	conflict := true
	c := migrationClient(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*corev1.ResourceQuota); ok && conflict {
				// A namespace quota was updated by the regular reconciler after the migration listed it.
				current := new(corev1.ResourceQuota)
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), current))
				current.Spec.Hard["pods"] = resource.MustParse("20")
				require.NoError(t, c.Update(ctx, current))
				conflict = false
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	}, p, replacement(p), legacy(), resourceQuota("example"))
	m := &ProjectQuota{Client: c}
	require.Error(t, m.Run(ctx))
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: p.Name}, legacy()))
	require.NoError(t, m.Run(ctx))
	rq := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "example", Name: "d8-project-quota"}, rq))
	assert.Equal(t, "20", rq.Spec.Hard.Pods().String())
	assert.Empty(t, rq.OwnerReferences)
	assertGone(t, c)
}

func TestOtherQuotasProgressWhileReplacementIsPending(t *testing.T) {
	p := project()
	other := legacy()
	other.SetName("gone-project")
	other.SetUID("other")
	c := migrationClient(t, interceptor.Funcs{}, p, legacy(), other)
	m := &ProjectQuota{Client: c}
	require.ErrorContains(t, m.Run(context.Background()), "wait for the replacement")
	assert.True(t, apierrors.IsNotFound(c.Get(context.Background(), client.ObjectKeyFromObject(other), other)))
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: p.Name}, legacy()))
}

func TestSameNameReplacementSurvives(t *testing.T) {
	ctx := context.Background()
	swapped := false
	c := migrationClient(t, interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if !swapped {
				current := legacy()
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), current))
				current.SetFinalizers(nil)
				require.NoError(t, c.Update(ctx, current))
				require.NoError(t, c.Delete(ctx, current))
				fresh := legacy()
				fresh.SetUID("new-legacy")
				require.NoError(t, c.Create(ctx, fresh))
				swapped = true
			}
			return c.Delete(ctx, obj, opts...)
		},
	}, legacy())
	require.Error(t, (&ProjectQuota{Client: c}).Run(ctx))
	current := legacy()
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(current), current))
	assert.Equal(t, types.UID("new-legacy"), current.GetUID())
	assert.Contains(t, current.GetFinalizers(), v1alpha1.ProjectQuotaFinalizer)
}

func TestLeaderRunnableStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := migrationClient(t, interceptor.Funcs{
		List: func(ctx context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
			return ctx.Err()
		},
	})
	m := &ProjectQuota{Client: c, Logger: logr.Discard()}
	assert.True(t, m.NeedLeaderElection())
	require.NoError(t, m.Start(ctx))
}

func TestRecreatedProjectUsesCurrentUID(t *testing.T) {
	ctx := context.Background()
	previous := project()
	current := project()
	current.UID = "recreated-project"
	status := replacement(previous)
	c := migrationClient(t, interceptor.Funcs{}, current, status, legacy(), resourceQuota("example"))
	m := &ProjectQuota{Client: c}

	require.ErrorContains(t, m.Run(ctx), "wait for the replacement")
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: current.Name}, legacy()))

	// The project controller has reconciled the replacement of the same-named Project.
	status.OwnerReferences = replacement(current).OwnerReferences
	require.NoError(t, c.Update(ctx, status))
	require.NoError(t, m.Run(ctx))
	assertGone(t, c)
	got := new(v1alpha1.ProjectQuotaStatus)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: current.Name}, got))
	assert.True(t, metav1.IsControlledBy(got, current))
}

func TestRestartAfterPartialNamespacePass(t *testing.T) {
	ctx := context.Background()
	p := project()
	fail := true
	c := migrationClient(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if rq, ok := obj.(*corev1.ResourceQuota); ok && rq.Namespace == "b" && fail {
				return apierrors.NewServiceUnavailable("injected second namespace failure")
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	}, p, replacement(p), legacy(), resourceQuota("a"), resourceQuota("b"))
	require.Error(t, (&ProjectQuota{Client: c}).Run(ctx))
	first := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "a", Name: "d8-project-quota"}, first))
	assert.Empty(t, first.OwnerReferences)
	second := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "b", Name: "d8-project-quota"}, second))
	assert.Equal(t, []metav1.OwnerReference{oldReference()}, second.OwnerReferences)
	old := legacy()
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(old), old))
	assert.True(t, old.GetDeletionTimestamp().IsZero())
	assert.Contains(t, old.GetFinalizers(), v1alpha1.ProjectQuotaFinalizer)

	fail = false
	require.NoError(t, (&ProjectQuota{Client: c}).Run(ctx))
	assertGone(t, c)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(second), second))
	assert.Empty(t, second.OwnerReferences)
}

func TestFinalizerConflictPreservesConcurrentWriter(t *testing.T) {
	ctx := context.Background()
	conflict := true
	c := migrationClient(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, old := obj.(*unstructured.Unstructured); old && conflict {
				current := legacy()
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), current))
				current.SetFinalizers(append(current.GetFinalizers(), "example.com/hold"))
				require.NoError(t, c.Update(ctx, current))
				conflict = false
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	}, legacy())
	m := &ProjectQuota{Client: c}
	require.Error(t, m.Run(ctx))
	current := legacy()
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(current), current))
	assert.Contains(t, current.GetFinalizers(), v1alpha1.ProjectQuotaFinalizer)
	assert.Contains(t, current.GetFinalizers(), "example.com/hold")
	require.ErrorContains(t, m.Run(ctx), "wait for legacy quota deletion")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(current), current))
	assert.Equal(t, []string{"example.com/hold"}, current.GetFinalizers())
	current.SetFinalizers(nil)
	require.NoError(t, c.Update(ctx, current))
	require.NoError(t, m.Run(ctx))
	assertGone(t, c)
}

func TestUnmanagedQuotaDoesNotStartMigration(t *testing.T) {
	for _, label := range []string{"", "external"} {
		t.Run("managed-by="+label, func(t *testing.T) {
			ctx := context.Background()
			old := legacy()
			if label == "" {
				old.SetLabels(nil)
			} else {
				old.SetLabels(map[string]string{"projects.deckhouse.io/managed-by": label})
			}
			old.Object["spec"] = map[string]interface{}{
				"hard": map[string]interface{}{"pods": "7"},
			}
			rqLists := 0
			c := migrationClient(t, interceptor.Funcs{
				List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*corev1.ResourceQuotaList); ok {
						rqLists++
					}
					return c.List(ctx, list, opts...)
				},
			}, old)
			before := legacy()
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(old), before))
			m := &ProjectQuota{Client: c}
			require.NoError(t, m.Run(ctx))
			require.NoError(t, m.Run(ctx))
			after := legacy()
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(old), after))
			assert.Equal(t, before.Object, after.Object)
			assert.Zero(t, rqLists)
		})
	}
}

func TestMixedLegacyQuotasSelectsOnlyAutomaticObjects(t *testing.T) {
	ctx := context.Background()
	p := project()
	unmanaged := legacy()
	unmanaged.SetName("unmanaged")
	unmanaged.SetUID("unmanaged")
	unmanaged.SetLabels(nil)
	unmanaged.Object["spec"] = map[string]interface{}{
		"hard": map[string]interface{}{"pods": "7"},
	}
	unmanagedRQ := resourceQuota("unmanaged")
	unmanagedRQ.OwnerReferences[0].UID = unmanaged.GetUID()
	unmanagedRQ.OwnerReferences[0].Name = unmanaged.GetName()
	c := migrationClient(t, interceptor.Funcs{}, p, replacement(p), legacy(), unmanaged, resourceQuota("example"), unmanagedRQ)
	beforeRQ := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(unmanagedRQ), beforeRQ))
	before := legacy()
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(unmanaged), before))

	require.NoError(t, (&ProjectQuota{Client: c}).Run(ctx))
	assertGone(t, c)
	after := legacy()
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(unmanaged), after))
	assert.Equal(t, before.Object, after.Object)
	afterRQ := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(unmanagedRQ), afterRQ))
	assert.Equal(t, beforeRQ, afterRQ)
	rq := new(corev1.ResourceQuota)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: "example", Name: "d8-project-quota"}, rq))
	assert.Empty(t, rq.OwnerReferences)
}
