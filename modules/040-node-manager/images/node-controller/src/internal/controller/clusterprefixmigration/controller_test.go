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

package clusterprefixmigration

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/register"
)

var mcGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	s.AddKnownTypeWithName(mcGVK, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(mcGVK.GroupVersion().WithKind("ModuleConfigList"), &unstructured.UnstructuredList{})
	return s
}

func clusterConfigSecret(cloudPrefix string) *corev1.Secret {
	y := "apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterType: Cloud\n"
	if cloudPrefix != "" {
		y += "cloud:\n  provider: Yandex\n  prefix: " + cloudPrefix + "\n"
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clusterConfigurationSecretName, Namespace: clusterConfigurationSecretNamespace},
		Data:       map[string][]byte{clusterConfigurationSecretKey: []byte(y)},
	}
}

func globalMC(prefix string) *unstructured.Unstructured {
	mc := newModuleConfig()
	mc.SetName(globalModuleConfigName)
	_ = unstructured.SetNestedField(mc.Object, int64(globalModuleConfigVersion), "spec", "version")
	if prefix != "" {
		_ = unstructured.SetNestedField(mc.Object, prefix, "spec", "settings", "prefix")
	}
	return mc
}

func mcPrefix(t *testing.T, cl client.Client) (string, bool) {
	t.Helper()
	mc := newModuleConfig()
	err := cl.Get(context.Background(), types.NamespacedName{Name: globalModuleConfigName}, mc)
	if apierrors.IsNotFound(err) {
		return "", false
	}
	if err != nil {
		t.Fatalf("get global MC: %v", err)
	}
	p, _, _ := unstructured.NestedString(mc.Object, "spec", "settings", "prefix")
	return p, true
}

func runReconcile(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objs...).Build()
	r := &Reconciler{Base: register.Base{Client: cl}, apiReader: cl}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	return cl
}

func TestSeed_PatchesExistingGlobalMC(t *testing.T) {
	cl := runReconcile(t, clusterConfigSecret("lysov-test"), globalMC(""))
	got, exists := mcPrefix(t, cl)
	if !exists {
		t.Fatal("global MC should exist")
	}
	if got != "lysov-test" {
		t.Fatalf("prefix = %q, want lysov-test", got)
	}
}

func TestSeed_CreatesGlobalMCWhenMissing(t *testing.T) {
	cl := runReconcile(t, clusterConfigSecret("lysov-test"))
	got, exists := mcPrefix(t, cl)
	if !exists {
		t.Fatal("global MC should have been created")
	}
	if got != "lysov-test" {
		t.Fatalf("prefix = %q, want lysov-test", got)
	}
}

func TestSeed_DoesNotOverwriteExistingPrefix(t *testing.T) {
	cl := runReconcile(t, clusterConfigSecret("lysov-test"), globalMC("already-set"))
	got, _ := mcPrefix(t, cl)
	if got != "already-set" {
		t.Fatalf("prefix = %q, want already-set (must not overwrite)", got)
	}
}

func TestSeed_NoCloudPrefixDoesNothing(t *testing.T) {
	// Static cluster / cloud.prefix absent: no global MC created.
	cl := runReconcile(t, clusterConfigSecret(""))
	if _, exists := mcPrefix(t, cl); exists {
		t.Fatal("global MC should not be created when cloud.prefix is absent")
	}
}
