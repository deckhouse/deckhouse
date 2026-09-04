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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
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

func helmLabels() map[string]string {
	return map[string]string{"heritage": "deckhouse", "module": "user-authz", "app.kubernetes.io/managed-by": "Helm"}
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
	crbGVR := keepPolicyBindingResources[0]
	rbGVR := keepPolicyBindingResources[1]

	c := newKeepPolicyFakeClient(
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user", helmLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:dev:user:custom-cluster-role:d8:user-authz:x:user", helmLabels(), nil),
		testBinding("ClusterRoleBinding", "", "user-authz:ops:editor", helmLabels(), map[string]string{helmResourcePolicyAnnotation: helmResourcePolicyKeep}),
		// module object that is not a rule binding: must not be touched
		testBinding("ClusterRoleBinding", "", "d8:user-authz:admin-kubeconfig", helmLabels(), nil),
		// already adopted by the controller: no Helm label, must not be touched
		testBinding("ClusterRoleBinding", "", "user-authz:adopted:user", map[string]string{"heritage": "deckhouse", "module": "user-authz"}, nil),
		// foreign binding without module labels
		testBinding("ClusterRoleBinding", "", "user-authz:foreign:user", nil, nil),
		testBinding("RoleBinding", "team", "user-authz:ns-rule:editor", helmLabels(), nil),
	)

	stamped, err := stampKeepPolicy(context.Background(), c, 4)
	if err != nil {
		t.Fatalf("stampKeepPolicy: %v", err)
	}
	if stamped != 3 {
		t.Fatalf("stamped = %d, want 3 (two CRBs without keep + one RB)", stamped)
	}

	for _, name := range []string{"user-authz:dev:user", "user-authz:dev:user:custom-cluster-role:d8:user-authz:x:user", "user-authz:ops:editor"} {
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
	for i := 0; i < 500; i++ {
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
