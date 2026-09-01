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

	apierrors "k8s.io/apimachinery/pkg/api/errors"

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
	// absentClassKind is registered by the envtest spec without a CRD behind it.
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
		// An absent field already reads as "no consumers", so the spare class is not written to.
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

	t.Run("a kind that prunes the patch is dropped after one attempt", func(t *testing.T) {
		patches := 0
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			// The apiserver accepts the write and prunes the field away, as the DVP CRD does.
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				patches++
				return nil
			},
		}).Build()

		sweeper := newClassSweeper(c)
		for range 2 {
			if err := sweeper.syncInstanceClassConsumers(context.Background()); err != nil {
				t.Fatalf("sync: %v", err)
			}
		}

		if patches != 1 {
			t.Errorf("patches = %d, want 1: a kind that never stores the field must be dropped", patches)
		}
	})

	t.Run("concurrent sweeps do not race", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(classUsageScheme(t)).WithObjects(
			testClassRegistration(),
			instanceClass(testClassKind, "used", nil),
			classUsageNodeGroup("alpha", testClassKind, "used", v1.NodeTypeCloudEphemeral),
		).WithInterceptorFuncs(interceptor.Funcs{
			// Pruned writes, so every worker also touches the unstorable-kinds memo.
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				return nil
			},
		}).Build()

		sweeper := newClassSweeper(c)
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := sweeper.syncInstanceClassConsumers(context.Background()); err != nil {
					t.Errorf("sync: %v", err)
				}
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
