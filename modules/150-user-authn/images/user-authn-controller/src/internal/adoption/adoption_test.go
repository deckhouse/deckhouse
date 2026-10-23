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

package adoption

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestMetadataApplyAdoptsChartObject(t *testing.T) {
	t.Parallel()

	obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{"app": "x", "team": "a", ManagedByLabel: "Helm", "argocd": "y"},
		Annotations: map[string]string{
			ReleaseNameAnnotation:            Release,
			"meta.helm.sh/release-namespace": "d8-system",
			"helm.sh/resource-policy":        "keep",
			"werf.io/fail-mode":              "IgnoreAndContinueDeployProcess",
			"werf.io/track-termination-mode": "NonBlocking",
			"foreign":                        "kept",
		},
	}}
	m := Metadata{Fixed: map[string]string{"app": "x"}, Labels: map[string]string{"team": "a"}, Track: true}
	m.Apply(obj)

	if obj.Labels[ManagedByLabel] != ManagedByValue {
		t.Errorf("managed-by = %q, want %q", obj.Labels[ManagedByLabel], ManagedByValue)
	}
	if obj.Labels["argocd"] != "y" || obj.Annotations["foreign"] != "kept" {
		t.Errorf("foreign metadata dropped: %v %v", obj.Labels, obj.Annotations)
	}
	for _, key := range releaseAnnotations {
		if _, ok := obj.Annotations[key]; ok {
			t.Errorf("%s kept", key)
		}
	}
	if RenderedByRelease(obj) || !HasManagedMetadata(obj) {
		t.Errorf("rendered=%v managed=%v, want an adopted object", RenderedByRelease(obj), HasManagedMetadata(obj))
	}

	// The next pass drops the user label that is no longer wanted, and only it.
	Metadata{Fixed: map[string]string{"app": "x"}, Track: true}.Apply(obj)
	if _, ok := obj.Labels["team"]; ok {
		t.Error("recorded label removed from the source is still there")
	}
	if obj.Labels["app"] != "x" || obj.Labels["argocd"] != "y" {
		t.Errorf("labels = %v", obj.Labels)
	}
}

// The release annotations of the desired metadata, from secretMetadata of a DexClient say, are
// dropped on a new object as on an existing one: the object would otherwise be taken for one of
// the chart until the next reconcile removed them.
func TestMetadataNewDropsReleaseAnnotations(t *testing.T) {
	t.Parallel()

	m := Metadata{
		Annotations: map[string]string{
			ReleaseNameAnnotation:     "user-authn",
			"helm.sh/resource-policy": "keep",
			"team":                    "platform",
		},
		Track: true,
	}
	obj := &corev1.Secret{}
	m.New(obj)
	got := obj.GetAnnotations()
	if _, ok := got[ReleaseNameAnnotation]; ok {
		t.Errorf("annotations = %v, want no release name", got)
	}
	if _, ok := got["helm.sh/resource-policy"]; ok {
		t.Errorf("annotations = %v, want no resource policy", got)
	}
	if got["team"] != "platform" {
		t.Errorf("annotations = %v, want the foreign one kept", got)
	}
	if strings.Contains(got[ManagedMetadataAnnotation], "helm.sh") {
		t.Errorf("bookkeeping = %s, want no release keys recorded", got[ManagedMetadataAnnotation])
	}
}

func TestMetadataFixedLabelsWin(t *testing.T) {
	t.Parallel()

	obj := &corev1.Secret{}
	Metadata{Fixed: map[string]string{"app": "dex-client"}, Labels: map[string]string{"app": "custom"}}.New(obj)
	if obj.Labels["app"] != "dex-client" {
		t.Errorf("app = %q, want the fixed value", obj.Labels["app"])
	}
	if obj.Annotations != nil {
		t.Errorf("annotations = %v, want none without tracking", obj.Annotations)
	}
}

func TestEnsureReadsConflictIntoEmptyObject(t *testing.T) {
	t.Parallel()

	foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}}
	c := newClient(t, foreign)
	w := Writer{Client: hideFromCache(c), Reader: c, Log: logr.Discard()}

	want := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns", Labels: map[string]string{"ours": "yes"}}}
	owns := func(obj client.Object) bool { return obj.GetLabels()["ours"] == "yes" }

	_, err := w.Ensure(context.Background(), nil, want, owns, func(client.Object) {})
	if !errors.Is(err, ErrForeign) || !errors.Is(err, reconcile.TerminalError(nil)) {
		t.Fatalf("Ensure = %v, want a terminal foreign-object error: the labels of want must not leak into the live object", err)
	}
}

func TestEnsureSkipsUnchanged(t *testing.T) {
	t.Parallel()

	current := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}}
	c := newClient(t, current)
	updates := 0
	counting := interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			updates++
			return cl.Update(ctx, obj, opts...)
		},
	})
	w := Writer{Client: counting, Reader: c, Log: logr.Discard()}

	live := &corev1.Secret{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(current), live); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Ensure(context.Background(), live, current, func(client.Object) bool { return true }, func(client.Object) {}); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if updates != 0 {
		t.Errorf("updates = %d, want none for an unchanged object", updates)
	}
}

func TestGetReadsPastTheCache(t *testing.T) {
	t.Parallel()

	live := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}, Data: map[string][]byte{"k": []byte("v")}}
	cached := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}}
	w := Writer{Client: newClient(t, cached), Reader: newClient(t, live), Log: logr.Discard()}

	got := &corev1.Secret{}
	found, err := w.Get(context.Background(), client.ObjectKeyFromObject(live), got)
	if err != nil || !found {
		t.Fatalf("Get = %v, %v", found, err)
	}
	if string(got.Data["k"]) != "v" {
		t.Errorf("Get returned the cached object %+v, want the live one: a cached OAuth2Client has no secret", got)
	}

	found, err = w.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "missing"}, &corev1.Secret{})
	if err != nil || found {
		t.Errorf("Get of a missing object = %v, %v, want not found", found, err)
	}
}

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// hideFromCache makes Get miss every Secret, the way a label-scoped cache misses a Secret without
// the labels.
func hideFromCache(c client.Client) client.Client {
	return interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				return apierrors.NewNotFound(corev1.Resource("secrets"), key.Name)
			}
			return cl.Get(ctx, key, obj, opts...)
		},
	})
}
