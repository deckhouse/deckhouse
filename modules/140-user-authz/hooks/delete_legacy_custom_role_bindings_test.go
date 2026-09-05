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

func legacyTestBinding(kind, namespace, name string, labels map[string]string) *unstructured.Unstructured {
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
	return obj
}

func moduleLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz", "app.kubernetes.io/managed-by": "Helm"}
}

func newLegacyBindingsFakeClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	gvrToListKind := map[schema.GroupVersionResource]string{
		ruleBindingResources[0]: "ClusterRoleBindingList",
		ruleBindingResources[1]: "RoleBindingList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objs...)
}

func legacyBindingExists(t *testing.T, c dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) bool {
	t.Helper()
	_, err := c.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		return true
	}
	if apierrors.IsNotFound(err) {
		return false
	}
	t.Fatalf("get %s/%s: %v", namespace, name, err)
	return false
}

func TestDeleteLegacyBindings_RemovesPerCustomRoleBindingsOnly(t *testing.T) {
	crbGVR := ruleBindingResources[0]
	rbGVR := ruleBindingResources[1]

	c := newLegacyBindingsFakeClient(
		// legacy per-custom-role bindings of the module: go
		legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:admin:custom-cluster-role:d8:user-authz:cert-manager:admin", moduleLabels()),
		legacyTestBinding("RoleBinding", "team", "user-authz:ns-rule:editor:custom-cluster-role:d8:user-authz:cert-manager:editor", moduleLabels()),
		// every other binding of the rule stays: level, aggregated, additional role, port-forward
		legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:admin", moduleLabels()),
		legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:admin:custom", moduleLabels()),
		legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:additional-role:custom-cluster-role-lookalike", moduleLabels()),
		legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:port-forward", moduleLabels()),
		// a foreign binding with a legacy-looking name but without the module labels stays
		legacyTestBinding("ClusterRoleBinding", "", "user-authz:foreign:user:custom-cluster-role:view", nil),
		// a module object that is not a rule binding stays
		legacyTestBinding("ClusterRoleBinding", "", "d8:user-authz:admin-kubeconfig", moduleLabels()),
	)

	deleted, err := deleteLegacyBindings(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("deleteLegacyBindings: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	if legacyBindingExists(t, c, crbGVR, "", "user-authz:dev:admin:custom-cluster-role:d8:user-authz:cert-manager:admin") {
		t.Error("the legacy ClusterRoleBinding must be deleted")
	}
	if legacyBindingExists(t, c, rbGVR, "team", "user-authz:ns-rule:editor:custom-cluster-role:d8:user-authz:cert-manager:editor") {
		t.Error("the legacy RoleBinding must be deleted")
	}
	for _, name := range []string{
		"user-authz:dev:admin",
		"user-authz:dev:admin:custom",
		"user-authz:dev:additional-role:custom-cluster-role-lookalike",
		"user-authz:dev:port-forward",
		"user-authz:foreign:user:custom-cluster-role:view",
		"d8:user-authz:admin-kubeconfig",
	} {
		if !legacyBindingExists(t, c, crbGVR, "", name) {
			t.Errorf("%s must survive", name)
		}
	}
}

func TestDeleteLegacyBindings_NoopWithoutLegacyBindingsAndReportsErrors(t *testing.T) {
	clean := newLegacyBindingsFakeClient(legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:admin:custom", moduleLabels()))
	if deleted, err := deleteLegacyBindings(context.Background(), clean, 4); err != nil || deleted != 0 {
		t.Fatalf("deleted = %d, err = %v, want 0 and no error", deleted, err)
	}

	failing := newLegacyBindingsFakeClient(legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:admin:custom-cluster-role:view", moduleLabels()))
	failing.PrependReactor("delete", "clusterrolebindings", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if _, err := deleteLegacyBindings(context.Background(), failing, 4); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v, want the delete error", err)
	}
}

var _ = Describe("User Authz hooks :: delete legacy custom-role bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	var fakeClient *dynamicfake.FakeDynamicClient

	BeforeEach(func() {
		fakeClient = newLegacyBindingsFakeClient(
			legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:admin:custom-cluster-role:view", moduleLabels()),
			legacyTestBinding("ClusterRoleBinding", "", "user-authz:dev:admin:custom", moduleLabels()),
		)
		newModuleBindingsClient = func(dependency.Container) (dynamic.Interface, error) { return fakeClient, nil }

		f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		f.RunHook()
	})

	It("Removes the legacy bindings before the release and keeps the aggregated one", func() {
		Expect(f).To(ExecuteSuccessfully())

		_, err := fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), "user-authz:dev:admin:custom-cluster-role:view", metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		_, err = fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), "user-authz:dev:admin:custom", metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
	})
})
