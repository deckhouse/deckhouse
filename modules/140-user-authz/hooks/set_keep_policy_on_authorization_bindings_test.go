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
	"fmt"
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

func testBinding(kind, namespace, name string, labels, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name": name,
		},
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

// helmLabels are the labels of a chart-rendered binding under the engine that adds the Helm
// ownership label; chartLabels those under the engine that does not. Both must be protected.
func helmLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz", "app.kubernetes.io/managed-by": "Helm"}
}

func chartLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz"}
}

// adoptedLabels are the labels of a binding the controller already owns.
func adoptedLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz", "user-authz.deckhouse.io/managed-by": "user-authz-controller"}
}

func newKeepPolicyFakeClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}: "ClusterRoleBindingList",
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}:        "RoleBindingList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...)
}

func keepAnnotationOf(t *testing.T, c *dynamicfake.FakeDynamicClient, gvr schema.GroupVersionResource, namespace, name string) string {
	t.Helper()
	obj, err := c.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s/%s: %v", namespace, name, err)
	}
	return obj.GetAnnotations()[helmResourcePolicyAnnotation]
}

func TestStampKeepPolicy_StampsHelmManagedRuleBindingsOnly(t *testing.T) {
	crbGVR := ruleBindingResources[0]
	rbGVR := ruleBindingResources[1]

	c := newKeepPolicyFakeClient(
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user:custom-cluster-role:d8:user-authz:x:user", helmLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:ops:editor", helmLabels(), map[string]string{helmResourcePolicyAnnotation: helmResourcePolicyKeep}),
		// rendered by the engine that does not label live objects: must be protected too
		testBinding("ClusterRoleBinding", "", "user-authz:plain:user", chartLabels(), nil),
		// module object that is not a rule binding: must not be touched
		testBinding("ClusterRoleBinding", "", "d8:user-authz:admin-kubeconfig", helmLabels(), nil),
		// already adopted by the controller: must not be touched
		testBinding("ClusterRoleBinding", "", "user-authz:adopted:user", adoptedLabels(), nil),
		// foreign binding without module labels
		testBinding("ClusterRoleBinding", "", "user-authz:foreign:user", nil, nil),
		testBinding("RoleBinding", "team", "user-authz:ns-rule:editor", helmLabels(), nil),
	)

	stamped, err := stampKeepPolicy(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("stampKeepPolicy: %v", err)
	}
	if stamped != 4 {
		t.Fatalf("stamped = %d, want 4 (three CRBs without keep + one RB)", stamped)
	}

	for _, name := range []string{"user-authz:dev:user", "user-authz:dev:user:custom-cluster-role:d8:user-authz:x:user", "user-authz:ops:editor", "user-authz:plain:user"} {
		if got := keepAnnotationOf(t, c, crbGVR, "", name); got != helmResourcePolicyKeep {
			t.Errorf("%s: keep = %q", name, got)
		}
	}
	if got := keepAnnotationOf(t, c, rbGVR, "team", "user-authz:ns-rule:editor"); got != helmResourcePolicyKeep {
		t.Errorf("rolebinding keep = %q", got)
	}
	for _, name := range []string{"d8:user-authz:admin-kubeconfig", "user-authz:adopted:user", "user-authz:foreign:user"} {
		if got := keepAnnotationOf(t, c, crbGVR, "", name); got != "" {
			t.Errorf("%s must not be stamped, got %q", name, got)
		}
	}

	if err := verifyKeepPolicy(context.Background(), c); err != nil {
		t.Fatalf("verify after stamping: %v", err)
	}
}

func TestStampKeepPolicy_IsIdempotentAndNoopWithoutTargets(t *testing.T) {
	c := newKeepPolicyFakeClient(
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), map[string]string{helmResourcePolicyAnnotation: helmResourcePolicyKeep}),
	)

	stamped, err := stampKeepPolicy(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("stampKeepPolicy: %v", err)
	}
	if stamped != 0 {
		t.Fatalf("stamped = %d, want 0", stamped)
	}
	for _, action := range c.Actions() {
		if action.GetVerb() == "patch" {
			t.Fatal("no patch must be issued when every binding already has keep")
		}
	}
}

func TestStampKeepPolicy_ReturnsPatchError(t *testing.T) {
	c := newKeepPolicyFakeClient(
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), nil),
	)
	c.PrependReactor("patch", "clusterrolebindings", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})

	if _, err := stampKeepPolicy(context.Background(), c, 4); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("err = %v, want the patch error", err)
	}
}

func TestVerifyKeepPolicy_RefusesWhenABindingIsUnprotected(t *testing.T) {
	c := newKeepPolicyFakeClient(
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), nil),
	)

	err := verifyKeepPolicy(context.Background(), c)
	if err == nil || !strings.Contains(err.Error(), "refusing to proceed") {
		t.Fatalf("err = %v, want refusal", err)
	}
	if !strings.Contains(err.Error(), "user-authz:dev:user") {
		t.Fatalf("err must name the unprotected binding: %v", err)
	}
}

func TestStampKeepPolicy_ManyBindingsWithWorkers(t *testing.T) {
	objs := make([]runtime.Object, 0, 500)
	for i := range 500 {
		objs = append(objs, testBinding("ClusterRoleBinding", "", fmt.Sprintf("user-authz:rule-%03d:user", i), helmLabels(), nil))
	}
	c := newKeepPolicyFakeClient(objs...)

	stamped, err := stampKeepPolicy(context.Background(), c, keepPolicyWorkers)
	if err != nil {
		t.Fatalf("stampKeepPolicy: %v", err)
	}
	if stamped != 500 {
		t.Fatalf("stamped = %d, want 500", stamped)
	}
	if err := verifyKeepPolicy(context.Background(), c); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

// The lists are paged: a continue token on the first page must lead to a second request, and the
// objects of both pages must be stamped.
func TestStampKeepPolicy_FollowsContinueTokens(t *testing.T) {
	crbGVR := ruleBindingResources[0]
	objs := []runtime.Object{
		testBinding("ClusterRoleBinding", "", "user-authz:a:user", helmLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:b:user", helmLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:c:user", helmLabels(), nil),
	}
	c := newKeepPolicyFakeClient(objs...)

	page := func(items []runtime.Object, next string) *unstructured.UnstructuredList {
		list := &unstructured.UnstructuredList{Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRoleBindingList",
			"metadata":   map[string]interface{}{"continue": next},
		}}
		for _, item := range items {
			list.Items = append(list.Items, *item.(*unstructured.Unstructured))
		}
		return list
	}
	lists := 0
	c.PrependReactor("list", "clusterrolebindings", func(_ clienttesting.Action) (bool, runtime.Object, error) {
		lists++
		if lists == 1 {
			return true, page(objs[:1], "page-2"), nil
		}
		return true, page(objs[1:], ""), nil
	})

	stamped, err := stampKeepPolicy(context.Background(), c, 2)
	if err != nil {
		t.Fatalf("stampKeepPolicy: %v", err)
	}
	if stamped != 3 {
		t.Fatalf("stamped = %d, want all 3 objects of both pages", stamped)
	}
	if lists != 2 {
		t.Fatalf("list calls = %d, want 2 (one per page)", lists)
	}
	for _, name := range []string{"user-authz:a:user", "user-authz:b:user", "user-authz:c:user"} {
		if got := keepAnnotationOf(t, c, crbGVR, "", name); got != helmResourcePolicyKeep {
			t.Errorf("%s: keep = %q", name, got)
		}
	}
}

var _ = Describe("User Authz hooks :: keep policy on authorization bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	var fakeClient *dynamicfake.FakeDynamicClient

	Context("Helm-managed bindings without the keep policy", func() {
		BeforeEach(func() {
			fakeClient = newKeepPolicyFakeClient(
				testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), nil),
				testBinding("RoleBinding", "team", "user-authz:ns-rule:editor", helmLabels(), nil),
			)
			newModuleBindingsClient = func(dependency.Container) (dynamic.Interface, error) { return fakeClient, nil }

			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Stamps the policy before the release and lets it proceed", func() {
			Expect(f).To(ExecuteSuccessfully())

			crb, err := fakeClient.Resource(ruleBindingResources[0]).Get(context.Background(), "user-authz:dev:user", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(crb.GetAnnotations()[helmResourcePolicyAnnotation]).To(Equal(helmResourcePolicyKeep))
			rb, err := fakeClient.Resource(ruleBindingResources[1]).Namespace("team").Get(context.Background(), "user-authz:ns-rule:editor", metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(rb.GetAnnotations()[helmResourcePolicyAnnotation]).To(Equal(helmResourcePolicyKeep))
		})
	})

	Context("A binding that cannot be stamped", func() {
		BeforeEach(func() {
			fakeClient = newKeepPolicyFakeClient(
				testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), nil),
			)
			fakeClient.PrependReactor("patch", "clusterrolebindings", func(_ clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("forbidden")
			})
			newModuleBindingsClient = func(dependency.Container) (dynamic.Interface, error) { return fakeClient, nil }

			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Fails the hook so that the release does not run", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("forbidden"))
		})
	})
})

// A binding deleted between the list and the patch (its rule was removed meanwhile) must not fail
// the hook: there is nothing left to protect.
func TestStampKeepPolicy_ToleratesNotFound(t *testing.T) {
	c := newKeepPolicyFakeClient(
		testBinding("ClusterRoleBinding", "", "user-authz:gone:user", helmLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), nil),
	)
	c.PrependReactor("patch", "clusterrolebindings", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.(clienttesting.PatchAction).GetName() == "user-authz:gone:user" {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterrolebindings"}, "user-authz:gone:user")
		}
		return false, nil, nil
	})

	stamped, err := stampKeepPolicy(context.Background(), c, 2)
	if err != nil {
		t.Fatalf("stampKeepPolicy: %v", err)
	}
	if stamped != 2 {
		t.Fatalf("stamped = %d, want 2 (the vanished object counts as done)", stamped)
	}
}
