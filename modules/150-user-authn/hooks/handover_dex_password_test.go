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
	"maps"
	"slices"
	"testing"

	"github.com/flant/addon-operator/sdk"

	"github.com/werf/nelm/v2/pkg/common"
	"github.com/werf/nelm/v2/pkg/resource"
	"github.com/werf/nelm/v2/pkg/resource/spec"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

var chartAnnotations = map[string]string{"meta.helm.sh/release-name": "user-authn", "meta.helm.sh/release-namespace": "d8-system"}

func handoverObject(gvr schema.GroupVersionResource, kind, namespace, name string, labels, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(schema.GroupVersion{Group: gvr.Group, Version: gvr.Version}.String())
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetLabels(labels)
	obj.SetAnnotations(annotations)
	return obj
}

func newHandoverFakeClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		passwordGVR: "PasswordList",
	}, objs...)
}

func policyOf(t *testing.T, c *dynamicfake.FakeDynamicClient, gvr schema.GroupVersionResource, namespace, name string) string {
	t.Helper()
	obj, err := c.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s/%s: %v", namespace, name, err)
	}
	return obj.GetAnnotations()[handover.ResourcePolicyAnnotation]
}

// addon-operator keeps the hooks of a module by name, the file they are registered from: a second
// hook from the same file replaces the first.
func TestHandoverHooksAreRegistered(t *testing.T) {
	names := map[string]int{}
	var beforeHelm bool
	for _, h := range sdk.Registry().GetModuleHooks("user-authn") {
		names[h.GetName()]++
		if h.GetName() == "150-user-authn/hooks/handover_protect.go" {
			beforeHelm = h.GetConfig().OnBeforeHelm != nil
		}
	}
	for name, n := range names {
		if n > 1 {
			t.Errorf("%d hooks are registered from %s; only the last one runs", n, name)
		}
	}
	if !beforeHelm {
		t.Errorf("OnBeforeHelm hook not registered; hooks %v", names)
	}
}

// Stands for the hash Dex wrote when the user changed the password. The hook does not read it, so
// any value proves that the Password keeps it.
const changedPasswordHash = "hash-written-by-dex"

// chartPassword is a Password as the chart of 1.76 rendered it from a User and Dex then updated.
func chartPassword(email string, annotations map[string]string) *unstructured.Unstructured {
	obj := handoverObject(passwordGVR, "Password", dexNamespace, encoding.ToFnvLikeDex(email),
		map[string]string{"heritage": "deckhouse", "module": "user-authn", "app": "dex"}, annotations)
	obj.Object["email"] = email
	obj.Object["username"] = email
	obj.Object["hash"] = changedPasswordHash
	return obj
}

func getPassword(t *testing.T, c *dynamicfake.FakeDynamicClient, email string) *unstructured.Unstructured {
	t.Helper()
	obj, err := c.Resource(passwordGVR).Namespace(dexNamespace).Get(context.Background(), encoding.ToFnvLikeDex(email), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get Password of %s: %v", email, err)
	}
	return obj
}

// prunedByNelm tells whether nelm, deploying a chart that no longer renders obj, deletes it: the
// policies come from the live object, the way nelm reads them before deleting a resource of the
// previous release.
func prunedByNelm(obj *unstructured.Unstructured) bool {
	meta := spec.NewResourceMetaFromUnstructured(obj, "d8-system", "")
	if err := resource.ValidateResourcePolicy(meta); err != nil {
		return false
	}
	return !slices.Contains(resource.ResourcePolicies(meta, "d8-system"), common.ResourcePolicySkipDelete)
}

// The update from 1.76 to 1.77 recreated every Password from User.spec.password: the chart of 1.77
// stopped rendering Passwords, the release pruned the ones it owned, and user-authn-controller
// created them anew from the password the User was created with.
func TestHandoverKeepsChartPasswords(t *testing.T) {
	foreignAnnotations := map[string]string{"meta.helm.sh/release-name": "other", "meta.helm.sh/release-namespace": "d8-system"}
	c := newHandoverFakeClient(
		chartPassword("jane@example.com", maps.Clone(chartAnnotations)),
		// Created by user-authn-controller: no release annotations, the keep policy set.
		chartPassword("john@example.com", map[string]string{handover.ResourcePolicyAnnotation: handover.ResourcePolicyKeep}),
		chartPassword("bob@example.com", foreignAnnotations),
	)

	if !prunedByNelm(getPassword(t, c, "jane@example.com")) {
		t.Fatal("nelm keeps the Password of the 1.76 chart without the hook; the test no longer reproduces the loss")
	}

	res, err := handover.Protect(context.Background(), c, handoverKinds)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if res.Stamped != 1 || res.Owned != 1 {
		t.Errorf("result = %+v, want 1 owned and stamped", res)
	}

	jane := getPassword(t, c, "jane@example.com")
	if prunedByNelm(jane) {
		t.Errorf("Password of the chart is pruned after Protect: annotations %v", jane.GetAnnotations())
	}
	if got := jane.Object["hash"]; got != changedPasswordHash {
		t.Errorf("hash = %v, want the one Dex wrote", got)
	}
	if got := policyOf(t, c, passwordGVR, dexNamespace, encoding.ToFnvLikeDex("bob@example.com")); got != "" {
		t.Errorf("Password of another release policy = %q, want untouched", got)
	}

	// A protected Password stays protected: the controller keeps the policy. Nothing is owned any
	// more, so the hook stops looking.
	res, err = handover.Protect(context.Background(), c, handoverKinds)
	if err != nil || res != (handover.Result{}) {
		t.Errorf("second Protect = %+v, %v; want nothing owned", res, err)
	}
}

func TestHandoverProtectWithoutCRDs(t *testing.T) {
	c := newHandoverFakeClient()
	// The API server answers a list of a resource it does not serve with NotFound.
	c.PrependReactor("list", "passwords", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(passwordGVR.GroupResource(), "")
	})
	if _, err := handover.Protect(context.Background(), c, handoverKinds); err != nil {
		t.Fatalf("Protect: %v", err)
	}
}

// A failed patch fails the run: the release would otherwise prune the object.
func TestHandoverProtectFailsWhenPatchFails(t *testing.T) {
	c := newHandoverFakeClient(chartPassword("jane@example.com", maps.Clone(chartAnnotations)))
	c.PrependReactor("patch", "passwords", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	res, err := handover.Protect(context.Background(), c, handoverKinds)
	if err == nil {
		t.Fatal("Protect succeeded, want the patch error")
	}
	if res.Stamped != 0 || res.Owned != 1 {
		t.Errorf("result = %+v, want 1 owned, none stamped", res)
	}
}
