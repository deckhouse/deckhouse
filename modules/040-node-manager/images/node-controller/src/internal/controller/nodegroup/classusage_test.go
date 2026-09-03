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

package nodegroup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func newClassSweeper(c client.Client) *Status {
	return &Status{Base: register.Base{Client: c}}
}

const (
	testClassKind  = "D8TestInstanceClass"
	otherClassKind = "D8OtherInstanceClass"
	// absentClassKind stands for a kind that is registered with no CRD installed for it.
	absentClassKind = "D8AbsentInstanceClass"
)

func classUsageScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1: %v", err)
	}
	for _, kind := range []string{testClassKind, otherClassKind} {
		gvk := classGVK(kind)
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(kind+"List"), &unstructured.UnstructuredList{})
	}
	return scheme
}

func classGVK(kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: v1.GroupVersion.Group, Version: "v1", Kind: kind}
}

func instanceClass(kind, name string, consumers []string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(classGVK(kind))
	u.SetName(name)
	if consumers != nil {
		values := make([]interface{}, 0, len(consumers))
		for _, c := range consumers {
			values = append(values, c)
		}
		if err := unstructured.SetNestedSlice(u.Object, values, "status", "nodeGroupConsumers"); err != nil {
			panic(err)
		}
	}
	return u
}

func registrationSecret(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: nodecommon.CloudProviderSecretNamespace,
			Labels:    map[string]string{nodecommon.CloudProviderRegistrationLabel: ""},
		},
		Data: data,
	}
}

func testClassRegistration() *corev1.Secret {
	return registrationSecret("d8-node-manager-cloud-provider", map[string][]byte{
		nodecommon.InstanceClassKindKey:       []byte(testClassKind),
		nodecommon.InstanceClassAPIVersionKey: []byte("v1"),
	})
}

func classUsageNodeGroup(name, kind, className string, nodeType v1.NodeType) *v1.NodeGroup {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: nodeType},
	}
	if kind != "" {
		ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
			ClassReference: v1.ClassReference{Kind: kind, Name: className},
		}
	}
	return ng
}

func getClass(t *testing.T, c client.Client, kind, name string) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(classGVK(kind))
	if err := c.Get(context.Background(), types.NamespacedName{Name: name}, u); err != nil {
		t.Fatalf("get %s/%s: %v", kind, name, err)
	}
	return u
}

// classConsumers returns status.nodeGroupConsumers and whether the field is present at all: the
// sweep writes an empty list only over a field that is there, so presence is part of the answer.
func classConsumers(t *testing.T, c client.Client, kind, name string) ([]string, bool) {
	t.Helper()
	got, found, err := unstructured.NestedStringSlice(getClass(t, c, kind, name).Object, "status", "nodeGroupConsumers")
	if err != nil {
		t.Fatalf("read consumers of %s/%s: %v", kind, name, err)
	}
	return got, found
}

func TestSyncInstanceClassConsumers(t *testing.T) {
	t.Run("cloud ephemeral nodegroups become sorted consumers", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			instanceClass(testClassKind, "spare", nil),
			classUsageNodeGroup("beta", testClassKind, "used", v1.NodeTypeCloudEphemeral),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).Build()

		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		got, found := classConsumers(t, c, testClassKind, "used")
		if !found || !slices.Equal(got, []string{"alpha", "beta"}) {
			t.Errorf("used consumers = %v (found=%v), want [alpha beta]", got, found)
		}
		// The spare class has no consumers and no field, so the sweep leaves it alone.
		if got, found = classConsumers(t, c, testClassKind, "spare"); found {
			t.Errorf("spare consumers = %v, want the field left absent", got)
		}
	})

	t.Run("static nodegroup is not a consumer", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("static", testClassKind, "used", v1.NodeTypeStatic),
		).Build()

		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		if got, found := classConsumers(t, c, testClassKind, "used"); found {
			t.Errorf("used consumers = %v, want the field left absent", got)
		}
	})

	t.Run("reference to another kind does not count", func(t *testing.T) {
		// Same class name under a different kind: only the kind+name pair may match.
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("worker", otherClassKind, "used", v1.NodeTypeCloudEphemeral),
		).Build()

		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		if got, found := classConsumers(t, c, testClassKind, "used"); found {
			t.Errorf("used consumers = %v, want the field left absent", got)
		}
	})

	t.Run("registration without an api version writes nothing", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			registrationSecret("d8-node-manager-cloud-provider", map[string][]byte{
				nodecommon.InstanceClassKindKey: []byte(testClassKind),
			}),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).Build()

		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		if _, found := classConsumers(t, c, testClassKind, "used"); found {
			t.Error("used got status.nodeGroupConsumers without a registered api version")
		}
	})

	t.Run("no registrations at all writes nothing", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).Build()

		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		if _, found := classConsumers(t, c, testClassKind, "used"); found {
			t.Error("used got status.nodeGroupConsumers with no provider registered")
		}
	})

	t.Run("a class that lost its consumers is cleared to an empty list", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", []string{"alpha"}),
		).Build()

		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		// NestedStringSlice errors on null, so this also pins the empty list to [].
		got, found := classConsumers(t, c, testClassKind, "used")
		if !found || len(got) != 0 {
			t.Errorf("used consumers = %v (found=%v), want an empty list", got, found)
		}
	})

	t.Run("a failing class does not abandon the rest of its kind", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "aaa-bad", nil),
			instanceClass(testClassKind, "zzz-good", nil),
			classUsageNodeGroup("alpha", testClassKind, "aaa-bad", v1.NodeTypeCloudEphemeral),
			classUsageNodeGroup("beta", testClassKind, "zzz-good", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if obj.GetName() == "aaa-bad" {
					return apierrors.NewConflict(schema.GroupResource{Resource: "d8testinstanceclasses"}, obj.GetName(), errors.New("conflict"))
				}
				return cl.Patch(ctx, obj, patch, opts...)
			},
		}).Build()

		err := newClassSweeper(c).syncInstanceClassConsumers(context.Background())
		if err == nil || !strings.Contains(err.Error(), "aaa-bad") {
			t.Fatalf("err = %v, want the aaa-bad failure reported", err)
		}

		got, found := classConsumers(t, c, testClassKind, "zzz-good")
		if !found || !slices.Equal(got, []string{"beta"}) {
			t.Errorf("zzz-good consumers = %v (found=%v), want [beta] despite the failure on aaa-bad", got, found)
		}
	})

	t.Run("a kind that prunes the patch is not patched again within the cooldown", func(t *testing.T) {
		patches := 0
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			// An accepted write that does not come back: the DVP-CRD case.
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				patches++
				return nil
			},
		}).Build()

		sweeper := newClassSweeper(c)
		clock := time.Now()
		sweeper.now = func() time.Time { return clock }

		for range 2 {
			if err := sweeper.syncInstanceClassConsumers(context.Background()); err != nil {
				t.Fatalf("sync: %v", err)
			}
		}
		clock = clock.Add(statusResyncInterval - time.Second)
		if err := sweeper.syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		if patches != 1 {
			t.Errorf("patches = %d, want 1: a kind that does not store the field is left alone until the cooldown expires", patches)
		}
	})

	t.Run("a kind that prunes the patch is retried after the cooldown", func(t *testing.T) {
		patches := 0
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				patches++
				return nil
			},
		}).Build()

		sweeper := newClassSweeper(c)
		clock := time.Now()
		sweeper.now = func() time.Time { return clock }

		if err := sweeper.syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}
		clock = clock.Add(statusResyncInterval + time.Second)
		if err := sweeper.syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		// A provider that fixes its CRD schema has to be picked up by the running pod: the memo
		// is a cooldown, not a switch that stays off until someone restarts it.
		if patches != 2 {
			t.Errorf("patches = %d, want 2: the kind must be retried once the cooldown expires", patches)
		}
	})

	t.Run("a class that prunes the patch does not stop the rest of its kind", func(t *testing.T) {
		// The first class the pass reaches is the one whose write is accepted and pruned.
		var patched []string
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "aaa-pruned", nil),
			instanceClass(testClassKind, "zzz-stored", nil),
			classUsageNodeGroup("alpha", testClassKind, "aaa-pruned", v1.NodeTypeCloudEphemeral),
			classUsageNodeGroup("beta", testClassKind, "zzz-stored", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				pruned := len(patched) == 0
				patched = append(patched, obj.GetName())
				if pruned {
					return nil
				}
				return cl.Patch(ctx, obj, patch, opts...)
			},
		}).Build()

		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		if len(patched) != 2 {
			t.Fatalf("patched = %v, want both classes of the kind attempted in the same pass", patched)
		}
		name := patched[1]
		want := map[string]string{"aaa-pruned": "alpha", "zzz-stored": "beta"}[name]
		got, found := classConsumers(t, c, testClassKind, name)
		if !found || !slices.Equal(got, []string{want}) {
			t.Errorf("%s consumers = %v (found=%v), want [%s]", name, got, found, want)
		}
	})

	t.Run("a kind whose crd is missing is skipped and the rest still run", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			registrationSecret("cloud-provider-absent", map[string][]byte{
				nodecommon.InstanceClassKindKey:       []byte(absentClassKind),
				nodecommon.InstanceClassAPIVersionKey: []byte("v1"),
			}),
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			// What a RESTMapper answers for a kind whose CRD is not installed.
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if u, ok := list.(*unstructured.UnstructuredList); ok && u.GetKind() == absentClassKind+"List" {
					return &meta.NoKindMatchError{
						GroupKind:        schema.GroupKind{Group: v1.GroupVersion.Group, Kind: absentClassKind},
						SearchedVersions: []string{"v1"},
					}
				}
				return cl.List(ctx, list, opts...)
			},
		}).Build()

		// The absent kind sorts before the served one, so it is swept first.
		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		got, found := classConsumers(t, c, testClassKind, "used")
		if !found || !slices.Equal(got, []string{"alpha"}) {
			t.Errorf("used consumers = %v (found=%v), want [alpha]", got, found)
		}
	})

	t.Run("a sweep arriving during another one is coalesced and the sweep re-runs", func(t *testing.T) {
		blocked, release := make(chan struct{}), make(chan struct{})
		var blockOnce, releaseOnce sync.Once
		unblock := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(unblock)

		ng := classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral)
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			ng,
		).WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*unstructured.UnstructuredList); ok {
					blockOnce.Do(func() {
						close(blocked)
						<-release
					})
				}
				return cl.List(ctx, list, opts...)
			},
		}).Build()

		r := newClassSweeper(c)
		first := make(chan struct{})
		go func() {
			defer close(first)
			r.sweepInstanceClassConsumers(context.Background())
		}()
		<-blocked

		// The running sweep already holds a NodeGroup snapshot taken before this delete.
		if err := c.Delete(context.Background(), ng); err != nil {
			t.Fatalf("delete nodegroup: %v", err)
		}

		second := make(chan struct{})
		go func() {
			defer close(second)
			r.sweepInstanceClassConsumers(context.Background())
		}()
		select {
		case <-second:
		case <-time.After(5 * time.Second):
			t.Fatal("the second sweep waited for the running one instead of marking it dirty")
		}

		unblock()
		<-first

		got, found := classConsumers(t, c, testClassKind, "used")
		if !found || len(got) != 0 {
			t.Errorf("used consumers = %v (found=%v), want an empty list from the re-run sweep", got, found)
		}
	})

	t.Run("a panic in the sweep does not kill the sweeps after it", func(t *testing.T) {
		exploded := false
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*unstructured.UnstructuredList); ok && !exploded {
					exploded = true
					panic("boom inside the sweep")
				}
				return cl.List(ctx, list, opts...)
			},
		}).Build()

		r := newClassSweeper(c)
		// controller-runtime recovers a Reconcile panic and keeps the worker, so the sweep has
		// to survive one too.
		func() {
			defer func() {
				if recover() == nil {
					t.Error("the injected panic did not reach the caller")
				}
			}()
			r.sweepInstanceClassConsumers(context.Background())
		}()

		r.sweepInstanceClassConsumers(context.Background())

		got, found := classConsumers(t, c, testClassKind, "used")
		if !found || !slices.Equal(got, []string{"alpha"}) {
			t.Errorf("used consumers = %v (found=%v), want [alpha] from the sweep after the panic", got, found)
		}
	})

	t.Run("a panic does not drop the mark the sweep had already taken", func(t *testing.T) {
		blocked, release := make(chan struct{}), make(chan struct{})
		var releaseOnce sync.Once
		unblock := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(unblock)

		passes := 0
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			// One class List per pass of the sweep, so this counts the passes.
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*unstructured.UnstructuredList); !ok {
					return cl.List(ctx, list, opts...)
				}
				passes++
				switch passes {
				case 1:
					close(blocked)
					<-release
				case 2:
					panic("boom in the pass the mark bought")
				}
				return cl.List(ctx, list, opts...)
			},
		}).Build()

		r := newClassSweeper(c)
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() {
				if recover() == nil {
					t.Error("the injected panic did not reach the caller")
				}
			}()
			r.sweepInstanceClassConsumers(context.Background())
		}()
		<-blocked

		// The mark this leaves is taken by the second pass, and that pass is the one that panics.
		r.sweepInstanceClassConsumers(context.Background())
		unblock()
		<-done

		before := passes
		r.sweepInstanceClassConsumers(context.Background())
		if got := passes - before; got != 2 {
			t.Errorf("the sweep after the panic made %d passes, want 2: the mark the panicked pass took has to be re-armed", got)
		}
	})

	t.Run("concurrent sweeps do not race", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			// Pruned writes, so the goroutine that wins the sweep also writes the memo.
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				return nil
			},
		}).Build()

		// Through the production entry point: it is the coalescing wrapper, not the sweep body,
		// that keeps two sweeps off the unstorable-kinds memo at once.
		sweeper := newClassSweeper(c)
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sweeper.sweepInstanceClassConsumers(context.Background())
			}()
		}
		wg.Wait()
	})

	t.Run("an up to date class is not patched", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", []string{"alpha"}),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).Build()

		before := getClass(t, c, testClassKind, "used").GetResourceVersion()
		if err := newClassSweeper(c).syncInstanceClassConsumers(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}

		if after := getClass(t, c, testClassKind, "used").GetResourceVersion(); after != before {
			t.Errorf("resourceVersion changed %s -> %s, want no patch", before, after)
		}
	})
}
