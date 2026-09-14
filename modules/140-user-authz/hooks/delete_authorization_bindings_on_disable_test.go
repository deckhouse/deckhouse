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
	"time"

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

func controllerLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz", "user-authz.deckhouse.io/managed-by": "user-authz-controller"}
}

func exists(t *testing.T, c *dynamicfake.FakeDynamicClient, gvr schema.GroupVersionResource, namespace, name string) bool {
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

func TestDeleteRuleBindings_RemovesRuleBindingsOfTheModuleOnly(t *testing.T) {
	crbGVR := ruleBindingResources[0]
	rbGVR := ruleBindingResources[1]

	c := newKeepPolicyFakeClient(
		// controller-owned and Helm-era rule bindings: both go
		testBinding("ClusterRoleBinding", "", "user-authz:dev:additional-role:cluster-admin", controllerLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:ops:editor", helmLabels(), map[string]string{helmResourcePolicyAnnotation: helmResourcePolicyKeep}),
		testBinding("RoleBinding", "team", "user-authz:ns-rule:editor", controllerLabels(), nil),
		// module object that is not a rule binding: stays (it belongs to the release)
		testBinding("ClusterRoleBinding", "", "d8:user-authz:admin-kubeconfig", helmLabels(), nil),
		// foreign binding with a rule-like name but without module labels: stays
		testBinding("ClusterRoleBinding", "", "user-authz:foreign:user", nil, nil),
	)

	deleted, err := deleteRuleBindings(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("deleteRuleBindings: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}

	for _, name := range []string{"user-authz:dev:additional-role:cluster-admin", "user-authz:ops:editor"} {
		if exists(t, c, crbGVR, "", name) {
			t.Errorf("%s must be deleted", name)
		}
	}
	if exists(t, c, rbGVR, "team", "user-authz:ns-rule:editor") {
		t.Error("the namespaced rule binding must be deleted")
	}
	for _, name := range []string{"d8:user-authz:admin-kubeconfig", "user-authz:foreign:user"} {
		if !exists(t, c, crbGVR, "", name) {
			t.Errorf("%s must survive", name)
		}
	}
}

func testRule(kind, apiVersion, namespace, name string, generation int64) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]interface{}{"name": name, "generation": generation},
		"spec":       map[string]interface{}{"accessLevel": "User"},
		"status": map[string]interface{}{
			"bindings":           int64(2),
			"observedGeneration": generation,
			"conditions":         []interface{}{map[string]interface{}{"type": "Ready", "status": "True", "reason": "BindingsApplied"}},
		},
	}}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	return obj
}

func TestMarkRulesModuleDisabled_RewritesTheStatusOfEveryRule(t *testing.T) {
	c := newKeepPolicyFakeClient(
		testRule("ClusterAuthorizationRule", "deckhouse.io/v1", "", "dev", 3),
		testRule("ClusterAuthorizationRule", "deckhouse.io/v1", "", "ops", 1),
		testRule("AuthorizationRule", "deckhouse.io/v1alpha1", "team", "dev", 2),
	)

	marked, err := markRulesModuleDisabled(context.Background(), c, 4, time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("markRulesModuleDisabled: %v", err)
	}
	if marked != 3 {
		t.Fatalf("marked = %d, want 3", marked)
	}

	got, err := c.Resource(ruleResources[0]).Get(context.Background(), "dev", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	conds, _, _ := unstructured.NestedSlice(got.Object, "status", "conditions")
	bindings, _, _ := unstructured.NestedInt64(got.Object, "status", "bindings")
	if len(conds) != 1 || bindings != 0 {
		t.Fatalf("status = %v", got.Object["status"])
	}
	cond, _ := conds[0].(map[string]interface{})
	if cond["status"] != "False" || cond["reason"] != reasonModuleDisabled || cond["observedGeneration"] != int64(3) {
		t.Errorf("condition = %v", cond)
	}
	ns, err := c.Resource(ruleResources[1]).Namespace("team").Get(context.Background(), "dev", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	nsConds, _, _ := unstructured.NestedSlice(ns.Object, "status", "conditions")
	if c0, _ := nsConds[0].(map[string]interface{}); c0["reason"] != reasonModuleDisabled {
		t.Errorf("namespaced rule condition = %v", nsConds)
	}
}

func TestDeleteRuleBindings_ReportsErrorsAndNoopWithoutBindings(t *testing.T) {
	empty := newKeepPolicyFakeClient()
	if deleted, err := deleteRuleBindings(context.Background(), empty, 4); err != nil || deleted != 0 {
		t.Fatalf("deleted = %d, err = %v, want 0 and no error", deleted, err)
	}

	failing := newKeepPolicyFakeClient(testBinding("ClusterRoleBinding", "", "user-authz:dev:user", controllerLabels(), nil))
	failing.PrependReactor("delete", "clusterrolebindings", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	if _, err := deleteRuleBindings(context.Background(), failing, 4); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v, want the delete error", err)
	}
}

var _ = Describe("User Authz hooks :: delete authorization bindings on module disable ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	var fakeClient *dynamicfake.FakeDynamicClient

	BeforeEach(func() {
		fakeClient = newKeepPolicyFakeClient(
			testBinding("ClusterRoleBinding", "", "user-authz:dev:additional-role:cluster-admin", controllerLabels(), nil),
			testBinding("ClusterRoleBinding", "", "d8:user-authz:admin-kubeconfig", helmLabels(), nil),
			testRule("ClusterAuthorizationRule", "deckhouse.io/v1", "", "dev", 1),
		)
		newModuleBindingsClient = func(dependency.Container) (dynamic.Interface, error) { return fakeClient, nil }

		f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
		f.RunHook()
	})

	It("Removes the rule bindings after the release is gone, leaves the rest and marks the rules", func() {
		Expect(f).To(ExecuteSuccessfully())

		rule, err := fakeClient.Resource(ruleResources[0]).Get(context.Background(), "dev", metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
		conds, _, _ := unstructured.NestedSlice(rule.Object, "status", "conditions")
		Expect(conds).To(HaveLen(1))
		Expect(conds[0].(map[string]interface{})["reason"]).To(Equal(reasonModuleDisabled))

		_, err = fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), "user-authz:dev:additional-role:cluster-admin", metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		_, err = fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), "d8:user-authz:admin-kubeconfig", metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
	})
})
