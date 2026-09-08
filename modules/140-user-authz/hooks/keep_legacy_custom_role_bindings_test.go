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

package hooks

import (
	"context"
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// Shared fixture of the two legacy custom-role bindings hooks.
const (
	legacyCRB      = "user-authz:dev:admin:custom-cluster-role:d8:user-authz:cert-manager:admin"
	legacyKeptCRB  = "user-authz:ops:editor:custom-cluster-role:d8:user-authz:cert-manager:editor"
	legacyRB       = "user-authz:ns-rule:editor:custom-cluster-role:d8:user-authz:cert-manager:editor"
	aggregatedCRB  = "user-authz:dev:admin:custom"
	levelCRB       = "user-authz:dev:admin"
	lookalikeCRB   = "user-authz:dev:additional-role:custom-cluster-role-lookalike"
	foreignCRB     = "user-authz:foreign:user:custom-cluster-role:view"
	nonRuleCRB     = "d8:user-authz:admin-kubeconfig"
	legacyRBNs     = "team"
	legacyTestKeep = "helm.sh/resource-policy"
)

func legacyTestBinding(kind, namespace, name string, labels, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       kind,
		"metadata":   map[string]interface{}{"name": name},
		"roleRef": map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "ClusterRole",
			"name":     "user-authz:user",
		},
	}}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	if labels != nil {
		obj.SetLabels(labels)
	}
	if annotations != nil {
		obj.SetAnnotations(annotations)
	}
	return obj
}

func moduleLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz", "app.kubernetes.io/managed-by": "Helm"}
}

func keepAnnotations() map[string]string {
	return map[string]string{legacyTestKeep: "keep"}
}

func newLegacyBindingsFakeClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	gvrToListKind := map[schema.GroupVersionResource]string{
		ruleBindingResources[0]: "ClusterRoleBindingList",
		ruleBindingResources[1]: "RoleBindingList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objs...)
}

// legacyBindingState returns "absent", "kept" or "plain" for the binding.
func legacyBindingState(t *testing.T, c dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) string {
	t.Helper()
	obj, err := c.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return "absent"
	}
	if err != nil {
		t.Fatalf("get %s/%s: %v", namespace, name, err)
	}
	if obj.GetAnnotations()[legacyTestKeep] == "keep" {
		return "kept"
	}
	return "plain"
}

// legacyFixture: two legacy ClusterRoleBindings (one already kept), a legacy RoleBinding and every
// kind of binding that must be left alone by both hooks.
func legacyFixture() []runtime.Object {
	return []runtime.Object{
		legacyTestBinding("ClusterRoleBinding", "", legacyCRB, moduleLabels(), nil),
		legacyTestBinding("ClusterRoleBinding", "", legacyKeptCRB, moduleLabels(), keepAnnotations()),
		legacyTestBinding("RoleBinding", legacyRBNs, legacyRB, moduleLabels(), nil),
		legacyTestBinding("ClusterRoleBinding", "", aggregatedCRB, moduleLabels(), nil),
		legacyTestBinding("ClusterRoleBinding", "", levelCRB, moduleLabels(), nil),
		legacyTestBinding("ClusterRoleBinding", "", lookalikeCRB, moduleLabels(), nil),
		legacyTestBinding("ClusterRoleBinding", "", foreignCRB, nil, nil),
		legacyTestBinding("ClusterRoleBinding", "", nonRuleCRB, moduleLabels(), nil),
	}
}

func TestStampKeepOnLegacyBindings_StampsUnprotectedLegacyBindingsOnly(t *testing.T) {
	crbGVR := ruleBindingResources[0]
	rbGVR := ruleBindingResources[1]
	c := newLegacyBindingsFakeClient(legacyFixture()...)

	stamped, err := stampKeepOnLegacyBindings(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("stampKeepOnLegacyBindings: %v", err)
	}
	if stamped != 2 {
		t.Fatalf("stamped = %d, want 2 (the kept one is skipped)", stamped)
	}
	if err := verifyKeepOnLegacyBindings(context.Background(), c); err != nil {
		t.Fatalf("verify after stamping: %v", err)
	}

	if s := legacyBindingState(t, c, crbGVR, "", legacyCRB); s != "kept" {
		t.Errorf("%s = %s, want kept", legacyCRB, s)
	}
	if s := legacyBindingState(t, c, rbGVR, legacyRBNs, legacyRB); s != "kept" {
		t.Errorf("%s = %s, want kept", legacyRB, s)
	}
	for _, name := range []string{aggregatedCRB, levelCRB, lookalikeCRB, foreignCRB, nonRuleCRB} {
		if s := legacyBindingState(t, c, crbGVR, "", name); s != "plain" {
			t.Errorf("%s = %s, want plain (untouched)", name, s)
		}
	}
}

func TestVerifyKeepOnLegacyBindings_RefusesWhileALegacyBindingIsUnprotected(t *testing.T) {
	c := newLegacyBindingsFakeClient(legacyTestBinding("ClusterRoleBinding", "", legacyCRB, moduleLabels(), nil))
	if err := verifyKeepOnLegacyBindings(context.Background(), c); err == nil || !strings.Contains(err.Error(), legacyCRB) {
		t.Fatalf("err = %v, want a refusal naming %s", err, legacyCRB)
	}

	clean := newLegacyBindingsFakeClient(legacyTestBinding("ClusterRoleBinding", "", aggregatedCRB, moduleLabels(), nil))
	if stamped, err := stampKeepOnLegacyBindings(context.Background(), clean, 4); err != nil || stamped != 0 {
		t.Fatalf("stamped = %d, err = %v, want 0 and no error", stamped, err)
	}

	failing := newLegacyBindingsFakeClient(legacyTestBinding("ClusterRoleBinding", "", legacyCRB, moduleLabels(), nil))
	failing.PrependReactor("patch", "clusterrolebindings", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if _, err := stampKeepOnLegacyBindings(context.Background(), failing, 4); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v, want the patch error", err)
	}
}

var _ = Describe("User Authz hooks :: keep legacy custom-role bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	var fakeClient *dynamicfake.FakeDynamicClient

	BeforeEach(func() {
		fakeClient = newLegacyBindingsFakeClient(
			legacyTestBinding("ClusterRoleBinding", "", legacyCRB, moduleLabels(), nil),
			legacyTestBinding("ClusterRoleBinding", "", aggregatedCRB, moduleLabels(), nil),
		)
		newModuleBindingsClient = func(dependency.Container) (dynamic.Interface, error) { return fakeClient, nil }

		f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		f.RunHook()
	})

	It("Protects the legacy binding with keep before the release and deletes nothing", func() {
		Expect(f).To(ExecuteSuccessfully())

		legacy, err := fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), legacyCRB, metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(legacy.GetAnnotations()).To(HaveKeyWithValue(legacyTestKeep, "keep"))
		aggregated, err := fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), aggregatedCRB, metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(aggregated.GetAnnotations()).NotTo(HaveKey(legacyTestKeep))
	})
})
