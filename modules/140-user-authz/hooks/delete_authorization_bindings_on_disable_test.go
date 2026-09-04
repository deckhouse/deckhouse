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
		)
		newModuleBindingsClient = func(dependency.Container) (dynamic.Interface, error) { return fakeClient, nil }

		f.BindingContexts.Set(f.GenerateAfterDeleteHelmContext())
		f.RunHook()
	})

	It("Removes the rule bindings after the release is gone and leaves the rest", func() {
		Expect(f).To(ExecuteSuccessfully())

		_, err := fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), "user-authz:dev:additional-role:cluster-admin", metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		_, err = fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), "d8:user-authz:admin-kubeconfig", metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
	})
})
